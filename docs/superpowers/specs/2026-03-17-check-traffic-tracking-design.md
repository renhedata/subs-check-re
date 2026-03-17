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

In `newProxyContext()` (or wherever the `http.Transport` is constructed), wrap it:

```go
ct := &countingTransport{base: transport}
// use ct as the RoundTripper for all requests on this node
```

In `checkNode()`, after all requests complete, read `ct.bytes` and include it in the returned `nodeCheckResult`.

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

When inserting each `check_result`, write `traffic_bytes`. After each node result, atomically add to a job-level counter. Write `total_traffic_bytes` to `check_jobs` on job completion (alongside the existing `available` count update).

## Database Schema

### Migration 10: `check_results` per-node traffic

```sql
-- 10_add_traffic_bytes.up.sql
ALTER TABLE check_results ADD COLUMN traffic_bytes BIGINT NOT NULL DEFAULT 0;
```

### Migration 11: `check_jobs` job-level total

```sql
-- 11_add_total_traffic_bytes_to_jobs.up.sql
ALTER TABLE check_jobs ADD COLUMN total_traffic_bytes BIGINT NOT NULL DEFAULT 0;
```

Down migrations drop the respective columns.

## Frontend Changes

### `node-table.tsx`

Add a "流量" column after the speed column. Format bytes as human-readable:

```typescript
function formatBytes(b: number): string {
    if (b === 0) return "—";
    if (b < 1024) return `${b} B`;
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
    return `${(b / (1024 * 1024)).toFixed(2)} MB`;
}
```

### `subscriptions/$id.tsx`

In the job history list and/or job detail header, display the job's `total_traffic_bytes` using the same `formatBytes` helper.

## Type Naming

The generated client (`client.gen.ts`) will pick up the new fields automatically after `encore gen client` is re-run. Response fields are snake_case (`traffic_bytes`, `total_traffic_bytes`).

## Regeneration

After backend changes are deployed, regenerate the client:
```bash
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts
```

## Out of Scope

- Upload bytes (requests sent to proxy) — response body bytes only
- Per-platform traffic breakdown
- Historical traffic aggregation or charts
