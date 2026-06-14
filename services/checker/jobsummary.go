package checker

import (
	"context"

	"encore.dev/beta/errs"

	subsvc "subs-check-re/services/subscription"
)

// inheritedJobNodesCTE yields one row per node in job $1 with unmeasured
// dimensions (speed, country, platforms) filled from that node's most recent
// known value within subscription $2. alive and latency_ms are always the
// current run's values. Prepend it to a query that selects FROM job_nodes.
const inheritedJobNodesCTE = `
	WITH latest_platform_kv AS (
		SELECT DISTINCT ON (cr2.node_name, kv.key) cr2.node_name, kv.key AS key, kv.value AS value
		FROM check_results cr2
		JOIN check_jobs cj2 ON cj2.id = cr2.job_id
		CROSS JOIN LATERAL jsonb_each(cr2.platforms) AS kv(key, value)
		WHERE cj2.subscription_id = $2 AND cr2.platforms IS NOT NULL AND cr2.platforms <> '{}'::jsonb
		ORDER BY cr2.node_name, kv.key, cr2.checked_at DESC
	),
	merged_platforms AS (
		SELECT node_name, jsonb_object_agg(key, value) AS platforms
		FROM latest_platform_kv
		GROUP BY node_name
	),
	job_nodes AS (
		SELECT cr.node_name,
		       cr.alive,
		       cr.latency_ms,
		       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
		            ELSE COALESCE((
		                SELECT cr2.speed_kbps FROM check_results cr2
		                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
		                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.speed_kbps > 0
		                ORDER BY cr2.checked_at DESC LIMIT 1
		            ), 0)
		       END AS speed_kbps,
		       CASE WHEN cr.country <> '' THEN cr.country
		            ELSE COALESCE((
		                SELECT cr2.country FROM check_results cr2
		                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
		                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.country <> ''
		                ORDER BY cr2.checked_at DESC LIMIT 1
		            ), '')
		       END AS country,
		       COALESCE(mp.platforms, cr.platforms, '{}'::jsonb) AS platforms
		FROM check_results cr
		LEFT JOIN merged_platforms mp ON mp.node_name = cr.node_name
		WHERE cr.job_id = $1
	)`

// loadJobSummary composes a JobDetailedSummary from the check_jobs + check_results tables.
// Each query lives in its own helper so they can be tested or reused independently
// (e.g. by notify formatters or CLI exporters).
func loadJobSummary(ctx context.Context, jobID string) (*JobDetailedSummary, error) {
	s := &JobDetailedSummary{
		JobID:     jobID,
		Platforms: PlatformUnlockSummary{},
		Countries: map[string]int{},
		TopNodes:  []TopNode{},
	}

	subID, userID, err := loadJobBasic(ctx, jobID, s)
	if err != nil {
		return nil, err
	}
	s.SubscriptionName = resolveSubscriptionName(ctx, userID, subID)
	loadPlatformCounts(ctx, jobID, subID, &s.Platforms)
	loadSpeedStats(ctx, jobID, subID, s)
	s.TopNodes = loadTopNodes(ctx, jobID, subID)
	s.Countries = loadCountryBreakdown(ctx, jobID, subID)
	return s, nil
}

func loadJobBasic(ctx context.Context, jobID string, s *JobDetailedSummary) (subID, userID string, err error) {
	err = db.QueryRow(ctx, `
		SELECT subscription_id, user_id, available, total
		FROM check_jobs WHERE id = $1
	`, jobID).Scan(&subID, &userID, &s.Available, &s.Total)
	if err != nil {
		return "", "", errs.B().Code(errs.NotFound).Msg("job not found").Err()
	}
	return subID, userID, nil
}

func resolveSubscriptionName(ctx context.Context, userID, subID string) string {
	resp, err := subsvc.GetSubscriptionNames(ctx, &subsvc.GetSubscriptionNamesParams{
		UserID: userID,
		IDs:    []string{subID},
	})
	if err != nil {
		return subID
	}
	if name, ok := resp.Names[subID]; ok {
		return name
	}
	return subID
}

func loadPlatformCounts(ctx context.Context, jobID, subID string, p *PlatformUnlockSummary) {
	if *p == nil {
		*p = PlatformUnlockSummary{}
	}
	rows, err := db.Query(ctx, inheritedJobNodesCTE+`
		SELECT key, COUNT(*)
		FROM job_nodes, jsonb_each(platforms) AS e(key, val)
		WHERE alive = true AND (val->>'unlocked')::boolean
		GROUP BY key
	`, jobID, subID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n int
		if rows.Scan(&key, &n) == nil {
			(*p)[key] = n
		}
	}
}

func loadSpeedStats(ctx context.Context, jobID, subID string, s *JobDetailedSummary) {
	_ = db.QueryRow(ctx, inheritedJobNodesCTE+`
		SELECT
			COALESCE(AVG(speed_kbps) FILTER (WHERE speed_kbps > 0), 0)::int,
			COALESCE(MAX(speed_kbps), 0),
			COALESCE(AVG(latency_ms) FILTER (WHERE alive AND latency_ms > 0), 0)::int
		FROM job_nodes
	`, jobID, subID).Scan(&s.AvgSpeedKbps, &s.MaxSpeedKbps, &s.AvgLatencyMs)
}

func loadTopNodes(ctx context.Context, jobID, subID string) []TopNode {
	rows, err := db.Query(ctx, inheritedJobNodesCTE+`
		SELECT COALESCE(node_name, ''), speed_kbps, latency_ms, COALESCE(country, '')
		FROM job_nodes
		WHERE alive = true AND speed_kbps > 0
		ORDER BY speed_kbps DESC
		LIMIT 5
	`, jobID, subID)
	if err != nil {
		return []TopNode{}
	}
	defer rows.Close()

	nodes := []TopNode{}
	for rows.Next() {
		var n TopNode
		if err := rows.Scan(&n.Name, &n.SpeedKbps, &n.LatencyMs, &n.Country); err == nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func loadCountryBreakdown(ctx context.Context, jobID, subID string) map[string]int {
	out := map[string]int{}
	rows, err := db.Query(ctx, inheritedJobNodesCTE+`
		SELECT COALESCE(country, 'Unknown'), COUNT(*)
		FROM job_nodes
		WHERE alive = true
		GROUP BY country
		ORDER BY COUNT(*) DESC
	`, jobID, subID)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var country string
		var count int
		if err := rows.Scan(&country, &count); err == nil {
			out[country] = count
		}
	}
	return out
}
