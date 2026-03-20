// services/checker/checker.go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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

// PlatformUnlockSummary holds how many nodes unlocked each streaming platform.
type PlatformUnlockSummary struct {
	Netflix        int `json:"netflix"`
	YouTube        int `json:"youtube"`
	YouTubePremium int `json:"youtube_premium"`
	OpenAI         int `json:"openai"`
	Claude         int `json:"claude"`
	Gemini         int `json:"gemini"`
	Grok           int `json:"grok"`
	Disney         int `json:"disney"`
	TikTok         int `json:"tiktok"`
}

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
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name"`
	NodeType  string `json:"node_type"`
	Alive     bool   `json:"alive"`
	LatencyMs int    `json:"latency_ms"`
	SpeedKbps int    `json:"speed_kbps"`
	Country   string `json:"country"`
	IP        string `json:"ip"`
	Netflix        bool   `json:"netflix"`
	YouTube        bool   `json:"youtube"`
	YouTubePremium bool   `json:"youtube_premium"`
	OpenAI         bool   `json:"openai"`
	Claude    bool   `json:"claude"`
	Gemini    bool   `json:"gemini"`
	Grok      bool   `json:"grok"`
	Disney    bool   `json:"disney"`
	TikTok       bool  `json:"tiktok"`
	TrafficBytes int64 `json:"traffic_bytes"`
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
	SpeedTest bool     `json:"speed_test"`
	MediaApps []string `json:"media_apps"`
}

func defaultCheckOptions() CheckOptions {
	return CheckOptions{
		SpeedTest: true,
		MediaApps: []string{"openai", "claude", "gemini", "grok", "netflix", "youtube", "disney", "tiktok"},
	}
}

func applyOptionDefaults(o *CheckOptions) {
	if o.SpeedTest == false && len(o.MediaApps) == 0 {
		*o = defaultCheckOptions()
		return
	}
	if o.MediaApps == nil {
		o.MediaApps = defaultCheckOptions().MediaApps
	}
}

func hasApp(opts CheckOptions, app string) bool {
	for _, a := range opts.MediaApps {
		if a == app {
			return true
		}
	}
	return false
}

// TriggerParams is the optional request body for POST /check/:subscriptionID.
type TriggerParams struct {
	SpeedTest *bool    `json:"speed_test"`
	MediaApps []string `json:"media_apps"`
}

// --- In-process SSE channels ---

type progressUpdate struct {
	Progress  int    `json:"progress"`
	Total     int    `json:"total"`
	NodeName  string `json:"node_name,omitempty"`
	Alive     bool   `json:"alive"`
	LatencyMs int    `json:"latency_ms,omitempty"`
	SpeedKbps int    `json:"speed_kbps,omitempty"`
}

var (
	jobChannels   = make(map[string][]chan progressUpdate)
	jobChannelsMu sync.Mutex
)

var (
	jobCancels   = make(map[string]context.CancelFunc)
	jobCancelsMu sync.Mutex
)

func storeJobCancel(jobID string, cancel context.CancelFunc) {
	jobCancelsMu.Lock()
	jobCancels[jobID] = cancel
	jobCancelsMu.Unlock()
}

func removeJobCancel(jobID string) {
	jobCancelsMu.Lock()
	delete(jobCancels, jobID)
	jobCancelsMu.Unlock()
}

func triggerJobCancel(jobID string) {
	jobCancelsMu.Lock()
	if fn, ok := jobCancels[jobID]; ok {
		fn()
		delete(jobCancels, jobID)
	}
	jobCancelsMu.Unlock()
}

func subscribeJobProgress(jobID string) chan progressUpdate {
	ch := make(chan progressUpdate, 100)
	jobChannelsMu.Lock()
	jobChannels[jobID] = append(jobChannels[jobID], ch)
	jobChannelsMu.Unlock()
	return ch
}

func unsubscribeJobProgress(jobID string, ch chan progressUpdate) {
	jobChannelsMu.Lock()
	defer jobChannelsMu.Unlock()
	channels := jobChannels[jobID]
	for i, c := range channels {
		if c == ch {
			jobChannels[jobID] = append(channels[:i], channels[i+1:]...)
			return
		}
	}
}

func broadcastProgress(jobID string, update progressUpdate) {
	jobChannelsMu.Lock()
	defer jobChannelsMu.Unlock()
	for _, ch := range jobChannels[jobID] {
		select {
		case ch <- update:
		default:
		}
	}
}

func closeJobChannels(jobID string) {
	jobChannelsMu.Lock()
	defer jobChannelsMu.Unlock()
	for _, ch := range jobChannels[jobID] {
		close(ch)
	}
	delete(jobChannels, jobID)
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
		WHERE subscription_id = $1 AND status = 'running'
	`, subscriptionID).Scan(&runningCount); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running").Err()
	}

	// Fetch user's configured speed test URL for scheduled runs
	speedCfg, _ := settingssvc.GetSpeedTestURLForUser(ctx, p.UserID)
	speedTestURL := ""
	if speedCfg != nil {
		speedTestURL = speedCfg.SpeedTestURL
	}

	applyOptionDefaults(&p.Options)
	optsJSON, _ := json.Marshal(p.Options)

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, options_json, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)
	`, jobID, subscriptionID, p.UserID, p.SubURL, speedTestURL, optsJSON, time.Now()); err != nil {
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
		WHERE subscription_id = $1 AND status = 'running'
	`, subscriptionID).Scan(&runningCount); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running for this subscription").Err()
	}

	// Fetch user's configured speed test URL (empty = use default)
	speedCfg, _ := settingssvc.GetSpeedTestURLForUser(ctx, claims.UserID)
	speedTestURL := ""
	if speedCfg != nil {
		speedTestURL = speedCfg.SpeedTestURL
	}

	opts := defaultCheckOptions()
	if p != nil {
		if p.SpeedTest != nil {
			opts.SpeedTest = *p.SpeedTest
		}
		if p.MediaApps != nil {
			opts.MediaApps = p.MediaApps
		}
	}
	optsJSON, _ := json.Marshal(opts)

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, options_json, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)
	`, jobID, subscriptionID, claims.UserID, sub.URL, speedTestURL, optsJSON, time.Now()); err != nil {
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

	ch := subscribeJobProgress(jobID)
	defer unsubscribeJobProgress(jobID, ch)

	for {
		select {
		case <-req.Context().Done():
			return
		case update, ok := <-ch:
			if !ok {
				writeSSE(w, flusher, map[string]any{"done": true})
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
	triggerJobCancel(jobID)
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

	// speed_kbps falls back to the most recent historical speed if this job skipped speed testing.
	rows, err := db.Query(ctx, `
		WITH r AS (
			SELECT cr.node_id,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       COALESCE(n.type, cr.node_type) AS node_type,
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
			       cr.country, cr.ip,
			       cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
			       cr.traffic_bytes
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
		if err := rows.Scan(
			&r.NodeID, &r.NodeName, &r.NodeType,
			&r.Alive, &r.LatencyMs, &r.SpeedKbps, &r.Country, &r.IP,
			&r.Netflix, &r.YouTube, &r.YouTubePremium, &r.OpenAI, &r.Claude, &r.Gemini, &r.Grok, &r.Disney, &r.TikTok,
			&r.TrafficBytes,
		); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
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

// --- Async job runner ---

const (
	checkConcurrency      = 20
	nodeTimeout           = 90 * time.Second
	defaultSpeedTestURL   = "https://speed.cloudflare.com/__down?bytes=204800"
)

func runJob(parentCtx context.Context, jobID, subscriptionID, userID string) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	storeJobCancel(jobID, cancel)
	defer removeJobCancel(jobID)

	markFailed := func() {
		db.Exec(context.Background(), `UPDATE check_jobs SET status='failed', finished_at=$2 WHERE id=$1`, jobID, time.Now())
		closeJobChannels(jobID)
	}

	// Mark as running
	db.Exec(context.Background(), `UPDATE check_jobs SET status='running' WHERE id=$1`, jobID)

	// Get subscription URL, speed test URL, and options from job row
	var subURL, speedTestURL string
	var optsJSON []byte
	if err := db.QueryRow(context.Background(),
		`SELECT sub_url, COALESCE(speed_test_url, ''), COALESCE(options_json, '{}') FROM check_jobs WHERE id=$1`,
		jobID).Scan(&subURL, &speedTestURL, &optsJSON); err != nil || subURL == "" {
		markFailed()
		return
	}
	if speedTestURL == "" {
		speedTestURL = defaultSpeedTestURL
	}
	var opts CheckOptions
	if err := json.Unmarshal(optsJSON, &opts); err != nil {
		opts = defaultCheckOptions()
	}

	// Fetch and parse proxies from subscription URL
	proxies, err := fetchProxies(subURL)
	if err != nil {
		markFailed()
		return
	}

	total := len(proxies)
	db.Exec(context.Background(), `UPDATE check_jobs SET total=$2 WHERE id=$1`, jobID, total)

	// Replace nodes for this subscription
	db.Exec(context.Background(), `DELETE FROM nodes WHERE subscription_id=$1`, subscriptionID)
	nodeIDs := make([]string, len(proxies))
	for i, p := range proxies {
		id := uuid.New().String()
		nodeIDs[i] = id
		name, _ := p["name"].(string)
		ptype, _ := p["type"].(string)
		server, _ := p["server"].(string)
		port := 0
		if v, ok := p["port"].(int); ok {
			port = v
		}
		configJSON, _ := json.Marshal(p)
		db.Exec(context.Background(), `
			INSERT INTO nodes (id, subscription_id, name, type, server, port, config)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, id, subscriptionID, name, ptype, server, port, configJSON)
	}

	// Run concurrent checks
	type task struct {
		index  int
		nodeID string
		proxy  map[string]any
	}
	taskCh := make(chan task, checkConcurrency)

	var processedCount atomic.Int64
	var totalTrafficBytes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < checkConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { recover() }() // prevent a mihomo panic from killing the worker
			for t := range taskCh {
				nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
				res := checkNode(nodeCtx, t.nodeID, t.proxy, speedTestURL, opts)
				cancel()

				nodeName := res.NodeName
				nodeType, _ := t.proxy["type"].(string)
				resultID := uuid.New().String()
				nodeConfigJSON, _ := json.Marshal(t.proxy)
				db.Exec(context.Background(), `
					INSERT INTO check_results
					  (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip,
					   netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok, traffic_bytes)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
				`, resultID, jobID, t.nodeID, nodeName, nodeType, nodeConfigJSON, time.Now(),
					res.Alive, res.LatencyMs, res.SpeedKbps, res.Country, res.IP,
					res.Netflix, res.YouTube, res.YouTubePremium, res.OpenAI,
					res.Claude, res.Gemini, res.Grok, res.Disney, res.TikTok,
					res.TrafficBytes,
				)

				totalTrafficBytes.Add(res.TrafficBytes)
				n := processedCount.Add(1)
				db.Exec(context.Background(), `UPDATE check_jobs SET progress=$2 WHERE id=$1`, jobID, n)
				broadcastProgress(jobID, progressUpdate{
					Progress:  int(n),
					Total:     total,
					NodeName:  res.NodeName,
					Alive:     res.Alive,
					LatencyMs: res.LatencyMs,
					SpeedKbps: res.SpeedKbps,
				})
			}
		}()
	}

	for i, p := range proxies {
		select {
		case <-ctx.Done():
			close(taskCh)
			wg.Wait()
			markFailed()
			return
		case taskCh <- task{index: i, nodeID: nodeIDs[i], proxy: p}:
		}
	}
	close(taskCh)
	wg.Wait()

	// Count available BEFORE updating (count query must run first).
	var available int
	db.QueryRow(context.Background(), `SELECT COUNT(*) FROM check_results WHERE job_id=$1 AND alive=true`, jobID).Scan(&available)

	db.Exec(context.Background(), `UPDATE check_jobs SET status='completed', finished_at=$2, available=$3, total_traffic_bytes=$4 WHERE id=$1`,
		jobID, time.Now(), available, totalTrafficBytes.Load())

	// Publish completion event — notify service fetches details on demand via GetJobDetailedSummary
	JobCompletedTopic.Publish(context.Background(), &JobCompletedEvent{
		JobID:          jobID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Available:      available,
		Total:          total,
	})

	closeJobChannels(jobID)
}
