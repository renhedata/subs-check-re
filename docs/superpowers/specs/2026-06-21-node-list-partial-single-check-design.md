# Node List as Primary Surface + Whole / Partial / Single Checks

- **Date:** 2026-06-21
- **Status:** Approved (brainstorming)
- **Topic:** Decouple node display from check jobs; allow checking all / a subset / a single node.

## Problem

Fetching and checking are already decoupled at the data layer (nodes are persisted; a
check tests existing nodes and never re-fetches). But two gaps remain:

1. **No way to display fetched nodes before a check.** Node info is only readable through
   `GetResults`, which requires a *completed* check job. After import/refresh, the user
   sees nothing until they run a full check.
2. **Checks are whole-subscription only.** `TriggerCheck` / `runJob` iterate *all* proxies.
   There is no way to re-check a single node or a chosen subset.

## Goals

- After import/refresh, the node list shows immediately (metrics blank until first check).
- Run a check over **all** nodes, a **selected subset**, or a **single** node.
- A partial/single check updates only the checked nodes; every other node keeps its last
  result ("checking a does not affect b" — now at the node level, mirroring the existing
  per-platform inheritance).

## Non-Goals

- No change to fetch-via-node, import, refresh, or test-fetch (shipped previously).
- No lightweight/concurrent single-node channel: single & partial checks reuse the normal
  job system and the existing "one active job per subscription" constraint.
- No rewrite of the existing `GetResults` per-job inheritance SQL.

## Decisions (from brainstorming)

1. **Display model:** the **node list** (latest-known value per node) is the primary
   surface. A subset check only updates checked nodes; others are untouched. We do *not*
   carry forward unchecked nodes' prior rows into the new job.
2. **Selection UX:** per-row checkboxes + a top **"Check selected (N)"** button; a per-row
   **"Check this node"** action; the existing top **"Run check"** = all nodes.
3. **Concurrency:** single/partial checks are ordinary jobs; the one-active-job-per-
   subscription rule (409 on conflict) still applies.
4. **Node selection storage:** add a dedicated `node_ids` column to `check_jobs` (clean
   separation from `options_json`, which means "which tests"; `node_ids` means "which
   nodes"). One migration.

## Architecture

### Backend (checker service)

#### 1. `ListNodes` — `GET /subscription/:subscriptionID/nodes`

Returns every persisted node enriched with its latest-known result.

- **Ownership:** verified via `subsvc.GetSubscription(ctx, subscriptionID)` (a freshly
  imported subscription may have zero `check_jobs`, so the `check_jobs`-based ownership
  check used elsewhere is insufficient here).
- **Latest-known semantics (per node identity `server:port`, same key as `GetResults`):**
  - `alive`, `latency_ms`, `ip` come from the node's *most recent* result row.
  - `speed_kbps`, `upload_speed_kbps`, `country` inherit the latest *non-empty* value.
  - `platforms` inherit per key (latest non-empty value of each platform key).
  - `last_checked_at` = the most recent result row's `checked_at`, or `null` if the node
    has never been checked.
- Implemented as one dedicated read-only query (a node `LEFT JOIN` against per-identity
  latest results). The existing `GetResults` SQL is left untouched.

Response shape:

```go
type Node struct {
    NodeID          string                     `json:"node_id"`
    NodeName        string                     `json:"node_name"`
    NodeType        string                     `json:"node_type"`
    Enabled         bool                       `json:"enabled"`
    Alive           bool                       `json:"alive"`
    LatencyMs       int                        `json:"latency_ms"`
    SpeedKbps       int                        `json:"speed_kbps"`
    UploadSpeedKbps int                        `json:"upload_speed_kbps"`
    Country         string                     `json:"country"`
    IP              string                     `json:"ip"`
    Server          string                     `json:"server"`
    Port            int                        `json:"port"`
    Config          string                     `json:"config"`
    Platforms       map[string]PlatformOutcome `json:"platforms"`
    TrafficBytes    int64                      `json:"traffic_bytes"`
    LastCheckedAt   *time.Time                 `json:"last_checked_at,omitempty"`
}

type ListNodesResponse struct {
    Nodes []Node `json:"nodes"`
}
```

#### 2. Partial/single check

- `TriggerParams` (the user-facing `POST /check/:subscriptionID` body) gains an optional
  `NodeIDs []string`; `nil`/empty ⇒ all nodes. The internal/scheduled path
  (`TriggerCheckInternal`) is unchanged — scheduled checks always cover all nodes.
- **Migration `10_add_node_ids_to_jobs`:** `ALTER TABLE check_jobs ADD COLUMN node_ids JSONB`
  (nullable; null/empty ⇒ all). Down: `DROP COLUMN node_ids`.
- `TriggerCheck` serializes `p.NodeIDs` into the new column at job creation.
- `jobStore.loadConfig` reads `node_ids` into `jobConfig.NodeIDs`.
- `jobRunner.run`: after `loadNodes`, if `cfg.NodeIDs` is non-empty, filter `existing` to
  those IDs (preserving DB order); `total = len(subset)`; progress and SSE cover only the
  subset. Bootstrap-fetch happens **only** when `NodeIDs` is empty AND `existing` is empty
  AND a URL is set (an explicit node selection never triggers a fetch).
- The one-active-job 409 check is unchanged.

#### 3. Results model

- `ListNodes` (latest-per-node) = the live/primary display.
- `GetResults` (per-job snapshot) = retained for history.
- Because a subset check writes new `check_results` rows only for checked nodes, every
  other node's latest row is untouched, so `ListNodes` shows "a does not affect b" with no
  carry-forward writes.

### Frontend

- **Node list view** in `detail-pane`: `useNodes(subscriptionId)` → `ListNodes`. Import /
  refresh / fetch-via-node mutations invalidate this query so the list populates at once.
  Never-checked nodes render metrics as "—".
- **Selection:** per-row checkbox; header **"Check selected (N)"** (disabled when none
  selected or a job is active); per-row **"Check this node"**; the existing top
  **"Run check"** = all.
  - All three call `TriggerCheck` with `NodeIDs` = `[]` (all) / `[id]` (single) /
    `[...selected]` (subset), then attach SSE to the returned job, overlay live rows onto
    the list, and refetch `ListNodes` on `done`.
- **History coexistence:** the existing job-history dropdown keeps working.
  - "Latest result" ⇒ the live node list (`ListNodes`).
  - A specific past job ⇒ that job's frozen `GetResults` snapshot (read-only, no
    checkboxes / no per-row check).

## Edge Cases

- **Stale/empty selection:** `NodeIDs` referencing nodes that no longer exist → filtered to
  the intersection; if empty, the job completes with `total=0` and writes no rows (other
  nodes unaffected).
- **Partial check on a 0-node subscription:** selection can't be non-empty, so this reduces
  to the existing bootstrap path only when `NodeIDs` is empty.
- **Disabled nodes:** still selectable/checkable; `enabled` only governs export, not
  checking (unchanged).
- **Live overlay vs latest:** during a run, only checked rows update live; on `done` the
  whole list refetches `ListNodes` to reconcile.

## Testing

**Backend (Go, run without `-race`):**
- `ListNodes`: never-checked node → blank metrics + `last_checked_at == nil`; checked node
  → latest-per-node values; after a subset check, non-selected nodes' values are unchanged.
- Ownership: a non-owner gets NotFound.
- Runner subset scoping: with `NodeIDs` set, only those nodes get new `check_results` rows
  and `total == len(subset)`; the injected fetcher is **not** called (no fetch on partial).
- One-active-job constraint still returns 409 for a second concurrent check.

**Frontend:** `bun run check-types`, `bunx biome check --write`, `bun run build`.

## Migration / Rollout

- Single additive migration (`10_add_node_ids_to_jobs`), backward compatible: existing jobs
  have `node_ids = null` ⇒ treated as "all".
- New endpoint + new optional request field; no breaking change to existing clients.
