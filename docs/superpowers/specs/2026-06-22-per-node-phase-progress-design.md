# Per-Node Phase Progress During Checks

- **Date:** 2026-06-22
- **Status:** Approved (brainstorming)
- **Topic:** Show each in-flight node's current test phase (latency / speed / upload / region / streaming) in the live progress feed during a check.

## Problem

Today the SSE progress stream emits exactly one event per node, published only
*after* that node has finished every phase (`recordResult` in `jobrunner.go`).
The UI therefore shows a node-count progress bar plus a collapsed log of
*completed* nodes. While a node is being checked there is no signal about what it
is doing. The user wants the progress area to communicate the detailed flow —
e.g. "latency testing", "streaming testing", "region testing".

## Goals

- During a run, each in-flight node shows its current phase, then resolves to its
  ✓/✗ result row in place.
- The live feed is expanded by default while a check runs, collapsible after.
- No regression to the existing node-count progress bar, alive-count, live result
  rows, or the live table overlay in `detail-pane`.

## Non-Goals

- No per-platform phase (Netflix/YouTube/…). Streaming rules run concurrently
  inside `runUserRulesWithDebug`; there is no clean sequential order and the event
  volume would explode. One `streaming` phase covers the whole platform batch.
- No change to progress-bar semantics: it stays `completed nodes / total`.
- No change to result persistence, inheritance, or `GetResults` / `ListNodes`.
- No new endpoint; reuse the existing SSE channel and event struct.

## Decisions (from brainstorming)

1. **Display model:** per-node phase. In the live feed, an in-flight node shows
   `spinner + node name + phase label`; on completion the same node becomes a
   ✓/✗ result row.
2. **Visibility:** the live feed is expanded by default while the check runs.
3. **Phase granularity:** test-type level — `latency`, `speed`, `upload`,
   `region`, `streaming` — matching the real order in `checkNode`.
4. **Labels:** English, to match the rest of the UI ("Checking nodes…",
   "Run Check").

## Phase Model

`checkNode` (`services/checker/mihomo.go`) runs phases in this order, each only if
applicable:

| Phase key   | Label (UI)   | Source call                         | Runs when                |
|-------------|--------------|-------------------------------------|--------------------------|
| `latency`   | Latency      | `probeLatencyWithRetry`             | always (first)           |
| `speed`     | Speed test   | `measureSpeedWithRetry`             | `opts.SpeedTest`         |
| `upload`    | Upload       | `measureUploadWithRetry`            | `opts.UploadSpeedTest`   |
| `region`    | Region       | `getProxyInfoFn` (IP/country)       | `len(opts.MediaApps) > 0`|
| `streaming` | Streaming    | `runUserRulesWithDebug`             | `len(opts.MediaApps) > 0`|

A dead node emits only `latency`, then resolves (no further phases).

## Architecture

### Backend (checker service)

#### 1. Phase event on the existing bus

Add one field to `progressUpdate` (`checker.go`):

```go
Phase string `json:"phase,omitempty"`
```

A **phase event** sets only `NodeID`, `NodeName`, `Phase`. It deliberately leaves
`Progress`/`Total` at zero (they are not advanced by phases) — the frontend
ignores counters on phase events (see below). All existing event kinds
(per-node result, initial snapshot, `done`) are unchanged.

#### 2. `emit` callback threaded into the check seam

`checkFunc` gains a trailing parameter:

```go
type phaseEmitter func(phase string)

type checkFunc func(ctx context.Context, nodeID string, mapping map[string]any,
    speedTestURL, uploadTestURL, latencyTestURL string,
    opts CheckOptions, rules []*PlatformRule, emit phaseEmitter) nodeCheckResult
```

Phase key constants live in `mihomo.go`:

```go
const (
    phaseLatency   = "latency"
    phaseSpeed     = "speed"
    phaseUpload    = "upload"
    phaseRegion    = "region"
    phaseStreaming = "streaming"
)
```

`checkNode` calls `emit(phaseX)` immediately before starting each applicable
phase:

- `emit(phaseLatency)` before `probeLatencyWithRetry`
- `emit(phaseSpeed)` before the speed block (inside `if opts.SpeedTest`)
- `emit(phaseUpload)` before the upload block (inside `if opts.UploadSpeedTest`)
- `emit(phaseRegion)` before `getProxyInfoFn`, then `emit(phaseStreaming)` before
  `runUserRulesWithDebug` (both inside `if len(opts.MediaApps) > 0`)

#### 3. Runner wires a per-node emitter

In `jobRunner.checkOne` (`jobrunner.go`), build a closure that publishes a phase
event for this node and pass it to `r.check(...)`:

```go
name, _ := proxy["name"].(string)
emit := func(phase string) {
    r.bus.Publish(jobID, progressUpdate{NodeID: nodeID, NodeName: name, Phase: phase})
}
```

`recordResult` (the result event) is unchanged.

#### 4. Reliability / performance

- Event volume rises to ~`N × (phases+1)`. `inProcessJobBus.Publish` is
  non-blocking (`select { case ch <- update: default: }`, buffer 100), so the
  extra volume **cannot block workers**; under backpressure it drops events.
- A dropped **phase** event is harmless (transient hint; the next phase or the
  result event corrects the display). Result correctness is unaffected: final
  state is reconciled when `done` fires (the client refetches `ListNodes` /
  results). No change to that reconciliation.

### Frontend

#### 1. `sseProgress.ts` — separate in-flight phase state

Add `inflight` to the hook result:

```ts
export interface InflightNode { node_id: string; node_name: string; phase: string; }
// hook returns: { progress, logEntries, debugData, connection, inflight }
// inflight is InflightNode[] (or a Record keyed by node_id, rendered as a list)
```

`onmessage` branches by event kind:

- **Phase event** (`data.phase` set): upsert `inflight[node_id] = {node_id, node_name, phase}`. Do **not** push to `logEntries`. Do **not** touch progress counters.
- **Result event** (`data.node_name` set, no `data.phase`): remove `node_id` from
  `inflight`; push to `logEntries` (existing behavior).
- **Counter/snapshot event** (no `node_name`, no `phase`): `setProgress(data)` —
  i.e. counters update only from non-phase events.
- **`done`**: flush, clear `inflight`, existing invalidations + `onDone`.

`inflight` updates are batched on the existing `FLUSH_INTERVAL_MS` timer so a burst
does not cause per-event renders. `logEntries` continues to hold result rows only,
so `detail-pane`'s `streamedResults` overlay and `aliveSoFar` are unaffected.

#### 2. `progress-panel.tsx` — phase feed

- The feed is **expanded by default while running** (the panel only renders during
  a run; default the disclosure open, still collapsible).
- Two sections inside the feed container:
  - **In progress** (top): one row per `inflight` node (≤ ~20 due to concurrency):
    `spinner + node name + phase label`. Labels via a `phase → label` map
    (`latency→Latency`, `speed→Speed test`, `upload→Upload`, `region→Region`,
    `streaming→Streaming`).
  - **Completed** (below): the existing ✓/✗ result rows, scrollable.
- The node-count progress bar, alive-count, elapsed/ETA, and Cancel button are
  unchanged.

## Edge Cases

- **Stale in-flight entry** (a node's result event was dropped under backpressure):
  cleared on `done`; never permanently stuck.
- **Alive-only run** (`MediaApps` empty, speed off): a node emits only `latency`,
  then resolves — feed shows a brief "Latency" then the result row.
- **Cancellation:** on cancel the job ends and `done` fires; `inflight` clears.
- **Phase event for an already-completed node** (out-of-order/duplicate): if the
  result already moved it to completed, a late phase upsert could re-add it to
  in-flight; mitigated because `done` clears all and renders reconcile from the
  refetch. Acceptable for a transient hint.

## Testing

**Backend (Go, run without `-race`):**
- `checkNode` with an injected fake `emit` (collects the phase sequence) and
  stubbed phase functions:
  - alive node, full options → emits `latency, speed, region, streaming` in order.
  - dead node (latency fails) → emits only `latency`.
  - speed off / media off → corresponding phases omitted.
- All existing `checkFunc` test fakes (`jobrunner_test.go`,
  `live_inheritance_test.go`, `partial_check_test.go`, and any others) get the new
  trailing `_ func(string)` parameter — mechanical, no behavior change.

**Frontend:** `bun run check-types`, `bunx biome check --write`, `bun run build`.
Manual: run a check, confirm in-flight nodes show phase labels that advance and
resolve into result rows; progress bar still counts completed/total.

## Migration / Rollout

- No DB migration. Additive optional `phase` field on the SSE payload; older
  clients ignore unknown fields. New optional `emit` parameter on an internal
  seam. No breaking API change.
