// services/checker/checker.go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
	subsvc "subs-check-re/services/subscription"
)

var db = sqldb.NewDatabase("checker", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- PubSub ---

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
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

// NodeResult represents a single node's check result for the API response.
type NodeResult struct {
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name"`
	NodeType  string `json:"node_type"`
	Alive     bool   `json:"alive"`
	LatencyMs int    `json:"latency_ms"`
	Country   string `json:"country"`
	IP        string `json:"ip"`
	Netflix   bool   `json:"netflix"`
	YouTube   string `json:"youtube"`
	OpenAI    bool   `json:"openai"`
	Claude    bool   `json:"claude"`
	Gemini    bool   `json:"gemini"`
	Disney    bool   `json:"disney"`
	TikTok    string `json:"tiktok"`
}

// ResultsResponse is returned by GET /check/:subscriptionID/results.
type ResultsResponse struct {
	Job     Job          `json:"job"`
	Results []NodeResult `json:"results"`
}

// --- In-process SSE channels ---

type progressUpdate struct {
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
	NodeName string `json:"node_name,omitempty"`
}

var (
	jobChannels   = make(map[string][]chan progressUpdate)
	jobChannelsMu sync.Mutex
)

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
	UserID string `json:"user_id"`
	SubURL string `json:"sub_url"`
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

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, status, created_at)
		VALUES ($1, $2, $3, $4, 'queued', $5)
	`, jobID, subscriptionID, p.UserID, p.SubURL, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	go runJob(context.Background(), jobID, subscriptionID, p.UserID)
	return &TriggerResponse{JobID: jobID}, nil
}

// TriggerCheck creates a new check job for the given subscription.
//
//encore:api auth method=POST path=/check/:subscriptionID
func TriggerCheck(ctx context.Context, subscriptionID string) (*TriggerResponse, error) {
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

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, status, created_at)
		VALUES ($1, $2, $3, $4, 'queued', $5)
	`, jobID, subscriptionID, claims.UserID, sub.URL, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	// Detached goroutine — outlives the HTTP request
	go runJob(context.Background(), jobID, subscriptionID, claims.UserID)

	return &TriggerResponse{JobID: jobID}, nil
}

// GetProgress streams real-time check progress via SSE.
//
//encore:api auth raw method=GET path=/check/:jobID/progress
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

// GetResults returns the latest check results for a subscription.
//
//encore:api auth method=GET path=/check/:subscriptionID/results
func GetResults(ctx context.Context, subscriptionID string) (*ResultsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var job Job
	err := db.QueryRow(ctx, `
		SELECT id, subscription_id, status, total, progress, created_at, finished_at
		FROM check_jobs
		WHERE subscription_id = $1 AND user_id = $2
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID, claims.UserID).Scan(
		&job.ID, &job.SubscriptionID, &job.Status,
		&job.Total, &job.Progress, &job.CreatedAt, &job.FinishedAt,
	)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("no check jobs found").Err()
	}

	rows, err := db.Query(ctx, `
		SELECT cr.node_id, n.name, n.type,
		       cr.alive, cr.latency_ms, cr.country, cr.ip,
		       cr.netflix, cr.youtube, cr.openai, cr.claude, cr.gemini, cr.disney, cr.tiktok
		FROM check_results cr
		JOIN nodes n ON n.id = cr.node_id
		WHERE cr.job_id = $1
		ORDER BY cr.alive DESC, cr.latency_ms ASC NULLS LAST
	`, job.ID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var results []NodeResult
	for rows.Next() {
		var r NodeResult
		if err := rows.Scan(
			&r.NodeID, &r.NodeName, &r.NodeType,
			&r.Alive, &r.LatencyMs, &r.Country, &r.IP,
			&r.Netflix, &r.YouTube, &r.OpenAI, &r.Claude, &r.Gemini, &r.Disney, &r.TikTok,
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

// --- Async job runner ---

const checkConcurrency = 20

func runJob(ctx context.Context, jobID, subscriptionID, userID string) {
	markFailed := func() {
		db.Exec(ctx, `UPDATE check_jobs SET status='failed', finished_at=$2 WHERE id=$1`, jobID, time.Now())
		closeJobChannels(jobID)
	}

	// Mark as running
	db.Exec(ctx, `UPDATE check_jobs SET status='running' WHERE id=$1`, jobID)

	// Get subscription URL from job row
	var subURL string
	if err := db.QueryRow(ctx, `SELECT sub_url FROM check_jobs WHERE id=$1`, jobID).Scan(&subURL); err != nil || subURL == "" {
		markFailed()
		return
	}

	// Fetch and parse proxies from subscription URL
	proxies, err := fetchProxies(subURL)
	if err != nil {
		markFailed()
		return
	}

	total := len(proxies)
	db.Exec(ctx, `UPDATE check_jobs SET total=$2 WHERE id=$1`, jobID, total)

	// Replace nodes for this subscription
	db.Exec(ctx, `DELETE FROM nodes WHERE subscription_id=$1`, subscriptionID)
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
		db.Exec(ctx, `
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

	var wg sync.WaitGroup
	for i := 0; i < checkConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				res := checkNode(t.nodeID, t.proxy)

				resultID := uuid.New().String()
				db.Exec(ctx, `
					INSERT INTO check_results
					  (id, job_id, node_id, checked_at, alive, latency_ms, country, ip,
					   netflix, youtube, openai, claude, gemini, disney, tiktok)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
				`, resultID, jobID, t.nodeID, time.Now(),
					res.Alive, res.LatencyMs, res.Country, res.IP,
					res.Netflix, res.YouTube, res.OpenAI,
					res.Claude, res.Gemini, res.Disney, res.TikTok,
				)

				db.Exec(ctx, `UPDATE check_jobs SET progress=progress+1 WHERE id=$1`, jobID)
				broadcastProgress(jobID, progressUpdate{
					Progress: t.index + 1,
					Total:    total,
					NodeName: res.NodeName,
				})
			}
		}()
	}

	for i, p := range proxies {
		taskCh <- task{index: i, nodeID: nodeIDs[i], proxy: p}
	}
	close(taskCh)
	wg.Wait()

	// Mark completed
	db.Exec(ctx, `UPDATE check_jobs SET status='completed', finished_at=$2 WHERE id=$1`, jobID, time.Now())

	// Count available nodes
	var available int
	db.QueryRow(ctx, `SELECT COUNT(*) FROM check_results WHERE job_id=$1 AND alive=true`, jobID).Scan(&available)

	// Publish completion event for notify service
	JobCompletedTopic.Publish(ctx, &JobCompletedEvent{
		JobID:          jobID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Available:      available,
		Total:          total,
	})

	closeJobChannels(jobID)
}
