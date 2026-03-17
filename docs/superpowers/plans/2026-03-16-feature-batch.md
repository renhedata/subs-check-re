# Feature Batch Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement speed-test fix, selective check options, job history, notify CRUD, export API, and dashboard docs in one coordinated batch.

**Architecture:** All backend changes are in existing Encore services. New DB columns are added via numbered migrations. The export endpoint is extracted into a dedicated `export.go` file in the checker service. Frontend pages are modified in place.

**Tech Stack:** Go 1.22 + Encore framework, PostgreSQL, React 19 + TanStack Router/Query, Biome for lint/format.

**Spec:** `docs/superpowers/specs/2026-03-16-feature-batch-spec.md`

---

## Chunk 1: Backend

### Task 1: Speed test fix (`services/checker/mihomo.go`)

**Files:**
- Modify: `services/checker/mihomo.go`
- Modify: `services/checker/checker_test.go`

**What changes:**
- `measureSpeed` gets a new signature: takes `http.RoundTripper` instead of `*http.Client`.
- Creates a fresh `http.Client` with 30 s timeout internally.
- Measures elapsed time only from after `resp` is returned (excludes TCP/TLS handshake).
- Partial downloads (err != nil but n > 0) still produce a speed value.
- Call site in `checkNode` changes to `pc.Client.Transport`.

- [ ] **Step 1: Write failing test**

Add to `services/checker/checker_test.go`:

```go
func TestMeasureSpeedPartialDownload(t *testing.T) {
	// measureSpeed should return a non-zero value even if the body is only
	// partially read (simulates a slow proxy where the client times out after
	// receiving some bytes).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write 512 bytes then hang forever — simulates a slow proxy.
		w.Write(make([]byte, 512))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(60 * time.Second)
	}))
	defer srv.Close()

	ctx := context.Background()
	// Use a 300 ms timeout so the test runs fast.
	transport := http.DefaultTransport
	speed := measureSpeedWithTimeout(ctx, transport, srv.URL, 300*time.Millisecond)
	if speed == 0 {
		t.Error("expected non-zero speed for partial download")
	}
}
```

Add required imports: `"net/http"`, `"net/http/httptest"`, `"time"`.

- [ ] **Step 2: Run to confirm it fails**

```bash
cd /Users/ashark/Code/subs-check-re
encore test ./services/checker/...
```

Expected: compile error — `measureSpeedWithTimeout` undefined.

- [ ] **Step 3: Implement the fix**

Replace the `measureSpeed` function in `services/checker/mihomo.go`:

```go
const speedTestTimeout = 30 * time.Second

// measureSpeed downloads from speedTestURL through the given transport and
// returns throughput in KB/s. It creates its own http.Client so the timeout
// covers only the download phase, not the proxy handshake.
func measureSpeed(ctx context.Context, transport http.RoundTripper, speedTestURL string) int {
	return measureSpeedWithTimeout(ctx, transport, speedTestURL, speedTestTimeout)
}

func measureSpeedWithTimeout(ctx context.Context, transport http.RoundTripper, speedTestURL string, timeout time.Duration) int {
	client := &http.Client{Timeout: timeout, Transport: transport}
	resp, err := get(ctx, client, speedTestURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	startDownload := time.Now()
	n, _ := io.Copy(io.Discard, resp.Body) // ignore err — partial download is fine
	if n == 0 {
		return 0
	}
	elapsed := time.Since(startDownload).Seconds()
	if elapsed == 0 {
		return 0
	}
	return int(float64(n) / 1024 / elapsed)
}
```

Update the call site in `checkNode` (line ~185):

```go
result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)
```

- [ ] **Step 4: Run tests**

```bash
encore test ./services/checker/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add services/checker/mihomo.go services/checker/checker_test.go
git commit -m "fix(checker): fix speed test using dedicated 30s timeout and partial-download measurement"
```

---

### Task 2: DB migrations

**Files:**
- Create: `services/checker/migrations/5_add_options_available.up.sql`
- Create: `services/checker/migrations/5_add_options_available.down.sql`
- Create: `services/settings/migrations/2_add_api_key.up.sql`
- Create: `services/settings/migrations/2_add_api_key.down.sql`

- [ ] **Step 1: Create checker migration 5**

`services/checker/migrations/5_add_options_available.up.sql`:
```sql
ALTER TABLE check_jobs ADD COLUMN IF NOT EXISTS options_json JSONB;
ALTER TABLE check_jobs ADD COLUMN IF NOT EXISTS available INT NOT NULL DEFAULT 0;
```

`services/checker/migrations/5_add_options_available.down.sql`:
```sql
ALTER TABLE check_jobs DROP COLUMN IF EXISTS options_json;
ALTER TABLE check_jobs DROP COLUMN IF EXISTS available;
```

- [ ] **Step 2: Create settings migration 2**

`services/settings/migrations/2_add_api_key.up.sql`:
```sql
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS api_key TEXT UNIQUE;
```

`services/settings/migrations/2_add_api_key.down.sql`:
```sql
ALTER TABLE user_settings DROP COLUMN IF EXISTS api_key;
```

- [ ] **Step 3: Verify migrations apply**

```bash
cd /Users/ashark/Code/subs-check-re
encore db migrate
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add services/checker/migrations/ services/settings/migrations/
git commit -m "chore(db): add options_json/available to check_jobs, api_key to user_settings"
```

---

### Task 3: Check options — backend (`services/checker/checker.go`)

**Files:**
- Modify: `services/checker/checker.go`
- Modify: `services/checker/mihomo.go`
- Modify: `services/checker/checker_test.go`

**What changes:**
- Add `CheckOptions` struct and `defaultCheckOptions()` helper.
- `TriggerCheck` and `TriggerCheckInternal` accept optional options in body; apply defaults before use.
- Options stored as non-null JSON in `check_jobs.options_json` on insert.
- `runJob` reads options from DB, passes to `checkNode`.
- `checkNode` and `mihomo.go:checkNode` accept `opts CheckOptions` and skip disabled checks.

- [ ] **Step 1: Write failing test**

Add to `services/checker/checker_test.go`:

```go
func TestDefaultCheckOptionsHasAllPlatforms(t *testing.T) {
	opts := defaultCheckOptions()
	if !opts.SpeedTest {
		t.Error("expected SpeedTest=true by default")
	}
	wantPlatforms := []string{"openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"}
	for _, p := range wantPlatforms {
		found := false
		for _, m := range opts.MediaApps {
			if m == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected platform %q in default MediaApps", p)
		}
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
encore test ./services/checker/...
```

Expected: compile error — `defaultCheckOptions` undefined.

- [ ] **Step 3: Add types and helpers to `checker.go`**

Add near the top of `checker.go`, after the existing type definitions:

```go
// CheckOptions controls which tests are run per node.
type CheckOptions struct {
	SpeedTest bool     `json:"speed_test"`
	MediaApps []string `json:"media_apps"`
}

func defaultCheckOptions() CheckOptions {
	return CheckOptions{
		SpeedTest: true,
		MediaApps: []string{"openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"},
	}
}

// applyOptionDefaults fills zero-value fields with defaults.
func applyOptionDefaults(o *CheckOptions) {
	if !o.SpeedTest && len(o.MediaApps) == 0 {
		// Completely zero — apply full defaults.
		*o = defaultCheckOptions()
		return
	}
	// SpeedTest: zero-value false may be intentional — leave it.
	// But if MediaApps was explicitly omitted (nil), fill all platforms.
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
```

- [ ] **Step 4: Add `TriggerParams` and modify `TriggerCheck`**

Add a new request type:

```go
// TriggerParams is the optional request body for POST /check/:subscriptionID.
type TriggerParams struct {
	SpeedTest *bool    `json:"speed_test"`
	MediaApps []string `json:"media_apps"`
}
```

Change `TriggerCheck` signature:

```go
//encore:api auth method=POST path=/check/:subscriptionID
func TriggerCheck(ctx context.Context, subscriptionID string, p *TriggerParams) (*TriggerResponse, error) {
```

After the existing `speedTestURL` resolution, resolve options:

```go
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
```

Update the INSERT to include `options_json`:

```go
if _, err := db.Exec(ctx, `
    INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, options_json, status, created_at)
    VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7)
`, jobID, subscriptionID, claims.UserID, sub.URL, speedTestURL, optsJSON, time.Now()); err != nil {
```

- [ ] **Step 5: Update `TriggerCheckInternal`**

Similarly update `TriggerInternalParams`:

```go
type TriggerInternalParams struct {
	UserID    string       `json:"user_id"`
	SubURL    string       `json:"sub_url"`
	Options   CheckOptions `json:"options"`
}
```

In `TriggerCheckInternal`, call `applyOptionDefaults(&p.Options)` before inserting, then include `options_json` in the INSERT (same pattern as TriggerCheck).

- [ ] **Step 6: Update `runJob` to read and use options**

In `runJob`, after reading `subURL` and `speedTestURL`, also read `options_json`:

```go
var subURL, speedTestURL string
var optsJSON []byte
if err := db.QueryRow(ctx,
    `SELECT sub_url, COALESCE(speed_test_url, ''), COALESCE(options_json, '{}') FROM check_jobs WHERE id=$1`,
    jobID).Scan(&subURL, &speedTestURL, &optsJSON); err != nil || subURL == "" {
    markFailed()
    return
}
var opts CheckOptions
if err := json.Unmarshal(optsJSON, &opts); err != nil {
    opts = defaultCheckOptions()
}
```

Pass `opts` to `checkNode`:

```go
res := checkNode(nodeCtx, t.nodeID, t.proxy, speedTestURL, opts)
```

Replace the two separate statements at the bottom of `runJob` (the `UPDATE status='completed'` and the `SELECT COUNT(*)`) with the following, **in this exact order** — the count query must run before the update so `available` is populated:

```go
// Count available nodes FIRST, then write the final update.
var available int
db.QueryRow(ctx, `SELECT COUNT(*) FROM check_results WHERE job_id=$1 AND alive=true`, jobID).Scan(&available)

db.Exec(ctx, `UPDATE check_jobs SET status='completed', finished_at=$2, available=$3 WHERE id=$1`,
    jobID, time.Now(), available)
```

Remove the original `var available int` / `db.QueryRow ... Scan(&available)` block that appeared after the old completion UPDATE.

- [ ] **Step 7: Update `checkNode` in `mihomo.go`**

Change signature:

```go
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL string, opts CheckOptions) nodeCheckResult {
```

Apply option guards:

```go
if opts.SpeedTest {
    result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)
}

result.IP, result.Country = getProxyInfo(ctx, mediaClient)
if hasApp(opts, "netflix") {
    result.Netflix, _ = checkNetflix(ctx, mediaClient)
}
if hasApp(opts, "youtube") {
    result.YouTube, _ = checkYouTube(ctx, mediaClient)
}
if hasApp(opts, "openai") {
    result.OpenAI, _ = checkOpenAI(ctx, mediaClient)
}
if hasApp(opts, "claude") {
    result.Claude, _ = checkClaude(ctx, mediaClient)
}
if hasApp(opts, "gemini") {
    result.Gemini, _ = checkGemini(ctx, mediaClient)
}
if hasApp(opts, "disney") {
    result.Disney, _ = checkDisney(ctx, mediaClient)
}
if hasApp(opts, "tiktok") {
    result.TikTok, _ = checkTikTok(ctx, mediaClient)
}
```

Skip the media block entirely if `opts.MediaApps` is empty:

```go
if len(opts.MediaApps) > 0 {
    mediaClient := &http.Client{
        Transport: pc.Transport,
        Timeout:   8 * time.Second,
    }
    result.IP, result.Country = getProxyInfo(ctx, mediaClient)
    // ... (individual platform checks as above)
}
```

- [ ] **Step 8: Update existing tests that break due to signature changes**

`TriggerCheck` now takes `*TriggerParams` as a third argument and `GetResults` now takes `*GetResultsParams`. The existing tests in `checker_test.go` call them with the old 2-argument signatures — update them:

```go
// Was: TriggerCheck(ctx, "nonexistent-sub-id")
_, err := TriggerCheck(ctx, "nonexistent-sub-id", nil)

// Was: GetResults(ctx, "nonexistent-sub-id")
_, err := GetResults(ctx, "nonexistent-sub-id", nil)
```

- [ ] **Step 9: Run all tests**

```bash
encore test ./services/checker/...
```

Expected: all pass.

- [ ] **Step 10: Commit**

```bash
git add services/checker/checker.go services/checker/mihomo.go services/checker/checker_test.go
git commit -m "feat(checker): add selective check options (speed_test, media_apps per job)"
```

---

### Task 4: Job history endpoints (`services/checker/checker.go`)

**Files:**
- Modify: `services/checker/checker.go`
- Modify: `services/checker/checker_test.go`

**What changes:**
- New `JobSummary` struct (includes `available`, `speed_test`, `media_apps`).
- New `ListJobsResponse` type.
- New endpoint `GET /check/:subscriptionID/jobs` with `limit`/`offset` query params.
- `GetResults` updated to filter `status = 'completed'` and accept optional `?job_id`.

- [ ] **Step 1: Write failing tests**

Add to `services/checker/checker_test.go`:

```go
func TestListJobsEmpty(t *testing.T) {
	ctx := withAuth()
	resp, err := ListJobs(ctx, "nonexistent-sub-id", &ListJobsParams{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestGetResultsWithJobIDNotFound(t *testing.T) {
	ctx := withAuth()
	_, err := GetResults(ctx, "nonexistent-sub-id", &GetResultsParams{JobID: "nonexistent-job"})
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
encore test ./services/checker/...
```

Expected: compile errors — `ListJobs`, `ListJobsParams`, `GetResultsParams` undefined.

- [ ] **Step 3: Add `JobSummary` and `ListJobs` endpoint**

Add types in `checker.go`:

```go
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
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

// ListJobsResponse is returned by GET /check/:subscriptionID/jobs.
type ListJobsResponse struct {
	Jobs  []JobSummary `json:"jobs"`
	Total int          `json:"total"`
}

// ListJobsParams are the query parameters for GET /check/:subscriptionID/jobs.
type ListJobsParams struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}
```

Add the endpoint:

```go
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
		       COALESCE(options_json, '{}'), created_at, finished_at
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
			&optsJSON, &j.CreatedAt, &j.FinishedAt); err != nil {
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
```

- [ ] **Step 4: Update `GetResults` to accept `job_id` and filter completed**

Change the signature:

```go
// GetResultsParams are the query parameters for GET /check/:subscriptionID/results.
type GetResultsParams struct {
	JobID string `query:"job_id"`
}

// GetResults returns check results for a subscription (latest completed job by default).
//
//encore:api auth method=GET path=/check/:subscriptionID/results
func GetResults(ctx context.Context, subscriptionID string, p *GetResultsParams) (*ResultsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var job Job
	if p != nil && p.JobID != "" {
		// Specific job requested — verify ownership.
		err := db.QueryRow(ctx, `
			SELECT id, subscription_id, status, total, progress, created_at, finished_at
			FROM check_jobs
			WHERE id=$1 AND subscription_id=$2 AND user_id=$3
		`, p.JobID, subscriptionID, claims.UserID).Scan(
			&job.ID, &job.SubscriptionID, &job.Status,
			&job.Total, &job.Progress, &job.CreatedAt, &job.FinishedAt,
		)
		if err != nil {
			return nil, errs.B().Code(errs.NotFound).Msg("job not found").Err()
		}
	} else {
		// Latest completed job.
		err := db.QueryRow(ctx, `
			SELECT id, subscription_id, status, total, progress, created_at, finished_at
			FROM check_jobs
			WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
			ORDER BY created_at DESC LIMIT 1
		`, subscriptionID, claims.UserID).Scan(
			&job.ID, &job.SubscriptionID, &job.Status,
			&job.Total, &job.Progress, &job.CreatedAt, &job.FinishedAt,
		)
		if err != nil {
			return nil, errs.B().Code(errs.NotFound).Msg("no check jobs found").Err()
		}
	}

	// ... rest of results query unchanged ...
```

- [ ] **Step 5: Run tests**

```bash
encore test ./services/checker/...
```

Expected: all pass. (The existing test updates from Task 3 Step 8 already fixed the broken signatures.)

- [ ] **Step 6: Commit**

```bash
git add services/checker/checker.go services/checker/checker_test.go
git commit -m "feat(checker): add job history endpoint and job_id param to GetResults"
```

---

### Task 5: Notify update endpoint (`services/notify/notify.go`)

**Files:**
- Modify: `services/notify/notify.go`
- Modify: `services/notify/notify_test.go`

- [ ] **Step 1: Write failing test**

Add to `services/notify/notify_test.go`:

```go
func TestUpdateChannel(t *testing.T) {
	ctx := withAuth()
	ch, err := CreateChannel(ctx, &CreateChannelParams{
		Name: "Before",
		Type: "webhook",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "After"
	enabled := false
	updated, err := UpdateChannel(ctx, ch.ID, &UpdateChannelParams{
		Name:    &newName,
		Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "After" {
		t.Errorf("expected name 'After', got %q", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected enabled=false after update")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
encore test ./services/notify/...
```

Expected: compile error — `UpdateChannel`, `UpdateChannelParams` undefined.

- [ ] **Step 3: Implement `UpdateChannel`**

Add to `services/notify/notify.go`:

```go
// UpdateChannelParams is the request body for PUT /notify/channels/:id.
type UpdateChannelParams struct {
	Name    *string         `json:"name"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled"`
}

// UpdateChannel modifies an existing notification channel.
//
//encore:api auth method=PUT path=/notify/channels/:id
func UpdateChannel(ctx context.Context, id string, p *UpdateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	result, err := db.Exec(ctx, `
		UPDATE notify_channels
		SET
			name    = COALESCE($3, name),
			config  = COALESCE($4, config),
			enabled = COALESCE($5, enabled)
		WHERE id=$1 AND user_id=$2
	`, id, claims.UserID, p.Name, nullableJSON(p.Config), p.Enabled)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}
	if result.RowsAffected() == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}

	var c Channel
	if err := db.QueryRow(ctx, `
		SELECT id, user_id, name, type, config, enabled, created_at
		FROM notify_channels WHERE id=$1
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("fetch after update failed").Err()
	}
	return &c, nil
}

// nullableJSON returns nil if the raw message is empty, otherwise the raw bytes.
// Used so that an absent JSON field does not overwrite an existing DB value.
func nullableJSON(r json.RawMessage) []byte {
	if len(r) == 0 {
		return nil
	}
	return []byte(r)
}
```

- [ ] **Step 4: Run tests**

```bash
encore test ./services/notify/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add services/notify/notify.go services/notify/notify_test.go
git commit -m "feat(notify): add PUT /notify/channels/:id update endpoint"
```

---

### Task 6: API key management (`services/settings/settings.go`)

**Files:**
- Modify: `services/settings/settings.go`
- Modify: `services/settings/settings_test.go` (create if missing)

- [ ] **Step 1: Write failing tests**

Create `services/settings/settings_test.go` if it doesn't exist:

```go
package settings

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

func TestGetAPIKeyCreatesOnFirstCall(t *testing.T) {
	ctx := withAuth()
	resp, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if resp.APIKey == "" {
		t.Error("expected non-empty API key")
	}

	// Second call returns same key.
	resp2, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey second call: %v", err)
	}
	if resp2.APIKey != resp.APIKey {
		t.Error("expected same key on second call")
	}
}

func TestRegenerateAPIKey(t *testing.T) {
	ctx := withAuth()
	first, _ := GetAPIKey(ctx)
	second, err := RegenerateAPIKey(ctx)
	if err != nil {
		t.Fatalf("RegenerateAPIKey: %v", err)
	}
	if second.APIKey == first.APIKey {
		t.Error("expected new key after regenerate")
	}
}
```

- [ ] **Step 2: Run to confirm failures**

```bash
encore test ./services/settings/...
```

Expected: compile errors.

- [ ] **Step 3: Implement API key endpoints**

Add to `services/settings/settings.go`:

```go
// APIKeyResponse is returned by GET /settings/api-key and POST /settings/api-key/regenerate.
type APIKeyResponse struct {
	APIKey string `json:"api_key"`
}

// GetAPIKey returns the user's API key, generating one if it doesn't exist yet.
//
//encore:api auth method=GET path=/settings/api-key
func GetAPIKey(ctx context.Context) (*APIKeyResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var key string
	err := db.QueryRow(ctx,
		`SELECT api_key FROM user_settings WHERE user_id=$1`,
		claims.UserID).Scan(&key)

	if err != nil || key == "" {
		key = uuid.New().String()
		if _, err := db.Exec(ctx, `
			INSERT INTO user_settings (user_id, api_key)
			VALUES ($1, $2)
			ON CONFLICT (user_id) DO UPDATE SET api_key = EXCLUDED.api_key
		`, claims.UserID, key); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to store API key").Err()
		}
	}
	return &APIKeyResponse{APIKey: key}, nil
}

// RegenerateAPIKey creates a new API key, invalidating the old one.
//
//encore:api auth method=POST path=/settings/api-key/regenerate
func RegenerateAPIKey(ctx context.Context) (*APIKeyResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	key := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO user_settings (user_id, api_key)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET api_key = EXCLUDED.api_key
	`, claims.UserID, key); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to regenerate API key").Err()
	}
	return &APIKeyResponse{APIKey: key}, nil
}
```

Add the internal lookup used by the export handler:

```go
// UserIDResponse is the response for internal API key lookups.
type UserIDResponse struct {
	UserID string `json:"user_id"`
}

// GetUserIDByAPIKey resolves an API key to a user ID. Used by the checker export endpoint.
//
//encore:api private method=GET path=/internal/settings/api-key/:apiKey
func GetUserIDByAPIKey(ctx context.Context, apiKey string) (*UserIDResponse, error) {
	var userID string
	err := db.QueryRow(ctx,
		`SELECT user_id FROM user_settings WHERE api_key=$1`, apiKey).Scan(&userID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("invalid API key").Err()
	}
	return &UserIDResponse{UserID: userID}, nil
}
```

Add `"github.com/google/uuid"` to imports.

- [ ] **Step 4: Run tests**

```bash
encore test ./services/settings/...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add services/settings/settings.go services/settings/settings_test.go
git commit -m "feat(settings): add API key generation and internal lookup endpoint"
```

---

### Task 7: Export endpoint (`services/checker/export.go`)

**Files:**
- Create: `services/checker/export.go`

**What it does:**
- `GET /export/:subscriptionID?token=<api_key>&target=clash` (public raw handler).
- Validates token via `settingssvc.GetUserIDByAPIKey`.
- Queries latest completed job results (alive nodes only).
- Appends platform tags to node names.
- Renders in `target` format: `clash` (YAML) or `base64` (URI list, base64 encoded).

- [ ] **Step 1: Create `export.go`**

```go
// services/checker/export.go
package checker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	settingssvc "subs-check-re/services/settings"
	"gopkg.in/yaml.v3"
)

// Export generates a subscription link from the latest completed check results.
//
//encore:api public raw method=GET path=/export/:subscriptionID
func Export(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Extract subscriptionID from path: /export/<subscriptionID>
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	subscriptionID := parts[1]

	token := req.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}
	target := req.URL.Query().Get("target")
	if target == "" {
		target = "clash"
	}

	// Resolve token → user_id.
	userResp, err := settingssvc.GetUserIDByAPIKey(ctx, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID := userResp.UserID

	// Find latest completed job for this subscription owned by this user.
	var jobID string
	if err := db.QueryRow(ctx, `
		SELECT id FROM check_jobs
		WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID, userID).Scan(&jobID); err != nil {
		http.Error(w, "no completed check found", http.StatusNotFound)
		return
	}

	// Query alive nodes with their config.
	rows, err := db.Query(ctx, `
		SELECT n.config, n.name,
		       cr.netflix, cr.youtube, cr.openai, cr.claude, cr.gemini, cr.disney, cr.tiktok,
		       cr.speed_kbps, cr.latency_ms
		FROM check_results cr
		JOIN nodes n ON n.id = cr.node_id
		WHERE cr.job_id=$1 AND cr.alive=true
		ORDER BY cr.speed_kbps DESC NULLS LAST, cr.latency_ms ASC NULLS LAST
	`, jobID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type nodeRow struct {
		config  map[string]any
		name    string
		netflix bool
		youtube string
		openai  bool
		claude  bool
		gemini  bool
		disney  bool
		tiktok  string
	}

	var nodes []nodeRow
	for rows.Next() {
		var nr nodeRow
		var configJSON []byte
		if err := rows.Scan(&configJSON, &nr.name,
			&nr.netflix, &nr.youtube, &nr.openai, &nr.claude, &nr.gemini, &nr.disney, &nr.tiktok,
			new(int), new(int)); err != nil {
			continue
		}
		if err := json.Unmarshal(configJSON, &nr.config); err != nil {
			continue
		}
		nodes = append(nodes, nr)
	}

	// Build tagged proxy configs.
	proxies := make([]map[string]any, 0, len(nodes))
	for _, nr := range nodes {
		cfg := make(map[string]any, len(nr.config))
		for k, v := range nr.config {
			cfg[k] = v
		}
		cfg["name"] = taggedName(nr.name, nr.netflix, nr.youtube, nr.openai, nr.claude, nr.gemini, nr.disney, nr.tiktok)
		proxies = append(proxies, cfg)
	}

	switch target {
	case "base64":
		renderBase64(w, proxies)
	default:
		renderClash(w, proxies)
	}
}

func taggedName(name string, netflix bool, youtube string, openai bool, claude bool, gemini bool, disney bool, tiktok string) string {
	var tags []string
	if netflix {
		tags = append(tags, "NF")
	}
	if openai {
		tags = append(tags, "GPT")
	}
	if gemini {
		tags = append(tags, "GM")
	}
	if claude {
		tags = append(tags, "CL")
	}
	if youtube != "" {
		tags = append(tags, "YT-"+youtube)
	}
	if disney {
		tags = append(tags, "D+")
	}
	if tiktok != "" {
		tags = append(tags, "TK-"+tiktok)
	}
	if len(tags) == 0 {
		return name
	}
	return name + "|" + strings.Join(tags, "|")
}

func renderClash(w http.ResponseWriter, proxies []map[string]any) {
	data, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		http.Error(w, "yaml error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(data)
}

func renderBase64(w http.ResponseWriter, proxies []map[string]any) {
	var lines []string
	for _, p := range proxies {
		uri := proxyToURI(p)
		if uri != "" {
			lines = append(lines, uri)
		}
	}
	raw := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, encoded)
}

// proxyToURI converts a mihomo proxy config map to a URI string.
// Supports ss, trojan, vless, vmess. Returns "" for unknown types.
func proxyToURI(p map[string]any) string {
	ptype, _ := p["type"].(string)
	name, _ := p["name"].(string)
	server, _ := p["server"].(string)
	port := fmt.Sprint(p["port"])

	switch ptype {
	case "ss":
		cipher, _ := p["cipher"].(string)
		password, _ := p["password"].(string)
		userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
		return fmt.Sprintf("ss://%s@%s:%s#%s", userinfo, server, port, urlEncode(name))
	case "trojan":
		password, _ := p["password"].(string)
		return fmt.Sprintf("trojan://%s@%s:%s#%s", password, server, port, urlEncode(name))
	case "vless":
		uuid, _ := p["uuid"].(string)
		network, _ := p["network"].(string)
		tls, _ := p["tls"].(bool)
		params := ""
		if network != "" {
			params += "type=" + network
		}
		if tls {
			if params != "" {
				params += "&"
			}
			params += "security=tls"
		}
		if params != "" {
			params = "?" + params
		}
		return fmt.Sprintf("vless://%s@%s:%s%s#%s", uuid, server, port, params, urlEncode(name))
	case "vmess":
		uuid, _ := p["uuid"].(string)
		network, _ := p["network"].(string)
		aid := 0
		if v, ok := p["alterId"].(int); ok {
			aid = v
		}
		vmessObj := map[string]any{
			"v": "2", "ps": name, "add": server, "port": port,
			"id": uuid, "aid": aid, "net": network, "type": "none",
			"host": "", "path": "", "tls": "",
		}
		if tls, ok := p["tls"].(bool); ok && tls {
			vmessObj["tls"] = "tls"
		}
		vmessJSON, _ := json.Marshal(vmessObj)
		return "vmess://" + base64.StdEncoding.EncodeToString(vmessJSON)
	}
	return ""
}

func urlEncode(s string) string {
	return strings.NewReplacer(" ", "%20", "#", "%23", "&", "%26").Replace(s)
}
```

Add `gopkg.in/yaml.v3` — it's already a dependency of the checker service (used by `fetch.go`).

- [ ] **Step 2: Run tests to confirm nothing broke**

```bash
encore test ./services/checker/...
```

Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add services/checker/export.go
git commit -m "feat(checker): add public /export/:subscriptionID endpoint with clash and base64 targets"
```

---

## Chunk 2: Frontend

### Task 8: Update API types (`frontend/apps/web/src/lib/api.ts`)

**Files:**
- Modify: `frontend/apps/web/src/lib/api.ts`

- [ ] **Step 1: Add new types**

Add/update in `api.ts`:

```ts
export interface CheckOptions {
	speed_test: boolean;
	media_apps: string[];
}

// Extended CheckJob with options fields
export interface CheckJob {
	id: string;
	subscription_id: string;
	status: "queued" | "running" | "completed" | "failed";
	total: number;
	progress: number;
	available: number;
	speed_test: boolean;
	media_apps: string[];
	created_at: string;
	finished_at?: string;
}

export interface UserSettings {
	speed_test_url: string;
	api_key?: string;
}
```

- [ ] **Step 2: Type-check**

```bash
cd /Users/ashark/Code/subs-check-re/frontend
bun check-types
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/lib/api.ts
git commit -m "feat(frontend): update API types for check options, available count, api key"
```

---

### Task 9: Check options UI in subscription list (`frontend/apps/web/src/routes/subscriptions/index.tsx`)

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/index.tsx`

**What changes:** The "Check" button opens a small popover/inline panel letting the user toggle speed test and select media platforms before triggering.

- [ ] **Step 1: Replace the inline Check button with a trigger that sends options**

Replace the `triggerMut` definition and the Check button in `SubRow`:

```tsx
const MEDIA_APPS = ["openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"] as const;

// In SubscriptionsPage, change triggerMut:
const triggerMut = useMutation({
	mutationFn: ({ id, opts }: { id: string; opts: { speed_test: boolean; media_apps: string[] } }) =>
		api.post<{ job_id: string }>(`/check/${id}`, opts),
	onSuccess: (resp, { id }) => {
		toast.success("Check started");
		navigate({ to: "/subscriptions/$id", params: { id }, search: { job: resp.job_id } });
	},
	onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed to start check"),
});
```

In `SubRow`, add a `CheckOptions` state and a small inline panel that expands when clicking the check button:

```tsx
function SubRow({
	sub,
	deleteMut,
	triggerMut,
}: {
	sub: Subscription;
	deleteMut: { mutate: (id: string) => void; isPending: boolean };
	triggerMut: { mutate: (args: { id: string; opts: { speed_test: boolean; media_apps: string[] } }) => void; isPending: boolean };
}) {
	const [showOpts, setShowOpts] = useState(false);
	const [speedTest, setSpeedTest] = useState(true);
	const [mediaApps, setMediaApps] = useState<string[]>([...MEDIA_APPS]);

	function toggleApp(app: string) {
		setMediaApps((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);
	}

	function handleCheck() {
		triggerMut.mutate({ id: sub.id, opts: { speed_test: speedTest, media_apps: mediaApps } });
		setShowOpts(false);
	}

	return (
		<div className="rounded-lg border" style={{ background: "#161b22", borderColor: "#30363d" }}>
			<div className="flex items-center gap-3 px-4 py-3">
				{/* ... existing status dot + info ... */}
				<div className="flex flex-shrink-0 items-center gap-2">
					<button
						type="button"
						onClick={() => setShowOpts(!showOpts)}
						className="flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs transition-colors hover:bg-white/5"
						style={{ borderColor: "#30363d", color: "#8b949e" }}
					>
						<Play size={11} strokeWidth={1.5} />
						Check
					</button>
					{/* delete button unchanged */}
				</div>
			</div>

			{showOpts && (
				<div className="border-t px-4 py-3 space-y-3" style={{ borderColor: "#30363d" }}>
					{/* Speed test toggle */}
					<label className="flex items-center gap-2 cursor-pointer select-none">
						<input
							type="checkbox"
							checked={speedTest}
							onChange={(e) => setSpeedTest(e.target.checked)}
							className="accent-[#58a6ff]"
						/>
						<span className="text-xs" style={{ color: "#8b949e" }}>Speed test</span>
					</label>
					{/* Platform checkboxes */}
					<div className="flex flex-wrap gap-2">
						{MEDIA_APPS.map((app) => (
							<label key={app} className="flex items-center gap-1 cursor-pointer select-none">
								<input
									type="checkbox"
									checked={mediaApps.includes(app)}
									onChange={() => toggleApp(app)}
									className="accent-[#58a6ff]"
								/>
								<span className="text-[11px] uppercase" style={{ color: "#8b949e" }}>{app}</span>
							</label>
						))}
					</div>
					<div className="flex gap-2">
						<button
							type="button"
							onClick={handleCheck}
							disabled={triggerMut.isPending}
							className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
							style={{ background: "#238636" }}
						>
							{triggerMut.isPending ? <Loader2 size={13} className="animate-spin" /> : "Start"}
						</button>
						<button type="button" onClick={() => setShowOpts(false)}
							className="rounded-md border px-3 py-1.5 text-sm"
							style={{ borderColor: "#30363d", color: "#8b949e" }}>
							Cancel
						</button>
					</div>
				</div>
			)}
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check-types
```

Expected: no errors.

- [ ] **Step 3: Lint**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check
```

Fix any Biome issues (tab indentation, import ordering).

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/index.tsx
git commit -m "feat(frontend): add check options panel with speed_test and media_apps toggles"
```

---

### Task 10: Job history in subscription detail (`frontend/apps/web/src/routes/subscriptions/$id.tsx`)

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx`

**What changes:**
- Fetch jobs list from `GET /check/:id/jobs`.
- Show a compact row of job history pills (date + available/total) above the progress/results.
- Clicking a pill sets `selectedJobId`; `GET /check/:id/results?job_id=` fetches that job's results.
- No results yet → clean empty state, no table.

- [ ] **Step 1: Add jobs query and job selector UI**

Add `useQueryClient` to the `@tanstack/react-query` import line (it will be used to invalidate the jobs list when a check completes).

Add the jobs query near the top of `SubscriptionDetailPage`:

```tsx
const qc = useQueryClient();
const jobsQuery = useQuery({
	queryKey: ["jobs", id],
	queryFn: () => api.get<{ jobs: CheckJob[]; total: number }>(`/check/${id}/jobs?limit=10`),
	staleTime: 5_000,
});

const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
```

Change `resultsQuery` to send `job_id` when selected:

```tsx
const resultsQuery = useQuery({
	queryKey: ["results", id, selectedJobId],
	queryFn: () => {
		const qs = selectedJobId ? `?job_id=${selectedJobId}` : "";
		return api.get<{ job: CheckJob; results: NodeResult[] }>(`/check/${id}/results${qs}`);
	},
	retry: false,
	staleTime: 0,
});
```

Add a job history pills section (insert above the progress bar section):

```tsx
{/* Job history pills */}
{(jobsQuery.data?.jobs.length ?? 0) > 0 && (
	<div className="flex flex-wrap gap-1.5">
		{jobsQuery.data!.jobs.map((j) => {
			const active = selectedJobId === j.id || (!selectedJobId && j.id === resultsQuery.data?.job.id);
			return (
				<button
					key={j.id}
					type="button"
					onClick={() => setSelectedJobId(j.id)}
					className="rounded-full border px-2.5 py-0.5 text-[11px] font-mono transition-colors"
					style={{
						borderColor: active ? "#58a6ff" : "#30363d",
						color: active ? "#58a6ff" : "#6e7681",
						background: active ? "#1a2a3a" : "transparent",
					}}
				>
					{new Date(j.created_at).toLocaleString(undefined, {
						month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
					})}
					{" · "}
					{j.available}/{j.total}
				</button>
			);
		})}
	</div>
)}
```

Replace the `resultsQuery.isError` block with a proper empty state:

```tsx
{resultsQuery.isError && !resultsQuery.isLoading && (
	<div className="py-12 text-center">
		<p className="text-sm" style={{ color: "#8b949e" }}>No checks run yet.</p>
		<p className="mt-1 text-xs" style={{ color: "#6e7681" }}>
			Click Check on the subscriptions page to start a check.
		</p>
	</div>
)}
```

- [ ] **Step 2: Fix SSE `done` handler to reset job selection and refresh jobs list**

In the existing `useEffect` that sets up the `EventSource` (the one that calls `resultsQuery.refetch()` on `data.done`), also call `setSelectedJobId(null)` and `qc.invalidateQueries({ queryKey: ["jobs", id] })`:

```ts
if (data.done) {
    es.close();
    setSelectedJobId(null);          // reset to latest job
    resultsQuery.refetch();
    qc.invalidateQueries({ queryKey: ["jobs", id] });  // refresh pills
}
```

Add `qc` to the `useEffect` dependency array. Since `qc` is stable (same reference), this does not cause extra re-runs.

- [ ] **Step 3: Type-check and lint**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check-types && bun check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/\$id.tsx
git commit -m "feat(frontend): add job history pill selector on subscription detail page"
```

---

### Task 11: Notify edit UI (`frontend/apps/web/src/routes/settings/notify.tsx`)

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/notify.tsx`

**What changes:** Add an edit button per channel that expands an inline edit form (same fields as create, pre-populated).

- [ ] **Step 1: Add update mutation and edit form**

Add the `updateMut` alongside existing mutations:

```tsx
const updateMut = useMutation({
	mutationFn: ({ id, data }: { id: string; data: { name?: string; enabled?: boolean } }) =>
		api.put(`/notify/channels/${id}`, data),
	onSuccess: () => {
		qc.invalidateQueries({ queryKey: ["notify-channels"] });
		toast.success("Updated");
	},
	onError: (e) => toast.error(e instanceof ApiError ? e.message : "Update failed"),
});
```

In the channel row, add an edit button (pencil icon) and an inline edit form. Use a `editingId` state at the page level. When `editingId === ch.id`, show a small inline form with name input and enabled toggle.

Add `import { Pencil } from "lucide-react"` to imports.

- [ ] **Step 2: Type-check and lint**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check-types && bun check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/settings/notify.tsx
git commit -m "feat(frontend): add inline edit form for notification channels"
```

---

### Task 12: Dashboard API documentation (`frontend/apps/web/src/routes/index.tsx`)

**Files:**
- Modify: `frontend/apps/web/src/routes/index.tsx`

**What changes:** Below the stat cards, add an "Export API" section showing:
- API key (masked) with copy and regenerate buttons.
- Per-subscription export URLs (clash + base64 copy buttons).
- Parameter reference table.

- [ ] **Step 1: Add imports and queries**

Add `useMutation`, `useQueryClient` to the `@tanstack/react-query` import. Add `const qc = useQueryClient()` inside `DashboardPage` alongside the existing `const { data, isLoading }` query.

Add queries:

```tsx
const apiKeyQuery = useQuery({
	queryKey: ["api-key"],
	queryFn: () => api.get<{ api_key: string }>("/settings/api-key"),
	staleTime: Infinity,
});

const apiKey = apiKeyQuery.data?.api_key ?? "";
const origin = typeof window !== "undefined" ? window.location.origin : "";
```

Add regenerate mutation:

```tsx
const regenerateMut = useMutation({
	mutationFn: () => api.post<{ api_key: string }>("/settings/api-key/regenerate"),
	onSuccess: () => {
		qc.invalidateQueries({ queryKey: ["api-key"] });
		toast.success("API key regenerated");
	},
});
```

After the stat cards, add the Export API section:

```tsx
{/* Export API */}
<div className="space-y-4">
	<div>
		<h2 className="font-semibold text-[#f0f6fc] text-sm">Export API</h2>
		<p className="mt-0.5 text-xs" style={{ color: "#8b949e" }}>
			Use these URLs as subscription links directly in your proxy client.
		</p>
	</div>

	{/* API Key */}
	<div className="rounded-lg border p-4 space-y-2" style={{ background: "#161b22", borderColor: "#30363d" }}>
		<p className="text-xs font-medium" style={{ color: "#8b949e" }}>API Key</p>
		<div className="flex items-center gap-2">
			<code className="flex-1 rounded bg-[#0d1117] px-3 py-1.5 font-mono text-xs text-[#f0f6fc] truncate">
				{apiKey || "—"}
			</code>
			<button
				type="button"
				onClick={() => { navigator.clipboard.writeText(apiKey); toast.success("Copied"); }}
				className="rounded-md border px-2.5 py-1.5 text-xs transition-colors hover:bg-white/5"
				style={{ borderColor: "#30363d", color: "#8b949e" }}
			>Copy</button>
			<button
				type="button"
				onClick={() => regenerateMut.mutate()}
				disabled={regenerateMut.isPending}
				className="rounded-md border px-2.5 py-1.5 text-xs transition-colors hover:bg-white/5 disabled:opacity-50"
				style={{ borderColor: "#30363d", color: "#8b949e" }}
			>Regenerate</button>
		</div>
	</div>

	{/* Per-subscription URLs */}
	{subs.length > 0 && apiKey && (
		<div className="space-y-2">
			{subs.map((sub) => {
				const base = `${origin}/api/export/${sub.id}?token=${apiKey}`;
				return (
					<div key={sub.id} className="rounded-lg border p-4 space-y-2"
						style={{ background: "#161b22", borderColor: "#30363d" }}>
						<p className="font-medium text-[#f0f6fc] text-sm">{sub.name || sub.url}</p>
						<div className="flex flex-col gap-1.5">
							{(["clash", "base64"] as const).map((t) => (
								<div key={t} className="flex items-center gap-2">
									<code className="flex-1 truncate rounded bg-[#0d1117] px-2 py-1 font-mono text-[11px] text-[#8b949e]">
										{base}&target={t}
									</code>
									<button
										type="button"
										onClick={() => { navigator.clipboard.writeText(`${base}&target=${t}`); toast.success("Copied"); }}
										className="flex-shrink-0 rounded border px-2 py-1 text-[11px] hover:bg-white/5"
										style={{ borderColor: "#30363d", color: "#6e7681" }}
									>{t}</button>
								</div>
							))}
						</div>
					</div>
				);
			})}
		</div>
	)}

	{/* Parameter reference */}
	<div className="rounded-lg border p-4" style={{ background: "#161b22", borderColor: "#30363d" }}>
		<p className="mb-2 text-xs font-medium" style={{ color: "#8b949e" }}>Parameters</p>
		<table className="w-full text-xs" style={{ color: "#8b949e" }}>
			<tbody>
				<tr><td className="pr-4 py-0.5 font-mono text-[#58a6ff]">token</td><td>Your API key (required)</td></tr>
				<tr><td className="pr-4 py-0.5 font-mono text-[#58a6ff]">target</td><td><code>clash</code> (default) — Clash YAML · <code>base64</code> — base64-encoded URI list</td></tr>
			</tbody>
		</table>
	</div>
</div>
```

(Imports added in Step 1 above.)

- [ ] **Step 2: Type-check and lint**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check-types && bun check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/index.tsx
git commit -m "feat(frontend): add Export API section to dashboard with per-subscription links"
```

---

## Final verification

- [ ] **Start backend and frontend, do a smoke test**

```bash
# Terminal 1
cd /Users/ashark/Code/subs-check-re && encore run

# Terminal 2
cd /Users/ashark/Code/subs-check-re/frontend && bun dev
```

Visit http://localhost:3001 and verify:
1. Dashboard shows API key and export links after loading.
2. Subscription list shows options panel when clicking Check.
3. Subscription detail shows job history pills (if any jobs exist).
4. Settings → Notify shows edit button per channel.

- [ ] **Run all backend tests**

```bash
cd /Users/ashark/Code/subs-check-re && encore test ./services/...
```

Expected: all pass.
