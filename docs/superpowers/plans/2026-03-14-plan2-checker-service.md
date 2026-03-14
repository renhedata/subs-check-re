# Checker Service Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `checker` Encore service — fetch subscription nodes, run concurrent proxy checks via mihomo, persist results, and stream real-time progress via SSE.

**Architecture:** Single `checker` service with three Encore-managed tables (`nodes`, `check_jobs`, `check_results`). Job execution runs in a detached goroutine; SSE endpoint polls job progress from DB and pushes events. On completion, a PubSub topic emits an event for the future `notify` service.

**Tech Stack:** `github.com/metacubex/mihomo v1.19.19`, `gopkg.in/yaml.v3`, Encore sqldb, Encore PubSub, SSE via `//encore:api raw`

> **Reference:** `docs/superpowers/specs/2026-03-14-subs-check-re-design.md`
> **Reference implementation:** `/Users/ashark/tmp/subs-check/check/` and `/Users/ashark/tmp/subs-check/proxy/`

---

## File Map

```
services/checker/
├── migrations/
│   ├── 1_create_nodes.up.sql
│   ├── 1_create_nodes.down.sql
│   ├── 2_create_check_jobs.up.sql
│   ├── 2_create_check_jobs.down.sql
│   ├── 3_create_check_results.up.sql
│   └── 3_create_check_results.down.sql
├── fetch.go          # Fetch subscription URL, parse to proxy maps
├── mihomo.go         # CreateClient, CheckAlive (wraps mihomo adapter)
├── platform.go       # Platform detection: Netflix, YouTube, OpenAI, Claude, Gemini, Disney, TikTok
├── checker.go        # API endpoints + async job execution + SSE
└── checker_test.go   # Integration tests
```

---

## Chunk 1: Dependencies + DB Migrations

### Task 1: Add mihomo and yaml dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add dependencies**

```bash
cd /Users/ashark/Code/subs-check-re
go get github.com/metacubex/mihomo@v1.19.19
go get gopkg.in/yaml.v3
go mod tidy
```

- [ ] **Step 2: Verify go.mod**

```bash
grep -E "mihomo|yaml" go.mod
```

Expected: both lines present.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(checker): add mihomo and yaml.v3 dependencies"
```

---

### Task 2: Checker DB migrations

**Files:**
- Create: `services/checker/migrations/1_create_nodes.up.sql`
- Create: `services/checker/migrations/1_create_nodes.down.sql`
- Create: `services/checker/migrations/2_create_check_jobs.up.sql`
- Create: `services/checker/migrations/2_create_check_jobs.down.sql`
- Create: `services/checker/migrations/3_create_check_results.up.sql`
- Create: `services/checker/migrations/3_create_check_results.down.sql`

- [ ] **Step 1: Create nodes migration**

```sql
-- services/checker/migrations/1_create_nodes.up.sql
CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    type            TEXT NOT NULL DEFAULT '',
    server          TEXT NOT NULL DEFAULT '',
    port            INT  NOT NULL DEFAULT 0,
    config          JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_nodes_subscription_id ON nodes (subscription_id);
```

```sql
-- services/checker/migrations/1_create_nodes.down.sql
DROP TABLE IF EXISTS nodes;
```

- [ ] **Step 2: Create check_jobs migration**

```sql
-- services/checker/migrations/2_create_check_jobs.up.sql
CREATE TABLE check_jobs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'queued',
    total           INT  NOT NULL DEFAULT 0,
    progress        INT  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_check_jobs_subscription_id ON check_jobs (subscription_id);
CREATE INDEX idx_check_jobs_user_id ON check_jobs (user_id);
```

```sql
-- services/checker/migrations/2_create_check_jobs.down.sql
DROP TABLE IF EXISTS check_jobs;
```

- [ ] **Step 3: Create check_results migration**

```sql
-- services/checker/migrations/3_create_check_results.up.sql
CREATE TABLE check_results (
    id          TEXT PRIMARY KEY,
    job_id      TEXT NOT NULL,
    node_id     TEXT NOT NULL,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alive       BOOL NOT NULL DEFAULT FALSE,
    latency_ms  INT,
    speed_kbps  INT,
    country     TEXT,
    ip          TEXT,
    openai      BOOL,
    netflix     BOOL,
    youtube     TEXT,
    disney      BOOL,
    claude      BOOL,
    gemini      BOOL,
    tiktok      TEXT
);

CREATE INDEX idx_check_results_job_id ON check_results (job_id);
```

```sql
-- services/checker/migrations/3_create_check_results.down.sql
DROP TABLE IF EXISTS check_results;
```

- [ ] **Step 4: Commit**

```bash
git add services/checker/migrations/
git commit -m "feat(checker): add nodes, check_jobs, check_results migrations"
```

---

## Chunk 2: Core Engine

### Task 3: Subscription fetcher

**Files:**
- Create: `services/checker/fetch.go`

This file fetches a subscription URL and parses the response into mihomo proxy maps.
Two formats supported: Clash YAML (`proxies:` key) and V2Ray base64 (via `mihomo/common/convert`).

- [ ] **Step 1: Write fetch.go**

```go
// services/checker/fetch.go
package checker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/metacubex/mihomo/common/convert"
	"gopkg.in/yaml.v3"
)

// fetchProxies fetches a subscription URL and returns parsed proxy maps.
// Supports Clash YAML format and V2Ray/base64 format.
func fetchProxies(url string) ([]map[string]any, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "ClashMeta/1.19")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseProxies(data)
}

// parseProxies tries Clash YAML first, then V2Ray format.
func parseProxies(data []byte) ([]map[string]any, error) {
	// Try Clash YAML
	var clash struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &clash); err == nil && len(clash.Proxies) > 0 {
		return clash.Proxies, nil
	}

	// Try V2Ray/base64 format
	proxyList, err := convert.ConvertsV2Ray(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse as Clash YAML or V2Ray format: %w", err)
	}
	if len(proxyList) == 0 {
		return nil, fmt.Errorf("subscription contains no proxies")
	}
	return proxyList, nil
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./services/checker/...
```

- [ ] **Step 3: Commit**

```bash
git add services/checker/fetch.go
git commit -m "feat(checker): subscription fetcher (Clash YAML + V2Ray)"
```

---

### Task 4: mihomo proxy client wrapper

**Files:**
- Create: `services/checker/mihomo.go`

Wraps `adapter.ParseProxy` + creates HTTP client that dials through the proxy.
Also implements `checkAlive` (connectivity test) and `getProxyIP` (IP lookup).

- [ ] **Step 1: Write mihomo.go**

```go
// services/checker/mihomo.go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

const (
	proxyTimeout  = 15 * time.Second
	aliveTestURL  = "http://www.gstatic.com/generate_204"
	ipLookupURL   = "http://ip-api.com/json/?fields=query,countryCode"
)

// proxyClient wraps an HTTP client and its underlying mihomo proxy.
type proxyClient struct {
	*http.Client
	proxy constant.Proxy
}

// close releases proxy resources.
func (pc *proxyClient) close() {
	if pc.Client != nil {
		pc.Client.CloseIdleConnections()
	}
	if pc.proxy != nil {
		pc.proxy.Close()
	}
}

// newProxyClient creates an HTTP client that routes through the given proxy map.
// Returns nil if the proxy config is invalid.
func newProxyClient(mapping map[string]any) *proxyClient {
	proxy, err := adapter.ParseProxy(mapping)
	if err != nil {
		return nil
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			port, err := strconv.ParseUint(portStr, 10, 16)
			if err != nil {
				return nil, err
			}
			return proxy.DialContext(ctx, &constant.Metadata{
				Host:    host,
				DstPort: uint16(port),
			})
		},
		DisableKeepAlives: true,
	}

	return &proxyClient{
		Client: &http.Client{
			Timeout:   proxyTimeout,
			Transport: transport,
		},
		proxy: proxy,
	}
}

// checkAlive returns true if the proxy can reach the connectivity test URL.
func checkAlive(client *http.Client) bool {
	resp, err := client.Get(aliveTestURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 302
}

// getProxyInfo retrieves the external IP and country code via the proxy.
func getProxyInfo(client *http.Client) (ip, country string) {
	resp, err := client.Get(ipLookupURL)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", ""
	}

	var result struct {
		Query       string `json:"query"`
		CountryCode string `json:"countryCode"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ""
	}
	return result.Query, result.CountryCode
}

// measureLatency measures round-trip latency by timing a GET to the alive URL.
func measureLatency(client *http.Client) int {
	start := time.Now()
	resp, err := client.Get(aliveTestURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 302 {
		return 0
	}
	return int(time.Since(start).Milliseconds())
}

// nodeResult holds the outcome of checking a single node.
type nodeResult struct {
	NodeID    string
	NodeName  string
	Alive     bool
	LatencyMs int
	IP        string
	Country   string
	Netflix   bool
	YouTube   string
	OpenAI    bool
	Claude    bool
	Gemini    bool
	Disney    bool
	TikTok    string
}

// checkNode runs all checks for a single proxy mapping and returns the result.
func checkNode(nodeID string, mapping map[string]any) nodeResult {
	name, _ := mapping["name"].(string)
	result := nodeResult{NodeID: nodeID, NodeName: name}

	pc := newProxyClient(mapping)
	if pc == nil {
		return result
	}
	defer pc.close()

	if !checkAlive(pc.Client) {
		return result
	}
	result.Alive = true
	result.LatencyMs = measureLatency(pc.Client)

	// Create a media-check client with the same transport but shorter timeout
	mediaClient := &http.Client{
		Transport: pc.Transport,
		Timeout:   10 * time.Second,
	}

	result.IP, result.Country = getProxyInfo(mediaClient)
	result.Netflix, _ = checkNetflix(mediaClient)
	result.YouTube, _ = checkYouTube(mediaClient)
	result.OpenAI, _ = checkOpenAI(mediaClient)
	result.Claude, _ = checkClaude(mediaClient)
	result.Gemini, _ = checkGemini(mediaClient)
	result.Disney, _ = checkDisney(mediaClient)
	result.TikTok, _ = checkTikTok(mediaClient)

	return result
}

// unused suppresses "declared and not used" if some fields aren't referenced yet
var _ = fmt.Sprintf
```

- [ ] **Step 2: Compile check (will fail until platform.go is added)**

Note: This file references `checkNetflix`, `checkYouTube`, etc. which will be defined in `platform.go` (Task 5). That's fine — compile after Task 5.

- [ ] **Step 3: Commit (even if not yet building)**

```bash
git add services/checker/mihomo.go
git commit -m "feat(checker): mihomo proxy client wrapper"
```

---

### Task 5: Platform detection

**Files:**
- Create: `services/checker/platform.go`

Minimal implementations ported from the reference project. Each function takes
`*http.Client` and returns a result + error. No dependency on reference's `config` package.

- [ ] **Step 1: Write platform.go**

```go
// services/checker/platform.go
package checker

import (
	"io"
	"net/http"
	"strings"
)

// checkNetflix returns true if the proxy can access non-originals Netflix content.
func checkNetflix(client *http.Client) (bool, error) {
	// Two titles: LEGO Ninjago (81280792) + Breaking Bad (70143836)
	for _, titleID := range []string{"81280792", "70143836"} {
		req, err := http.NewRequest("GET", "https://www.netflix.com/title/"+titleID, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if !strings.Contains(string(body), "Oh no!") {
			return true, nil
		}
	}
	return false, nil
}

// checkYouTube returns the region code if YouTube Premium is available, else "".
func checkYouTube(client *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://www.youtube.com/premium", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()

	bodyStr := string(body)
	if strings.Contains(bodyStr, "Premium is not available in your country") {
		return "", nil
	}
	if strings.Contains(bodyStr, "ad-free") || strings.Contains(bodyStr, "YouTube Premium") {
		return "YES", nil
	}
	return "", nil
}

// checkOpenAI returns true if OpenAI API/chat is accessible.
func checkOpenAI(client *http.Client) (bool, error) {
	resp, err := client.Get("https://api.openai.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	// OpenAI returns 200 or 401 for API root — both mean reachable
	return resp.StatusCode == 200 || resp.StatusCode == 401, nil
}

// checkClaude returns true if Anthropic's Claude is accessible.
func checkClaude(client *http.Client) (bool, error) {
	resp, err := client.Get("https://api.anthropic.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 404, nil
}

// checkGemini returns true if Google Gemini API is accessible.
func checkGemini(client *http.Client) (bool, error) {
	resp, err := client.Get("https://generativelanguage.googleapis.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 400, nil
}

// checkDisney returns true if Disney+ is accessible.
func checkDisney(client *http.Client) (bool, error) {
	resp, err := client.Get("https://www.disneyplus.com/")
	if err != nil {
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	resp.Body.Close()
	bodyStr := string(body)
	notAvail := strings.Contains(bodyStr, "not available in your region") ||
		strings.Contains(bodyStr, "unavailable in your region")
	return !notAvail && resp.StatusCode == 200, nil
}

// checkTikTok returns the region code if TikTok is accessible, else "".
func checkTikTok(client *http.Client) (string, error) {
	resp, err := client.Get("https://www.tiktok.com/")
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	resp.Body.Close()
	if strings.Contains(string(body), "tiktok") && resp.StatusCode == 200 {
		return "YES", nil
	}
	return "", nil
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./services/checker/...
```

Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add services/checker/platform.go
git commit -m "feat(checker): platform detection (Netflix, YouTube, OpenAI, Claude, Gemini, Disney, TikTok)"
```

---

## Chunk 3: Job Management + API

### Task 6: Check job execution + API endpoints

**Files:**
- Create: `services/checker/checker.go`

This is the main service file. Contains:
- `POST /check/:subscriptionId` — create job, launch async goroutine
- `GET /check/:jobID/progress` — SSE raw endpoint
- `GET /check/:subscriptionId/results` — return latest job results
- Internal: `runJob` function that does the actual work

> **SSE in Encore:** Use `//encore:api raw` annotation — handler gets `(w http.ResponseWriter, req *http.Request)`.
> Path params on raw endpoints are NOT passed as function params — extract from `req.URL.Path` manually.

- [ ] **Step 1: Write checker.go**

```go
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
)

var db = sqldb.NewDatabase("checker", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

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

// --- Request/Response types ---

// TriggerResponse is returned by POST /check/:subscriptionId.
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

// NodeResult represents a single node's check result.
type NodeResult struct {
	NodeID    string  `json:"node_id"`
	NodeName  string  `json:"node_name"`
	NodeType  string  `json:"node_type"`
	Alive     bool    `json:"alive"`
	LatencyMs int     `json:"latency_ms"`
	Country   string  `json:"country"`
	IP        string  `json:"ip"`
	Netflix   bool    `json:"netflix"`
	YouTube   string  `json:"youtube"`
	OpenAI    bool    `json:"openai"`
	Claude    bool    `json:"claude"`
	Gemini    bool    `json:"gemini"`
	Disney    bool    `json:"disney"`
	TikTok    string  `json:"tiktok"`
}

// ResultsResponse is returned by GET /check/:subscriptionId/results.
type ResultsResponse struct {
	Job     Job          `json:"job"`
	Results []NodeResult `json:"results"`
}

// --- Active job tracker for SSE ---

type progressUpdate struct {
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
	NodeName string `json:"node_name"`
}

var (
	jobChannels   = make(map[string][]chan progressUpdate)
	jobChannelsMu sync.Mutex
)

func subscribeProgress(jobID string) chan progressUpdate {
	ch := make(chan progressUpdate, 50)
	jobChannelsMu.Lock()
	jobChannels[jobID] = append(jobChannels[jobID], ch)
	jobChannelsMu.Unlock()
	return ch
}

func publishProgress(jobID string, update progressUpdate) {
	jobChannelsMu.Lock()
	defer jobChannelsMu.Unlock()
	for _, ch := range jobChannels[jobID] {
		select {
		case ch <- update:
		default:
		}
	}
}

func cleanupJobChannels(jobID string) {
	jobChannelsMu.Lock()
	defer jobChannelsMu.Unlock()
	for _, ch := range jobChannels[jobID] {
		close(ch)
	}
	delete(jobChannels, jobID)
}

// --- API endpoints ---

// TriggerCheck creates a new check job for the given subscription and starts it asynchronously.
//
//encore:api auth method=POST path=/check/:subscriptionID
func TriggerCheck(ctx context.Context, subscriptionID string) (*TriggerResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	// Check for already-running job
	var runningCount int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_jobs
		WHERE subscription_id = $1 AND status = 'running'
	`, subscriptionID).Scan(&runningCount)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running for this subscription").Err()
	}

	jobID := uuid.New().String()
	_, err = db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, created_at)
		VALUES ($1, $2, $3, 'queued', $4)
	`, jobID, subscriptionID, claims.UserID, time.Now())
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	// Launch async job (detached context so it survives the request)
	go runJob(context.Background(), jobID, subscriptionID, claims.UserID)

	return &TriggerResponse{JobID: jobID}, nil
}

// GetProgress streams real-time check progress via SSE.
//
//encore:api auth raw method=GET path=/check/:jobID/progress
func GetProgress(w http.ResponseWriter, req *http.Request) {
	// Extract jobID from URL: /check/<jobID>/progress
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	// parts: ["check", "<jobID>", "progress"]
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

	// Check current job state from DB first
	var status string
	var progress, total int
	err := db.QueryRow(req.Context(), `
		SELECT status, progress, total FROM check_jobs WHERE id = $1
	`, jobID).Scan(&status, &progress, &total)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// If already done, send final event and return
	if status == "completed" || status == "failed" {
		sendSSE(w, flusher, map[string]any{"done": true, "status": status})
		return
	}

	// Subscribe to live updates
	ch := subscribeProgress(jobID)
	defer func() {
		jobChannelsMu.Lock()
		channels := jobChannels[jobID]
		for i, c := range channels {
			if c == ch {
				jobChannels[jobID] = append(channels[:i], channels[i+1:]...)
				break
			}
		}
		jobChannelsMu.Unlock()
	}()

	// Send current state immediately
	sendSSE(w, flusher, map[string]any{"progress": progress, "total": total})

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-ch:
			if !ok {
				// Channel closed = job done
				sendSSE(w, flusher, map[string]any{"done": true})
				return
			}
			sendSSE(w, flusher, update)
		}
	}
}

func sendSSE(w http.ResponseWriter, f http.Flusher, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// GetResults returns the latest check results for a subscription.
// Use ?job_id=<id> to query a specific historical job.
//
//encore:api auth method=GET path=/check/:subscriptionID/results
func GetResults(ctx context.Context, subscriptionID string) (*ResultsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	// Find latest (or specific) job
	jobID := "" // TODO: support ?job_id query param via raw endpoint if needed
	var job Job
	var err error

	if jobID == "" {
		err = db.QueryRow(ctx, `
			SELECT id, subscription_id, status, total, progress, created_at, finished_at
			FROM check_jobs
			WHERE subscription_id = $1 AND user_id = $2
			ORDER BY created_at DESC LIMIT 1
		`, subscriptionID, claims.UserID).Scan(
			&job.ID, &job.SubscriptionID, &job.Status,
			&job.Total, &job.Progress, &job.CreatedAt, &job.FinishedAt,
		)
	}
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
		ORDER BY cr.alive DESC, cr.latency_ms ASC
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

// --- Job runner ---

const checkConcurrency = 20

func runJob(ctx context.Context, jobID, subscriptionID, userID string) {
	setJobStatus := func(status string, total int) {
		db.Exec(ctx, `
			UPDATE check_jobs SET status = $2, total = $3 WHERE id = $1
		`, jobID, status, total)
	}
	failJob := func() {
		db.Exec(ctx, `
			UPDATE check_jobs SET status = 'failed', finished_at = $2 WHERE id = $1
		`, jobID, time.Now())
		cleanupJobChannels(jobID)
	}

	// Mark running
	db.Exec(ctx, `UPDATE check_jobs SET status = 'running' WHERE id = $1`, jobID)

	// Fetch subscription URL from subscription service DB
	// Note: cross-service DB access is not allowed in Encore.
	// We fetch the URL via a workaround: store it in the checker call params,
	// OR use Encore service-to-service calls.
	// For now: query the subscription DB directly via its service API call pattern.
	// Since Encore doesn't allow cross-DB access, we pass the URL via a helper approach.
	// IMPLEMENTATION NOTE: TriggerCheck will need to pass the URL via a small job_meta table
	// or we call the subscription service API.
	// For MVP simplicity: store sub_url in check_jobs at creation time.
	// See Task 6 addendum below.

	var subURL string
	err := db.QueryRow(ctx, `SELECT sub_url FROM check_jobs WHERE id = $1`, jobID).Scan(&subURL)
	if err != nil || subURL == "" {
		failJob()
		return
	}

	// Fetch and parse proxies
	proxies, err := fetchProxies(subURL)
	if err != nil {
		failJob()
		return
	}

	total := len(proxies)
	setJobStatus("running", total)

	// Replace nodes in DB
	db.Exec(ctx, `DELETE FROM nodes WHERE subscription_id = $1`, subscriptionID)
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

	// Concurrent node checking
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
				result := checkNode(t.nodeID, t.proxy)

				// Persist result
				resultID := uuid.New().String()
				db.Exec(ctx, `
					INSERT INTO check_results
					  (id, job_id, node_id, checked_at, alive, latency_ms, country, ip,
					   netflix, youtube, openai, claude, gemini, disney, tiktok)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
				`, resultID, jobID, t.nodeID, time.Now(),
					result.Alive, result.LatencyMs, result.Country, result.IP,
					result.Netflix, result.YouTube, result.OpenAI,
					result.Claude, result.Gemini, result.Disney, result.TikTok,
				)

				// Update progress
				db.Exec(ctx, `UPDATE check_jobs SET progress = progress + 1 WHERE id = $1`, jobID)
				publishProgress(jobID, progressUpdate{
					Progress: t.index + 1,
					Total:    total,
					NodeName: result.NodeName,
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
	now := time.Now()
	db.Exec(ctx, `
		UPDATE check_jobs SET status = 'completed', finished_at = $2 WHERE id = $1
	`, jobID, now)

	// Count available
	var available int
	db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_results WHERE job_id = $1 AND alive = true
	`, jobID).Scan(&available)

	// Publish completion event
	JobCompletedTopic.Publish(ctx, &JobCompletedEvent{
		JobID:          jobID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Available:      available,
		Total:          total,
	})

	// Update subscription last_run_at (cross-service: skip for now, can be done via API call)

	cleanupJobChannels(jobID)
}
```

> **IMPORTANT ADDENDUM — sub_url storage:**
> The `runJob` function reads `sub_url` from `check_jobs`, but the migration for `check_jobs` (Task 2) doesn't have that column. You need to add `sub_url TEXT NOT NULL DEFAULT ''` to the `check_jobs` migration AND update `TriggerCheck` to:
> 1. Fetch the subscription URL from the `subscription` service via an internal API call
> 2. Store it in the `check_jobs` row at creation time
>
> The cross-service call pattern in Encore: import `subscription` package and call `subscription.GetByID(ctx, id)`. But since `subscription` service doesn't expose a `GetByID` endpoint, add one (see Task 6b).

- [ ] **Step 2: Add `sub_url` column to check_jobs migration**

Edit `services/checker/migrations/2_create_check_jobs.up.sql` to add `sub_url TEXT NOT NULL DEFAULT ''` after `user_id`.

- [ ] **Step 3: Add `GetSubscription` internal endpoint to subscription service**

In `services/subscription/subscription.go`, add:

```go
// GetSubscription returns a subscription by ID. Internal endpoint for cross-service calls.
//
//encore:api auth method=GET path=/subscriptions/:id
func GetSubscription(ctx context.Context, id string) (*Subscription, error) {
	uid := encauth.Data().(*authsvc.UserClaims).UserID
	var s Subscription
	err := db.QueryRow(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at
		FROM subscriptions WHERE id = $1 AND user_id = $2
	`, id, uid).Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled, &s.CronExpr, &s.CreatedAt, &s.LastRunAt)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	return &s, nil
}
```

- [ ] **Step 4: Update TriggerCheck to fetch sub_url**

In `TriggerCheck`, before inserting the job, call the subscription service:

```go
// Fetch subscription (validates ownership + gets URL)
sub, err := subscription.GetSubscription(ctx, subscriptionID)
if err != nil {
    return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
}
```

Then include `sub.URL` in the INSERT:
```go
_, err = db.Exec(ctx, `
    INSERT INTO check_jobs (id, subscription_id, user_id, status, sub_url, created_at)
    VALUES ($1, $2, $3, 'queued', $4, $5)
`, jobID, subscriptionID, claims.UserID, sub.URL, time.Now())
```

- [ ] **Step 5: Compile check**

```bash
go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 6: Commit**

```bash
git add services/checker/checker.go services/subscription/subscription.go \
        services/checker/migrations/2_create_check_jobs.up.sql
git commit -m "feat(checker): job management, SSE progress endpoint, results endpoint"
```

---

### Task 7: Integration tests

**Files:**
- Create: `services/checker/checker_test.go`

Tests focus on the API layer (TriggerCheck, GetResults). The actual proxy checks are skipped in tests.

- [ ] **Step 1: Write tests**

```go
// services/checker/checker_test.go
package checker

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestTriggerCheckMissingSubscription(t *testing.T) {
	ctx := withAuth()
	// Trigger check for non-existent subscription — should fail gracefully
	_, err := TriggerCheck(ctx, "nonexistent-sub-id")
	if err == nil {
		t.Error("expected error for missing subscription")
	}
}

func TestGetResultsNoJobs(t *testing.T) {
	ctx := withAuth()
	_, err := GetResults(ctx, "nonexistent-sub-id")
	if err == nil {
		t.Error("expected error when no jobs exist")
	}
}
```

- [ ] **Step 2: Run tests**

```bash
encore test ./services/checker/...
```

Expected: tests pass.

- [ ] **Step 3: Run all tests**

```bash
encore test ./...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add services/checker/checker_test.go
git commit -m "test(checker): basic checker API tests"
```

---

## What's Next

- **Plan 3:** `scheduler` service (cron management) + `notify` service (webhook/telegram/email)
- **Plan 4:** Frontend — React app with TanStack Router, auth flow, subscription management, real-time SSE progress
