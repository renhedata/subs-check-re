package checker

import (
	"context"

	"encore.dev/beta/errs"

	subsvc "subs-check-re/services/subscription"
)

// TopNode represents one high-performing node for notification summaries.
type TopNode struct {
	Name      string `json:"name"`
	SpeedKbps int    `json:"speed_kbps"`
	LatencyMs int    `json:"latency_ms"`
	Country   string `json:"country"`
}

// JobDetailedSummary contains detailed statistics for a completed job.
type JobDetailedSummary struct {
	JobID            string                `json:"job_id"`
	SubscriptionName string                `json:"subscription_name"`
	Available        int                   `json:"available"`
	Total            int                   `json:"total"`
	Platforms        PlatformUnlockSummary `json:"platforms"`
	AvgSpeedKbps     int                   `json:"avg_speed_kbps"`
	MaxSpeedKbps     int                   `json:"max_speed_kbps"`
	AvgLatencyMs     int                   `json:"avg_latency_ms"`
	TopNodes         []TopNode             `json:"top_nodes"`
	Countries        map[string]int        `json:"countries"`
}

// GetJobDetailedSummary returns detailed statistics for a completed check job.
//
//encore:api private method=GET path=/internal/check/:jobID/summary
func GetJobDetailedSummary(ctx context.Context, jobID string) (*JobDetailedSummary, error) {
	var s JobDetailedSummary
	s.JobID = jobID
	s.Countries = make(map[string]int)

	// Basic job info
	var subID, userID string
	err := db.QueryRow(ctx, `
		SELECT subscription_id, user_id, available, total
		FROM check_jobs WHERE id = $1
	`, jobID).Scan(&subID, &userID, &s.Available, &s.Total)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("job not found").Err()
	}

	// Subscription name
	s.SubscriptionName = subID
	if resp, err := subsvc.GetSubscriptionNames(ctx, &subsvc.GetSubscriptionNamesParams{
		UserID: userID,
		IDs:    []string{subID},
	}); err == nil {
		if name, ok := resp.Names[subID]; ok {
			s.SubscriptionName = name
		}
	}

	// Platform summary
	db.QueryRow(ctx, `
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
	`, jobID).Scan( //nolint:errcheck
		&s.Platforms.Netflix, &s.Platforms.YouTube, &s.Platforms.YouTubePremium,
		&s.Platforms.OpenAI, &s.Platforms.Claude, &s.Platforms.Gemini,
		&s.Platforms.Grok, &s.Platforms.Disney, &s.Platforms.TikTok,
	)

	// Speed and latency stats
	db.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(speed_kbps) FILTER (WHERE speed_kbps > 0), 0)::int,
			COALESCE(MAX(speed_kbps), 0),
			COALESCE(AVG(latency_ms) FILTER (WHERE alive AND latency_ms > 0), 0)::int
		FROM check_results WHERE job_id=$1
	`, jobID).Scan(&s.AvgSpeedKbps, &s.MaxSpeedKbps, &s.AvgLatencyMs) //nolint:errcheck

	// Top 5 fastest nodes
	topRows, err := db.Query(ctx, `
		SELECT COALESCE(node_name, ''), speed_kbps, latency_ms, COALESCE(country, '')
		FROM check_results
		WHERE job_id=$1 AND alive=true AND speed_kbps > 0
		ORDER BY speed_kbps DESC
		LIMIT 5
	`, jobID)
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var n TopNode
			if err := topRows.Scan(&n.Name, &n.SpeedKbps, &n.LatencyMs, &n.Country); err == nil {
				s.TopNodes = append(s.TopNodes, n)
			}
		}
	}
	if s.TopNodes == nil {
		s.TopNodes = []TopNode{}
	}

	// Country breakdown
	countryRows, err := db.Query(ctx, `
		SELECT COALESCE(country, 'Unknown'), COUNT(*)
		FROM check_results
		WHERE job_id=$1 AND alive=true
		GROUP BY country
		ORDER BY COUNT(*) DESC
	`, jobID)
	if err == nil {
		defer countryRows.Close()
		for countryRows.Next() {
			var country string
			var count int
			if err := countryRows.Scan(&country, &count); err == nil {
				s.Countries[country] = count
			}
		}
	}

	return &s, nil
}
