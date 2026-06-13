# Per-Subscription Export Settings — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-subscription `export_include_dead` (default false) and `export_sort` (`speed_desc` default | `latency_asc`) settings, edited in the Add/Edit Subscription dialog and applied to that subscription's clash/base64 export.

**Architecture:** Two new columns on `subscriptions` (subscription service) flow through its endpoints/params; the checker export path resolves them per subscription and threads an `exportPrefs{IncludeDead, Sort}` into `loadJobProxies`, which makes the `alive` WHERE filter conditional and the ORDER BY selectable. RouterOS export (`latestServerAddresses`) is untouched. Defaults reproduce current output exactly.

**Tech Stack:** Go + Encore (subscription, checker services), TanStack Start + React 19, Base UI, Bun, Biome.

**Spec:** `docs/superpowers/specs/2026-06-14-per-subscription-export-settings-design.md`

---

## Prerequisites & Conventions

1. **Branch:** `feat/node-details-export-tags` (verify with git; do NOT switch).
2. **Backend tests:** `encore test ./services/...` **without `-race`** (known harness hang). Needs Docker.
3. **Frontend:** from `frontend/`: `bun check-types`, `bun check` (Biome tabs/double-quotes, auto-fix), `bun run build`.
4. **Client regen after backend type changes:** `cd frontend && bun run gen:client`.
5. Conventional commits, no attribution footer. Commit at each task boundary.

## File Map

- **Modify** `services/subscription/subscription.go` — struct, 4 SELECTs, Create/Update params+SQL, `normalizeExportSort` (Task 1)
- **Create** `services/subscription/migrations/2_add_export_settings.up.sql` / `.down.sql` (Task 1)
- **Create** `services/subscription/subscription_test.go` (Task 1)
- **Modify** `services/checker/export_data.go` — `exportPrefs`, `loadJobProxies`, `latestUsableProxies*` (Task 2)
- **Modify** `services/checker/export.go` — `loadExportProxies` resolves prefs (Task 2)
- **Modify** `services/checker/export_test.go` — `loadJobProxies` prefs tests (Task 2)
- **Modify** `frontend/src/components/workbench/subscription-dialog.tsx` — two controls (Task 3)
- **Modify** `frontend/src/routes/index.tsx` — enable-toggle omit path (Task 3)
- **Regenerate** `frontend/src/lib/client.gen.ts` (Task 1 + Task 3)

---

## Task 1: subscription service — columns, endpoints, params (TDD)

**Files:**
- Create: `services/subscription/migrations/2_add_export_settings.up.sql`, `.down.sql`
- Modify: `services/subscription/subscription.go`
- Create: `services/subscription/subscription_test.go`

- [ ] **Step 1: Migrations**

`services/subscription/migrations/2_add_export_settings.up.sql`:
```sql
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_include_dead BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_sort TEXT NOT NULL DEFAULT 'speed_desc';
```
`.down.sql`:
```sql
ALTER TABLE subscriptions DROP COLUMN IF EXISTS export_sort;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS export_include_dead;
```

- [ ] **Step 2: Write the failing test**

Create `services/subscription/subscription_test.go`:
```go
// services/subscription/subscription_test.go
package subscription

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func subCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func TestNormalizeExportSort(t *testing.T) {
	cases := map[string]string{
		"speed_desc": "speed_desc", "latency_asc": "latency_asc",
		"": "speed_desc", "bogus": "speed_desc",
	}
	for in, want := range cases {
		if got := normalizeExportSort(in); got != want {
			t.Errorf("normalizeExportSort(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCreateDefaultsAndUpdateExportSettings(t *testing.T) {
	ctx := subCtx("exp-user-1")

	// Create with no export settings -> defaults.
	created, err := Create(ctx, &CreateParams{Name: "S", URL: "https://e.com/s"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ExportIncludeDead != false || created.ExportSort != "speed_desc" {
		t.Errorf("create defaults wrong: %+v", created)
	}

	// Update both fields; bogus sort normalizes.
	inc := true
	sort := "latency_asc"
	updated, err := Update(ctx, created.ID, &UpdateParams{
		ExportIncludeDead: &inc,
		ExportSort:        &sort,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.ExportIncludeDead || updated.ExportSort != "latency_asc" {
		t.Errorf("update wrong: %+v", updated)
	}

	// List reflects the update.
	resp, err := List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, s := range resp.Subscriptions {
		if s.ID == created.ID {
			found = true
			if !s.ExportIncludeDead || s.ExportSort != "latency_asc" {
				t.Errorf("list row wrong: %+v", s)
			}
		}
	}
	if !found {
		t.Error("created subscription not in list")
	}

	// Updating only the name leaves export settings intact (pointer COALESCE).
	name := "S2"
	again, err := Update(ctx, created.ID, &UpdateParams{Name: &name})
	if err != nil {
		t.Fatalf("update2: %v", err)
	}
	if !again.ExportIncludeDead || again.ExportSort != "latency_asc" {
		t.Errorf("partial update clobbered export settings: %+v", again)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

```bash
encore test ./services/subscription/ -run "TestNormalizeExportSort|TestCreateDefaultsAndUpdateExportSettings" -v
```
Expected: compile FAIL (`normalizeExportSort`, `ExportIncludeDead`, `ExportSort`, params undefined).

- [ ] **Step 4: Struct + normalizer**

In `services/subscription/subscription.go`, add to the `Subscription` struct (after `LastRunAt`):
```go
	ExportIncludeDead bool   `json:"export_include_dead"`
	ExportSort        string `json:"export_sort"`
```
Add near the top (after the `Subscription` struct):
```go
// normalizeExportSort guards the export_sort enum, defaulting unknown values.
func normalizeExportSort(s string) string {
	if s == "latency_asc" {
		return "latency_asc"
	}
	return "speed_desc"
}
```

- [ ] **Step 5: Extend the four SELECTs + scans**

There are 4 queries selecting `id, user_id, name, url, enabled, cron_expr, created_at, last_run_at` (in `List`, `Update`'s fetch-after, `GetSubscription`, `GetSubscriptionByID`). For EACH:
- append `, export_include_dead, export_sort` to the column list, and
- append `, &s.ExportIncludeDead, &s.ExportSort` to the matching `.Scan(...)` (for `List`, the row scan target is the loop's `s`).

Concretely the column list becomes:
```sql
SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at, export_include_dead, export_sort
```
and each scan gains `&s.ExportIncludeDead, &s.ExportSort` at the end (List scans into its loop var; the others into their `s`).

- [ ] **Step 6: Create — params, INSERT, return**

Add to `CreateParams`:
```go
	ExportIncludeDead bool   `json:"export_include_dead"`
	ExportSort        string `json:"export_sort"`
```
In `Create`, change the INSERT to include the two columns and a normalized sort:
```go
	sort := normalizeExportSort(p.ExportSort)
	_, err := db.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, name, url, cron_expr, created_at, export_include_dead, export_sort)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, uid, p.Name, p.URL, p.CronExpr, time.Now(), p.ExportIncludeDead, sort)
```
And the returned struct literal gains:
```go
		ExportIncludeDead: p.ExportIncludeDead,
		ExportSort:        sort,
```

- [ ] **Step 7: Update — params + SQL**

Add to `UpdateParams`:
```go
	ExportIncludeDead *bool   `json:"export_include_dead"`
	ExportSort        *string `json:"export_sort"`
```
In `Update`, before the `db.Exec`, normalize the sort pointer:
```go
	var exportSortSQL any
	if p.ExportSort != nil {
		exportSortSQL = normalizeExportSort(*p.ExportSort)
	}
```
Extend the UPDATE statement's SET clause (it currently ends at `cron_expr = CASE ...`):
```go
	_, err := db.Exec(ctx, `
		UPDATE subscriptions SET
			name      = COALESCE($2, name),
			url       = COALESCE($3, url),
			enabled   = COALESCE($4, enabled),
			cron_expr = CASE WHEN $6::boolean THEN NULL ELSE COALESCE($5, cron_expr) END,
			export_include_dead = COALESCE($7, export_include_dead),
			export_sort         = COALESCE($8, export_sort)
		WHERE id = $1
	`, id, p.Name, p.URL, p.Enabled, cronExprSQL, p.ClearCronExpr, p.ExportIncludeDead, exportSortSQL)
```

- [ ] **Step 8: Run tests + full subscription suite**

```bash
encore test ./services/subscription/ -run "TestNormalizeExportSort|TestCreateDefaultsAndUpdateExportSettings" -v
encore test ./services/subscription/
```
Expected: PASS, suite green.

- [ ] **Step 9: Regenerate client + commit**

```bash
cd frontend && bun run gen:client && cd ..
git add services/subscription/ frontend/src/lib/client.gen.ts
git commit -m "feat(subscription): per-subscription export_include_dead and export_sort settings"
```
Expected: `client.gen.ts` shows the two fields on `Subscription`, `CreateParams`, `UpdateParams`.

---

## Task 2: checker export — thread prefs into loadJobProxies (TDD)

**Files:**
- Modify: `services/checker/export_data.go`, `services/checker/export.go`
- Modify: `services/checker/export_test.go`

- [ ] **Step 1: Write the failing test**

Append to `services/checker/export_test.go`:
```go
func seedExportNode(t *testing.T, ctx context.Context, subID, jobID, name string, alive bool, speedKbps int, latencyMs *int) {
	t.Helper()
	nodeID := "expn-" + name + "-" + jobID
	cfg := `{"type":"ss","name":"` + name + `","server":"1.1.1.1","port":1,"cipher":"aes-256-gcm","password":"x"}`
	if _, err := db.Exec(ctx, `
		INSERT INTO nodes (id, subscription_id, name, type, server, port, config, enabled)
		VALUES ($1,$2,$3,'ss','1.1.1.1',1,$4::jsonb,true)
	`, nodeID, subID, name, cfg); err != nil {
		t.Fatalf("seed node %s: %v", name, err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, alive, latency_ms, speed_kbps, upload_speed_kbps, country, ip,
			netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok, traffic_bytes, extra_platforms)
		VALUES ($1,$2,$3,$4,'ss',$5::jsonb,$6,$7,$8,0,'','',false,false,false,false,false,false,false,false,false,0,'{}'::jsonb)
	`, "expr-"+name+"-"+jobID, jobID, nodeID, name, cfg, alive, latencyMs, speedKbps); err != nil {
		t.Fatalf("seed result %s: %v", name, err)
	}
}

func proxyNames(proxies []map[string]any) []string {
	out := make([]string, len(proxies))
	for i, p := range proxies {
		out[i], _ = p["name"].(string)
	}
	return out
}

func TestLoadJobProxiesIncludeDeadAndSort(t *testing.T) {
	ctx := context.Background()
	subID := "expsub-" + expUniq()
	jobID := "expjob-" + expUniq()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1,$2,'u','completed',3,2,NOW(),NOW())
	`, jobID, subID); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	lat100, lat30 := 100, 30
	seedExportNode(t, ctx, subID, jobID, "A", true, 2000, &lat100) // fast, slow latency
	seedExportNode(t, ctx, subID, jobID, "C", true, 500, &lat30)   // slow, fast latency
	seedExportNode(t, ctx, subID, jobID, "B", false, 0, nil)       // dead

	defCfg := settingssvc.ExportTagConfig{} // tags irrelevant here

	// default: exclude dead, speed desc -> [A, C]
	got, err := loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: false, Sort: "speed_desc"})
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if g := proxyNames(got); len(g) != 2 || g[0] != "A" || g[1] != "C" {
		t.Errorf("default speed_desc exclude-dead: got %v", g)
	}

	// include dead, speed desc -> [A, C, B] (dead speed 0 last)
	got, _ = loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: true, Sort: "speed_desc"})
	if g := proxyNames(got); len(g) != 3 || g[2] != "B" {
		t.Errorf("include-dead speed_desc: got %v", g)
	}

	// exclude dead, latency asc -> [C, A]
	got, _ = loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: false, Sort: "latency_asc"})
	if g := proxyNames(got); len(g) != 2 || g[0] != "C" || g[1] != "A" {
		t.Errorf("latency_asc exclude-dead: got %v", g)
	}
}
```
Add a tiny helper if not present (top of the test file): `func expUniq() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }` and import `"context"`, `"fmt"`, `"time"`.

- [ ] **Step 2: Run to verify it fails**

```bash
encore test ./services/checker/ -run TestLoadJobProxiesIncludeDeadAndSort -v
```
Expected: compile FAIL (`exportPrefs` undefined, `loadJobProxies` arity).

- [ ] **Step 3: Add `exportPrefs` + rewrite `loadJobProxies`**

In `services/checker/export_data.go`, add near the top (after imports):
```go
// exportPrefs are the per-subscription export options resolved from the
// subscription record.
type exportPrefs struct {
	IncludeDead bool
	Sort        string // "speed_desc" | "latency_asc"
}

func orderClause(sort string) string {
	if sort == "latency_asc" {
		return "latency_ms ASC NULLS LAST, speed_kbps DESC NULLS LAST"
	}
	return "speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST"
}
```
Change `loadJobProxies` signature to take `prefs exportPrefs` (last param) and build the query dynamically. Replace the `db.Query(ctx, ` ... ``` ``` ... `, jobID, subscriptionID)` block's static SQL with a built string:
```go
	aliveClause := "cr.alive = true AND "
	if prefs.IncludeDead {
		aliveClause = ""
	}
	query := `
		WITH r AS (
			SELECT COALESCE(n.config, cr.node_config) AS config,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
			       cr.country, cr.extra_platforms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps
			                FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name
			                  AND cj2.subscription_id = $2
			                  AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC
			                LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       cr.latency_ms
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			WHERE cr.job_id = $1 AND ` + aliveClause + `COALESCE(n.enabled, true) = true
		)
		SELECT config, node_name, netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok,
		       country, extra_platforms, speed_kbps, latency_ms
		FROM r
		ORDER BY ` + orderClause(prefs.Sort)
	rows, err := db.Query(ctx, query, jobID, subscriptionID)
```
> `aliveClause` and `orderClause` are derived from a bool and a normalized two-value enum — no user string is interpolated, so no injection risk.

Then update the post-query Go `sort.Slice` to honor `prefs.Sort`:
```go
	sort.Slice(nodes, func(i, j int) bool {
		if prefs.Sort == "latency_asc" {
			if nodes[i].latencyMs != nodes[j].latencyMs {
				return nodes[i].latencyMs < nodes[j].latencyMs
			}
			return nodes[i].speedKbps > nodes[j].speedKbps
		}
		if nodes[i].speedKbps != nodes[j].speedKbps {
			return nodes[i].speedKbps > nodes[j].speedKbps
		}
		return nodes[i].latencyMs < nodes[j].latencyMs
	})
```
> Note: dead nodes scanned with NULL `latency_ms` — confirm the `latencyMs` scan target tolerates NULL. The current scan uses `var ... latencyMs int`; a NULL latency would error. Change that scan var to `latencyMs sql.NullInt64` (import `database/sql`) and use `int(latencyMs.Int64)` when building `rankedNode` (0 when null). This is required for include-dead since dead rows can have NULL latency.

- [ ] **Step 4: Thread prefs through callers**

`latestUsableProxies` gains `prefs exportPrefs` and passes it:
```go
func latestUsableProxies(ctx context.Context, subscriptionID, userID string, cfg settingssvc.ExportTagConfig, prefs exportPrefs) ([]map[string]any, error) {
	// ...unchanged until:
	return loadJobProxies(ctx, jobID, subscriptionID, "", cfg, prefs)
}
```
`latestUsableProxiesAcrossAllSubs` — replace the `GetSubscriptionNames` batch with a per-sub `GetSubscriptionByID` lookup that yields both the name prefix and prefs:
```go
func latestUsableProxiesAcrossAllSubs(ctx context.Context, userID string, cfg settingssvc.ExportTagConfig) ([]map[string]any, error) {
	// ...build the []jobSub list exactly as today...
	var all []map[string]any
	for _, js := range jobs {
		sub, err := subsvc.GetSubscriptionByID(ctx, &subsvc.GetByIDParams{ID: js.subscriptionID})
		if err != nil || sub.Name == "" {
			continue
		}
		prefs := exportPrefs{IncludeDead: sub.ExportIncludeDead, Sort: sub.ExportSort}
		proxies, _ := loadJobProxies(ctx, js.jobID, js.subscriptionID, sub.Name, cfg, prefs)
		all = append(all, proxies...)
	}
	return all, nil
}
```
Remove the now-unused `GetSubscriptionNames` call/imports if nothing else uses them in this file (check; `subsvc` is still used via `GetSubscriptionByID`).

- [ ] **Step 5: Resolve prefs in `loadExportProxies`**

In `services/checker/export.go`:
```go
func loadExportProxies(ctx context.Context, subID, userID string) ([]map[string]any, error) {
	cfg, err := settingssvc.GetExportTagsForUser(ctx, userID)
	if err != nil || cfg == nil {
		d := settingssvc.DefaultExportTags()
		cfg = &d
	}
	if subID == "all" {
		return latestUsableProxiesAcrossAllSubs(ctx, userID, *cfg)
	}
	prefs := exportPrefs{Sort: "speed_desc"}
	if sub, err := subsvc.GetSubscriptionByID(ctx, &subsvc.GetByIDParams{ID: subID}); err == nil {
		prefs = exportPrefs{IncludeDead: sub.ExportIncludeDead, Sort: sub.ExportSort}
	}
	return latestUsableProxies(ctx, subID, userID, *cfg, prefs)
}
```
Add `subsvc "subs-check-re/services/subscription"` to `export.go` imports if not already present.

- [ ] **Step 6: Run tests + full checker suite**

```bash
encore test ./services/checker/ -run TestLoadJobProxiesIncludeDeadAndSort -v
encore test ./services/checker/
```
Expected: the prefs test PASSES; suite green (existing `TestTaggedName*` and export tests still pass — the default prefs path matches prior behavior).

- [ ] **Step 7: Commit**

```bash
git add services/checker/
git commit -m "feat(checker): apply per-subscription export include-dead and sort order"
```

---

## Task 3: frontend — subscription dialog controls

**Files:**
- Modify: `frontend/src/components/workbench/subscription-dialog.tsx`
- Modify: `frontend/src/routes/index.tsx`

- [ ] **Step 1: Add the two controls to the dialog**

In `frontend/src/components/workbench/subscription-dialog.tsx`:

a) Add imports (merge with existing):
```tsx
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
```

b) Add state next to the existing form state (`name`/`url`/`enabled`):
```tsx
	const [includeDead, setIncludeDead] = useState(false);
	const [exportSort, setExportSort] = useState("speed_desc");
```

c) In the `useEffect` that seeds the form on open, add:
```tsx
			setIncludeDead(sub?.export_include_dead ?? false);
			setExportSort(sub?.export_sort ?? "speed_desc");
```

d) In the create branch (`createMut.mutate({ name, url, cron_expr: "" }, …)`), pass the fields:
```tsx
			createMut.mutate(
				{
					name,
					url,
					cron_expr: "",
					export_include_dead: includeDead,
					export_sort: exportSort,
				},
				{ /* unchanged onSuccess/onError */ },
			);
```

e) In the update branch (`updateMut.mutate({ id: sub.id, params: { … } }, …)`), add to `params`:
```tsx
						export_include_dead: includeDead,
						export_sort: exportSort,
```

f) Add the UI controls inside the form body, after the existing `Enabled` checkbox block (show in both add and edit modes — move them out of the `editing ? … : null` guard so they always render):
```tsx
					<div className="space-y-1.5">
						<Label htmlFor="sub-sort" className="text-xs">
							Export order
						</Label>
						<Select value={exportSort} onValueChange={(v) => v && setExportSort(v)}>
							<SelectTrigger id="sub-sort" className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="speed_desc">
									Download speed (high→low)
								</SelectItem>
								<SelectItem value="latency_asc">
									Latency (low→high)
								</SelectItem>
							</SelectContent>
						</Select>
					</div>
					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={includeDead}
							onCheckedChange={(v) => setIncludeDead(v === true)}
						/>
						Include dead nodes in export
					</label>
```

> `Checkbox` and `Label` are already imported in this file. `Select` props match the existing usage in `schedule-dialog.tsx`/`export.tsx` (`onValueChange` receives `string | null`).

- [ ] **Step 2: Preserve export settings in the enable-toggle path**

In `frontend/src/routes/index.tsx`, `handleToggleEnabled` builds an `UpdateParams` to flip `enabled`. Because `export_include_dead`/`export_sort` are optional (pointer) params, simply DO NOT include them in that payload — the backend COALESCE leaves them unchanged. Verify the existing `updateMut.mutate({ id, params: { name, url, enabled, cron_expr, clear_cron_expr } })` call omits the two new fields (it already does; no change needed unless TS now requires them — it should not, since they're optional). If TS flags them as required, the client regen made them optional pointers → they're omittable; no edit needed.

- [ ] **Step 3: Verify compile + browser**

```bash
cd frontend && bun check-types && bun check
```
Then `encore run` + `bun dev`; open Add subscription dialog → the Export order select + Include-dead checkbox render; edit an existing subscription → they seed from its values; save → persists. Toggling a subscription's enable from the ⋯ menu does NOT reset them.

Browser computed-style check (recent Tailwind-utility-generation bug): confirm the new Select/checkbox render styled (e.g. the dialog `bg-popover`/`bg-card` resolves to a non-transparent color). Report the value.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): per-subscription export order and include-dead controls in dialog"
```

---

## Task 4: Full verification + E2E

- [ ] **Step 1: Automated gate**

```bash
cd frontend && bun check-types && bun check && bun run test:unit && bun run build
cd .. && encore test ./services/...
```
All green.

- [ ] **Step 2: End-to-end (real backend)**

`encore run` + `bun dev`. Register a user (API or UI). Create a subscription. Seed (via `encore db shell checker`) one completed job with: node A (alive, speed high), node C (alive, speed lower / latency lower), node B (dead). Then:
1. Default export (`/export/<sub>?token=<key>&target=base64`, decode): only A & C, ordered A then C (speed desc); B absent.
2. Edit subscription → Export order = Latency (low→high) → save → export reorders to C then A.
3. Edit subscription → Include dead = on → save → export now includes B (last).
4. RouterOS export (`target=routeros`) unchanged across all three (still lists enabled servers, unaffected by alive/sort).
Report the decoded export lines for each step.

- [ ] **Step 3: Final commit (if walkthrough fixes)**

```bash
git add -A && git commit -m "fix(per-sub-export): walkthrough fixes"
```
(Skip if nothing changed.)

---

## Execution order

```
Task 1  subscription columns/endpoints/params (TDD) + client regen
Task 2  checker export prefs threading (TDD)
Task 3  frontend dialog controls
Task 4  full verification + E2E
```
