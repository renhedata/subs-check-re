# Check Traffic Tracking Design

**Date:** 2026-03-17
**Status:** Approved

## Goal

Record the actual bytes consumed through each proxy node during a check job — per node and aggregated per job — and display them in the UI.

## Context

Each check job tests N nodes concurrently (20 workers). Per node, the following HTTP requests are made through the proxy:

- Alive check — tiny request/response (~0.5 KB)
- Speed test — downloads a fixed 200 KB file (optional, controlled by `speed_test` option)
- IP/Country lookup — small JSON response (~0.3 KB)
- Platform checks (Netflix, YouTube, OpenAI, etc.) — small responses (~0.5–1 KB each)

Currently, bytes downloaded during the speed test are counted in a local variable `n` inside `measureSpeed()` but discarded. No traffic tracking exists at any level.

## Approach: Counting Transport Wrapper

Wrap each node's `http.Transport` with a `countingTransport` that intercepts `RoundTrip` and wraps every response `Body` in a `countingReader`. All bytes flowing through the proxy for that node (alive, speed test, IP lookup, platform checks) are accumulated in a single `int64` counter.

This is accurate (measures actual bytes read, not estimated), transparent to callers, and requires no changes to individual request code.

## Backend Changes

### `services/checker/mihomo.go`

Add two types alongside `proxyContext`:

```go
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

**Wiring into `proxyClient`:** Embed `*countingTransport` in the `proxyClient` struct so `checkNode()` can read the counter after all requests complete:

```go
type proxyClient struct {
    *http.Client
    counter *countingTransport  // NEW: exposes accumulated bytes
    // ... existing fields
}
```

In `newProxyContext()`, wrap the existing transport before assigning it to `http.Client`:

```go
ct := &countingTransport{base: transport}
pc := &proxyClient{
    Client:  &http.Client{Transport: ct, ...},
    counter: ct,
}
```

In `checkNode()`, after all requests complete, read `pc.counter.bytes`:

```go
result.TrafficBytes = atomic.LoadInt64(&pc.counter.bytes)
```

Add field to `nodeCheckResult`:
```go
TrafficBytes int64
```

### `services/checker/checker.go`

**`NodeResult` struct** — add field:
```go
TrafficBytes int64 `json:"traffic_bytes"`
```

**`JobSummary` and `Job` structs** — add field:
```go
TotalTrafficBytes int64 `json:"total_traffic_bytes"`
```

**Job-level accumulation:** Mirror the existing `atomic.Int64` pattern used for `processedCount` — add a `totalTrafficBytes atomic.Int64` in `runJob()`. After each node result is written, add `result.TrafficBytes` to it.

**Final job update:** Extend the completion `UPDATE check_jobs` statement to include `total_traffic_bytes`:

```go
db.Exec(ctx,
    `UPDATE check_jobs SET status='completed', finished_at=$2, available=$3, total_traffic_bytes=$4 WHERE id=$1`,
    jobID, finishedAt, available, totalTrafficBytes.Load(),
)
```

**`GetResults()` query update:** The CTE SELECT and corresponding `rows.Scan()` must include `cr.traffic_bytes` for the new `NodeResult.TrafficBytes` field. Add `cr.traffic_bytes` to the column list and `&r.TrafficBytes` to the Scan call.

**`ListJobs()` query update:** The SELECT and Scan for `check_jobs` must include `total_traffic_bytes`. Add the column to the SELECT list and `&j.TotalTrafficBytes` to the Scan call.

## Database Schema

### Migration 12: `check_results` per-node traffic

```sql
-- 12_add_traffic_bytes_to_results.up.sql
ALTER TABLE check_results ADD COLUMN traffic_bytes BIGINT NOT NULL DEFAULT 0;

-- 12_add_traffic_bytes_to_results.down.sql
ALTER TABLE check_results DROP COLUMN traffic_bytes;
```

### Migration 13: `check_jobs` job-level total

```sql
-- 13_add_total_traffic_bytes_to_jobs.up.sql
ALTER TABLE check_jobs ADD COLUMN total_traffic_bytes BIGINT NOT NULL DEFAULT 0;

-- 13_add_total_traffic_bytes_to_jobs.down.sql
ALTER TABLE check_jobs DROP COLUMN total_traffic_bytes;
```

## Frontend Changes

### `src/lib/format.ts` (new shared helper)

Create a shared `formatBytes` utility importable by both components:

```typescript
export function formatBytes(b: number): string {
    if (b === 0) return "—";
    if (b < 1024) return `${b} B`;
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
    return `${(b / (1024 * 1024)).toFixed(2)} MB`;
}
```

### `node-table.tsx`

Add a "流量" column after the speed column. Import `formatBytes` from `@/lib/format` and render `formatBytes(result.traffic_bytes)`.

### `subscriptions/$id.tsx`

In the job history list and/or job detail header, display the job's `total_traffic_bytes` using `formatBytes` from `@/lib/format`.

## Type Naming

The generated client (`client.gen.ts`) will pick up the new fields automatically after `encore gen client` is re-run. Response fields are snake_case (`traffic_bytes`, `total_traffic_bytes`).

## Regeneration

After backend changes are complete, regenerate the frontend client:
```bash
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts
```

## Out of Scope

- Upload bytes (requests sent to proxy) — response body bytes only
- Per-platform traffic breakdown
- Historical traffic aggregation or charts
