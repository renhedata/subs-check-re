# Check Traffic Tracking Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record bytes consumed through each proxy node during a check job, store per-node and per-job, and display in the UI.

**Architecture:** Wrap each node's HTTP transport with a `countingTransport` that accumulates response body bytes. Store per-node bytes in `check_results.traffic_bytes` and aggregate into `check_jobs.total_traffic_bytes`. Surface both fields through the API and render them in the frontend.

**Tech Stack:** Go, Encore framework, PostgreSQL, React 19, TanStack Query, Encore generated TypeScript client

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `services/checker/migrations/12_add_traffic_bytes_to_results.up.sql` | Add `traffic_bytes` to `check_results` |
| Create | `services/checker/migrations/12_add_traffic_bytes_to_results.down.sql` | Drop `traffic_bytes` from `check_results` |
| Create | `services/checker/migrations/13_add_total_traffic_bytes_to_jobs.up.sql` | Add `total_traffic_bytes` to `check_jobs` |
| Create | `services/checker/migrations/13_add_total_traffic_bytes_to_jobs.down.sql` | Drop `total_traffic_bytes` from `check_jobs` |
| Modify | `services/checker/mihomo.go` | Add `countingTransport`, `countingReader`, wire into `proxyClient`, add `TrafficBytes` to `nodeCheckResult` |
| Modify | `services/checker/checker_test.go` | Add `TestCountingTransport` |
| Modify | `services/checker/checker.go` | Add fields to `NodeResult`, `Job`, `JobSummary`; update INSERT, completion UPDATE, `GetResults` and `ListJobs` queries |
| Create | `frontend/apps/web/src/lib/format.ts` | Shared `formatBytes` helper |
| Modify | `frontend/apps/web/src/components/node-table.tsx` | Add "流量" column |
| Modify | `frontend/apps/web/src/routes/subscriptions/$id.tsx` | Show `total_traffic_bytes` in job pills and job summary |
| Modify | `frontend/apps/web/src/lib/client.gen.ts` | Regenerate via `encore gen client` |

---

## Chunk 1: Database Migrations

### Task 1: Create migration files

**Files:**
- Create: `services/checker/migrations/12_add_traffic_bytes_to_results.up.sql`
- Create: `services/checker/migrations/12_add_traffic_bytes_to_results.down.sql`
- Create: `services/checker/migrations/13_add_total_traffic_bytes_to_jobs.up.sql`
- Create: `services/checker/migrations/13_add_total_traffic_bytes_to_jobs.down.sql`

- [ ] **Step 1: Create migration 12 up**

```sql
-- services/checker/migrations/12_add_traffic_bytes_to_results.up.sql
ALTER TABLE check_results ADD COLUMN traffic_bytes BIGINT NOT NULL DEFAULT 0;
```

- [ ] **Step 2: Create migration 12 down**

```sql
-- services/checker/migrations/12_add_traffic_bytes_to_results.down.sql
ALTER TABLE check_results DROP COLUMN traffic_bytes;
```

- [ ] **Step 3: Create migration 13 up**

```sql
-- services/checker/migrations/13_add_total_traffic_bytes_to_jobs.up.sql
ALTER TABLE check_jobs ADD COLUMN total_traffic_bytes BIGINT NOT NULL DEFAULT 0;
```

- [ ] **Step 4: Create migration 13 down**

```sql
-- services/checker/migrations/13_add_total_traffic_bytes_to_jobs.down.sql
ALTER TABLE check_jobs DROP COLUMN total_traffic_bytes;
```

- [ ] **Step 5: Verify `encore run` starts without migration errors**

```bash
cd /Users/ashark/Code/subs-check-re && encore run &
sleep 5 && kill %1
```

Expected: server starts, no migration failure logs.

- [ ] **Step 6: Commit**

```bash
git add services/checker/migrations/
git commit -m "feat(checker): add traffic_bytes columns (migrations 12+13)"
```

---

## Chunk 2: Backend — countingTransport

### Task 2: Add counting transport and wire into proxyClient

**Files:**
- Modify: `services/checker/mihomo.go`
- Modify: `services/checker/checker_test.go`

**Context:** `mihomo.go` defines `proxyClient`, `newProxyClient()`, `nodeCheckResult`, and `checkNode()`. The current `proxyClient` struct is:
```go
type proxyClient struct {
    *http.Client
    proxy constant.Proxy
}
```
All HTTP requests for a node go through `pc.Client`. `checkNode()` calls `isAlive`, `measureLatency`, `measureSpeed`, `getProxyInfo`, and platform checks — all using `pc.Client` or `pc.Client.Transport`.

- [ ] **Step 1: Write failing test for countingTransport**

Add to `services/checker/checker_test.go`:

```go
func TestCountingTransport(t *testing.T) {
	body := "hello world" // 11 bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	ct := &countingTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: ct}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	got := atomic.LoadInt64(&ct.bytes)
	if got != int64(len(body)) {
		t.Errorf("want %d bytes, got %d", len(body), got)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd /Users/ashark/Code/subs-check-re && encore test -run TestCountingTransport ./services/checker/...
```

Expected: FAIL — `countingTransport` undefined.

- [ ] **Step 3: Add countingTransport and countingReader to mihomo.go**

Add after the existing imports block (add `"sync/atomic"` to imports if not already present):

```go
// countingTransport wraps an http.RoundTripper and counts response body bytes read.
type countingTransport struct {
	base  http.RoundTripper
	bytes int64
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	resp.Body = &countingReader{ReadCloser: resp.Body, n: &t.bytes}
	return resp, nil
}

type countingReader struct {
	io.ReadCloser
	n *int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	atomic.AddInt64(r.n, int64(n))
	return n, err
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/ashark/Code/subs-check-re && encore test -run TestCountingTransport ./services/checker/...
```

Expected: PASS.

- [ ] **Step 5: Update proxyClient struct to embed counter**

Change:
```go
type proxyClient struct {
	*http.Client
	proxy constant.Proxy
}
```
To:
```go
type proxyClient struct {
	*http.Client
	proxy   constant.Proxy
	counter *countingTransport
}
```

- [ ] **Step 6: Update newProxyClient to wrap transport with countingTransport**

Change the return statement in `newProxyClient()` from:
```go
return &proxyClient{
	Client: &http.Client{
		Timeout:   proxyTimeout,
		Transport: transport,
	},
	proxy: proxy,
}
```
To:
```go
ct := &countingTransport{base: transport}
return &proxyClient{
	Client: &http.Client{
		Timeout:   proxyTimeout,
		Transport: ct,
	},
	proxy:   proxy,
	counter: ct,
}
```

- [ ] **Step 7: Add TrafficBytes to nodeCheckResult**

Add field to `nodeCheckResult` struct (after `TikTok bool`):
```go
TrafficBytes int64
```

- [ ] **Step 8: Read counter at end of checkNode**

At the very end of `checkNode()`, just before `return result`:
```go
if pc.counter != nil {
	result.TrafficBytes = atomic.LoadInt64(&pc.counter.bytes)
}
```

- [ ] **Step 9: Verify compile**

```bash
cd services/checker && go build ./...
```

Expected: 0 errors.

- [ ] **Step 10: Run all checker tests**

```bash
cd services/checker && encore test ./...
```

Expected: all existing tests pass + `TestCountingTransport` passes.

- [ ] **Step 11: Commit**

```bash
git add services/checker/mihomo.go services/checker/checker_test.go
git commit -m "feat(checker): add countingTransport to track bytes per node"
```

---

## Chunk 3: Backend — checker.go wiring

### Task 3: Wire traffic bytes through structs, DB writes, and queries

**Files:**
- Modify: `services/checker/checker.go`

**Context:** Four changes needed in `checker.go`:
1. Add `TrafficBytes int64` to `NodeResult` (line ~70)
2. Add `TotalTrafficBytes int64` to `JobSummary` (line ~91) and `Job` (line ~53)
3. Extend the `INSERT INTO check_results` statement to include `traffic_bytes` (line ~710)
4. Add `totalTrafficBytes atomic.Int64` accumulator in `runJob()` and write it on completion (line ~752)
5. Update `GetResults()` CTE SELECT and `rows.Scan` to include `cr.traffic_bytes` (line ~529)
6. Update `ListJobs()` SELECT and `rows.Scan` to include `total_traffic_bytes` (line ~458)

- [ ] **Step 1: Add TrafficBytes to NodeResult struct**

In `checker.go`, find the `NodeResult` struct (around line 64). Add after `TikTok bool`:
```go
TrafficBytes int64 `json:"traffic_bytes"`
```

- [ ] **Step 2: Add TotalTrafficBytes to Job and JobSummary structs**

In the `Job` struct (around line 53), add after `FinishedAt`:
```go
TotalTrafficBytes int64 `json:"total_traffic_bytes"`
```

In the `JobSummary` struct (around line 91), add after `FinishedAt`:
```go
TotalTrafficBytes int64 `json:"total_traffic_bytes"`
```

- [ ] **Step 3: Extend INSERT INTO check_results**

Find the `INSERT INTO check_results` in `runJob()` (around line 710). Change:
```go
db.Exec(context.Background(), `
    INSERT INTO check_results
      (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip,
       netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
`, resultID, jobID, t.nodeID, nodeName, nodeType, nodeConfigJSON, time.Now(),
    res.Alive, res.LatencyMs, res.SpeedKbps, res.Country, res.IP,
    res.Netflix, res.YouTube, res.YouTubePremium, res.OpenAI,
    res.Claude, res.Gemini, res.Grok, res.Disney, res.TikTok,
)
```
To:
```go
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
```

- [ ] **Step 4: Add totalTrafficBytes accumulator in runJob()**

After `var processedCount atomic.Int64` (around line 694), add:
```go
var totalTrafficBytes atomic.Int64
```

Inside the worker goroutine, after the `INSERT INTO check_results` call (and before or after `processedCount.Add(1)`), add:
```go
totalTrafficBytes.Add(res.TrafficBytes)
```

- [ ] **Step 5: Write total_traffic_bytes on job completion**

Find the completion `UPDATE check_jobs` (around line 752):
```go
db.Exec(context.Background(), `UPDATE check_jobs SET status='completed', finished_at=$2, available=$3 WHERE id=$1`,
    jobID, time.Now(), available)
```
Change to:
```go
db.Exec(context.Background(), `UPDATE check_jobs SET status='completed', finished_at=$2, available=$3, total_traffic_bytes=$4 WHERE id=$1`,
    jobID, time.Now(), available, totalTrafficBytes.Load())
```

- [ ] **Step 6: Update GetResults() to select and scan traffic_bytes**

Find the CTE query in `GetResults()` (around line 529). The current CTE column list ends with `...cr.tiktok`. Add `cr.traffic_bytes` to the CTE's SELECT list:

Change the end of the CTE from:
```sql
           cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok
    FROM check_results cr
```
To:
```sql
           cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
           cr.traffic_bytes
    FROM check_results cr
```

Find the corresponding `rows.Scan` call (around line 564). Change:
```go
if err := rows.Scan(
    &r.NodeID, &r.NodeName, &r.NodeType,
    &r.Alive, &r.LatencyMs, &r.SpeedKbps, &r.Country, &r.IP,
    &r.Netflix, &r.YouTube, &r.YouTubePremium, &r.OpenAI, &r.Claude, &r.Gemini, &r.Grok, &r.Disney, &r.TikTok,
); err != nil {
```
To:
```go
if err := rows.Scan(
    &r.NodeID, &r.NodeName, &r.NodeType,
    &r.Alive, &r.LatencyMs, &r.SpeedKbps, &r.Country, &r.IP,
    &r.Netflix, &r.YouTube, &r.YouTubePremium, &r.OpenAI, &r.Claude, &r.Gemini, &r.Grok, &r.Disney, &r.TikTok,
    &r.TrafficBytes,
); err != nil {
```

- [ ] **Step 7: Update GetResults() job query to include total_traffic_bytes**

Both `SELECT` statements in `GetResults()` that read `check_jobs` (the specific-job query and the latest-completed query) currently end with `...created_at, finished_at`. Extend both to add `total_traffic_bytes`:

```sql
-- old (appears twice)
SELECT id, subscription_id, status, total, progress, created_at, finished_at
FROM check_jobs WHERE ...

-- new (apply to both)
SELECT id, subscription_id, status, total, progress, total_traffic_bytes, created_at, finished_at
FROM check_jobs WHERE ...
```

And update both `Scan` calls (around lines 505 and 519) from:
```go
&job.ID, &job.SubscriptionID, &job.Status,
&job.Total, &job.Progress, &job.CreatedAt, &job.FinishedAt,
```
To:
```go
&job.ID, &job.SubscriptionID, &job.Status,
&job.Total, &job.Progress, &job.TotalTrafficBytes, &job.CreatedAt, &job.FinishedAt,
```

- [ ] **Step 8: Update ListJobs() to select and scan total_traffic_bytes**

Find the `ListJobs()` query (around line 458). The SELECT currently reads:
```sql
SELECT id, subscription_id, status, total, available,
       COALESCE(options_json, '{}'), created_at, finished_at
FROM check_jobs
```
Change to:
```sql
SELECT id, subscription_id, status, total, available,
       COALESCE(options_json, '{}'), total_traffic_bytes, created_at, finished_at
FROM check_jobs
```

Update the `rows.Scan` call (around line 475) from:
```go
if err := rows.Scan(&j.ID, &j.SubscriptionID, &j.Status, &j.Total, &j.Available,
    &optsJSON, &j.CreatedAt, &j.FinishedAt); err != nil {
```
To:
```go
if err := rows.Scan(&j.ID, &j.SubscriptionID, &j.Status, &j.Total, &j.Available,
    &optsJSON, &j.TotalTrafficBytes, &j.CreatedAt, &j.FinishedAt); err != nil {
```

- [ ] **Step 9: Verify compile**

```bash
cd /Users/ashark/Code/subs-check-re/services/checker && go build ./...
```

Expected: 0 errors.

- [ ] **Step 10: Run all tests**

```bash
cd /Users/ashark/Code/subs-check-re && encore test ./services/checker/...
```

Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
git add services/checker/checker.go
git commit -m "feat(checker): wire traffic_bytes into structs, DB writes, and queries"
```

---

## Chunk 4: Frontend

### Task 4: Regenerate client and update UI

**Files:**
- Modify: `frontend/apps/web/src/lib/client.gen.ts` (regenerated)
- Create: `frontend/apps/web/src/lib/format.ts`
- Modify: `frontend/apps/web/src/components/node-table.tsx`
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx`

**Context:**
- `node-table.tsx` renders a `<table>` with columns: `["Node", "Status", "Latency", "Speed", "Country", "Unlocks"]`. The speed column is around line 122. Add a "流量" column after "Speed".
- `$id.tsx` shows job history pills (around line 178) that display `{j.available}/{j.total}`. Add `· formatBytes(j.total_traffic_bytes)` after this.
- `$id.tsx` also has a job summary section (around line 311) showing alive/total count. Add total traffic there too.
- The generated client already has snake_case field names matching the JSON tags.

- [ ] **Step 1: Regenerate client.gen.ts**

```bash
cd /Users/ashark/Code/subs-check-re
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts
```

Expected: `client.gen.ts` updated with `traffic_bytes` on `NodeResult` and `total_traffic_bytes` on `JobSummary` / `Job`.

Verify the new fields appear:
```bash
grep "traffic_bytes" frontend/apps/web/src/lib/client.gen.ts
```
Expected: at least 2 matches.

- [ ] **Step 2: Create format.ts**

Create `frontend/apps/web/src/lib/format.ts`:

```typescript
// frontend/apps/web/src/lib/format.ts
export function formatBytes(b: number): string {
	if (b === 0) return "—";
	if (b < 1024) return `${b} B`;
	if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
	return `${(b / (1024 * 1024)).toFixed(2)} MB`;
}
```

- [ ] **Step 3: Add 流量 column to node-table.tsx**

In `node-table.tsx`:

**3a.** Add import at top:
```typescript
import { formatBytes } from "@/lib/format";
```

**3b.** In the column headers array (around line 84), change:
```typescript
{["Node", "Status", "Latency", "Speed", "Country", "Unlocks"].map(
```
To:
```typescript
{["Node", "Status", "Latency", "Speed", "流量", "Country", "Unlocks"].map(
```

**3c.** After the speed `<td>` (around line 122–131), add a new `<td>` for traffic:
```tsx
<td className="px-3 py-2 text-xs" style={{ color: "#8b949e" }}>
    {formatBytes(r.traffic_bytes)}
</td>
```

- [ ] **Step 4: Show total_traffic_bytes in $id.tsx job pills**

In `$id.tsx`:

**4a.** Add import at top of file (alongside other imports):
```typescript
import { formatBytes } from "@/lib/format";
```

**4b.** In the job history pill (around line 201), change:
```tsx
{j.available}/{j.total}
```
To:
```tsx
{j.available}/{j.total}{j.total_traffic_bytes > 0 ? ` · ${formatBytes(j.total_traffic_bytes)}` : ""}
```

- [ ] **Step 5: Show total traffic in job summary section**

In `$id.tsx`, find the job summary section (around line 314):
```tsx
<span style={{ color: "#8b949e" }}>
    {results.filter((r) => r.alive).length} / {job.total} alive
</span>
```

Add a traffic span after it:
```tsx
{job.total_traffic_bytes > 0 && (
    <span style={{ color: "#6e7681" }}>
        · {formatBytes(job.total_traffic_bytes)}
    </span>
)}
```

- [ ] **Step 6: Type-check**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check-types 2>&1 | grep -v "@types/node"
```

Expected: 0 errors (the `@types/node` error in `@frontend/ui` is pre-existing, ignore it).

- [ ] **Step 7: Lint**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun check
```

Expected: passes (auto-fixes applied).

- [ ] **Step 8: Build**

```bash
cd /Users/ashark/Code/subs-check-re/frontend && bun build
```

Expected: build succeeds.

- [ ] **Step 9: Commit**

```bash
git add frontend/apps/web/src/lib/client.gen.ts \
        frontend/apps/web/src/lib/format.ts \
        frontend/apps/web/src/components/node-table.tsx \
        "frontend/apps/web/src/routes/subscriptions/\$id.tsx"
git commit -m "feat(frontend): show per-node and per-job traffic in UI"
```
