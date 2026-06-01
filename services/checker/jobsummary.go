package checker

import (
	"context"

	"encore.dev/beta/errs"

	subsvc "subs-check-re/services/subscription"
)

// loadJobSummary composes a JobDetailedSummary from the check_jobs + check_results tables.
// Each query lives in its own helper so they can be tested or reused independently
// (e.g. by notify formatters or CLI exporters).
func loadJobSummary(ctx context.Context, jobID string) (*JobDetailedSummary, error) {
	s := &JobDetailedSummary{
		JobID:     jobID,
		Countries: map[string]int{},
		TopNodes:  []TopNode{},
	}

	subID, userID, err := loadJobBasic(ctx, jobID, s)
	if err != nil {
		return nil, err
	}
	s.SubscriptionName = resolveSubscriptionName(ctx, userID, subID)
	loadPlatformCounts(ctx, jobID, &s.Platforms)
	loadSpeedStats(ctx, jobID, s)
	s.TopNodes = loadTopNodes(ctx, jobID)
	s.Countries = loadCountryBreakdown(ctx, jobID)
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

func loadPlatformCounts(ctx context.Context, jobID string, p *PlatformUnlockSummary) {
	_ = db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN netflix THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN youtube THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN youtube_premium THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN openai THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN claude THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN gemini THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN grok THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN disney THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tiktok THEN 1 ELSE 0 END), 0)
		FROM check_results WHERE job_id=$1 AND alive=true
	`, jobID).Scan(
		&p.Netflix, &p.YouTube, &p.YouTubePremium,
		&p.OpenAI, &p.Claude, &p.Gemini,
		&p.Grok, &p.Disney, &p.TikTok,
	)
}

func loadSpeedStats(ctx context.Context, jobID string, s *JobDetailedSummary) {
	_ = db.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(speed_kbps) FILTER (WHERE speed_kbps > 0), 0)::int,
			COALESCE(MAX(speed_kbps), 0),
			COALESCE(AVG(latency_ms) FILTER (WHERE alive AND latency_ms > 0), 0)::int
		FROM check_results WHERE job_id=$1
	`, jobID).Scan(&s.AvgSpeedKbps, &s.MaxSpeedKbps, &s.AvgLatencyMs)
}

func loadTopNodes(ctx context.Context, jobID string) []TopNode {
	rows, err := db.Query(ctx, `
		SELECT COALESCE(node_name, ''), speed_kbps, latency_ms, COALESCE(country, '')
		FROM check_results
		WHERE job_id=$1 AND alive=true AND speed_kbps > 0
		ORDER BY speed_kbps DESC
		LIMIT 5
	`, jobID)
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

func loadCountryBreakdown(ctx context.Context, jobID string) map[string]int {
	out := map[string]int{}
	rows, err := db.Query(ctx, `
		SELECT COALESCE(country, 'Unknown'), COUNT(*)
		FROM check_results
		WHERE job_id=$1 AND alive=true
		GROUP BY country
		ORDER BY COUNT(*) DESC
	`, jobID)
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
