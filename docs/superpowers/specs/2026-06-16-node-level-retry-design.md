# Node-Level Retry for Alive + Selected Sub-Tests

**Status:** Approved — 2026-06-16

## Problem

The per-node check (`services/checker/mihomo.go` `checkNode`) is single-shot at every
stage. The fan-out and job state machine already guarantee *completeness* — a
`completed` job has exactly one `check_results` row per node, panics degrade to a
dead-node row, and a cut-short run becomes `failed` rather than a misleading
partial success. What is **not** guaranteed is *fidelity* for a node that is
genuinely alive:

1. **Alive probe is one GET, no retry** (`probeLatency`, `mihomo.go:119`). A
   transient blip or a node whose handshake exceeds the 10s client timeout is
   recorded as dead even though it is reachable. This is the dominant source of
   false negatives ("good node marked dead").
2. **Speed / upload are single-shot** (`measureSpeed`, `measureUploadSpeed`). A
   transient download failure returns `0`, indistinguishable in the stored result
   from a node that is alive but slow.
3. **Streaming checks discard their error and never retry**
   (`runUserRulesWithDebug`, `engine.go:54` does `outcome, _ := runRule(...)`). A
   network failure reaching a platform is stored as `locked`, identical to a
   definitive "you are blocked" answer.

## Goal

For any node that passes the alive probe, every **selected** sub-test (speed,
upload, each selected streaming platform) is executed, and a transient glitch in
any stage triggers a bounded retry instead of being recorded as a false
dead / 0 / locked.

Non-goal: changing completeness guarantees (already correct), adding user-facing
retry configuration, or retrying the manual single-rule TestRule path.

## Approach

Per-phase retry wrappers with **function-typed package seams** for mocking,
mirroring the existing `fetchWithRetry` / `fetchBackoff` pattern in `fetch.go`.
Retry counts are package constants; backoffs are `var` so tests shrink them.

All retries run inside the existing **90s per-node budget** (`nodeTimeout`, a
child of the job context created in `jobRunner.checkOne`). Exhausting that budget
degrades only that one node — it never cancels the parent job context, so retries
can never stall or fail the job. Context cancellation aborts retry loops
immediately.

### Retry policy (Balanced)

| Stage   | Attempts | Retry trigger                                    |
|---------|----------|--------------------------------------------------|
| Alive   | up to 3  | probe returns not-alive                          |
| Speed   | up to 2  | result is `0`                                    |
| Upload  | up to 2  | result is `0`                                    |
| Media   | up to 2 per platform | `runRule` returns `err != nil` (transient) — **never** on a definitive `(outcome, nil)` |

### Component changes

**1. Alive probe — `mihomo.go`**
- Keep `probeLatency` as the single-attempt primitive.
- Add `var probeLatencyFn = probeLatency` (seam).
- Add `probeLatencyWithRetry(ctx, client, testURL) (bool, int)`: loop up to
  `aliveProbeAttempts` (3), return on first success; between failed attempts wait
  `aliveProbeBackoff` (var, default ~300ms) or abort on `ctx.Done()`.
- `checkNode` calls `probeLatencyWithRetry` instead of `probeLatency`.

**2. Speed + upload — `mihomo.go`**
- Add seams `var measureSpeedFn = measureSpeed`, `var measureUploadFn = measureUploadSpeed`.
- Add `measureSpeedWithRetry` / `measureUploadWithRetry`: call the seam; if the
  result is `0` and attempts remain and `ctx.Err() == nil`, retry once more
  (`speedTestAttempts` = 2). Return the last result.
- `checkNode` calls the retry variants for the `opts.SpeedTest` /
  `opts.UploadSpeedTest` branches.

**3. Streaming — `engine.go`**
- Add seam `var runRuleFn = runRule`.
- In `runUserRulesWithDebug`, replace `outcome, _ := runRule(...)` with a bounded
  retry loop over `runRuleFn`: up to `mediaRuleAttempts` (2); retry only while the
  returned `err != nil`; stop immediately on `err == nil` (definitive locked or
  unlocked). Use a **fresh `DebugRecorder` per attempt** so the recorded trace
  reflects the final attempt with no duplicated steps; assign the surviving
  recorder back into `results[rule.Key]`. Optional small `mediaRuleBackoff` (var)
  between attempts, abortable on `ctx.Done()`.
- The manual `evaluateRuleForNode` path calls `runRule` directly and is unchanged
  — interactive single-rule tests stay single-shot.

### Why the media error distinction is safe

`runRule` returns `(PlatformOutcome, error)`:
- `error != nil` — the engine could not complete the check (HTTP failure for
  `condition` rules, engine/script error). Transient → worth retrying.
- `error == nil` — the platform answered; `outcome.Unlocked` is the verdict
  (locked or unlocked). Definitive → never retried.

Caveat (documented, accepted): a script rule (js/ts/tengo/lua) that catches its
own network error internally and returns `(false, nil)` will not be retried. That
is the script author's choice to swallow the error; out of scope here.

## Constants

```go
// mihomo.go
const aliveProbeAttempts = 3
const speedTestAttempts  = 2
var   aliveProbeBackoff  = 300 * time.Millisecond

// engine.go
const mediaRuleAttempts = 2
var   mediaRuleBackoff  = 200 * time.Millisecond
```

## Testing (all mocked, no network)

**`mihomo.go` seams — new `mihomo_retry_test.go`:**
- `probeLatencyWithRetry`: `[fail, fail, success]` → alive, 3 calls; `[success]`
  → 1 call; `[fail, fail, fail]` → dead, 3 calls; canceled ctx → stops early
  (≤1 call).
- `measureSpeedWithRetry`: `[0, 1500]` → 1500, 2 calls; `[1500]` → 1 call;
  `[0, 0]` → 0, 2 calls (capped). Same shape for `measureUploadWithRetry`.

**`engine.go` media retry — new `rule_retry_test.go`:**
- `runRuleFn` → `(locked, nil)` : 1 call, result locked (no retry on definitive).
- `runRuleFn` → `(_, err)` then `(unlocked, nil)` : 2 calls, result unlocked.
- `runRuleFn` → `(_, err)` ×2 : 2 calls (capped), error path surfaces last outcome.
- Disabled rule is skipped (unchanged behavior).

**`checkNode` integration — `mihomo_retry_test.go`:**
- Provide a minimal offline-parseable `ss` proxy mapping (valid for
  `adapter.ParseProxy`, no dial), override all four seams
  (`probeLatencyFn`, `measureSpeedFn`, `measureUploadFn`, `runRuleFn`).
- Assert: an alive node with `opts.SpeedTest = true` and
  `opts.MediaApps = [...]` returns `Alive == true`, `SpeedKbps > 0`, and a
  populated `result.Platforms` for each selected rule — proving alive nodes get
  every selected test "to completion".
- Assert: a node whose `probeLatencyFn` always fails returns `Alive == false`
  and skips speed/media (no wasted sub-tests on a dead node).

Run: `encore test ./services/checker/` (no `-race`; see
`mem:encore-test-race-hangs`).

## Files touched

- `services/checker/mihomo.go` — alive/speed/upload seams + retry wrappers; `checkNode` rewired.
- `services/checker/engine.go` — `runRuleFn` seam + media retry loop in `runUserRulesWithDebug`.
- `services/checker/mihomo_retry_test.go` — new; probe/speed/upload + checkNode integration tests.
- `services/checker/rule_retry_test.go` — new; media retry tests.

## Out of scope

- User-configurable retry counts (hardcoded constants; revisit if requested).
- Retrying script rules that swallow their own errors.
- Any change to fan-out, job state machine, or completeness guarantees.
