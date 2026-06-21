# Per-Node Phase Progress Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show each in-flight node's current test phase (latency / speed / upload / region / streaming) in the live check feed, so the progress area communicates the detailed flow instead of only completed nodes.

**Architecture:** The per-node check seam (`checkNode`) gains an `emit(phase)` callback that publishes a lightweight phase event on the existing job bus before each phase. The SSE `progressUpdate` carries a new optional `phase` field. The frontend hook classifies phase vs result vs counter events, keeps an `inflight` map of node→phase, and `ProgressPanel` renders an in-progress section (expanded by default during a run) above the existing completed rows.

**Tech Stack:** Go + Encore (checker service), React 19 + TanStack Query, EventSource SSE, Vitest, Biome.

**Reference spec:** `docs/superpowers/specs/2026-06-22-per-node-phase-progress-design.md`

**Conventions for this repo (do not deviate):**
- Run Go tests WITHOUT `-race` (it hangs the Encore harness): `encore test ./services/checker/`.
- Frontend lives at `frontend/src/` (flat). Run from `frontend/`: `bun run test:unit`, `bun run check-types`, `bunx biome check --write <files>`, `bun run build`.
- Commit messages OMIT any Co-Authored-By / attribution trailer. Stage files by explicit name (never `git add -A`).
- Do NOT stage the pre-existing untracked docs `docs/superpowers/plans/2026-06-10-detection-stability.md` or `2026-06-16-node-level-retry.md`.

---

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `services/checker/jobrunner.go` | per-node check seam + runner | add `phaseEmitter` type; add `emit` param to `checkFunc`; build per-node emit closure in `checkOne` |
| `services/checker/checker.go` | SSE event struct | add `Phase` field to `progressUpdate` |
| `services/checker/mihomo.go` | `checkNode` pipeline | add phase constants + `newProxyClientFn` seam; `emit()` before each phase |
| `services/checker/phase_emit_test.go` | NEW — phase ordering test | create |
| `services/checker/jobrunner_test.go` | existing fakes | add trailing `phaseEmitter` param to 3 fakes |
| `services/checker/live_inheritance_test.go` | existing fake | add trailing `phaseEmitter` param |
| `services/checker/partial_check_test.go` | existing fake | add trailing `phaseEmitter` param |
| `frontend/src/queries/sseProgress.ts` | SSE hook | add `InflightNode`, event predicates, `inflight` state; return `inflight` |
| `frontend/src/queries/sseProgress.test.ts` | NEW — predicate unit tests | create |
| `frontend/src/components/workbench/progress-panel.tsx` | progress UI | render in-progress phase rows; default expanded during run |
| `frontend/src/components/workbench/detail-pane.tsx` | passes SSE props | thread `inflight` prop |
| `frontend/src/routes/index.tsx` | owns the SSE hook | destructure + pass `inflight` |

---

## Task 1: Backend — emit phase events from `checkNode`

**Files:**
- Modify: `services/checker/jobrunner.go` (type `checkFunc` at line 27; `checkOne` at lines 204-215)
- Modify: `services/checker/checker.go` (`progressUpdate` struct, ends ~line 202)
- Modify: `services/checker/mihomo.go` (seams ~line 30-63; `checkNode` lines 354-435)
- Modify: `services/checker/jobrunner_test.go` (fakes at lines 33, 94, 164)
- Modify: `services/checker/live_inheritance_test.go` (fake at line 33)
- Modify: `services/checker/partial_check_test.go` (fake at line 49)
- Create: `services/checker/phase_emit_test.go`

- [ ] **Step 1: Write the failing test**

Create `services/checker/phase_emit_test.go`:

```go
package checker

import (
	"context"
	"net/http"
	"testing"
)

func TestCheckNodeEmitsPhasesInOrder(t *testing.T) {
	origNPC := newProxyClientFn
	newProxyClientFn = func(map[string]any) *proxyClient {
		return &proxyClient{Client: &http.Client{Transport: http.DefaultTransport}}
	}
	defer func() { newProxyClientFn = origNPC }()

	origProbe := probeLatencyFn
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return true, 12 }
	defer func() { probeLatencyFn = origProbe }()

	origSpeed := measureSpeedFn
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int { return 100 }
	defer func() { measureSpeedFn = origSpeed }()

	origUpload := measureUploadFn
	measureUploadFn = func(context.Context, http.RoundTripper, string, string) int { return 50 }
	defer func() { measureUploadFn = origUpload }()

	origInfo := getProxyInfoFn
	getProxyInfoFn = func(context.Context, *http.Client) (string, string) { return "1.2.3.4", "US" }
	defer func() { getProxyInfoFn = origInfo }()

	var phases []string
	emit := func(p string) { phases = append(phases, p) }

	opts := CheckOptions{SpeedTest: true, UploadSpeedTest: true, MediaApps: []string{"netflix"}}
	checkNode(context.Background(), "n1", map[string]any{"name": "x"}, "", "", "", opts, nil, emit)

	want := []string{phaseLatency, phaseSpeed, phaseUpload, phaseRegion, phaseStreaming}
	if len(phases) != len(want) {
		t.Fatalf("phases = %v, want %v", phases, want)
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Fatalf("phase[%d] = %q, want %q (full %v)", i, phases[i], want[i], phases)
		}
	}
}

func TestCheckNodeDeadEmitsOnlyLatency(t *testing.T) {
	origNPC := newProxyClientFn
	newProxyClientFn = func(map[string]any) *proxyClient {
		return &proxyClient{Client: &http.Client{Transport: http.DefaultTransport}}
	}
	defer func() { newProxyClientFn = origNPC }()

	origProbe := probeLatencyFn
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return false, 0 }
	defer func() { probeLatencyFn = origProbe }()

	origBackoff := aliveProbeBackoff
	aliveProbeBackoff = 0 // don't sleep between the 3 dead-probe attempts
	defer func() { aliveProbeBackoff = origBackoff }()

	var phases []string
	emit := func(p string) { phases = append(phases, p) }

	opts := CheckOptions{SpeedTest: true, MediaApps: []string{"netflix"}}
	checkNode(context.Background(), "n1", map[string]any{"name": "x"}, "", "", "", opts, nil, emit)

	if len(phases) != 1 || phases[0] != phaseLatency {
		t.Fatalf("phases = %v, want [%s]", phases, phaseLatency)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (does not compile)**

Run: `encore test ./services/checker/ -run TestCheckNode`
Expected: FAIL — compile errors: `phaseLatency` undefined, `newProxyClientFn` undefined, and `checkNode` "too many arguments" (no `emit` param yet).

- [ ] **Step 3: Add the `phaseEmitter` type and `emit` param to the check seam**

In `services/checker/jobrunner.go`, replace the `checkFunc` type (line 27) with:

```go
// phaseEmitter reports the per-node check phase about to start (latency, speed,
// upload, region, streaming). It feeds the live "what is this node doing now"
// feed; emissions are best-effort and never block the worker.
type phaseEmitter func(phase string)

// checkFunc is the per-node check seam; production wires checkNode (mihomo),
// tests inject fakes so the runner is testable without real proxies.
type checkFunc func(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL, uploadTestURL, latencyTestURL string, opts CheckOptions, rules []*PlatformRule, emit phaseEmitter) nodeCheckResult
```

- [ ] **Step 4: Build the per-node emit closure in `checkOne`**

In `services/checker/jobrunner.go`, replace the `nodeCtx`/`return` lines at the end of `checkOne` (lines 212-214) so it constructs and passes `emit`:

```go
	nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
	defer cancel()
	name, _ := proxy["name"].(string)
	emit := func(phase string) {
		r.bus.Publish(jobID, progressUpdate{NodeID: nodeID, NodeName: name, Phase: phase})
	}
	return r.check(nodeCtx, nodeID, proxy, cfg.SpeedTestURL, uploadTestURL, cfg.LatencyTestURL, cfg.Options, rules, emit)
```

- [ ] **Step 5: Add the `Phase` field to `progressUpdate`**

In `services/checker/checker.go`, add this field to the `progressUpdate` struct (immediately after the `Debug *NodeDebug` line, before the closing brace):

```go
	Phase           string                     `json:"phase,omitempty"`
```

- [ ] **Step 6: Add phase constants + proxy-client seam, and emit in `checkNode`**

In `services/checker/mihomo.go`, add the seam next to the existing ones (after `var probeLatencyFn = probeLatency` at line 30):

```go
// newProxyClientFn is a seam so phase-emission tests can stub proxy creation
// (avoiding a real mihomo dial).
var newProxyClientFn = newProxyClient
```

Add phase constants (place them just above `func checkNode`):

```go
const (
	phaseLatency   = "latency"
	phaseSpeed     = "speed"
	phaseUpload    = "upload"
	phaseRegion    = "region"
	phaseStreaming = "streaming"
)
```

Change the `checkNode` signature to accept `emit`:

```go
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL, uploadTestURL, latencyTestURL string, opts CheckOptions, rules []*PlatformRule, emit phaseEmitter) nodeCheckResult {
```

In `checkNode`, change the proxy-client construction (line 362) to use the seam:

```go
	pc := newProxyClientFn(mapping)
```

Insert `emit(phaseLatency)` immediately before the latency probe (before line 375 `alive, latency := probeLatencyWithRetry(...)`):

```go
	emit(phaseLatency)
	alive, latency := probeLatencyWithRetry(ctx, pc.Client, latencyTestURL)
```

Inside `if opts.SpeedTest {` (line 398) make the first statement:

```go
	if opts.SpeedTest {
		emit(phaseSpeed)
		result.SpeedKbps = measureSpeedWithRetry(ctx, pc.Client.Transport, speedTestURL)
	}
```

Inside `if opts.UploadSpeedTest {` (line 401) make the first statement:

```go
	if opts.UploadSpeedTest {
		emit(phaseUpload)
		result.UploadSpeedKbps = measureUploadWithRetry(ctx, pc.Client.Transport, speedTestURL, uploadTestURL)
	}
```

Inside `if len(opts.MediaApps) > 0 {` (line 405): emit `region` before `getProxyInfoFn`, and `streaming` before `runUserRulesWithDebug` (line 418). Insert only the two `emit(...)` lines; keep everything between them unchanged:

```go
		emit(phaseRegion)
		result.IP, result.Country = getProxyInfoFn(ctx, mediaClient)
```
```go
		emit(phaseStreaming)
		outcomes := runUserRulesWithDebug(ctx, mediaClient, rules, ruleRecorders)
```

- [ ] **Step 7: Update the 5 existing check-func fakes to the new signature**

The fakes are assigned to a `checkFunc` field, so their `emit` parameter type MUST be `phaseEmitter` (a named type — `func(string)` is not identical and will not compile).

`services/checker/jobrunner_test.go`:
- Line 33 — `aliveCheck`: append `, _ phaseEmitter` before `) nodeCheckResult {`.
- Line 94 — `panicky`: append `, _ phaseEmitter` before `) nodeCheckResult {`.
- Line 164 — `recording`: append `, _ phaseEmitter` before `) nodeCheckResult {`.

`services/checker/live_inheritance_test.go`:
- Line 33 — the `check :=` fake: append `, _ phaseEmitter` before `) nodeCheckResult {`.

`services/checker/partial_check_test.go`:
- Line 49 — the inline `check:` fake: append `, _ phaseEmitter` before `) nodeCheckResult {`.

Example (aliveCheck before → after):
```go
// before
func aliveCheck(ctx context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule) nodeCheckResult {
// after
func aliveCheck(ctx context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule, _ phaseEmitter) nodeCheckResult {
```

- [ ] **Step 8: Run the phase test and the whole checker suite**

Run: `encore test ./services/checker/ -run TestCheckNode`
Expected: PASS (both `TestCheckNodeEmitsPhasesInOrder` and `TestCheckNodeDeadEmitsOnlyLatency`).

Run: `encore test ./services/checker/`
Expected: PASS — entire suite green (no `-race`).

- [ ] **Step 9: Commit**

```bash
git add services/checker/jobrunner.go services/checker/checker.go services/checker/mihomo.go services/checker/phase_emit_test.go services/checker/jobrunner_test.go services/checker/live_inheritance_test.go services/checker/partial_check_test.go
git commit -m "feat(checker): emit per-node check phases over SSE"
```

---

## Task 2: Frontend — classify phase events and track in-flight nodes

**Files:**
- Modify: `frontend/src/queries/sseProgress.ts`
- Create: `frontend/src/queries/sseProgress.test.ts`

- [ ] **Step 1: Write the failing predicate test**

Create `frontend/src/queries/sseProgress.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { isPhaseEvent, isResultEvent } from "./sseProgress";

describe("sse event classification", () => {
	it("treats an event with a phase as an in-flight phase event", () => {
		const e = { node_id: "n1", node_name: "x", phase: "streaming" };
		expect(isPhaseEvent(e)).toBe(true);
		expect(isResultEvent(e)).toBe(false);
	});

	it("treats a node event without a phase as a completed result", () => {
		const e = { node_id: "n1", node_name: "x", alive: true };
		expect(isResultEvent(e)).toBe(true);
		expect(isPhaseEvent(e)).toBe(false);
	});

	it("treats a counter/snapshot event (no node, no phase) as neither", () => {
		const e = { progress: 5, total: 10 };
		expect(isPhaseEvent(e)).toBe(false);
		expect(isResultEvent(e)).toBe(false);
	});
});
```

- [ ] **Step 2: Run it to verify it fails**

Run (from `frontend/`): `bun run test:unit src/queries/sseProgress.test.ts`
Expected: FAIL — `isPhaseEvent` / `isResultEvent` are not exported.

- [ ] **Step 3: Add `InflightNode` + predicates + `phase` field to `sseProgress.ts`**

In `frontend/src/queries/sseProgress.ts`, add `phase?: string;` to the `SSEProgress` interface (after the `traffic_bytes?: number;` line).

Then, after the `SSEProgress` interface (line 30) add:

```ts
export interface InflightNode {
	node_id: string;
	node_name: string;
	phase: string;
}

// A phase event announces the test a node is starting (in-flight); it carries a
// `phase` and never advances the completed counter.
export function isPhaseEvent(d: SSEProgress): boolean {
	return !!d.phase && !!d.node_id;
}

// A result event is a finished node (no phase, has a name) — the existing
// per-node payload that feeds the completed log and the live table.
export function isResultEvent(d: SSEProgress): boolean {
	return !d.phase && !!d.node_name;
}
```

- [ ] **Step 4: Run the predicate test to verify it passes**

Run (from `frontend/`): `bun run test:unit src/queries/sseProgress.test.ts`
Expected: PASS (3/3).

- [ ] **Step 5: Wire `inflight` state into the hook**

In `frontend/src/queries/sseProgress.ts`:

Add to `UseSSEProgressResult` (after `connection: SSEConnection;`):

```ts
	inflight: InflightNode[];
```

Add state inside the hook (after the `connection` state, line 65):

```ts
	const [inflight, setInflight] = useState<InflightNode[]>([]);
```

Reset it alongside the others in the first effect (the effect at line 70 that clears on `jobId` change) by adding:

```ts
		setInflight([]);
```

In the second effect, maintain an in-flight map and batch it on the existing flush timer. Replace the buffer/flush/`onmessage` block (lines 80-114) with:

```ts
		const buffer: SSEProgress[] = [];
		const debugBuffer: NodeDebug[] = [];
		const inflightMap = new Map<string, InflightNode>();
		let inflightDirty = false;
		const flush = () => {
			if (buffer.length > 0) {
				const batch = buffer.splice(0, buffer.length);
				setLogEntries((prev) => [...prev, ...batch].slice(-MAX_LOG_ENTRIES));
			}
			if (debugBuffer.length > 0) {
				const batch = debugBuffer.splice(0, debugBuffer.length);
				setDebugData((prev) => [...prev, ...batch]);
			}
			if (inflightDirty) {
				inflightDirty = false;
				setInflight([...inflightMap.values()]);
			}
		};
		const timer = setInterval(flush, FLUSH_INTERVAL_MS);

		const es = new EventSource(
			`${window.location.origin}/api/check/${jobId}/progress`,
		);
		es.onopen = () => setConnection("open");
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			if (data.debug) debugBuffer.push(data.debug);

			if (isPhaseEvent(data)) {
				// In-flight: record/update this node's current phase. Do not touch
				// progress counters or the completed log.
				inflightMap.set(data.node_id as string, {
					node_id: data.node_id as string,
					node_name: data.node_name ?? "",
					phase: data.phase as string,
				});
				inflightDirty = true;
				return;
			}

			if (isResultEvent(data)) {
				// Finished: drop from in-flight, append to the completed log.
				if (data.node_id && inflightMap.delete(data.node_id)) {
					inflightDirty = true;
				}
				buffer.push(data);
			}

			// Counters advance only from non-phase events.
			setProgress(data);

			if (data.done) {
				inflightMap.clear();
				setInflight([]);
				flush();
				setConnection("done");
				es.close();
				qc.invalidateQueries({ queryKey: queryKeys.jobs(subscriptionId) });
				qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
				qc.invalidateQueries({ queryKey: queryKeys.results(subscriptionId) });
				onDoneRef.current?.();
			}
		};
```

Add `inflight` to the hook's return (line 126):

```ts
	return { progress, logEntries, debugData, connection, inflight };
```

- [ ] **Step 6: Type-check and run the unit suite**

Run (from `frontend/`): `bun run check-types`
Expected: no errors.

Run (from `frontend/`): `bun run test:unit src/queries/sseProgress.test.ts`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/queries/sseProgress.ts frontend/src/queries/sseProgress.test.ts
git commit -m "feat(web): classify SSE phase events and track in-flight nodes"
```

---

## Task 3: Frontend — render the in-flight phase feed (expanded during run)

**Files:**
- Modify: `frontend/src/components/workbench/progress-panel.tsx`
- Modify: `frontend/src/components/workbench/detail-pane.tsx` (around lines 15/34/48/198)
- Modify: `frontend/src/routes/index.tsx` (lines 69, 155-162)

- [ ] **Step 1: Thread `inflight` from the hook through to the panel**

In `frontend/src/routes/index.tsx`, add `inflight` to the destructure (line 69):

```tsx
	const { progress, logEntries, debugData, connection, inflight } = useSSEProgress({
```

Pass it to `<DetailPane>` (after `connection={connection}`, line 162):

```tsx
						connection={connection}
						inflight={inflight}
```

In `frontend/src/components/workbench/detail-pane.tsx`:

Extend the existing `@/queries` type import (line 15) to include `InflightNode`:

```tsx
import type { InflightNode, SSEConnection, SSEProgress } from "@/queries";
```

Add the prop to the destructure (after `connection,`, line ~36) and to the prop type (after `connection: SSEConnection;`, line ~50):

```tsx
	connection,
	inflight,
```
```tsx
	connection: SSEConnection;
	inflight: InflightNode[];
```

Pass it to `<ProgressPanel>` (after `connection={connection}`, line ~201):

```tsx
						connection={connection}
						inflight={inflight}
```

- [ ] **Step 2: Render the in-flight section in `ProgressPanel`**

In `frontend/src/components/workbench/progress-panel.tsx`:

Replace the existing `import type { SSEConnection, SSEProgress } from "@/queries";` (line 7) with the type plus a phase-label map:

```tsx
import type { InflightNode, SSEConnection, SSEProgress } from "@/queries";

const PHASE_LABELS: Record<string, string> = {
	latency: "Latency",
	speed: "Speed test",
	upload: "Upload",
	region: "Region",
	streaming: "Streaming",
};
```

Add `inflight` to the component props (signature + type):

```tsx
export function ProgressPanel({
	progress,
	logEntries,
	connection,
	inflight,
	cancelPending,
	onCancel,
}: {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	connection: SSEConnection;
	inflight: InflightNode[];
	cancelPending: boolean;
	onCancel: () => void;
}) {
```

Default the disclosure open so the feed is visible during a run — change the `logOpen` initial state (line 27):

```tsx
	const [logOpen, setLogOpen] = useState(true);
```

Update the disclosure toggle label (line 100) to count both lists:

```tsx
						Live log ({inflight.length + logEntries.length})
```

Replace the disclosure body block (lines 108-135, the `{logOpen && logEntries.length > 0 ? (...) : null}`) with one that shows in-flight rows first, then completed rows:

```tsx
			{logOpen ? (
				<div
					ref={logRef}
					className="max-h-52 overflow-y-auto rounded-md bg-background/60 p-2"
				>
					{inflight.map((n: InflightNode) => (
						<div
							key={`live-${n.node_id}`}
							className="flex items-baseline gap-2 py-0.5 font-mono text-[11px] tabular-nums"
						>
							<Spinner className="size-3 text-info" />
							<span className="min-w-0 flex-1 truncate text-foreground">
								{n.node_name}
							</span>
							<span className="text-muted-foreground">
								{PHASE_LABELS[n.phase] ?? n.phase}
							</span>
						</div>
					))}
					{logEntries.map((e, i) => (
						<div
							key={`${i}-${e.node_name ?? ""}`}
							className="flex items-baseline gap-2 py-0.5 font-mono text-[11px] tabular-nums"
						>
							<span className={cn(e.alive ? "text-success" : "text-danger")}>
								{e.alive ? "✓" : "✗"}
							</span>
							<span className="min-w-0 flex-1 truncate text-foreground">
								{e.node_name}
							</span>
							{e.alive && e.latency_ms ? <span>{e.latency_ms}ms</span> : null}
							{e.alive && e.speed_kbps ? (
								<span className="text-muted-foreground">
									{e.speed_kbps >= 1024
										? `${(e.speed_kbps / 1024).toFixed(1)}MB/s`
										: `${e.speed_kbps}KB/s`}
								</span>
							) : null}
						</div>
					))}
				</div>
			) : null}
```

- [ ] **Step 3: Type-check, lint, build**

Run (from `frontend/`): `bun run check-types`
Expected: no errors.

Run (from `frontend/`): `bunx biome check --write src/components/workbench/progress-panel.tsx src/components/workbench/detail-pane.tsx src/routes/index.tsx`
Expected: formatted, no errors on these files.

Run (from `frontend/`): `bun run build`
Expected: build succeeds.

- [ ] **Step 4: Manual verification**

Start both servers (`encore run`; `cd frontend && bun dev`). Run a check on a subscription with nodes and confirm:
- During the run, the live feed is expanded; in-flight nodes show a spinner + name + a phase label (Latency / Speed test / Upload / Region / Streaming) that advances.
- Each node's row resolves into a ✓/✗ result row when done.
- The node-count progress bar still advances completed/total and does not flicker to 0 on phase events.
- After completion, the feed can be collapsed.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/workbench/progress-panel.tsx frontend/src/components/workbench/detail-pane.tsx frontend/src/routes/index.tsx
git commit -m "feat(web): show per-node check phase in the live progress feed"
```

---

## Final Verification (after all tasks)

- [ ] `encore test ./services/checker/` — green (no `-race`).
- [ ] From `frontend/`: `bun run check-types` && `bun run test:unit` && `bun run build` — all green.
- [ ] Manual flow (Task 3 Step 4) confirmed in a browser.
- [ ] `git status` clean except the two pre-existing untracked plan docs (must remain unstaged).

---

## Self-Review Notes (author)

- **Spec coverage:** phase model (Task 1 Step 6) ✓; phase event field (Step 5) ✓; emit seam (Steps 3-4) ✓; non-blocking/best-effort (reuses existing `Publish`, unchanged) ✓; frontend in-flight classification (Task 2) ✓; default-expanded feed + phase labels (Task 3) ✓; counters only from non-phase events (Task 2 Step 5) ✓; `logEntries` stays result-only so the live table overlay is unaffected (Task 2 pushes to `buffer` only for result events) ✓; tests backend + frontend ✓.
- **Type consistency:** `phaseEmitter` used uniformly for the Go param (prod + all 5 fakes); `InflightNode {node_id,node_name,phase}` identical in `sseProgress.ts`, `detail-pane.tsx`, `progress-panel.tsx`; SSE field `phase` matches Go json tag `phase`.
- **No placeholders:** every step has concrete code and exact commands.
