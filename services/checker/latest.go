// services/checker/latest.go
package checker

import (
	"context"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
)

// LatestJobSummary is the most recent check job for one subscription.
type LatestJobSummary struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Available    int        `json:"available"`
	Total        int        `json:"total"`
	AvgLatencyMs int        `json:"avg_latency_ms"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// LatestJobsResponse maps subscription ID → latest job summary.
type LatestJobsResponse struct {
	Jobs map[string]LatestJobSummary `json:"jobs"`
}

// LatestJobs returns the most recent check job per subscription for the
// current user, with the alive-node average latency. Powers the workbench
// subscription list and the scheduler's "Last check" column in one request.
//
//encore:api auth method=GET path=/check-summaries
func LatestJobs(ctx context.Context) (*LatestJobsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (cj.subscription_id)
		       cj.subscription_id, cj.id, cj.status, cj.available, cj.total,
		       cj.created_at, cj.finished_at,
		       COALESCE((
		           SELECT ROUND(AVG(cr.latency_ms))::int
		           FROM check_results cr
		           WHERE cr.job_id = cj.id AND cr.alive AND cr.latency_ms IS NOT NULL
		       ), 0) AS avg_latency_ms
		FROM check_jobs cj
		WHERE cj.user_id = $1
		ORDER BY cj.subscription_id, cj.created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	jobs := make(map[string]LatestJobSummary)
	for rows.Next() {
		var subID string
		var s LatestJobSummary
		if err := rows.Scan(&subID, &s.ID, &s.Status, &s.Available, &s.Total,
			&s.CreatedAt, &s.FinishedAt, &s.AvgLatencyMs); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan error").Err()
		}
		jobs[subID] = s
	}
	if err := rows.Err(); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("rows iteration failed").Err()
	}
	return &LatestJobsResponse{Jobs: jobs}, nil
}
