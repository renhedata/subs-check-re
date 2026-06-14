# Partial Checks with Inherited Results + Alive-Only Mode — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a check measure only the selected dimensions; every unselected dimension (speed, upload, country, each streaming platform) falls back to the node's most recent known value at read/export time. Add an explicit "Alive only" mode.

**Architecture:** Read-time inheritance (no schema change). The checker writes only what each run measured; `GetResults` and the export query merge in each node's latest-known value by `node_name`. Platform selection becomes a real filter on which rules run (today it is only an on/off gate). Alive + latency are always measured and never inherited.

**Tech Stack:** Go + Encore (backend, `services/checker`), PostgreSQL (jsonb merge in SQL), React 19 + TanStack Query + Vitest (frontend), Biome.

**Spec:** `docs/superpowers/specs/2026-06-14-partial-checks-inheritance-alive-only-design.md`

**Testing note:** Run backend tests with `encore test ./...` — **never** `-race` (known harness hang, per project memory). Frontend: `bun check-types`, `bun check`, `bun test` from `frontend/`.

---

## File Structure

**Backend (`services/checker/`)**
- `engine.go` — add `filterRulesBySelection` (pure helper).
- `jobrunner.go` — wire the filter into `run` (one line).
- `checker.go` — rewrite `applyOptionDefaults`; extend the `GetResults` SQL (country + platform inheritance).
- `export_data.go` — extend the `loadJobProxies` SQL (country + platform inheritance).
- Tests: `rule_filter_test.go` (new), `jobrunner_test.go`, `checker_test.go`, `results_test.go`, `export_data_test.go`.

**Frontend (`frontend/src/`)**
- `lib/checkOptions.ts` — add `hasStoredCheckOptions`, `reconcileMediaApps`.
- `lib/checkOptions.test.ts` — cover the new helpers.
- `components/check-options-fields.tsx` — rule-driven platform list (prop), "Alive only" + "All" presets, always-tested note.
- `components/workbench/run-check-button.tsx` — fetch rules, reconcile selection, pass `availablePlatforms`.
- `components/schedule-dialog.tsx` — fetch rules, default/intersect selection, pass `availablePlatforms`.

No API shape changes → **no client regeneration**.

---

## Task 1: Filter platform rules by selection

Today `MediaApps` only gates media on/off; all enabled rules run regardless of selection. Make selection a real filter so unselected platforms aren't re-measured and can inherit.

**Files:**
- Modify: `services/checker/engine.go`
- Modify: `services/checker/jobrunner.go:96-99` (after `loadUserRules`)
- Test: `services/checker/rule_filter_test.go` (create), `services/checker/jobrunner_test.go`

- [ ] **Step 1: Write the failing unit test for the pure helper**

Create `services/checker/rule_filter_test.go`:

```go
package checker

import "testing"

func keysOf(rules []*PlatformRule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.Key
	}
	return out
}

func TestFilterRulesBySelection(t *testing.T) {
	rules := []*PlatformRule{{Key: "netflix"}, {Key: "youtube"}, {Key: "disney"}}

	got := filterRulesBySelection(rules, []string{"netflix", "disney", "nope"})
	if len(got) != 2 || got[0].Key != "netflix" || got[1].Key != "disney" {
		t.Fatalf("want [netflix disney], got %v", keysOf(got))
	}
	if filterRulesBySelection(rules, nil) != nil {
		t.Error("nil selection must yield no rules")
	}
	if len(filterRulesBySelection(rules, []string{})) != 0 {
		t.Error("empty selection must yield no rules")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `encore test ./services/checker/ -run TestFilterRulesBySelection`
Expected: FAIL — `undefined: filterRulesBySelection`.

- [ ] **Step 3: Implement the helper**

Append to `services/checker/engine.go`:

```go
// filterRulesBySelection returns only the rules whose Key is in selected.
// An empty/nil selection yields no rules — an alive-only check tests no
// platforms, so every platform inherits its last-known value at read time.
func filterRulesBySelection(rules []*PlatformRule, selected []string) []*PlatformRule {
	if len(selected) == 0 {
		return nil
	}
	want := make(map[string]bool, len(selected))
	for _, k := range selected {
		want[k] = true
	}
	out := make([]*PlatformRule, 0, len(rules))
	for _, r := range rules {
		if want[r.Key] {
			out = append(out, r)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the unit test to verify it passes**

Run: `encore test ./services/checker/ -run TestFilterRulesBySelection`
Expected: PASS.

- [ ] **Step 5: Write the failing wiring test (runner passes only selected rules to check)**

Add to `services/checker/jobrunner_test.go` (add `"sync"` to imports):

```go
func TestJobRunnerFiltersRulesBySelection(t *testing.T) {
	userID := "filter-user-" + uuid.New().String()
	subID := "runner-sub-" + uuid.New().String()
	for _, k := range []string{"netflix", "youtube"} {
		if _, err := db.Exec(context.Background(),
			`INSERT INTO platform_rules (id, user_id, name, key, rule_type) VALUES ($1,$2,$3,$4,'condition')`,
			uuid.New().String(), userID, k, k); err != nil {
			t.Fatalf("seed rule %s: %v", k, err)
		}
	}

	opts := CheckOptions{SpeedTest: false, MediaApps: []string{"netflix"}}
	optsJSON, _ := json.Marshal(opts)
	jobID := uuid.New().String()
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://example.test/sub',$4,'queued',NOW())
	`, jobID, subID, userID, optsJSON); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	var mu sync.Mutex
	var gotKeys []string
	recording := func(_ context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, rules []*PlatformRule) nodeCheckResult {
		mu.Lock()
		if gotKeys == nil {
			gotKeys = keysOf(rules)
			if gotKeys == nil {
				gotKeys = []string{}
			}
		}
		mu.Unlock()
		name, _ := mapping["name"].(string)
		return nodeCheckResult{NodeID: nodeID, NodeName: name, Alive: true}
	}

	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: runnerProxies()},
		bus:     newInProcessJobBus(),
		check:   recording,
	}
	r.run(context.Background(), jobID, subID, userID)

	if len(gotKeys) != 1 || gotKeys[0] != "netflix" {
		t.Errorf("runner must pass only selected rules; want [netflix], got %v", gotKeys)
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `encore test ./services/checker/ -run TestJobRunnerFiltersRulesBySelection`
Expected: FAIL — `gotKeys` is `[netflix youtube]` (filter not wired yet).

- [ ] **Step 7: Wire the filter into the runner**

In `services/checker/jobrunner.go`, immediately after the `loadUserRules` block (the `if err != nil { rlog.Warn(...) }` ending at line ~99), add:

```go
	// Only evaluate rules the caller selected for this run; unselected
	// platforms inherit their last-known result at read time.
	userRules = filterRulesBySelection(userRules, cfg.Options.MediaApps)
```

- [ ] **Step 8: Run both tests + the existing runner suite**

Run: `encore test ./services/checker/ -run 'TestFilterRulesBySelection|TestJobRunner'`
Expected: PASS (existing `TestJobRunnerHappyPath` etc. use `aliveCheck`, unaffected).

- [ ] **Step 9: Commit**

```bash
git add services/checker/engine.go services/checker/jobrunner.go services/checker/rule_filter_test.go services/checker/jobrunner_test.go
git commit -m "feat(checker): filter platform rules by per-run selection"
```

---

## Task 2: Make "Alive only" expressible (`applyOptionDefaults`)

Today `applyOptionDefaults` coerces "no speed + empty media" into a full check, so alive-only is impossible via the scheduler/internal path. Default only a genuinely omitted (`nil`) media list; preserve an explicit empty slice.

**Files:**
- Modify: `services/checker/checker.go:152-160`
- Test: `services/checker/checker_test.go`

- [ ] **Step 1: Write the failing test**

Add to `services/checker/checker_test.go`:

```go
func TestApplyOptionDefaults_PreservesExplicitAliveOnly(t *testing.T) {
	// Explicit empty media + no speed = alive-only. Must NOT be reset to full.
	o := CheckOptions{SpeedTest: false, UploadSpeedTest: false, MediaApps: []string{}}
	applyOptionDefaults(&o)
	if o.SpeedTest || len(o.MediaApps) != 0 {
		t.Errorf("alive-only must be preserved, got %+v", o)
	}
}

func TestApplyOptionDefaults_DefaultsNilMedia(t *testing.T) {
	// Omitted media (nil) defaults to the built-in platform list.
	o := CheckOptions{SpeedTest: true, MediaApps: nil}
	applyOptionDefaults(&o)
	if len(o.MediaApps) == 0 {
		t.Error("nil media must default to built-in list")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `encore test ./services/checker/ -run TestApplyOptionDefaults`
Expected: FAIL — `TestApplyOptionDefaults_PreservesExplicitAliveOnly` gets a full reset.

- [ ] **Step 3: Rewrite `applyOptionDefaults`**

Replace the function body at `services/checker/checker.go:152`:

```go
func applyOptionDefaults(o *CheckOptions) {
	// A nil slice means the field was omitted → default to the built-in
	// platform list. An explicit empty slice means "test no platforms"
	// (e.g. an alive-only run) and must be preserved.
	if o.MediaApps == nil {
		o.MediaApps = defaultCheckOptions().MediaApps
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `encore test ./services/checker/ -run TestApplyOptionDefaults`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/checker.go services/checker/checker_test.go
git commit -m "feat(checker): preserve explicit alive-only options in applyOptionDefaults"
```

---

## Task 3: Read-time inheritance in `GetResults`

Fill unmeasured speed/upload/country/platforms from each node's latest-known value (by `node_name` within the subscription). Alive/latency stay fresh from the current job.

**Files:**
- Modify: `services/checker/checker.go:503-545` (the `GetResults` results query)
- Test: `services/checker/results_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `services/checker/results_test.go` (add `"encoding/json"` to imports):

```go
func seedResult(t *testing.T, jobID, subID, userID, nodeName string, ageHours int,
	alive bool, latency, speed int, country string, platforms map[string]PlatformOutcome) {
	t.Helper()
	jobExists := 0
	db.QueryRow(context.Background(), `SELECT COUNT(*) FROM check_jobs WHERE id=$1`, jobID).Scan(&jobExists)
	if jobExists == 0 {
		if _, err := db.Exec(context.Background(), `
			INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
			VALUES ($1,$2,$3,'completed',1,1,NOW(),NOW())
		`, jobID, subID, userID); err != nil {
			t.Fatalf("seed job: %v", err)
		}
	}
	pj, _ := json.Marshal(platforms)
	if string(pj) == "null" {
		pj = []byte("{}")
	}
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,$4,'ss', NOW() - make_interval(hours => $5::int), $6,$7,$8,$9,'', $10)
	`, uuid.New().String(), jobID, uuid.New().String(), nodeName, ageHours, alive, latency, speed, country, pj); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

func TestGetResultsInheritsUnmeasuredDimensions(t *testing.T) {
	userID := "inh-user-" + uuid.New().String()
	subID := "inh-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Older full check: speed 5000, HK, netflix + youtube unlocked.
	seedResult(t, jobA, subID, userID, "N1", 2, true, 50, 5000, "HK",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}, "youtube": {Unlocked: true}})
	// Newer alive-only check: fresh alive/latency, nothing else.
	seedResult(t, jobB, subID, userID, "N1", 0, true, 30, 0, "",
		map[string]PlatformOutcome{})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobB})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.LatencyMs != 30 {
		t.Errorf("latency must be fresh from alive-only run, got %d", r.LatencyMs)
	}
	if r.SpeedKbps != 5000 {
		t.Errorf("speed must inherit, got %d", r.SpeedKbps)
	}
	if r.Country != "HK" {
		t.Errorf("country must inherit, got %q", r.Country)
	}
	if !r.Platforms["netflix"].Unlocked || !r.Platforms["youtube"].Unlocked {
		t.Errorf("platforms must inherit, got %+v", r.Platforms)
	}
}

func TestGetResultsPlatformInheritanceIsPerKey(t *testing.T) {
	userID := "perkey-user-" + uuid.New().String()
	subID := "perkey-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobC := uuid.New().String(), uuid.New().String()

	// Older: netflix + youtube both unlocked.
	seedResult(t, jobA, subID, userID, "N1", 2, true, 100, 1000, "US",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}, "youtube": {Unlocked: true}})
	// Newer: tested netflix only, now locked. youtube NOT tested.
	seedResult(t, jobC, subID, userID, "N1", 1, true, 100, 1000, "US",
		map[string]PlatformOutcome{"netflix": {Unlocked: false}})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobC})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.Platforms["netflix"].Unlocked {
		t.Error("netflix must reflect the fresh (locked) result")
	}
	if !r.Platforms["youtube"].Unlocked {
		t.Error("youtube must inherit the older unlocked result (per-key)")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `encore test ./services/checker/ -run TestGetResults`
Expected: FAIL — inherited fields are `0`/`""`/empty (no fallback yet).

- [ ] **Step 3: Extend the `GetResults` query**

In `services/checker/checker.go`, replace the results query (the `db.Query(ctx, ...)` block starting at line ~503, `WITH r AS (...)`) with:

```go
	rows, err := db.Query(ctx, `
		WITH latest_platform_kv AS (
			SELECT DISTINCT ON (cr2.node_name, kv.key) cr2.node_name, kv.key AS key, kv.value AS value
			FROM check_results cr2
			JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			CROSS JOIN LATERAL jsonb_each(cr2.platforms) AS kv(key, value)
			WHERE cj2.subscription_id = $2 AND cr2.platforms IS NOT NULL AND cr2.platforms <> '{}'::jsonb
			ORDER BY cr2.node_name, kv.key, cr2.checked_at DESC
		),
		merged_platforms AS (
			SELECT node_name, jsonb_object_agg(key, value) AS platforms
			FROM latest_platform_kv
			GROUP BY node_name
		),
		r AS (
			SELECT cr.node_id,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       COALESCE(n.type, cr.node_type) AS node_type,
			       COALESCE(n.enabled, true) AS enabled,
			       COALESCE(n.server, '') AS server,
			       COALESCE(n.port, 0) AS port,
			       COALESCE(COALESCE(n.config, cr.node_config)::text, '') AS config,
			       cr.alive, cr.latency_ms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       CASE WHEN cr.upload_speed_kbps > 0 THEN cr.upload_speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.upload_speed_kbps FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.upload_speed_kbps > 0
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), 0)
			       END AS upload_speed_kbps,
			       CASE WHEN cr.country <> '' THEN cr.country
			            ELSE COALESCE((
			                SELECT cr2.country FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.country <> ''
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), '')
			       END AS country,
			       cr.ip,
			       COALESCE(mp.platforms, cr.platforms, '{}'::jsonb) AS platforms,
			       cr.traffic_bytes
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			LEFT JOIN merged_platforms mp ON mp.node_name = cr.node_name
			WHERE cr.job_id = $1
		)
		SELECT * FROM r
		ORDER BY alive DESC, speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST
	`, job.ID, subscriptionID)
```

The `SELECT`/scan column order is unchanged (speed, upload, country, ip, platforms, traffic), so the existing `rows.Scan(...)` below is untouched.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `encore test ./services/checker/ -run 'TestGetResults'`
Expected: PASS (including the existing `TestGetResultsReturnsServerPortConfig`).

- [ ] **Step 5: Commit**

```bash
git add services/checker/checker.go services/checker/results_test.go
git commit -m "feat(checker): inherit unmeasured speed/upload/country/platforms in GetResults"
```

---

## Task 4: Read-time inheritance in export (`loadJobProxies`)

The exported subscription reads the latest completed job. Apply the same per-key platform + country inheritance so an alive-only run keeps its streaming/country tags, while the `alive = true` filter still uses fresh liveness.

**Files:**
- Modify: `services/checker/export_data.go:160-190` (the `loadJobProxies` query)
- Test: `services/checker/export_data_test.go`

- [ ] **Step 1: Write the failing test**

Add to `services/checker/export_data_test.go` (add imports `context`, `encoding/json`, `strings`, `github.com/google/uuid`; `settingssvc` is already imported):

```go
func TestLoadJobProxies_InheritsPlatformsAfterAliveOnly(t *testing.T) {
	subID := "exp-sub-" + uuid.New().String()
	userID := "exp-user-" + uuid.New().String()
	jobA, jobB := uuid.New().String(), uuid.New().String()
	cfgRow := func(jobID string, ageHours int) {
		db.Exec(context.Background(), `
			INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
			VALUES ($1,$2,$3,'completed',1,1,NOW(),NOW())
		`, jobID, subID, userID)
	}
	cfgRow(jobA, 2)
	cfgRow(jobB, 0)

	nodeCfg := `{"type":"ss","server":"1.1.1.1","port":1,"name":"N1"}`
	pjA, _ := json.Marshal(map[string]PlatformOutcome{"netflix": {Unlocked: true}})
	// Older full check on N1.
	db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,'N1','ss',$4::jsonb, NOW() - interval '2 hours', true, 50, 2048, 'HK', '', $5)
	`, uuid.New().String(), jobA, uuid.New().String(), nodeCfg, pjA)
	// Newer alive-only check on N1: empty platforms, no speed/country.
	db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,'N1','ss',$4::jsonb, NOW(), true, 30, 0, '', '', '{}'::jsonb)
	`, uuid.New().String(), jobB, uuid.New().String(), nodeCfg)

	cfg := settingssvc.DefaultExportTags() // netflix→"NF" enabled, speed on
	proxies, err := loadJobProxies(context.Background(), jobB, subID, "", cfg, exportPrefs{Sort: "speed_desc"})
	if err != nil {
		t.Fatalf("loadJobProxies: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("want 1 proxy, got %d", len(proxies))
	}
	name, _ := proxies[0]["name"].(string)
	if !strings.Contains(name, "NF") {
		t.Errorf("alive-only export must keep inherited Netflix tag, got %q", name)
	}
	if !strings.Contains(name, "2.0MB") {
		t.Errorf("alive-only export must keep inherited speed tag, got %q", name)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `encore test ./services/checker/ -run TestLoadJobProxies_InheritsPlatformsAfterAliveOnly`
Expected: FAIL — name is `"N1"` (no inherited tags).

- [ ] **Step 3: Extend the `loadJobProxies` query**

In `services/checker/export_data.go`, replace the `query :=` string (lines ~165-185) with:

```go
	query := `
		WITH latest_platform_kv AS (
			SELECT DISTINCT ON (cr2.node_name, kv.key) cr2.node_name, kv.key AS key, kv.value AS value
			FROM check_results cr2
			JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			CROSS JOIN LATERAL jsonb_each(cr2.platforms) AS kv(key, value)
			WHERE cj2.subscription_id = $2 AND cr2.platforms IS NOT NULL AND cr2.platforms <> '{}'::jsonb
			ORDER BY cr2.node_name, kv.key, cr2.checked_at DESC
		),
		merged_platforms AS (
			SELECT node_name, jsonb_object_agg(key, value) AS platforms
			FROM latest_platform_kv
			GROUP BY node_name
		),
		r AS (
			SELECT COALESCE(n.config, cr.node_config) AS config,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       CASE WHEN cr.country <> '' THEN cr.country
			            ELSE COALESCE((
			                SELECT cr2.country FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.country <> ''
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), '')
			       END AS country,
			       COALESCE(mp.platforms, cr.platforms, '{}'::jsonb) AS platforms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       cr.latency_ms
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			LEFT JOIN merged_platforms mp ON mp.node_name = cr.node_name
			WHERE cr.job_id = $1 AND ` + aliveClause + `COALESCE(n.enabled, true) = true
		)
		SELECT config, node_name, country, platforms, speed_kbps, latency_ms
		FROM r
		ORDER BY ` + orderClause(prefs.Sort)
```

`aliveClause` still references `cr.alive = true` (fresh liveness) — unchanged. Params `$1=jobID`, `$2=subscriptionID` are already passed.

- [ ] **Step 4: Run the test (and the existing export suite) to verify it passes**

Run: `encore test ./services/checker/ -run 'TestLoadJobProxies|TestTaggedName'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/export_data.go services/checker/export_data_test.go
git commit -m "feat(checker): inherit platforms/country/speed in subscription export"
```

---

## Task 5: Frontend option helpers (`checkOptions.ts`)

Add the two pure helpers the UI needs: detect whether a subscription has stored prefs (to decide default-all vs intersect), and intersect a stored selection with the currently-available rule keys.

**Files:**
- Modify: `frontend/src/lib/checkOptions.ts`
- Test: `frontend/src/lib/checkOptions.test.ts`

- [ ] **Step 1: Write the failing tests**

Add to `frontend/src/lib/checkOptions.test.ts` (extend the import from `./checkOptions`):

```ts
import {
	DEFAULT_CHECK_OPTIONS,
	hasStoredCheckOptions,
	loadCheckOptions,
	reconcileMediaApps,
	saveCheckOptions,
} from "./checkOptions";

describe("checkOptions rule reconciliation", () => {
	it("hasStoredCheckOptions reflects whether prefs were saved", () => {
		const s = fakeStorage();
		expect(hasStoredCheckOptions("sub1", s)).toBe(false);
		saveCheckOptions("sub1", DEFAULT_CHECK_OPTIONS, s);
		expect(hasStoredCheckOptions("sub1", s)).toBe(true);
	});

	it("reconcileMediaApps drops keys not in the available set", () => {
		expect(
			reconcileMediaApps(["netflix", "gone", "disney"], ["netflix", "disney", "max"]),
		).toEqual(["netflix", "disney"]);
		expect(reconcileMediaApps([], ["netflix"])).toEqual([]);
	});
});
```

- [ ] **Step 2: Run them to verify they fail**

Run (from `frontend/`): `bun test src/lib/checkOptions.test.ts`
Expected: FAIL — `hasStoredCheckOptions`/`reconcileMediaApps` not exported.

- [ ] **Step 3: Implement the helpers**

Append to `frontend/src/lib/checkOptions.ts`:

```ts
export function hasStoredCheckOptions(
	subscriptionId: string,
	storage: Storage = localStorage,
): boolean {
	try {
		return storage.getItem(keyFor(subscriptionId)) !== null;
	} catch {
		return false;
	}
}

export function reconcileMediaApps(
	mediaApps: string[],
	availableKeys: string[],
): string[] {
	const allowed = new Set(availableKeys);
	return mediaApps.filter((k) => allowed.has(k));
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `bun test src/lib/checkOptions.test.ts`
Expected: PASS (existing persistence tests still green).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/checkOptions.ts frontend/src/lib/checkOptions.test.ts
git commit -m "feat(web): add check-option reconciliation helpers"
```

---

## Task 6: Rule-driven `CheckOptionsFields` + Alive-only preset

Make the platform list come from a prop (the user's rules) instead of the hardcoded `MEDIA_APPS`, add an always-tested note, and add "Alive only" / "All" presets.

**Files:**
- Modify: `frontend/src/components/check-options-fields.tsx`

- [ ] **Step 1: Replace the component**

Overwrite `frontend/src/components/check-options-fields.tsx`:

```tsx
import { RulePlatformIcon } from "@/components/rule-icon";
import { Checkbox } from "@/components/ui/checkbox";
import type { CheckFormOptions } from "@/lib/checkOptions";
import { cn } from "@/lib/utils";

// Controlled fieldset for check options. Immutable updates only — parent owns
// the state. `availablePlatforms` is the set of selectable platform keys,
// derived from the user's enabled rules by the parent. Connectivity + latency
// are always measured and are not represented here.
export function CheckOptionsFields({
	value,
	onChange,
	availablePlatforms,
	showDebug = false,
}: {
	value: CheckFormOptions;
	onChange: (next: CheckFormOptions) => void;
	availablePlatforms: string[];
	showDebug?: boolean;
}) {
	const toggleApp = (app: string) =>
		onChange({
			...value,
			media_apps: value.media_apps.includes(app)
				? value.media_apps.filter((a) => a !== app)
				: [...value.media_apps, app],
		});

	const aliveOnly = () =>
		onChange({
			...value,
			speed_test: false,
			upload_speed_test: false,
			media_apps: [],
		});

	const selectAll = () =>
		onChange({ ...value, media_apps: [...availablePlatforms] });

	return (
		<div className="space-y-3">
			<p className="rounded-md bg-secondary px-2 py-1.5 text-[11px] text-muted-foreground">
				Connectivity + latency: always tested.
			</p>

			<div className="flex items-center justify-between">
				<p className="font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Check options
				</p>
				<button
					type="button"
					onClick={aliveOnly}
					className="rounded-full border border-border px-2 py-0.5 text-[11px] text-muted-foreground hover:bg-secondary"
				>
					Alive only
				</button>
			</div>

			<div className="space-y-1.5">
				<label className="flex cursor-pointer items-center gap-2 text-sm">
					<Checkbox
						checked={value.speed_test}
						onCheckedChange={(v) => onChange({ ...value, speed_test: v === true })}
					/>
					Speed test <span className="text-muted-foreground text-xs">(download)</span>
				</label>
				<label className="flex cursor-pointer items-center gap-2 text-sm">
					<Checkbox
						checked={value.upload_speed_test}
						onCheckedChange={(v) =>
							onChange({ ...value, upload_speed_test: v === true })
						}
					/>
					Upload test
				</label>
				{showDebug ? (
					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={value.debug}
							onCheckedChange={(v) => onChange({ ...value, debug: v === true })}
						/>
						Debug mode
					</label>
				) : null}
			</div>

			<div>
				<div className="mb-1.5 flex items-center justify-between">
					<p className="font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
						Media platforms
					</p>
					<button
						type="button"
						onClick={selectAll}
						className="text-[11px] text-muted-foreground hover:text-foreground"
					>
						All
					</button>
				</div>
				<div className="flex flex-wrap gap-1.5">
					{availablePlatforms.map((app) => {
						const active = value.media_apps.includes(app);
						return (
							<button
								key={app}
								type="button"
								aria-pressed={active}
								onClick={() => toggleApp(app)}
								className={cn(
									"inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors",
									active
										? "border-info-line bg-info-muted text-info"
										: "border-border text-muted-foreground hover:bg-secondary",
								)}
							>
								<RulePlatformIcon platformKey={app} size={12} showLabel />
							</button>
						);
					})}
				</div>
			</div>
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

Run (from `frontend/`): `bun check-types`
Expected: FAIL — `run-check-button.tsx` and `schedule-dialog.tsx` don't yet pass `availablePlatforms`. (Fixed in Tasks 7-8.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/check-options-fields.tsx
git commit -m "feat(web): rule-driven platform list + alive-only preset in check options"
```

---

## Task 7: Wire `run-check-button` to rules

Fetch the user's rules, derive selectable keys, reconcile the persisted selection once rules load, and pass `availablePlatforms`.

**Files:**
- Modify: `frontend/src/components/workbench/run-check-button.tsx`

- [ ] **Step 1: Update imports**

Replace the React import and add the helpers/query. At the top of `run-check-button.tsx`:

```tsx
import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, Play } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { CheckOptionsFields } from "@/components/check-options-fields";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	type CheckFormOptions,
	hasStoredCheckOptions,
	loadCheckOptions,
	reconcileMediaApps,
	saveCheckOptions,
} from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import { queryKeys, useRules, useTriggerCheck } from "@/queries";
```

- [ ] **Step 2: Derive platforms + reconcile selection**

Inside `RunCheckButton`, immediately after the existing `const [opts, setOpts] = useState(...)` line, add:

```tsx
	const { data: rules } = useRules();
	const availablePlatforms = useMemo(
		() => (rules ?? []).filter((r) => r.enabled).map((r) => r.key),
		[rules],
	);
	const reconciled = useRef(false);
	useEffect(() => {
		if (!rules || reconciled.current) return;
		reconciled.current = true;
		setOpts((prev) => ({
			...prev,
			media_apps: hasStoredCheckOptions(subscriptionId)
				? reconcileMediaApps(prev.media_apps, availablePlatforms)
				: availablePlatforms,
		}));
	}, [rules, availablePlatforms, subscriptionId]);
```

- [ ] **Step 3: Pass the prop**

Change the `CheckOptionsFields` usage (line ~88) to:

```tsx
						<CheckOptionsFields
							value={opts}
							onChange={setOpts}
							availablePlatforms={availablePlatforms}
							showDebug
						/>
```

- [ ] **Step 4: Type-check**

Run (from `frontend/`): `bun check-types`
Expected: FAIL only in `schedule-dialog.tsx` now (fixed next task).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/workbench/run-check-button.tsx
git commit -m "feat(web): drive Run Check platform list from rules + reconcile selection"
```

---

## Task 8: Wire `schedule-dialog` to rules

Same wiring for the scheduler: default a new schedule's selection to all available platforms, intersect an edited schedule's stored selection, and pass `availablePlatforms`.

**Files:**
- Modify: `frontend/src/components/schedule-dialog.tsx`

- [ ] **Step 1: Update imports**

Add `useMemo` and the query/helper. Adjust the existing imports:

```tsx
import { useEffect, useMemo, useState } from "react";
```

```tsx
import {
	type CheckFormOptions,
	DEFAULT_CHECK_OPTIONS,
	reconcileMediaApps,
} from "@/lib/checkOptions";
```

```tsx
import { useCreateScheduledJob, useRules } from "@/queries";
```

- [ ] **Step 2: Derive platforms and use them in the open-effect**

After `const [opts, setOpts] = useState<CheckFormOptions>(DEFAULT_CHECK_OPTIONS);` add:

```tsx
	const { data: rules } = useRules();
	const availablePlatforms = useMemo(
		() => (rules ?? []).filter((r) => r.enabled).map((r) => r.key),
		[rules],
	);
```

Then replace the `media_apps` line inside the `useEffect` (line ~64) so a new schedule defaults to all available platforms and an edited one is intersected:

```tsx
				media_apps: editing?.media_apps
					? reconcileMediaApps(editing.media_apps, availablePlatforms)
					: availablePlatforms.length > 0
						? availablePlatforms
						: [...DEFAULT_CHECK_OPTIONS.media_apps],
```

Add `availablePlatforms` to the effect's dependency array: `}, [open, editing, availablePlatforms]);`

- [ ] **Step 3: Pass the prop**

Change the `CheckOptionsFields` usage (line ~160) to:

```tsx
						<CheckOptionsFields
							value={opts}
							onChange={setOpts}
							availablePlatforms={availablePlatforms}
						/>
```

- [ ] **Step 4: Type-check + lint**

Run (from `frontend/`): `bun check-types && bun check`
Expected: PASS (no remaining `availablePlatforms` errors).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/schedule-dialog.tsx
git commit -m "feat(web): drive Schedule dialog platform list from rules"
```

---

## Task 9: Full verification

**Files:** none (verification only).

- [ ] **Step 1: Backend test suite**

Run: `encore test ./...`
Expected: PASS. (No `-race`.)

- [ ] **Step 2: Frontend checks + build**

Run (from `frontend/`): `bun test && bun check-types && bun check && bun build`
Expected: PASS.

- [ ] **Step 3: Manual smoke (both servers running)**

With `encore run` + `bun dev`:
1. Run a full check on a subscription → results show speed + platforms + country.
2. Open Run Check → click **Alive only** → speed/upload off, platforms cleared → start.
3. After it finishes: the results table still shows the previous speed/platform/country values (inherited), latency is freshly updated, and genuinely-dead nodes are dropped.
4. Export the subscription → node names still carry the streaming/country/speed tags from the earlier full check.
5. Toggle a subset of platforms (e.g. only Netflix) → run → only Netflix is re-tested; other platforms keep their prior status.

- [ ] **Step 4: Final commit (if any cleanup)**

```bash
git add -A
git commit -m "chore(checker): partial-check inheritance + alive-only verification"
```

---

## Self-Review Notes

- **Spec coverage:** rule filtering (T1), alive-only expressibility (T2), read-time inheritance for speed/upload/country/platforms in both read paths (T3, T4), rule-driven UI + alive-only preset + always-tested note (T6), default/intersect selection (T5, T7, T8), scheduler reuse (T8). Country inheritance included (T3, T4). Fresh-liveness export filter preserved (T4).
- **No migration / no client regen:** response shapes unchanged.
- **Edge cases** (per spec) hold: `node_name` keying, global "latest" semantics, never-tested → empty, dead-node inherited rows dropped from export, bare-API omitted-options default.
