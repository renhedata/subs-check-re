// services/checker/checker.go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
	settingssvc "subs-check-re/services/settings"
	subsvc "subs-check-re/services/subscription"
)

var db = sqldb.NewDatabase("checker", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- PubSub ---

// PlatformUnlockSummary maps each platform key to how many nodes unlocked it.
type PlatformUnlockSummary map[string]int

// JobCompletedEvent is published when a check job finishes.
type JobCompletedEvent struct {
	JobID          string `json:"job_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	Available      int    `json:"available"`
	Total          int    `json:"total"`
}

// JobCompletedTopic is the PubSub topic for job completion events.
var JobCompletedTopic = pubsub.NewTopic[*JobCompletedEvent]("job-completed", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

// --- Response types ---

// TriggerResponse is returned by POST /check/:subscriptionID.
type TriggerResponse struct {
	JobID string `json:"job_id"`
}

// Job represents a check job.
type Job struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscription_id"`
	Status         string     `json:"status"`
	Total          int        `json:"total"`
	Progress       int        `json:"progress"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	TotalTrafficBytes int64      `json:"total_traffic_bytes"`
}

// NodeResult represents a single node's check result for the API response.
type NodeResult struct {
	NodeID          string `json:"node_id"`
	NodeName        string `json:"node_name"`
	NodeType        string `json:"node_type"`
	Enabled         bool   `json:"enabled"`
	Alive           bool   `json:"alive"`
	LatencyMs       int    `json:"latency_ms"`
	SpeedKbps       int    `json:"speed_kbps"`
	UploadSpeedKbps int    `json:"upload_speed_kbps"`
	Country         string `json:"country"`
	IP              string `json:"ip"`
	Server          string `json:"server"`
	Port            int    `json:"port"`
	Config          string `json:"config"`
	Platforms    map[string]PlatformOutcome `json:"platforms"`
	TrafficBytes int64                      `json:"traffic_bytes"`
}

// ResultsResponse is returned by GET /check/:subscriptionID/results.
type ResultsResponse struct {
	Job     Job          `json:"job"`
	Results []NodeResult `json:"results"`
}

// JobSummary is one entry in the job history list.
type JobSummary struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscription_id"`
	Status         string     `json:"status"`
	Total          int        `json:"total"`
	Available      int        `json:"available"`
	SpeedTest      bool       `json:"speed_test"`
	MediaApps      []string   `json:"media_apps"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	TotalTrafficBytes int64      `json:"total_traffic_bytes"`
}

// ListJobsResponse is returned by GET /check/:subscriptionID/jobs.
type ListJobsResponse struct {
	Jobs  []JobSummary `json:"jobs"`
	Total int          `json:"total"`
}

// ExportLog is one export request record.
type ExportLog struct {
	ID          string    `json:"id"`
	IP          string    `json:"ip"`
	RequestedAt time.Time `json:"requested_at"`
}

// ExportLogsResponse is returned by GET /export-logs/:subscriptionID.
type ExportLogsResponse struct {
	Logs []ExportLog `json:"logs"`
}

// ListJobsParams are the query parameters for GET /check/:subscriptionID/jobs.
type ListJobsParams struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}

// GetResultsParams are the query parameters for GET /check/:subscriptionID/results.
type GetResultsParams struct {
	JobID string `query:"job_id"`
}

// CheckOptions controls which tests are run per node.
type CheckOptions struct {
	SpeedTest       bool     `json:"speed_test"`
	UploadSpeedTest bool     `json:"upload_speed_test"`
	MediaApps       []string `json:"media_apps"`
	Debug           bool     `json:"debug"`
}

func defaultCheckOptions() CheckOptions {
	return CheckOptions{
		SpeedTest: true,
		MediaApps: []string{
			"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
			"claude", "gemini", "grok", "disney", "tiktok",
			"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video",
		},
	}
}

func applyOptionDefaults(o *CheckOptions) {
	// A nil slice means the field was omitted → default to the built-in
	// platform list. An explicit empty slice means "test no platforms"
	// (e.g. an alive-only run) and must be preserved.
	if o.MediaApps == nil {
		o.MediaApps = defaultCheckOptions().MediaApps
	}
}

// isActiveJobConflict reports whether err is a violation of the
// idx_check_jobs_one_active partial unique index (another queued/running job
// exists for the subscription).
func isActiveJobConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "idx_check_jobs_one_active")
}

// TriggerParams is the optional request body for POST /check/:subscriptionID.
type TriggerParams struct {
	SpeedTest       *bool    `json:"speed_test"`
	UploadSpeedTest *bool    `json:"upload_speed_test"`
	MediaApps       []string `json:"media_apps"`
	Debug           *bool    `json:"debug"`
}

// --- Progress event payload ---
// The bus that routes these to subscribers lives in jobbus.go.

type progressUpdate struct {
	Progress        int        `json:"progress"`
	Total           int        `json:"total"`
	NodeName        string     `json:"node_name,omitempty"`
	Alive           bool       `json:"alive"`
	LatencyMs       int        `json:"latency_ms,omitempty"`
	SpeedKbps       int        `json:"speed_kbps,omitempty"`
	UploadSpeedKbps int        `json:"upload_speed_kbps,omitempty"`
	Debug           *NodeDebug `json:"debug,omitempty"`
}

// --- API endpoints ---

// TriggerInternalParams is the request body for TriggerCheckInternal.
type TriggerInternalParams struct {
	UserID  string       `json:"user_id"`
	SubURL  string       `json:"sub_url"`
	Options CheckOptions `json:"options"`
}

// TriggerCheckInternal triggers a check job from an internal caller (e.g., scheduler).
// Does not require auth — caller supplies userID and subURL directly.
//
//encore:api private method=POST path=/internal/check/:subscriptionID
func TriggerCheckInternal(ctx context.Context, subscriptionID string, p *TriggerInternalParams) (*TriggerResponse, error) {
	var runningCount int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_jobs
		WHERE subscription_id = $1 AND status IN ('queued', 'running')
	`, subscriptionID).Scan(&runningCount); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running").Err()
	}

	// Fetch user's configured test URLs for scheduled runs
	userCfg, _ := settingssvc.GetSpeedTestURLForUser(ctx, p.UserID)
	speedTestURL := ""
	latencyTestURL := ""
	if userCfg != nil {
		speedTestURL = userCfg.SpeedTestURL
		latencyTestURL = userCfg.LatencyTestURL
	}

	applyOptionDefaults(&p.Options)
	optsJSON, _ := json.Marshal(p.Options)

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, latency_test_url, options_json, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'queued', $8)
	`, jobID, subscriptionID, p.UserID, p.SubURL, speedTestURL, latencyTestURL, optsJSON, time.Now()); err != nil {
		if isActiveJobConflict(err) {
			return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running").Err()
		}
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	go runJob(context.Background(), jobID, subscriptionID, p.UserID)
	return &TriggerResponse{JobID: jobID}, nil
}

// TriggerCheck creates a new check job for the given subscription.
//
//encore:api auth method=POST path=/check/:subscriptionID
func TriggerCheck(ctx context.Context, subscriptionID string, p *TriggerParams) (*TriggerResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	// Verify ownership and get subscription URL
	sub, err := subsvc.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}

	// Reject if a job is already running
	var runningCount int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_jobs
		WHERE subscription_id = $1 AND status IN ('queued', 'running')
	`, subscriptionID).Scan(&runningCount); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running for this subscription").Err()
	}

	// Fetch user's configured test URLs (empty = use defaults)
	userCfg, _ := settingssvc.GetSpeedTestURLForUser(ctx, claims.UserID)
	speedTestURL := ""
	latencyTestURL := ""
	if userCfg != nil {
		speedTestURL = userCfg.SpeedTestURL
		latencyTestURL = userCfg.LatencyTestURL
	}

	opts := defaultCheckOptions()
	if p != nil {
		if p.SpeedTest != nil {
			opts.SpeedTest = *p.SpeedTest
		}
		if p.UploadSpeedTest != nil {
			opts.UploadSpeedTest = *p.UploadSpeedTest
		}
		if p.MediaApps != nil {
			opts.MediaApps = p.MediaApps
		}
		if p.Debug != nil {
			opts.Debug = *p.Debug
		}
	}
	optsJSON, _ := json.Marshal(opts)

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, latency_test_url, options_json, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'queued', $8)
	`, jobID, subscriptionID, claims.UserID, sub.URL, speedTestURL, latencyTestURL, optsJSON, time.Now()); err != nil {
		if isActiveJobConflict(err) {
			return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running for this subscription").Err()
		}
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	// Detached goroutine — outlives the HTTP request
	go runJob(context.Background(), jobID, subscriptionID, claims.UserID)

	return &TriggerResponse{JobID: jobID}, nil
}

// GetProgress streams real-time check progress via SSE.
//
//encore:api public raw method=GET path=/check/:jobID/progress
func GetProgress(w http.ResponseWriter, req *http.Request) {
	// Extract jobID from path: /check/<jobID>/progress
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	jobID := parts[1]

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Check current job state
	var status string
	var progress, total int
	err := db.QueryRow(req.Context(), `
		SELECT status, progress, total FROM check_jobs WHERE id = $1
	`, jobID).Scan(&status, &progress, &total)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// Already finished — send done event immediately
	if status == "completed" || status == "failed" {
		writeSSE(w, flusher, map[string]any{"done": true, "status": status})
		return
	}

	// Send current snapshot
	writeSSE(w, flusher, progressUpdate{Progress: progress, Total: total})

	ch := defaultJobBus.Subscribe(jobID)
	defer defaultJobBus.Unsubscribe(jobID, ch)

	// Re-check after subscribing: the job may have finished between the
	// snapshot query above and Subscribe, in which case Close already ran and
	// no events (including the final close) will ever arrive on ch. Use
	// context.Background() so a request-context race can't skip the check and
	// leave the client waiting on a channel that never closes.
	if err := db.QueryRow(context.Background(), `SELECT status FROM check_jobs WHERE id = $1`, jobID).Scan(&status); err == nil &&
		(status == "completed" || status == "failed") {
		writeSSE(w, flusher, map[string]any{"done": true, "status": status})
		return
	}

	for {
		select {
		case <-req.Context().Done():
			return
		case update, ok := <-ch:
			if !ok {
				done := map[string]any{"done": true}
				var finalStatus string
				var available int
				if err := db.QueryRow(context.Background(),
					`SELECT status, available FROM check_jobs WHERE id = $1`, jobID).Scan(&finalStatus, &available); err == nil {
					done["status"] = finalStatus
					done["available"] = available
				}
				writeSSE(w, flusher, done)
				return
			}
			writeSSE(w, flusher, update)
		}
	}
}

func writeSSE(w http.ResponseWriter, f http.Flusher, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// CancelCheck stops a running check job.
//
//encore:api auth method=DELETE path=/check/:jobID
func CancelCheck(ctx context.Context, jobID string) error {
	claims := encauth.Data().(*authsvc.UserClaims)
	var count int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_jobs WHERE id=$1 AND user_id=$2 AND status IN ('running','queued')`,
		jobID, claims.UserID).Scan(&count); err != nil || count == 0 {
		return errs.B().Code(errs.NotFound).Msg("active job not found").Err()
	}
	defaultJobBus.TriggerCancel(jobID)
	return nil
}

// ListJobs returns paginated check job history for a subscription.
//
//encore:api auth method=GET path=/check/:subscriptionID/jobs
func ListJobs(ctx context.Context, subscriptionID string, p *ListJobsParams) (*ListJobsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	limit := p.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_jobs WHERE subscription_id=$1 AND user_id=$2`,
		subscriptionID, claims.UserID).Scan(&total); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}

	rows, err := db.Query(ctx, `
		SELECT id, subscription_id, status, total, available,
		       COALESCE(options_json, '{}'), total_traffic_bytes, created_at, finished_at
		FROM check_jobs
		WHERE subscription_id=$1 AND user_id=$2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, subscriptionID, claims.UserID, limit, offset)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var jobs []JobSummary
	for rows.Next() {
		var j JobSummary
		var optsJSON []byte
		if err := rows.Scan(&j.ID, &j.SubscriptionID, &j.Status, &j.Total, &j.Available,
			&optsJSON, &j.TotalTrafficBytes, &j.CreatedAt, &j.FinishedAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan error").Err()
		}
		var opts CheckOptions
		if err := json.Unmarshal(optsJSON, &opts); err == nil {
			j.SpeedTest = opts.SpeedTest
			j.MediaApps = opts.MediaApps
		}
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []JobSummary{}
	}
	return &ListJobsResponse{Jobs: jobs, Total: total}, nil
}

// GetResults returns the latest check results for a subscription.
//
//encore:api auth method=GET path=/check/:subscriptionID/results
func GetResults(ctx context.Context, subscriptionID string, p *GetResultsParams) (*ResultsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var job Job
	if p != nil && p.JobID != "" {
		// Specific job requested — verify ownership.
		err := db.QueryRow(ctx, `
			SELECT id, subscription_id, status, total, progress, total_traffic_bytes, created_at, finished_at
			FROM check_jobs
			WHERE id=$1 AND subscription_id=$2 AND user_id=$3
		`, p.JobID, subscriptionID, claims.UserID).Scan(
			&job.ID, &job.SubscriptionID, &job.Status,
			&job.Total, &job.Progress, &job.TotalTrafficBytes, &job.CreatedAt, &job.FinishedAt,
		)
		if err != nil {
			return nil, errs.B().Code(errs.NotFound).Msg("job not found").Err()
		}
	} else {
		// Latest completed job.
		err := db.QueryRow(ctx, `
			SELECT id, subscription_id, status, total, progress, total_traffic_bytes, created_at, finished_at
			FROM check_jobs
			WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
			ORDER BY created_at DESC LIMIT 1
		`, subscriptionID, claims.UserID).Scan(
			&job.ID, &job.SubscriptionID, &job.Status,
			&job.Total, &job.Progress, &job.TotalTrafficBytes, &job.CreatedAt, &job.FinishedAt,
		)
		if err != nil {
			return nil, errs.B().Code(errs.NotFound).Msg("no check jobs found").Err()
		}
	}

	// speed_kbps / upload_speed_kbps fall back to the most recent historical value if this job skipped speed testing.
	rows, err := db.Query(ctx, `
		WITH r AS (
			SELECT cr.node_id,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       COALESCE(n.type, cr.node_type) AS node_type,
			       COALESCE(n.enabled, true) AS enabled,
			       COALESCE(n.server, '') AS server,
			       COALESCE(n.port, 0) AS port,
			       COALESCE(COALESCE(n.config, cr.node_config)::text, '') AS config,
			       cr.alive, cr.latency_ms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps
			                FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name
			                  AND cj2.subscription_id = $2
			                  AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC
			                LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       CASE WHEN cr.upload_speed_kbps > 0 THEN cr.upload_speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.upload_speed_kbps
			                FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name
			                  AND cj2.subscription_id = $2
			                  AND cr2.upload_speed_kbps > 0
			                ORDER BY cr2.checked_at DESC
			                LIMIT 1
			            ), 0)
			       END AS upload_speed_kbps,
			       cr.country, cr.ip,
			       cr.platforms, cr.traffic_bytes
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			WHERE cr.job_id = $1
		)
		SELECT * FROM r
		ORDER BY alive DESC, speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST
	`, job.ID, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var results []NodeResult
	for rows.Next() {
		var r NodeResult
		var platformsJSON []byte
		if err := rows.Scan(
			&r.NodeID, &r.NodeName, &r.NodeType, &r.Enabled,
			&r.Server, &r.Port, &r.Config,
			&r.Alive, &r.LatencyMs, &r.SpeedKbps, &r.UploadSpeedKbps, &r.Country, &r.IP,
			&platformsJSON, &r.TrafficBytes,
		); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &r.Platforms)
		}
		if r.Platforms == nil {
			r.Platforms = map[string]PlatformOutcome{}
		}
		results = append(results, r)
	}
	if results == nil {
		results = []NodeResult{}
	}
	return &ResultsResponse{Job: job, Results: results}, nil
}

// GetExportLogs returns the export request log for a subscription.
//
//encore:api auth method=GET path=/export-logs/:subscriptionID
func GetExportLogs(ctx context.Context, subscriptionID string) (*ExportLogsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	// Verify subscription ownership
	var count int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_jobs WHERE subscription_id=$1 AND user_id=$2`,
		subscriptionID, claims.UserID).Scan(&count); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}

	rows, err := db.Query(ctx, `
		SELECT id, ip, requested_at FROM export_logs
		WHERE subscription_id=$1 AND user_id=$2
		ORDER BY requested_at DESC LIMIT 50
	`, subscriptionID, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var logs []ExportLog
	for rows.Next() {
		var l ExportLog
		if err := rows.Scan(&l.ID, &l.IP, &l.RequestedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []ExportLog{}
	}
	return &ExportLogsResponse{Logs: logs}, nil
}
