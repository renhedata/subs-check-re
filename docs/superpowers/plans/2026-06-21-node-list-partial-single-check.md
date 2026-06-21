# Node List + Whole/Partial/Single Checks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the persisted node list the primary surface (visible right after fetch/import) and allow checking all nodes, a selected subset, or a single node — where a partial check updates only the checked nodes and leaves every other node's last result intact.

**Architecture:** A new read-only `ListNodes` endpoint returns each persisted node enriched with its latest-known result (per `server:port` identity, same inheritance rule as `GetResults`). `TriggerCheck` gains an optional `node_ids`, stored in a new `check_jobs.node_ids` column; the job runner scopes the run to that subset and never re-fetches when a selection is given. The frontend renders the node list (via `ListNodes`) as the default view with per-row checkboxes, a "Check selected" button, and a per-row "Check this node" action; the existing per-job results view is kept for history.

**Tech Stack:** Go + Encore (PostgreSQL via `sqldb`), React 19 + TanStack Query/Router, Bun, Biome.

**Spec:** `docs/superpowers/specs/2026-06-21-node-list-partial-single-check-design.md`

**Conventions for every backend test step:** run with `encore test ./services/checker/ -run <TestName>` (NEVER pass `-race` — it hangs this harness). Commit messages omit any `Co-Authored-By` trailer. Stage files by name; never `git add -A`. Do NOT stage the pre-existing untracked files under `docs/superpowers/plans/` other than ones this plan creates.

---

## File Structure

**Backend (`services/checker/`):**
- Create: `migrations/10_add_node_ids_to_jobs.up.sql`, `migrations/10_add_node_ids_to_jobs.down.sql` — add `node_ids JSONB` to `check_jobs`.
- Create: `list_nodes.go` — `Node`, `ListNodesResponse`, `ListNodes` endpoint, `jobStore.listNodes`.
- Create: `list_nodes_test.go` — tests for `listNodes`.
- Modify: `jobstore.go` — `jobConfig.NodeIDs`; `loadConfig` reads `node_ids`.
- Modify: `jobrunner.go` — `filterNodesByIDs`; scope `run()` to the subset; never bootstrap-fetch on a partial run.
- Modify: `checker.go` — `TriggerParams.NodeIDs`; persist it in `TriggerCheck`'s INSERT.
- Create test: `partial_check_test.go` — runner subset scoping + no-fetch.

**Frontend (`frontend/src/`):**
- Modify: `queries/queryKeys.ts` — add `nodes(subId)`.
- Create: `queries/nodes.ts` — `useNodes`.
- Modify: `queries/index.ts` — export `./nodes`.
- Modify: `queries/jobs.ts` — invalidate `nodes` in import/refresh/trigger/setNodeEnabled.
- Modify: `queries/subscriptions.ts` — invalidate `nodes` in `useSetFetchProxy`.
- Modify: `components/workbench/node-table.tsx` — optional selection + per-row check.
- Modify: `components/workbench/results-section.tsx` — optional selection toolbar + pass-through.
- Modify: `components/workbench/detail-pane.tsx` — node-list view, selection → check, refetch on done, history snapshot path.
- Regenerate: `lib/client.gen.ts`.

---

## Task 1: Migration — add `node_ids` to `check_jobs`

**Files:**
- Create: `services/checker/migrations/10_add_node_ids_to_jobs.up.sql`
- Create: `services/checker/migrations/10_add_node_ids_to_jobs.down.sql`

- [ ] **Step 1: Write the up migration**

`services/checker/migrations/10_add_node_ids_to_jobs.up.sql`:
```sql
-- node_ids restricts a check to a subset of the subscription's nodes.
-- NULL or empty array = check all nodes (the default / scheduled behavior).
ALTER TABLE check_jobs ADD COLUMN node_ids JSONB;
```

- [ ] **Step 2: Write the down migration**

`services/checker/migrations/10_add_node_ids_to_jobs.down.sql`:
```sql
ALTER TABLE check_jobs DROP COLUMN node_ids;
```

- [ ] **Step 3: Apply migrations**

Run: `encore db migrate`
Expected: no error; the migration applies. (If `encore run` is active it auto-migrates on next build.)

- [ ] **Step 4: Commit**

```bash
git add services/checker/migrations/10_add_node_ids_to_jobs.up.sql services/checker/migrations/10_add_node_ids_to_jobs.down.sql
git commit -m "feat(checker): add node_ids column to check_jobs for partial checks"
```

---

## Task 2: `loadConfig` reads `node_ids`

**Files:**
- Modify: `services/checker/jobstore.go` (the `jobConfig` struct ~line 22, `loadConfig` ~line 29)
- Test: `services/checker/jobstore_nodeids_test.go` (Create)

- [ ] **Step 1: Write the failing test**

`services/checker/jobstore_nodeids_test.go`:
```go
package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// loadConfig must surface node_ids when present, and leave it empty when NULL.
func TestLoadConfigReadsNodeIDs(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()

	// Job with an explicit node subset.
	jobWithIDs := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at, node_ids)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4,$5::jsonb)
	`, jobWithIDs, subID, uuid.New().String(), time.Now(), `["a","b"]`); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	cfg, err := defaultJobStore.loadConfig(ctx, jobWithIDs)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.NodeIDs) != 2 || cfg.NodeIDs[0] != "a" || cfg.NodeIDs[1] != "b" {
		t.Fatalf("expected [a b], got %v", cfg.NodeIDs)
	}

	// Job with no subset (NULL) → empty.
	jobAll := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4)
	`, jobAll, subID, uuid.New().String(), time.Now()); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	cfgAll, err := defaultJobStore.loadConfig(ctx, jobAll)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfgAll.NodeIDs) != 0 {
		t.Fatalf("expected no node_ids, got %v", cfgAll.NodeIDs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestLoadConfigReadsNodeIDs`
Expected: FAIL — `cfg.NodeIDs` undefined (field doesn't exist yet).

- [ ] **Step 3: Add the field and read it**

In `services/checker/jobstore.go`, add `NodeIDs` to `jobConfig`:
```go
type jobConfig struct {
	SubURL         string
	SpeedTestURL   string
	LatencyTestURL string
	Options        CheckOptions
	NodeIDs        []string
}
```

Replace the body of `loadConfig` so it also selects and decodes `node_ids`:
```go
func (s *jobStore) loadConfig(ctx context.Context, jobID string) (*jobConfig, error) {
	var cfg jobConfig
	var optsJSON []byte
	var nodeIDsJSON string
	if err := db.QueryRow(ctx,
		`SELECT sub_url, COALESCE(speed_test_url, ''), COALESCE(latency_test_url, ''),
		        COALESCE(options_json, '{}'), COALESCE(node_ids::text, '')
		 FROM check_jobs WHERE id=$1`,
		jobID).Scan(&cfg.SubURL, &cfg.SpeedTestURL, &cfg.LatencyTestURL, &optsJSON, &nodeIDsJSON); err != nil {
		return nil, fmt.Errorf("load job config: %w", err)
	}
	if cfg.SubURL == "" {
		return nil, fmt.Errorf("job %s has no subscription URL", jobID)
	}
	if cfg.SpeedTestURL == "" {
		cfg.SpeedTestURL = defaultSpeedTestURL
	}
	if err := json.Unmarshal(optsJSON, &cfg.Options); err != nil {
		cfg.Options = defaultCheckOptions()
	}
	if nodeIDsJSON != "" {
		_ = json.Unmarshal([]byte(nodeIDsJSON), &cfg.NodeIDs)
	}
	return &cfg, nil
}
```

(`encoding/json` and `fmt` are already imported in this file.)

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestLoadConfigReadsNodeIDs`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/jobstore.go services/checker/jobstore_nodeids_test.go
git commit -m "feat(checker): loadConfig surfaces job node_ids subset"
```

---

## Task 3: Runner scopes the run to the subset (and never fetches on a partial run)

**Files:**
- Modify: `services/checker/jobrunner.go` (`run()` ~lines 84-110; add `filterNodesByIDs`)
- Test: `services/checker/partial_check_test.go` (Create)

- [ ] **Step 1: Write the failing test**

`services/checker/partial_check_test.go`:
```go
package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// failIfCalledFetcher fails the test if the runner attempts a fetch. A partial
// check (node_ids set) must test existing nodes only and never re-fetch.
type failIfCalledFetcher struct{ t *testing.T }

func (f failIfCalledFetcher) Fetch(_ context.Context, _ string) ([]map[string]any, error) {
	f.t.Fatal("fetcher must not be called on a partial check")
	return nil, nil
}

// A partial check writes results only for the selected nodes and sets the job
// total to the subset size; non-selected nodes get no new row.
func TestPartialCheckScopesToSubset(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	userID := uuid.New().String()

	// Two persisted nodes.
	ids, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
		{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222},
	})
	if err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}

	// Job selecting only node A.
	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at, node_ids)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4,$5::jsonb)
	`, jobID, subID, userID, time.Now(), `["`+ids[0]+`"]`); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: failIfCalledFetcher{t},
		bus:     newJobBus(),
		check: func(_ context.Context, nodeID string, _ map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule) nodeCheckResult {
			return nodeCheckResult{NodeID: nodeID, NodeName: "A", Alive: true, LatencyMs: 10}
		},
	}
	r.run(ctx, jobID, subID, userID)

	// Exactly one result row (node A); job total == 1.
	var rows int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM check_results WHERE job_id=$1`, jobID).Scan(&rows); err != nil {
		t.Fatalf("count results: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 result row, got %d", rows)
	}
	var total int
	if err := db.QueryRow(ctx, `SELECT total FROM check_jobs WHERE id=$1`, jobID).Scan(&total); err != nil {
		t.Fatalf("read total: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
}
```

> Note: `newJobBus()` is the constructor used by existing tests in this package (see `jobbus.go`). If the existing tests use a different constructor name, use that one.

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestPartialCheckScopesToSubset`
Expected: FAIL — currently the runner checks all nodes (2 rows / total 2), or `filterNodesByIDs` is undefined.

- [ ] **Step 3: Add `filterNodesByIDs` and scope the run**

In `services/checker/jobrunner.go`, add the helper near `nodeKeyForProxy`:
```go
// filterNodesByIDs returns only the nodes whose id is in ids, preserving the
// input order. An empty ids slice means "all nodes" and returns the input as-is.
func filterNodesByIDs(nodes []storedNode, ids []string) []storedNode {
	if len(ids) == 0 {
		return nodes
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := make([]storedNode, 0, len(ids))
	for _, n := range nodes {
		if want[n.id] {
			out = append(out, n)
		}
	}
	return out
}
```

Replace the node-loading block in `run()` (currently lines ~84-110) with:
```go
	existing, err := r.store.loadNodes(context.Background(), subscriptionID)
	if err != nil {
		fail("load nodes", err)
		return
	}

	// An explicit node selection scopes the run to those nodes and never
	// re-fetches; only a whole-subscription run with no nodes yet bootstraps.
	partial := len(cfg.NodeIDs) > 0
	if partial {
		existing = filterNodesByIDs(existing, cfg.NodeIDs)
	}

	var proxies []map[string]any
	var nodeIDs []string
	if !partial && len(existing) == 0 && cfg.SubURL != "" {
		proxies, err = fetchWithRetry(ctx, r.fetcher, cfg.SubURL)
		if err != nil {
			fail("fetch subscription", err)
			return
		}
		nodeIDs, err = r.store.replaceNodes(context.Background(), subscriptionID, proxies)
		if err != nil {
			fail("replace nodes", err)
			return
		}
	} else {
		proxies = make([]map[string]any, len(existing))
		nodeIDs = make([]string, len(existing))
		for i, n := range existing {
			proxies[i] = n.config
			nodeIDs[i] = n.id
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestPartialCheckScopesToSubset`
Expected: PASS.

- [ ] **Step 5: Run the full checker suite (no regressions)**

Run: `encore test ./services/checker/`
Expected: PASS (existing lifecycle/inheritance tests still green).

- [ ] **Step 6: Commit**

```bash
git add services/checker/jobrunner.go services/checker/partial_check_test.go
git commit -m "feat(checker): scope check run to selected node subset; no fetch on partial"
```

---

## Task 4: `TriggerCheck` accepts and persists `node_ids`

**Files:**
- Modify: `services/checker/checker.go` (`TriggerParams` ~line 169; `TriggerCheck` INSERT ~lines 306-314)

- [ ] **Step 1: Add the field to `TriggerParams`**

In `services/checker/checker.go`, extend `TriggerParams`:
```go
type TriggerParams struct {
	SpeedTest       *bool    `json:"speed_test"`
	UploadSpeedTest *bool    `json:"upload_speed_test"`
	MediaApps       []string `json:"media_apps"`
	Debug           *bool    `json:"debug"`
	NodeIDs         []string `json:"node_ids"`
}
```

- [ ] **Step 2: Persist it in the INSERT**

In `TriggerCheck`, just before the `db.Exec` INSERT (after `optsJSON, _ := json.Marshal(opts)`), compute the node_ids payload:
```go
	var nodeIDsArg any
	if p != nil && len(p.NodeIDs) > 0 {
		b, _ := json.Marshal(p.NodeIDs)
		nodeIDsArg = string(b)
	}
```

Replace the INSERT statement and its args with:
```go
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, speed_test_url, latency_test_url, options_json, status, created_at, node_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'queued', $8, $9::jsonb)
	`, jobID, subscriptionID, claims.UserID, sub.URL, speedTestURL, latencyTestURL, optsJSON, time.Now(), nodeIDsArg); err != nil {
```

(A `nil` `nodeIDsArg` stores `NULL::jsonb` ⇒ "all nodes". `TriggerCheckInternal` is unchanged — scheduled checks always cover all nodes.)

- [ ] **Step 3: Verify it builds and the suite passes**

Run: `encore test ./services/checker/`
Expected: PASS (compiles; existing tests unaffected).

- [ ] **Step 4: Commit**

```bash
git add services/checker/checker.go
git commit -m "feat(checker): TriggerCheck accepts node_ids for partial/single checks"
```

---

## Task 5: `ListNodes` endpoint + `listNodes` store method

**Files:**
- Create: `services/checker/list_nodes.go`
- Create: `services/checker/list_nodes_test.go`

- [ ] **Step 1: Write the failing tests**

`services/checker/list_nodes_test.go`:
```go
package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// insertJob creates a minimal completed job row so insertResult's FK is satisfied.
func insertJob(t *testing.T, ctx context.Context, subID, userID string) string {
	t.Helper()
	id := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://x','{}','completed',$4)
	`, id, subID, userID, time.Now()); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	return id
}

// A node that has never been checked shows blank metrics and a nil last_checked_at.
func TestListNodesNeverChecked(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	if _, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
	}); err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}

	nodes, err := defaultJobStore.listNodes(ctx, subID)
	if err != nil {
		t.Fatalf("listNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.Alive || n.LatencyMs != 0 || n.SpeedKbps != 0 || n.LastCheckedAt != nil {
		t.Fatalf("unchecked node should be blank: %+v", n)
	}
}

// listNodes shows latest-per-node values; checking one node does not change another.
func TestListNodesLatestAndIsolation(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	userID := uuid.New().String()
	ids, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
		{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222},
	})
	if err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}
	proxyA := map[string]any{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111}
	proxyB := map[string]any{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222}

	// Job 1: both nodes measured fully.
	job1 := insertJob(t, ctx, subID, userID)
	if err := defaultJobStore.insertResult(ctx, job1, ids[0], proxyA, nodeCheckResult{
		NodeName: "A", Alive: true, LatencyMs: 30, SpeedKbps: 500, Country: "US",
		Platforms: map[string]PlatformOutcome{"netflix": {Unlocked: true}},
	}); err != nil {
		t.Fatalf("insert A: %v", err)
	}
	if err := defaultJobStore.insertResult(ctx, job1, ids[1], proxyB, nodeCheckResult{
		NodeName: "B", Alive: true, LatencyMs: 80, SpeedKbps: 200, Country: "JP",
	}); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	// Job 2: alive-only re-check of A (speed 0, no platforms, no country).
	job2 := insertJob(t, ctx, subID, userID)
	if err := defaultJobStore.insertResult(ctx, job2, ids[0], proxyA, nodeCheckResult{
		NodeName: "A", Alive: true, LatencyMs: 25,
	}); err != nil {
		t.Fatalf("insert A2: %v", err)
	}

	nodes, err := defaultJobStore.listNodes(ctx, subID)
	if err != nil {
		t.Fatalf("listNodes: %v", err)
	}
	byName := map[string]Node{}
	for _, n := range nodes {
		byName[n.NodeName] = n
	}
	a, b := byName["A"], byName["B"]

	// A: latency from latest (25), speed/country/platforms inherited from job1.
	if a.LatencyMs != 25 || a.SpeedKbps != 500 || a.Country != "US" {
		t.Fatalf("A latest+inherited wrong: %+v", a)
	}
	if o, ok := a.Platforms["netflix"]; !ok || !o.Unlocked {
		t.Fatalf("A should inherit netflix unlock: %+v", a.Platforms)
	}
	if a.LastCheckedAt == nil {
		t.Fatalf("A should have a last_checked_at")
	}
	// B: untouched by job2 — still job1's values.
	if b.LatencyMs != 80 || b.SpeedKbps != 200 || b.Country != "JP" {
		t.Fatalf("B should be unchanged: %+v", b)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `encore test ./services/checker/ -run TestListNodes`
Expected: FAIL — `listNodes` / `Node.LastCheckedAt` undefined.

- [ ] **Step 3: Implement `list_nodes.go`**

`services/checker/list_nodes.go`:
```go
// services/checker/list_nodes.go
package checker

import (
	"context"
	"encoding/json"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
	subsvc "subs-check-re/services/subscription"
)

// Node is a persisted node enriched with its latest-known result. Metrics are
// zero / Platforms empty / LastCheckedAt nil for a node that has never been
// checked. Inheritance mirrors GetResults: alive/latency/ip come from the most
// recent result row; speed/upload/country/platforms take the latest non-empty
// value per node identity (server:port).
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

// ListNodesResponse is returned by GET /subscription/:subscriptionID/nodes.
type ListNodesResponse struct {
	Nodes []Node `json:"nodes"`
}

// ListNodes returns the subscription's persisted nodes with their latest-known
// results, so the UI can display nodes immediately after fetch/import — before
// any check has run.
//
//encore:api auth method=GET path=/subscription/:subscriptionID/nodes
func ListNodes(ctx context.Context, subscriptionID string) (*ListNodesResponse, error) {
	_ = encauth.Data().(*authsvc.UserClaims)
	// Ownership: GetSubscription is user-scoped and errors if not owned.
	if _, err := subsvc.GetSubscription(ctx, subscriptionID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}

	nodes, err := defaultJobStore.listNodes(ctx, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	return &ListNodesResponse{Nodes: nodes}, nil
}

// listNodes is the read-only query behind ListNodes. It is a store method so it
// can be tested without an auth context.
func (s *jobStore) listNodes(ctx context.Context, subscriptionID string) ([]Node, error) {
	crKey := nodeIdentityKey("cr")
	// nodes-table identity must match the check_results identity (server:port
	// from the proxy config, falling back to the display name).
	nodeKey := `CASE WHEN COALESCE(n.config->>'server','') <> '' ` +
		`THEN (n.config->>'server') || ':' || COALESCE(n.config->>'port','') ` +
		`ELSE n.name END`

	rows, err := db.Query(ctx, `
		WITH node_keys AS (
			SELECT n.id, n.name, COALESCE(n.type,'') AS type, n.enabled,
			       COALESCE(n.server,'') AS server, COALESCE(n.port,0) AS port,
			       COALESCE(n.config::text,'') AS config,
			       (`+nodeKey+`) AS node_key
			FROM nodes n
			WHERE n.subscription_id = $1
		),
		hist AS (
			SELECT `+crKey+` AS node_key, cr.alive, cr.latency_ms, cr.ip,
			       cr.speed_kbps, cr.upload_speed_kbps, cr.country, cr.platforms,
			       cr.traffic_bytes, cr.checked_at
			FROM check_results cr
			JOIN check_jobs cj ON cj.id = cr.job_id
			WHERE cj.subscription_id = $1
		),
		latest AS (
			SELECT DISTINCT ON (node_key) node_key, alive, latency_ms, ip, traffic_bytes, checked_at
			FROM hist ORDER BY node_key, checked_at DESC
		),
		spd AS (
			SELECT DISTINCT ON (node_key) node_key, speed_kbps
			FROM hist WHERE speed_kbps > 0 ORDER BY node_key, checked_at DESC
		),
		upl AS (
			SELECT DISTINCT ON (node_key) node_key, upload_speed_kbps
			FROM hist WHERE upload_speed_kbps > 0 ORDER BY node_key, checked_at DESC
		),
		ctry AS (
			SELECT DISTINCT ON (node_key) node_key, country
			FROM hist WHERE country <> '' ORDER BY node_key, checked_at DESC
		),
		plat_kv AS (
			SELECT DISTINCT ON (node_key, kv.key) node_key, kv.key AS key, kv.value AS value
			FROM hist CROSS JOIN LATERAL jsonb_each(hist.platforms) AS kv(key, value)
			WHERE hist.platforms IS NOT NULL AND hist.platforms <> '{}'::jsonb
			ORDER BY node_key, kv.key, hist.checked_at DESC
		),
		plat AS (
			SELECT node_key, jsonb_object_agg(key, value) AS platforms
			FROM plat_kv GROUP BY node_key
		)
		SELECT nk.id, nk.name, nk.type, nk.enabled, nk.server, nk.port, nk.config,
		       COALESCE(latest.alive, false), COALESCE(latest.latency_ms, 0),
		       COALESCE(spd.speed_kbps, 0), COALESCE(upl.upload_speed_kbps, 0),
		       COALESCE(ctry.country, ''), COALESCE(latest.ip, ''),
		       COALESCE(plat.platforms, '{}'::jsonb),
		       COALESCE(latest.traffic_bytes, 0),
		       latest.checked_at
		FROM node_keys nk
		LEFT JOIN latest ON latest.node_key = nk.node_key
		LEFT JOIN spd  ON spd.node_key  = nk.node_key
		LEFT JOIN upl  ON upl.node_key  = nk.node_key
		LEFT JOIN ctry ON ctry.node_key = nk.node_key
		LEFT JOIN plat ON plat.node_key = nk.node_key
		ORDER BY nk.name
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Node
	for rows.Next() {
		var n Node
		var platformsJSON []byte
		var checkedAt *time.Time
		if err := rows.Scan(
			&n.NodeID, &n.NodeName, &n.NodeType, &n.Enabled, &n.Server, &n.Port, &n.Config,
			&n.Alive, &n.LatencyMs, &n.SpeedKbps, &n.UploadSpeedKbps, &n.Country, &n.IP,
			&platformsJSON, &n.TrafficBytes, &checkedAt,
		); err != nil {
			return nil, err
		}
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &n.Platforms)
		}
		if n.Platforms == nil {
			n.Platforms = map[string]PlatformOutcome{}
		}
		n.LastCheckedAt = checkedAt
		out = append(out, n)
	}
	if out == nil {
		out = []Node{}
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `encore test ./services/checker/ -run TestListNodes`
Expected: PASS.

- [ ] **Step 5: Run the full checker suite**

Run: `encore test ./services/checker/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/checker/list_nodes.go services/checker/list_nodes_test.go
git commit -m "feat(checker): ListNodes endpoint with latest-known per-node results"
```

---

## Task 6: Regenerate the frontend API client

**Files:**
- Modify: `frontend/src/lib/client.gen.ts` (generated)

- [ ] **Step 1: Ensure the backend builds, then regenerate**

Run:
```bash
encore gen client subs-check-uqti --lang=typescript --output=./frontend/src/lib/client.gen.ts
```
Expected: file updates; `checker.Node`, `checker.ListNodesResponse`, `checker.ListNodes`, and `node_ids` on `checker.TriggerParams` now exist.

- [ ] **Step 2: Verify the new symbols are present**

Run: `grep -n "ListNodes\|node_ids\|last_checked_at" frontend/src/lib/client.gen.ts | head`
Expected: matches for `ListNodes`, `node_ids`, and `last_checked_at`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/client.gen.ts
git commit -m "chore(client): regenerate for ListNodes + node_ids"
```

---

## Task 7: `useNodes` query + key + barrel export

**Files:**
- Modify: `frontend/src/queries/queryKeys.ts`
- Create: `frontend/src/queries/nodes.ts`
- Modify: `frontend/src/queries/index.ts`

- [ ] **Step 1: Add the query key**

In `frontend/src/queries/queryKeys.ts`, add inside the `queryKeys` object (after the `results` entry):
```ts
	nodes: (subscriptionId: string) => ["nodes", subscriptionId] as const,
```

- [ ] **Step 2: Create the query hook**

`frontend/src/queries/nodes.ts`:
```ts
import { useQuery } from "@tanstack/react-query";
import { client } from "../lib/client";
import { queryKeys } from "./queryKeys";

// useNodes returns the subscription's persisted nodes with their latest-known
// results. Populated right after fetch/import — independent of any check job.
export function useNodes(subscriptionId: string) {
	return useQuery({
		queryKey: queryKeys.nodes(subscriptionId),
		queryFn: () => client.checker.ListNodes(subscriptionId),
		enabled: !!subscriptionId,
	});
}
```

- [ ] **Step 3: Export from the barrel**

In `frontend/src/queries/index.ts`, add:
```ts
export * from "./nodes";
```

- [ ] **Step 4: Type-check**

Run: `cd frontend && bun run check-types`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/queries/queryKeys.ts frontend/src/queries/nodes.ts frontend/src/queries/index.ts
git commit -m "feat(web): useNodes query for the persisted node list"
```

---

## Task 8: Invalidate the nodes query on node-changing mutations

**Files:**
- Modify: `frontend/src/queries/jobs.ts`
- Modify: `frontend/src/queries/subscriptions.ts`

- [ ] **Step 1: Invalidate in import / refresh / trigger / setNodeEnabled**

In `frontend/src/queries/jobs.ts`, add a nodes invalidation to each `onSuccess` below.

`useTriggerCheck` `onSuccess(_data, vars)` — add:
```ts
				qc.invalidateQueries({ queryKey: queryKeys.nodes(vars.subscriptionId) });
```

`useSetNodeEnabled` `onSuccess` — add:
```ts
				qc.invalidateQueries({ queryKey: queryKeys.nodes(subscriptionId) });
```

`useImportNodes` `onSuccess` — add:
```ts
				qc.invalidateQueries({ queryKey: queryKeys.nodes(subscriptionId) });
```

`useRefreshSubscription` `onSuccess` — add:
```ts
				qc.invalidateQueries({ queryKey: queryKeys.nodes(subscriptionId) });
```

- [ ] **Step 2: Invalidate in setFetchProxy**

In `frontend/src/queries/subscriptions.ts`, `useSetFetchProxy` `onSuccess` — add (alongside the existing subscriptions invalidation):
```ts
			qc.invalidateQueries({ queryKey: queryKeys.nodes(args.id) });
```

- [ ] **Step 3: Type-check**

Run: `cd frontend && bun run check-types`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/queries/jobs.ts frontend/src/queries/subscriptions.ts
git commit -m "feat(web): refresh node list when nodes change"
```

---

## Task 9: NodeTable — optional selection + per-row check

**Files:**
- Modify: `frontend/src/components/workbench/node-table.tsx`

These props are all optional, so the historical job-snapshot rendering (which omits them) is unchanged.

- [ ] **Step 1: Extend `NodeTableProps`**

Replace the `NodeTableProps` interface:
```ts
export interface NodeTableProps {
	results: NodeResult[];
	rules?: PlatformRule[];
	sortKey: SortKey;
	sortDir: SortDir;
	onSort: (key: SortKey) => void;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
	// Selection + per-row check (node-list view only).
	selectable?: boolean;
	selectedIds?: Set<string>;
	onToggleSelect?: (nodeId: string) => void;
	onToggleSelectAll?: (visibleIds: string[], select: boolean) => void;
	onCheckNode?: (nodeId: string) => void;
	checkDisabled?: boolean;
}
```

- [ ] **Step 2: Accept the new props**

Update the function signature destructuring and add the `allSelected` derived value:
```ts
export function NodeTable({
	results,
	rules = [],
	sortKey,
	sortDir,
	onSort,
	onToggleEnabled,
	selectable,
	selectedIds,
	onToggleSelect,
	onToggleSelectAll,
	onCheckNode,
	checkDisabled,
}: NodeTableProps) {
	const ruleByKey = Object.fromEntries(rules.map((r) => [r.key, r]));
	const [detail, setDetail] = useState<NodeResult | null>(null);
	const allSelected =
		selectable &&
		results.length > 0 &&
		results.every((r) => selectedIds?.has(r.node_id));
```

- [ ] **Step 3: Add the desktop header cells**

In the desktop `<thead><tr>`, immediately before the existing `<th className="w-8 px-2 py-2" aria-label="Enabled" />`, insert:
```tsx
								{selectable ? (
									<th className="w-8 px-2 py-2">
										<input
											type="checkbox"
											aria-label="Select all"
											checked={!!allSelected}
											onChange={(e) =>
												onToggleSelectAll?.(
													results.map((r) => r.node_id),
													e.target.checked,
												)
											}
										/>
									</th>
								) : null}
```

And after the "Unlocks" header `<th>`, add a trailing actions header:
```tsx
								{onCheckNode ? (
									<th className="px-3 py-2 text-right font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
										Check
									</th>
								) : null}
```

- [ ] **Step 4: Add the desktop row cells**

In the desktop `<tbody>` row, immediately before `<td className="px-2 py-1.5"><EnableToggle .../></td>`, insert:
```tsx
									{selectable ? (
										<td className="px-2 py-1.5">
											<input
												type="checkbox"
												aria-label={`Select ${r.node_name}`}
												checked={!!selectedIds?.has(r.node_id)}
												onChange={() => onToggleSelect?.(r.node_id)}
											/>
										</td>
									) : null}
```

After the Unlocks `<td>` (the last cell in the row), add:
```tsx
									{onCheckNode ? (
										<td className="px-3 py-1.5 text-right">
											<button
												type="button"
												disabled={checkDisabled}
												onClick={() => onCheckNode(r.node_id)}
												className="rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-secondary disabled:opacity-40"
											>
												Check
											</button>
										</td>
									) : null}
```

- [ ] **Step 5: Add selection + check to the mobile card**

In the mobile card header `<div className="flex items-center gap-2">`, before `<button ...>{r.node_name}</button>`, insert:
```tsx
							{selectable ? (
								<input
									type="checkbox"
									aria-label={`Select ${r.node_name}`}
									checked={!!selectedIds?.has(r.node_id)}
									onChange={() => onToggleSelect?.(r.node_id)}
								/>
							) : null}
```

And after the `<EnableToggle .../>` inside that same header row, insert:
```tsx
							{onCheckNode ? (
								<button
									type="button"
									disabled={checkDisabled}
									onClick={() => onCheckNode(r.node_id)}
									className="rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-secondary disabled:opacity-40"
								>
									Check
								</button>
							) : null}
```

- [ ] **Step 6: Type-check**

Run: `cd frontend && bun run check-types`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/workbench/node-table.tsx
git commit -m "feat(web): NodeTable optional selection + per-row check"
```

---

## Task 10: ResultsSection — selection toolbar + pass-through

**Files:**
- Modify: `frontend/src/components/workbench/results-section.tsx`

- [ ] **Step 1: Extend the props and own selection state**

Replace the `ResultsSection` signature and the top of its body:
```tsx
export function ResultsSection({
	results,
	rules = [],
	onToggleEnabled,
	selectable,
	onCheck,
	checkDisabled,
}: {
	results: NodeResult[];
	rules?: PlatformRule[];
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
	// When set, rows are selectable and a "Check selected" button appears.
	selectable?: boolean;
	onCheck?: (nodeIds: string[]) => void;
	checkDisabled?: boolean;
}) {
	const [text, setText] = useState("");
	const [aliveOnly, setAliveOnly] = useState(false);
	const [platforms, setPlatforms] = useState<string[]>([]);
	const [sortKey, setSortKey] = useState<SortKey>("latency");
	const [sortDir, setSortDir] = useState<SortDir>("asc");
	const [selected, setSelected] = useState<Set<string>>(new Set());

	const toggleSelect = (id: string) =>
		setSelected((prev) => {
			const next = new Set(prev);
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
			return next;
		});
	const toggleSelectAll = (ids: string[], select: boolean) =>
		setSelected((prev) => {
			const next = new Set(prev);
			for (const id of ids) {
				if (select) {
					next.add(id);
				} else {
					next.delete(id);
				}
			}
			return next;
		});
```

- [ ] **Step 2: Add the "Check selected" button to the filter bar**

In the filter bar, replace the trailing `<span className="ml-auto text-[11px] text-muted-foreground tabular-nums">{visible.length} of {results.length} shown</span>` with:
```tsx
					{selectable && onCheck ? (
						<button
							type="button"
							disabled={checkDisabled || selected.size === 0}
							onClick={() => onCheck([...selected])}
							className="ml-auto rounded-full border border-info-line bg-info-muted px-3 py-1 font-medium text-info text-xs transition-colors disabled:opacity-40"
						>
							Check selected{selected.size > 0 ? ` (${selected.size})` : ""}
						</button>
					) : null}
					<span
						className={cn(
							"text-[11px] text-muted-foreground tabular-nums",
							selectable && onCheck ? "" : "ml-auto",
						)}
					>
						{visible.length} of {results.length} shown
					</span>
```

- [ ] **Step 3: Pass selection + per-row check into NodeTable**

Replace the `<NodeTable .../>` usage with:
```tsx
				<NodeTable
					results={visible}
					rules={rules}
					sortKey={sortKey}
					sortDir={sortDir}
					onSort={handleSort}
					onToggleEnabled={onToggleEnabled}
					selectable={selectable}
					selectedIds={selected}
					onToggleSelect={toggleSelect}
					onToggleSelectAll={toggleSelectAll}
					onCheckNode={onCheck ? (id) => onCheck([id]) : undefined}
					checkDisabled={checkDisabled}
				/>
```

- [ ] **Step 4: Type-check**

Run: `cd frontend && bun run check-types`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/workbench/results-section.tsx
git commit -m "feat(web): ResultsSection selection toolbar + check wiring"
```

---

## Task 11: detail-pane — node list as default view, selection → check, refetch on done

**Files:**
- Modify: `frontend/src/components/workbench/detail-pane.tsx`

The default ("Latest result", `selectedJobId === null`) non-running view becomes the live node list with selection. A specific past job still renders its frozen `GetResults` snapshot (no selection). On `progress.done`, the node list is refetched.

- [ ] **Step 1: Add imports, the nodes query, mapper, check handler, and done-refetch**

Add/merge these imports at the top of the file:
```tsx
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { loadCheckOptions } from "@/lib/checkOptions";
```
And extend the existing `@/queries` import to include `queryKeys`, `useNodes`, and `useTriggerCheck`:
```tsx
import {
	queryKeys,
	useCancelCheck,
	useExportLogs,
	useJobs,
	useNodes,
	useResults,
	useRules,
	useSetNodeEnabled,
	useTriggerCheck,
} from "@/queries";
```

Inside `DetailPane`, after the existing `toggleNodeMut` hook, add:
```tsx
	const qc = useQueryClient();
	const nodesQuery = useNodes(sub.id);
	const triggerMut = useTriggerCheck();

	// On the default "Latest result" view, the node list is the source of truth.
	const showNodeList = selectedJobId === null;

	// Refetch the node list once a run finishes so live overlay reconciles with
	// the persisted latest-per-node state.
	useEffect(() => {
		if (progress?.done) {
			qc.invalidateQueries({ queryKey: queryKeys.nodes(sub.id) });
		}
	}, [progress?.done, qc, sub.id]);

	// Map a Node (superset) to the NodeResult shape the table renders.
	const nodeResults: checker.NodeResult[] = useMemo(
		() =>
			(nodesQuery.data?.nodes ?? []).map((n) => ({
				node_id: n.node_id,
				node_name: n.node_name,
				node_type: n.node_type,
				enabled: n.enabled,
				alive: n.alive,
				latency_ms: n.latency_ms,
				speed_kbps: n.speed_kbps,
				upload_speed_kbps: n.upload_speed_kbps,
				country: n.country,
				ip: n.ip,
				server: n.server,
				port: n.port,
				config: n.config,
				platforms: n.platforms,
				traffic_bytes: n.traffic_bytes,
			})),
		[nodesQuery.data],
	);

	// Trigger a check over the given node ids ([] = all), reusing the saved
	// per-subscription check options.
	const handleCheckNodes = (nodeIds: string[]) => {
		const opts = loadCheckOptions(sub.id);
		triggerMut.mutate(
			{ subscriptionId: sub.id, params: { ...opts, node_ids: nodeIds } },
			{
				onSuccess: (resp) => {
					toast.success(
						nodeIds.length === 0
							? "Check started"
							: `Checking ${nodeIds.length} node${nodeIds.length > 1 ? "s" : ""}`,
					);
					onRunStarted(resp.job_id);
				},
				onError: (e) => {
					if (isApiError(e) && (e.status === 409 || e.status === 412)) {
						toast.error("A check is already running for this subscription");
						return;
					}
					toast.error(isApiError(e) ? e.message : "Failed to start check");
				},
			},
		);
	};
```

> The existing `useMemo`/`useState` import line is replaced by the merged React import above. Keep `toast`, `isApiError`, and the `checker` type import that are already in the file.

- [ ] **Step 2: Render the node list in the default non-running branch**

Replace the final `) : (` … `<ResultsSection results={results} … />` … `)}` branch (the last alternative of the results conditional) with this split between the live node list (default) and a frozen job snapshot:
```tsx
					) : showNodeList ? (
						nodesQuery.isLoading ? (
							<div className="space-y-2">
								<Skeleton className="h-8 w-2/3" />
								<Skeleton className="h-40 w-full" />
							</div>
						) : nodeResults.length === 0 ? (
							<EmptyState
								icon={PlayCircle}
								title="No nodes yet"
								description="Refresh from the URL or import nodes to populate this list, then run a check."
							/>
						) : (
							<ResultsSection
								results={nodeResults}
								rules={rulesQuery.data?.rules}
								onToggleEnabled={handleToggleNode}
								selectable
								onCheck={handleCheckNodes}
								checkDisabled={!!activeJobId}
							/>
						)
					) : (
						<ResultsSection
							results={results}
							rules={rulesQuery.data?.rules}
							onToggleEnabled={handleToggleNode}
						/>
					)}
```

> Keep the earlier `running ? (...) :`, `resultsQuery.isLoading && !noChecksYet ? (...)`, and `noChecksYet ? (...)` branches unchanged. Only the trailing default branch is replaced. On the default node-list view (`selectedJobId === null`), `noChecksYet` may still be true (results 404) — that's fine; the node list renders regardless because it does not depend on `resultsQuery`.

- [ ] **Step 3: Type-check and lint**

Run:
```bash
cd frontend && bun run check-types && bunx biome check --write src/components/workbench/detail-pane.tsx src/components/workbench/results-section.tsx src/components/workbench/node-table.tsx src/queries/nodes.ts
```
Expected: no type errors; Biome reports formatted/clean. If `resultsQuery` becomes unused on some path, it is still referenced by the running/loading/noChecksYet guards — no unused-var error expected.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/workbench/detail-pane.tsx
git commit -m "feat(web): node list as default view with whole/partial/single checks"
```

---

## Task 12: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Backend suite**

Run: `encore test ./services/checker/`
Expected: PASS (no `-race`).

- [ ] **Step 2: Frontend type-check, lint, build**

Run:
```bash
cd frontend && bun run check-types && bunx biome check --write src && bun run build
```
Expected: type-check clean; Biome clean; build succeeds.

- [ ] **Step 3: Commit any formatting changes**

```bash
git add -u frontend/src
git commit -m "chore(web): formatting after node-list feature" || echo "nothing to commit"
```

- [ ] **Step 4: Manual browser verification (flag if skipped)**

Per the project rule, start `encore run` + `cd frontend && bun dev` and verify in a browser:
1. Open a subscription with nodes → the node list shows immediately (metrics "—" if never checked).
2. Import / Refresh → list repopulates without running a check.
3. Select 2 nodes → "Check selected (2)" runs only those; others keep their prior metrics.
4. Per-row "Check" runs a single node; others unaffected.
5. Top "Run Check" checks all; history dropdown → a past job shows its frozen snapshot (no checkboxes).

If browser verification is not performed, explicitly say so in the final report.

---

## Self-Review

- **Spec coverage:** ListNodes (Task 5) ✓; `last_checked_at` (Task 5) ✓; node_ids column + migration (Task 1) ✓; TriggerParams.NodeIDs (Task 4) ✓; loadConfig reads it (Task 2) ✓; runner subset scoping + no-fetch-on-partial (Task 3) ✓; node list default surface + selection + single/partial/all (Tasks 7-11) ✓; history snapshot coexistence (Task 11 Step 2) ✓; invalidation after import/refresh/fetch-proxy (Task 8) ✓; ownership via GetSubscription (Task 5) ✓; tests for never-checked / latest-per-node / isolation / subset scoping / no-fetch / loadConfig (Tasks 2,3,5) ✓.
- **Placeholder scan:** none — every code step contains full code or precise edit blocks.
- **Type consistency:** `Node` (Go) ↔ `checker.Node` (TS) fields match; `listNodes`/`ListNodes`/`filterNodesByIDs`/`jobConfig.NodeIDs`/`TriggerParams.NodeIDs`/`queryKeys.nodes`/`useNodes`/`onCheck`/`onCheckNode`/`selectedIds` used consistently across tasks.
- **Assumptions to confirm against the structs when writing the first backend test:** the job-bus constructor name (`newJobBus()` in `partial_check_test.go` — match whatever existing checker tests use); `PlatformOutcome`'s unlocked field is `Unlocked` (json `unlocked`); `nodeCheckResult` has fields `NodeID/NodeName/Alive/LatencyMs/SpeedKbps/UploadSpeedKbps/Country/IP/Platforms/TrafficBytes`. If any differ, adjust the test literals accordingly.
