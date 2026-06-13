# UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the frontend as a check-centric three-pane workbench (icon rail + subscription list + detail pane) with a unified GitHub+Linear design system, dialog-based interactions, and a consolidated Settings section.

**Architecture:** All create/edit flows move to dialogs, all deletes get confirm dialogs, the Dashboard dissolves into the workbench, and Settings gains four tabs (General / Notifications / Platform Rules / Export API). Two small backend endpoints are added: `checker.LatestJobs` (per-subscription latest-job summaries, powers list column + scheduler "Last check") and `scheduler.SetEnabled` (pause/resume a schedule). Everything else is frontend.

**Tech Stack:** Go + Encore (backend), TanStack Start + Router + Query, React 19, Tailwind v4, Base UI (`@base-ui/react`), Biome (tabs, double quotes), Bun, vitest (new, logic-only tests), `cron-parser` + `cronstrue` (cron math/description).

**Spec:** `docs/superpowers/specs/2026-06-12-ui-redesign-design.md`

---

## Prerequisites & Conventions

1. **Phase 0 (separate PR, do first):** Execute `docs/superpowers/plans/2026-06-01-tanstack-start-migration.md` completely. This plan assumes the migrated layout: single `frontend/` package, sources under `frontend/src/`, UI primitives in `frontend/src/components/ui/`, alias `@/* → src/*`, merged `frontend/src/styles.css`, dev server `bun dev` on :3001, Nitro proxying `/api/*` to Encore on :4000.
2. **Branch:** work on `feat/ui-redesign` (already exists, cut from `origin/main`).
3. **Backend tests:** run `encore test ./services/...` **without `-race`** (known harness hang with -race in this repo — see memory note).
4. **Frontend checks:** run from `frontend/`: `bun check-types` (tsc), `bun check` (Biome — tabs, auto-fix), `bun test:unit` (vitest, added in Task 3).
5. **Code style:** Biome enforces tab indentation and double quotes. All code below follows that.
6. **Dev servers for visual checkpoints:** terminal 1 `encore run`, terminal 2 `cd frontend && bun dev`, open http://localhost:3001.
7. **Commits:** every task ends with a commit; conventional commit format, no attribution footer.

### Spec amendments locked in by this plan

The approved spec said "extend `GET /subscriptions` with `latest_job`". That is not implementable as written: Encore services own separate databases (`check_jobs` lives in the **checker** DB, not the subscription DB), and `subscription` cannot import `checker` (checker→subscription already exists; the reverse would be an import cycle). So:

- `latest_job` data ships as a new **checker** endpoint `GET /check-summaries` returning a map keyed by subscription id. The frontend zips it with `GET /subscriptions` (2 requests total, no N+1).
- The Scheduler "Enabled" switch requires a way to pause without deleting: new `PATCH /scheduler/:id` (`SetEnabled`). The `scheduled_jobs.enabled` column already exists and `initService` already loads only enabled rows.
- Scheduler "edit" needs no new endpoint: `scheduler.Create` is already an upsert (`ON CONFLICT (subscription_id) DO UPDATE`).

Task 1 updates the spec document to record these three corrections.

---

## File Map

**Backend — create:**
- `services/checker/latest.go` + `services/checker/latest_test.go` — LatestJobs endpoint
- `services/scheduler/scheduler.go` (modify) + `services/scheduler/scheduler_test.go` (extend) — SetEnabled endpoint

**Frontend — create (all under `frontend/src/`):**
- `components/ui/badge.tsx`, `components/ui/status-dot.tsx`, `components/ui/empty-state.tsx`, `components/ui/progress.tsx`, `components/ui/spinner.tsx` — simple primitives
- `components/ui/dialog.tsx`, `components/ui/confirm-dialog.tsx`, `components/ui/popover.tsx`, `components/ui/switch.tsx`, `components/ui/tooltip.tsx` — Base UI wrappers
- `lib/cron.ts` + `lib/cron.test.ts` — next-run computation + description
- `lib/nodeFilters.ts` + `lib/nodeFilters.test.ts` — node sort/filter/latency-tone logic
- `lib/checkOptions.ts` + `lib/checkOptions.test.ts` — per-subscription check-options persistence
- `components/rail.tsx`, `components/mobile-tabbar.tsx` — new navigation shell
- `components/workbench/sub-list.tsx`, `components/workbench/unlock-strip.tsx` — list column
- `components/workbench/subscription-dialog.tsx` — add/edit dialog
- `components/workbench/detail-header.tsx`, `components/workbench/run-check-button.tsx`, `components/workbench/export-popover.tsx`, `components/check-options-fields.tsx` — detail header cluster
- `components/workbench/results-section.tsx`, `components/workbench/node-table.tsx`, `components/workbench/progress-panel.tsx` — results + running state
- `components/schedule-dialog.tsx` — scheduler create/edit dialog
- `components/notify-channel-dialog.tsx` — notification channel dialog
- `routes/settings.tsx` (tab layout), `routes/settings/index.tsx` (redirect), `routes/settings/export.tsx` (Export API tab)
- `frontend/vitest.config.ts`

**Frontend — rewrite:**
- `src/styles.css` (extend tokens), `components/ui/button.tsx` (variants + loading)
- `routes/__root.tsx` (shell), `routes/index.tsx` (workbench), `routes/scheduler.tsx`, `routes/settings/general.tsx`, `routes/settings/notify.tsx`, `routes/login.tsx`
- `routes/subscriptions/index.tsx`, `routes/subscriptions/$id.tsx` (become pure redirects)
- `queries/queryKeys.ts`, `queries/jobs.ts`, `queries/scheduler.ts`, `queries/sseProgress.ts` (additions/rewrites)
- `components/platforms/RuleCard.tsx`, `routes/settings/platforms.tsx` (restyle only)

**Frontend — delete:**
- `components/sidebar.tsx`, `components/mobile-nav.tsx` (replaced by rail + tabbar)
- `components/node-table.tsx` (replaced by `components/workbench/node-table.tsx`)

**Unchanged on purpose:** `components/platform-icons.tsx`, `components/debug-panel.tsx`, `components/cron-picker.tsx` (reused as-is), `components/platforms/{engine.ts,ConsolePanel,DocsPanel,IconPicker,ConditionEditor,ScriptEditorArea}` (internal editor machinery), all of `lib/client.ts`, `lib/auth.ts`, `lib/theme.ts`.

---

## Task 1: Amend spec + commit plan docs

**Files:**
- Modify: `docs/superpowers/specs/2026-06-12-ui-redesign-design.md`

- [ ] **Step 1: Replace the "Backend Touchpoint" section of the spec**

In `docs/superpowers/specs/2026-06-12-ui-redesign-design.md`, replace the entire `## Backend Touchpoint (the only one)` section (heading through the paragraph ending "stays public raw (EventSource cannot send auth headers).") with:

```markdown
## Backend Touchpoints

Two small endpoints (amended from the original "extend GET /subscriptions" wording:
Encore services own separate databases, and subscription→checker would be an import
cycle, so the summary lives on the checker service instead).

### 1. `GET /check-summaries` (checker service)

Returns the latest check job per subscription for the current user:

```json
{
  "jobs": {
    "<subscription_id>": {
      "id": "…",
      "status": "completed",
      "available": 86,
      "total": 120,
      "avg_latency_ms": 42,
      "created_at": "2026-06-12T03:40:00Z",
      "finished_at": "2026-06-12T03:42:00Z"
    }
  }
}
```

One `DISTINCT ON (subscription_id)` query over `check_jobs` plus an alive-only
latency average from `check_results`. Powers the workbench list column and the
Scheduler "Last check" column. The frontend zips this with `GET /subscriptions`
(two requests, no N+1).

### 2. `PATCH /scheduler/:id` (scheduler service)

`SetEnabled { enabled: bool }` — pauses/resumes a schedule without deleting it
(drives the Scheduler tab's Enabled switch). Updates `scheduled_jobs.enabled`
and registers/removes the in-memory cron entry.

Scheduler **edit** needs no new endpoint: `scheduler.Create` already upserts on
`subscription_id`.

No other backend changes. SSE protocol, auth, check semantics unchanged. The
`/check/:jobId/progress` endpoint stays public raw (EventSource cannot send
auth headers).
```

Also update the "Out of Scope" bullet `Backend changes beyond the latest_job field` to read `Backend changes beyond the two endpoints above`.

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-06-12-ui-redesign-design.md docs/superpowers/plans/2026-06-12-ui-redesign.md
git commit -m "docs: amend UI redesign spec backend touchpoints, add implementation plan"
```

---

## Task 2: Backend — `checker.LatestJobs` (TDD)

**Files:**
- Create: `services/checker/latest_test.go`
- Create: `services/checker/latest.go`

- [ ] **Step 1: Write the failing test**

Create `services/checker/latest_test.go`:

```go
// services/checker/latest_test.go
package checker

import (
	"context"
	"testing"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func latestTestCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func seedJob(t *testing.T, ctx context.Context, subID, userID, status string, available, total int, createdAt time.Time) string {
	t.Helper()
	id := uuid.New().String()
	finished := createdAt.Add(2 * time.Minute)
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, subID, userID, status, total, available, createdAt, finished); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	return id
}

func seedResult(t *testing.T, ctx context.Context, jobID string, alive bool, latencyMs int) {
	t.Helper()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, alive, latency_ms)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New().String(), jobID, uuid.New().String(), alive, latencyMs); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

func TestLatestJobsPicksNewestPerSubscriptionAndAveragesAliveLatency(t *testing.T) {
	userID := "latest-user-" + uuid.New().String()
	ctx := latestTestCtx(userID)
	subA := "latest-sub-a-" + uuid.New().String()
	subB := "latest-sub-b-" + uuid.New().String()

	base := time.Now().Add(-2 * time.Hour)
	seedJob(t, ctx, subA, userID, "completed", 1, 5, base) // older — must lose
	newest := seedJob(t, ctx, subA, userID, "completed", 2, 6, base.Add(30*time.Minute))
	jobB := seedJob(t, ctx, subB, userID, "failed", 0, 4, base)
	// Another user's job must not leak.
	seedJob(t, ctx, "latest-sub-other", "other-user", "completed", 9, 9, base)

	seedResult(t, ctx, newest, true, 40)
	seedResult(t, ctx, newest, true, 44)
	seedResult(t, ctx, newest, false, 999) // dead node — excluded from avg

	resp, err := LatestJobs(ctx)
	if err != nil {
		t.Fatalf("LatestJobs: %v", err)
	}

	a, ok := resp.Jobs[subA]
	if !ok {
		t.Fatalf("missing summary for subA; got %v", resp.Jobs)
	}
	if a.ID != newest {
		t.Errorf("subA should pick newest job: want %s got %s", newest, a.ID)
	}
	if a.Available != 2 || a.Total != 6 {
		t.Errorf("subA counters: want 2/6 got %d/%d", a.Available, a.Total)
	}
	if a.AvgLatencyMs != 42 {
		t.Errorf("subA avg latency: want 42 got %d", a.AvgLatencyMs)
	}
	if a.FinishedAt == nil {
		t.Error("subA finished_at should be set")
	}

	b, ok := resp.Jobs[subB]
	if !ok {
		t.Fatalf("missing summary for subB")
	}
	if b.ID != jobB || b.Status != "failed" {
		t.Errorf("subB: want failed job %s, got %+v", jobB, b)
	}
	if b.AvgLatencyMs != 0 {
		t.Errorf("subB avg latency with no results: want 0 got %d", b.AvgLatencyMs)
	}

	if _, leaked := resp.Jobs["latest-sub-other"]; leaked {
		t.Error("other user's subscription leaked into response")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
encore test ./services/checker/ -run TestLatestJobsPicksNewest -v
```

Expected: FAIL — compile error `undefined: LatestJobs`.

- [ ] **Step 3: Implement the endpoint**

Create `services/checker/latest.go`:

```go
// services/checker/latest.go
package checker

import (
	"context"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
)

// LatestJobSummary is the most recent check job for one subscription.
type LatestJobSummary struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Available    int        `json:"available"`
	Total        int        `json:"total"`
	AvgLatencyMs int        `json:"avg_latency_ms"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// LatestJobsResponse maps subscription ID → latest job summary.
type LatestJobsResponse struct {
	Jobs map[string]LatestJobSummary `json:"jobs"`
}

// LatestJobs returns the most recent check job per subscription for the
// current user, with the alive-node average latency. Powers the workbench
// subscription list and the scheduler's "Last check" column in one request.
//
//encore:api auth method=GET path=/check-summaries
func LatestJobs(ctx context.Context) (*LatestJobsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (cj.subscription_id)
		       cj.subscription_id, cj.id, cj.status, cj.available, cj.total,
		       cj.created_at, cj.finished_at,
		       COALESCE((
		           SELECT ROUND(AVG(cr.latency_ms))::int
		           FROM check_results cr
		           WHERE cr.job_id = cj.id AND cr.alive AND cr.latency_ms IS NOT NULL
		       ), 0) AS avg_latency_ms
		FROM check_jobs cj
		WHERE cj.user_id = $1
		ORDER BY cj.subscription_id, cj.created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	jobs := make(map[string]LatestJobSummary)
	for rows.Next() {
		var subID string
		var s LatestJobSummary
		if err := rows.Scan(&subID, &s.ID, &s.Status, &s.Available, &s.Total,
			&s.CreatedAt, &s.FinishedAt, &s.AvgLatencyMs); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan error").Err()
		}
		jobs[subID] = s
	}
	if err := rows.Err(); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("rows iteration failed").Err()
	}
	return &LatestJobsResponse{Jobs: jobs}, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
encore test ./services/checker/ -run TestLatestJobsPicksNewest -v
```

Expected: PASS.

- [ ] **Step 5: Run the full checker test suite (no -race)**

```bash
encore test ./services/checker/
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add services/checker/latest.go services/checker/latest_test.go
git commit -m "feat(checker): add /check-summaries endpoint with latest job per subscription"
```

---

## Task 3: Backend — `scheduler.SetEnabled` (TDD)

**Files:**
- Modify: `services/scheduler/scheduler_test.go` (append tests)
- Modify: `services/scheduler/scheduler.go` (add endpoint)

- [ ] **Step 1: Write the failing tests**

Append to `services/scheduler/scheduler_test.go`:

```go
func TestSetEnabledTogglesRowAndCronEntry(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService: %v", err)
	}
	ctx := withAuth()
	subID := "toggle-sub-" + fmt.Sprint(time.Now().UnixNano())
	jobID := "toggle-job-" + fmt.Sprint(time.Now().UnixNano())
	if _, err := db.Exec(context.Background(), `
		INSERT INTO scheduled_jobs (id, subscription_id, user_id, cron_expr, enabled, created_at)
		VALUES ($1, $2, 'test-user-id', '0 3 * * *', true, NOW())
		ON CONFLICT (subscription_id) DO NOTHING
	`, jobID, subID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc.registerCron(subID, "0 3 * * *", defaultCheckOptions())

	if _, err := svc.SetEnabled(ctx, jobID, &SetEnabledParams{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	var enabled bool
	if err := db.QueryRow(context.Background(),
		`SELECT enabled FROM scheduled_jobs WHERE id=$1`, jobID).Scan(&enabled); err != nil {
		t.Fatalf("query: %v", err)
	}
	if enabled {
		t.Error("row should be disabled")
	}
	svc.mu.Lock()
	_, registered := svc.entries[subID]
	svc.mu.Unlock()
	if registered {
		t.Error("cron entry should be removed on disable")
	}

	if _, err := svc.SetEnabled(ctx, jobID, &SetEnabledParams{Enabled: true}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := db.QueryRow(context.Background(),
		`SELECT enabled FROM scheduled_jobs WHERE id=$1`, jobID).Scan(&enabled); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !enabled {
		t.Error("row should be enabled")
	}
	svc.mu.Lock()
	_, registered = svc.entries[subID]
	svc.mu.Unlock()
	if !registered {
		t.Error("cron entry should be registered on enable")
	}
}

func TestSetEnabledRejectsForeignJob(t *testing.T) {
	svc, _ := initService()
	ctx := withAuth()
	if _, err := svc.SetEnabled(ctx, "no-such-job-id", &SetEnabledParams{Enabled: false}); err == nil {
		t.Error("expected NotFound for unknown/foreign job id")
	}
}
```

Also add `"time"` to the test file's imports.

- [ ] **Step 2: Run to verify failure**

```bash
encore test ./services/scheduler/ -run TestSetEnabled -v
```

Expected: FAIL — `undefined: SetEnabledParams` / `svc.SetEnabled`.

- [ ] **Step 3: Implement SetEnabled**

Append to `services/scheduler/scheduler.go` (after `Delete`):

```go
// SetEnabledParams is the request body for PATCH /scheduler/:id.
type SetEnabledParams struct {
	Enabled bool `json:"enabled"`
}

// SetEnabled pauses or resumes a scheduled job without deleting it.
//
//encore:api auth method=PATCH path=/scheduler/:id
func (s *Service) SetEnabled(ctx context.Context, id string, p *SetEnabledParams) (*ScheduledJob, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var j ScheduledJob
	var optsJSON []byte
	if err := db.QueryRow(ctx, `
		SELECT id, subscription_id, cron_expr, created_at, COALESCE(options_json, '{}')
		FROM scheduled_jobs WHERE id = $1 AND user_id = $2
	`, id, claims.UserID).Scan(&j.ID, &j.SubscriptionID, &j.CronExpr, &j.CreatedAt, &optsJSON); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("scheduled job not found").Err()
	}

	if _, err := db.Exec(ctx,
		`UPDATE scheduled_jobs SET enabled = $2 WHERE id = $1`, id, p.Enabled); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}

	var opts checkersvc.CheckOptions
	if err := json.Unmarshal(optsJSON, &opts); err != nil {
		opts = defaultCheckOptions()
	}

	if p.Enabled {
		s.registerCron(j.SubscriptionID, j.CronExpr, opts)
	} else {
		s.removeCron(j.SubscriptionID)
	}

	j.Enabled = p.Enabled
	j.SpeedTest = opts.SpeedTest
	j.MediaApps = opts.MediaApps
	return &j, nil
}
```

- [ ] **Step 4: Run to verify pass, then full suite**

```bash
encore test ./services/scheduler/ -run TestSetEnabled -v
encore test ./services/...
```

Expected: PASS, full suite green.

- [ ] **Step 5: Regenerate the typed client**

```bash
cd frontend
bun run gen:client
cd ..
git diff --stat frontend/src/lib/client.gen.ts
```

Expected: diff shows new `LatestJobs` / `LatestJobsResponse` / `SetEnabled` / `SetEnabledParams` symbols.

- [ ] **Step 6: Commit**

```bash
git add services/scheduler/ frontend/src/lib/client.gen.ts
git commit -m "feat(scheduler): add SetEnabled endpoint; regenerate typed client"
```

---

## Task 4: Frontend deps + vitest scaffolding

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/vitest.config.ts`

- [ ] **Step 1: Add dependencies and test script**

```bash
cd frontend
bun add cron-parser@^5.4.0
bun add -d vitest@^3.2.0
```

Then add to `frontend/package.json` `"scripts"`:

```json
"test:unit": "vitest run"
```

- [ ] **Step 2: Create vitest.config.ts**

Create `frontend/vitest.config.ts`:

```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
	test: {
		include: ["src/**/*.test.ts"],
		environment: "node",
	},
});
```

- [ ] **Step 3: Verify vitest runs (zero tests is OK)**

```bash
bun run test:unit
```

Expected: "No test files found" exit 1 is acceptable here — or pass `--passWithNoTests`: adjust the script to `vitest run --passWithNoTests` so CI-style runs don't fail before Task 6 adds tests. Use the `--passWithNoTests` form.

- [ ] **Step 4: Commit**

```bash
git add frontend/package.json frontend/bun.lock frontend/vitest.config.ts
git commit -m "chore(frontend): add cron-parser and vitest scaffolding"
```

---

## Task 5: Design tokens

**Files:**
- Modify: `frontend/src/styles.css`

Adds the semantic scale (`success/warning/danger/info` × `fg/muted/line`) as Tailwind v4 theme colors, plus shadow vars. Old ad-hoc vars (`--color-badge-*`, `--color-btn-success`, `--color-success`, `--color-warning`) **stay for now** — not-yet-rewritten pages still reference them; Task 23 deletes them.

- [ ] **Step 1: Insert new tokens**

In `frontend/src/styles.css`, inside the `:root` block, after `--radius: 0.5rem;`, insert:

```css
	/* Semantic scale (new design system) */
	--success: #1a7f37;
	--success-muted: #dafbe1;
	--success-line: rgba(26, 127, 55, 0.4);
	--warning: #9a6700;
	--warning-muted: #fff8c5;
	--warning-line: rgba(154, 103, 0, 0.4);
	--danger: #d1242f;
	--danger-muted: #ffd7d9;
	--danger-line: rgba(209, 36, 47, 0.4);
	--info: #0969da;
	--info-muted: #ddf4ff;
	--info-line: rgba(9, 105, 218, 0.4);
	--solid-success: #1f883d;
	--shadow-popover: 0 8px 24px rgba(31, 35, 40, 0.12);
	--shadow-dialog: 0 16px 48px rgba(31, 35, 40, 0.2);
```

Inside the `.dark` block, after `--radius: 0.5rem;`, insert:

```css
	/* Semantic scale (new design system) */
	--success: #3fb950;
	--success-muted: #1a4731;
	--success-line: rgba(63, 185, 80, 0.4);
	--warning: #d29922;
	--warning-muted: #3a2d12;
	--warning-line: rgba(210, 153, 34, 0.4);
	--danger: #f85149;
	--danger-muted: #3d1a1a;
	--danger-line: rgba(248, 81, 73, 0.4);
	--info: #58a6ff;
	--info-muted: #1a2a3a;
	--info-line: rgba(88, 166, 255, 0.4);
	--solid-success: #238636;
	--shadow-popover: 0 8px 24px rgba(1, 4, 9, 0.6);
	--shadow-dialog: 0 16px 48px rgba(1, 4, 9, 0.8);
```

- [ ] **Step 2: Expose tokens as Tailwind utilities**

After the `.dark { … }` block (before `@layer base`), add:

```css
/* Map semantic tokens to Tailwind v4 utilities:
   text-success, bg-success-muted, border-success-line, bg-solid-success, … */
@theme inline {
	--color-success: var(--success);
	--color-success-muted: var(--success-muted);
	--color-success-line: var(--success-line);
	--color-warning: var(--warning);
	--color-warning-muted: var(--warning-muted);
	--color-warning-line: var(--warning-line);
	--color-danger: var(--danger);
	--color-danger-muted: var(--danger-muted);
	--color-danger-line: var(--danger-line);
	--color-info: var(--info);
	--color-info-muted: var(--info-muted);
	--color-info-line: var(--info-line);
	--color-solid-success: var(--solid-success);
}
```

> ⚠️ `@theme inline` emits `--color-success` which collides with the legacy `--color-success` var still defined in `:root`/`.dark`. To avoid the collision **rename the legacy vars now**: in both theme blocks rename `--color-success` → `--legacy-success` and `--color-warning` → `--legacy-warning`, then run a project-wide replace of their usages:
>
> ```bash
> cd frontend
> grep -rl -- "--color-success\|--color-warning" src --include="*.tsx" --include="*.ts" | \
>   xargs sed -i '' 's/var(--color-success)/var(--legacy-success)/g; s/var(--color-warning)/var(--legacy-warning)/g'
> ```
>
> (`--color-badge-*`, `--color-btn-success`, `--color-dimmed`, `--color-code`, `--color-active-*`, `--color-progress` don't collide — leave them.)

- [ ] **Step 3: Verify**

```bash
cd frontend
bun check-types && bun check
bun dev
```

Open http://localhost:3001 — app renders identically (legacy vars renamed, nothing else visible yet).

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): add semantic design tokens and tailwind theme mapping"
```

---

## Task 6: Logic utilities (TDD): cron, node filters, check options

**Files:**
- Create: `frontend/src/lib/cron.test.ts`, `frontend/src/lib/cron.ts`
- Create: `frontend/src/lib/nodeFilters.test.ts`, `frontend/src/lib/nodeFilters.ts`
- Create: `frontend/src/lib/checkOptions.test.ts`, `frontend/src/lib/checkOptions.ts`

- [ ] **Step 1: Write failing cron tests**

Create `frontend/src/lib/cron.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { describeCron, formatUntil, nextRun } from "./cron";

describe("nextRun", () => {
	it("computes the next fire time for an every-6h cron", () => {
		const from = new Date("2026-06-12T03:30:00Z");
		const next = nextRun("0 */6 * * *", from);
		expect(next?.toISOString()).toBe("2026-06-12T06:00:00.000Z");
	});

	it("returns null for an invalid expression", () => {
		expect(nextRun("not-a-cron", new Date())).toBeNull();
	});
});

describe("formatUntil", () => {
	const now = new Date("2026-06-12T00:00:00Z");
	it("formats hours and minutes", () => {
		expect(formatUntil(new Date("2026-06-12T02:14:00Z"), now)).toBe("in 2h 14m");
	});
	it("formats days", () => {
		expect(formatUntil(new Date("2026-06-14T12:00:00Z"), now)).toBe("in 2d 12h");
	});
	it("formats sub-minute as now", () => {
		expect(formatUntil(new Date("2026-06-12T00:00:30Z"), now)).toBe("in <1m");
	});
});

describe("describeCron", () => {
	it("describes a valid cron", () => {
		expect(describeCron("0 4 * * *")).toMatch(/4:00 AM/);
	});
	it("returns the raw expression when unparseable", () => {
		expect(describeCron("???")).toBe("???");
	});
});
```

- [ ] **Step 2: Run to verify failure**

```bash
cd frontend && bun run test:unit
```

Expected: FAIL — `./cron` not found.

- [ ] **Step 3: Implement lib/cron.ts**

Create `frontend/src/lib/cron.ts`:

```ts
// Cron helpers for the scheduler UI: next-run computation (cron-parser),
// human description (cronstrue) and relative formatting.
import { CronExpressionParser } from "cron-parser";
import cronstrue from "cronstrue";

export function nextRun(expr: string, from: Date = new Date()): Date | null {
	try {
		const parsed = CronExpressionParser.parse(expr, { currentDate: from });
		return parsed.next().toDate();
	} catch {
		return null;
	}
}

export function describeCron(expr: string): string {
	try {
		return cronstrue.toString(expr, { verbose: false });
	} catch {
		return expr;
	}
}

export function formatUntil(target: Date, now: Date = new Date()): string {
	const ms = target.getTime() - now.getTime();
	if (ms < 60_000) return "in <1m";
	const totalMinutes = Math.floor(ms / 60_000);
	const days = Math.floor(totalMinutes / (60 * 24));
	const hours = Math.floor((totalMinutes % (60 * 24)) / 60);
	const minutes = totalMinutes % 60;
	if (days > 0) return `in ${days}d ${hours}h`;
	if (hours > 0) return `in ${hours}h ${minutes}m`;
	return `in ${minutes}m`;
}
```

> If `CronExpressionParser` fails to import, check the installed cron-parser major version (`bun pm ls cron-parser`): v5 exports `CronExpressionParser.parse`, v4 exports `parseExpression`. The test suite catches a mismatch immediately.

- [ ] **Step 4: Write failing nodeFilters tests**

Create `frontend/src/lib/nodeFilters.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
	filterNodes,
	latencyTone,
	type NodeLike,
	sortNodes,
} from "./nodeFilters";

function node(partial: Partial<NodeLike>): NodeLike {
	return {
		node_id: partial.node_id ?? "id",
		node_name: partial.node_name ?? "node",
		alive: partial.alive ?? true,
		latency_ms: partial.latency_ms ?? 100,
		speed_kbps: partial.speed_kbps ?? 0,
		netflix: false,
		youtube: false,
		youtube_premium: false,
		openai: false,
		claude: false,
		gemini: false,
		grok: false,
		disney: false,
		tiktok: false,
		extra_platforms: {},
		...partial,
	};
}

describe("sortNodes", () => {
	it("keeps dead nodes last regardless of sort key", () => {
		const sorted = sortNodes(
			[
				node({ node_id: "dead", alive: false, latency_ms: 1 }),
				node({ node_id: "slow", latency_ms: 300 }),
				node({ node_id: "fast", latency_ms: 20 }),
			],
			"latency",
			"asc",
		);
		expect(sorted.map((n) => n.node_id)).toEqual(["fast", "slow", "dead"]);
	});

	it("sorts by speed descending", () => {
		const sorted = sortNodes(
			[
				node({ node_id: "a", speed_kbps: 100 }),
				node({ node_id: "b", speed_kbps: 9000 }),
			],
			"speed",
			"desc",
		);
		expect(sorted[0].node_id).toBe("b");
	});
});

describe("filterNodes", () => {
	const nodes = [
		node({ node_id: "hk", node_name: "HK-IEPL-01", netflix: true }),
		node({ node_id: "jp", node_name: "JP Tokyo", alive: false }),
		node({
			node_id: "sg",
			node_name: "SG Marina",
			extra_platforms: { spotify: true },
		}),
	];

	it("filters by text, case-insensitive", () => {
		expect(filterNodes(nodes, { text: "iepl" }).map((n) => n.node_id)).toEqual([
			"hk",
		]);
	});

	it("filters alive only", () => {
		expect(filterNodes(nodes, { aliveOnly: true })).toHaveLength(2);
	});

	it("filters by built-in and extra platforms", () => {
		expect(
			filterNodes(nodes, { platforms: ["netflix"] }).map((n) => n.node_id),
		).toEqual(["hk"]);
		expect(
			filterNodes(nodes, { platforms: ["spotify"] }).map((n) => n.node_id),
		).toEqual(["sg"]);
	});
});

describe("latencyTone", () => {
	it("maps thresholds", () => {
		expect(latencyTone(49)).toBe("success");
		expect(latencyTone(200)).toBe("warning");
		expect(latencyTone(201)).toBe("danger");
	});
});
```

- [ ] **Step 5: Implement lib/nodeFilters.ts**

Create `frontend/src/lib/nodeFilters.ts`:

```ts
// Pure sort/filter helpers for the workbench node table. Kept free of React
// so they unit-test in node and stay reusable between table and chips.

export const BUILTIN_PLATFORMS = [
	"netflix",
	"youtube",
	"youtube_premium",
	"openai",
	"claude",
	"gemini",
	"grok",
	"disney",
	"tiktok",
] as const;

export type BuiltinPlatform = (typeof BUILTIN_PLATFORMS)[number];

// Structural subset of checker.NodeResult that the helpers need. The real
// NodeResult satisfies it.
export type NodeLike = {
	node_id: string;
	node_name: string;
	alive: boolean;
	latency_ms: number;
	speed_kbps: number;
	extra_platforms: Record<string, boolean>;
} & Record<BuiltinPlatform, boolean>;

export type SortKey = "latency" | "speed" | "name";
export type SortDir = "asc" | "desc";

export interface NodeFilter {
	text?: string;
	aliveOnly?: boolean;
	platforms?: string[];
}

export function nodeHasPlatform(n: NodeLike, platform: string): boolean {
	if ((BUILTIN_PLATFORMS as readonly string[]).includes(platform)) {
		return n[platform as BuiltinPlatform] === true;
	}
	return n.extra_platforms?.[platform] === true;
}

export function filterNodes<T extends NodeLike>(
	nodes: T[],
	filter: NodeFilter,
): T[] {
	const text = filter.text?.trim().toLowerCase();
	return nodes.filter((n) => {
		if (filter.aliveOnly && !n.alive) return false;
		if (text && !n.node_name.toLowerCase().includes(text)) return false;
		if (filter.platforms && filter.platforms.length > 0) {
			if (!filter.platforms.some((p) => nodeHasPlatform(n, p))) return false;
		}
		return true;
	});
}

export function sortNodes<T extends NodeLike>(
	nodes: T[],
	key: SortKey,
	dir: SortDir,
): T[] {
	const mul = dir === "asc" ? 1 : -1;
	return [...nodes].sort((a, b) => {
		// Dead nodes always sink to the bottom.
		if (a.alive !== b.alive) return a.alive ? -1 : 1;
		switch (key) {
			case "latency":
				return (a.latency_ms - b.latency_ms) * mul;
			case "speed":
				return (a.speed_kbps - b.speed_kbps) * mul;
			case "name":
				return a.node_name.localeCompare(b.node_name) * mul;
		}
	});
}

export type Tone = "success" | "warning" | "danger";

export function latencyTone(ms: number): Tone {
	if (ms < 50) return "success";
	if (ms <= 200) return "warning";
	return "danger";
}
```

- [ ] **Step 6: Write failing checkOptions tests**

Create `frontend/src/lib/checkOptions.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
	DEFAULT_CHECK_OPTIONS,
	loadCheckOptions,
	saveCheckOptions,
} from "./checkOptions";

function fakeStorage(): Storage {
	const map = new Map<string, string>();
	return {
		getItem: (k) => map.get(k) ?? null,
		setItem: (k, v) => void map.set(k, v),
		removeItem: (k) => void map.delete(k),
		clear: () => map.clear(),
		key: () => null,
		get length() {
			return map.size;
		},
	} as Storage;
}

describe("checkOptions persistence", () => {
	it("returns defaults when nothing stored", () => {
		const s = fakeStorage();
		expect(loadCheckOptions("sub1", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});

	it("round-trips saved options per subscription", () => {
		const s = fakeStorage();
		const opts = {
			speed_test: false,
			upload_speed_test: true,
			media_apps: ["netflix"],
			debug: false,
		};
		saveCheckOptions("sub1", opts, s);
		expect(loadCheckOptions("sub1", s)).toEqual(opts);
		expect(loadCheckOptions("sub2", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});

	it("falls back to defaults on corrupt JSON", () => {
		const s = fakeStorage();
		s.setItem("check-options:sub1", "{nope");
		expect(loadCheckOptions("sub1", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});
});
```

- [ ] **Step 7: Implement lib/checkOptions.ts**

Create `frontend/src/lib/checkOptions.ts`:

```ts
// Persists the last-used Run Check options per subscription so the primary
// button can re-run with one click. Storage is injectable for tests.

export interface CheckFormOptions {
	speed_test: boolean;
	upload_speed_test: boolean;
	media_apps: string[];
	debug: boolean;
}

export const MEDIA_APPS = [
	"openai",
	"claude",
	"gemini",
	"grok",
	"netflix",
	"youtube",
	"disney",
	"tiktok",
] as const;

export const DEFAULT_CHECK_OPTIONS: CheckFormOptions = {
	speed_test: true,
	upload_speed_test: false,
	media_apps: [...MEDIA_APPS],
	debug: false,
};

const keyFor = (subscriptionId: string) => `check-options:${subscriptionId}`;

export function loadCheckOptions(
	subscriptionId: string,
	storage: Storage = localStorage,
): CheckFormOptions {
	try {
		const raw = storage.getItem(keyFor(subscriptionId));
		if (!raw) return { ...DEFAULT_CHECK_OPTIONS };
		const parsed = JSON.parse(raw) as Partial<CheckFormOptions>;
		return {
			speed_test: parsed.speed_test ?? DEFAULT_CHECK_OPTIONS.speed_test,
			upload_speed_test:
				parsed.upload_speed_test ?? DEFAULT_CHECK_OPTIONS.upload_speed_test,
			media_apps: Array.isArray(parsed.media_apps)
				? parsed.media_apps
				: [...DEFAULT_CHECK_OPTIONS.media_apps],
			debug: parsed.debug ?? DEFAULT_CHECK_OPTIONS.debug,
		};
	} catch {
		return { ...DEFAULT_CHECK_OPTIONS };
	}
}

export function saveCheckOptions(
	subscriptionId: string,
	opts: CheckFormOptions,
	storage: Storage = localStorage,
): void {
	try {
		storage.setItem(keyFor(subscriptionId), JSON.stringify(opts));
	} catch {
		// Quota/security errors are non-fatal; next run just uses defaults.
	}
}
```

- [ ] **Step 8: Run all unit tests, verify pass**

```bash
cd frontend && bun run test:unit && bun check-types && bun check
```

Expected: 3 test files, all PASS; tsc and Biome clean.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib
git commit -m "feat(frontend): add cron, node-filter and check-options utilities with tests"
```

---

## Task 7: UI primitives — Button upgrade, Badge, StatusDot, EmptyState, Progress, Spinner

**Files:**
- Create: `frontend/src/components/ui/spinner.tsx`
- Modify: `frontend/src/components/ui/button.tsx`
- Create: `frontend/src/components/ui/badge.tsx`
- Create: `frontend/src/components/ui/status-dot.tsx`
- Create: `frontend/src/components/ui/empty-state.tsx`
- Create: `frontend/src/components/ui/progress.tsx`

- [ ] **Step 1: Create Spinner**

Create `frontend/src/components/ui/spinner.tsx`:

```tsx
import { cn } from "@/lib/utils";
import { Loader2 } from "lucide-react";

export function Spinner({ className }: { className?: string }) {
	return (
		<Loader2
			aria-label="Loading"
			className={cn("size-4 animate-spin", className)}
		/>
	);
}
```

- [ ] **Step 2: Extend Button with `success` / `destructive-solid` variants and a `loading` prop**

In `frontend/src/components/ui/button.tsx`:

a) Inside `buttonVariants` → `variants.variant`, after the `destructive` entry, add:

```ts
				"destructive-solid":
					"bg-danger text-white hover:bg-danger/90 focus-visible:ring-danger/30",
				success:
					"bg-solid-success text-white hover:bg-solid-success/90 focus-visible:ring-success/30",
```

b) Replace the `Button` function with:

```tsx
function Button({
	className,
	variant = "default",
	size = "default",
	loading = false,
	disabled,
	children,
	...props
}: ButtonPrimitive.Props &
	VariantProps<typeof buttonVariants> & { loading?: boolean }) {
	return (
		<ButtonPrimitive
			data-slot="button"
			className={cn(buttonVariants({ variant, size, className }))}
			disabled={disabled || loading}
			{...props}
		>
			{loading ? <Spinner className="size-3.5" /> : null}
			{children}
		</ButtonPrimitive>
	);
}
```

c) Add the import at the top: `import { Spinner } from "@/components/ui/spinner";`

> The migration already rewrote `@frontend/ui/lib/utils` imports to `@/lib/utils`; keep whatever the file uses post-migration.

- [ ] **Step 3: Create Badge**

Create `frontend/src/components/ui/badge.tsx`:

```tsx
import { cn } from "@/lib/utils";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";

const badgeVariants = cva(
	"inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 font-medium text-[11px] leading-4",
	{
		variants: {
			tone: {
				success: "border-success-line bg-success-muted text-success",
				danger: "border-danger-line bg-danger-muted text-danger",
				warning: "border-warning-line bg-warning-muted text-warning",
				info: "border-info-line bg-info-muted text-info",
				neutral: "border-border bg-secondary text-muted-foreground",
			},
		},
		defaultVariants: { tone: "neutral" },
	},
);

function Badge({
	className,
	tone,
	...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
	return (
		<span className={cn(badgeVariants({ tone }), className)} {...props} />
	);
}

export { Badge, badgeVariants };
```

- [ ] **Step 4: Create StatusDot**

Create `frontend/src/components/ui/status-dot.tsx`:

```tsx
import { cn } from "@/lib/utils";

export type DotTone = "success" | "danger" | "info" | "neutral";

const toneClass: Record<DotTone, string> = {
	success: "bg-success",
	danger: "bg-danger",
	info: "bg-info",
	neutral: "bg-muted-foreground/50",
};

export function StatusDot({
	tone,
	pulse = false,
	className,
}: {
	tone: DotTone;
	pulse?: boolean;
	className?: string;
}) {
	return (
		<span
			aria-hidden
			className={cn(
				"inline-block size-2 shrink-0 rounded-full",
				toneClass[tone],
				pulse && "animate-pulse",
				className,
			)}
		/>
	);
}
```

- [ ] **Step 5: Create EmptyState**

Create `frontend/src/components/ui/empty-state.tsx`:

```tsx
import type { LucideIcon } from "lucide-react";
import type * as React from "react";

export function EmptyState({
	icon: Icon,
	title,
	description,
	action,
}: {
	icon: LucideIcon;
	title: string;
	description?: string;
	action?: React.ReactNode;
}) {
	return (
		<div className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
			<div className="flex size-10 items-center justify-center rounded-full bg-secondary">
				<Icon className="size-5 text-muted-foreground" strokeWidth={1.5} />
			</div>
			<p className="font-medium text-foreground text-sm">{title}</p>
			{description ? (
				<p className="max-w-xs text-muted-foreground text-xs">{description}</p>
			) : null}
			{action ? <div className="mt-2">{action}</div> : null}
		</div>
	);
}
```

- [ ] **Step 6: Create Progress**

Create `frontend/src/components/ui/progress.tsx`:

```tsx
import { cn } from "@/lib/utils";

export function Progress({
	value,
	className,
}: {
	value: number; // 0–100
	className?: string;
}) {
	const clamped = Math.max(0, Math.min(100, value));
	return (
		<div
			role="progressbar"
			aria-valuenow={Math.round(clamped)}
			aria-valuemin={0}
			aria-valuemax={100}
			className={cn(
				"h-1.5 w-full overflow-hidden rounded-full bg-secondary",
				className,
			)}
		>
			<div
				className="h-full rounded-full transition-[width] duration-300 ease-out"
				style={{ width: `${clamped}%`, background: "var(--color-progress)" }}
			/>
		</div>
	);
}
```

- [ ] **Step 7: Verify and commit**

```bash
cd frontend && bun check-types && bun check
git add src/components/ui
git commit -m "feat(frontend): button variants + loading, badge, status-dot, empty-state, progress primitives"
```

---

## Task 8: UI primitives — Dialog, ConfirmDialog, Popover, Switch, Tooltip

**Files:**
- Create: `frontend/src/components/ui/dialog.tsx`
- Create: `frontend/src/components/ui/confirm-dialog.tsx`
- Create: `frontend/src/components/ui/popover.tsx`
- Create: `frontend/src/components/ui/switch.tsx`
- Create: `frontend/src/components/ui/tooltip.tsx`

- [ ] **Step 1: Create the styled Dialog wrapper**

Create `frontend/src/components/ui/dialog.tsx`:

```tsx
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

function DialogContent({
	className,
	children,
	...props
}: DialogPrimitive.Popup.Props) {
	return (
		<DialogPrimitive.Portal>
			<DialogPrimitive.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
			<DialogPrimitive.Popup
				className={cn(
					// Mobile: full screen. ≥sm: centered card.
					"fixed z-50 flex flex-col bg-popover text-popover-foreground outline-none",
					"inset-0 overflow-y-auto p-5",
					"sm:inset-auto sm:top-1/2 sm:left-1/2 sm:max-h-[85vh] sm:w-full sm:max-w-md sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:border sm:border-border sm:shadow-[var(--shadow-dialog)]",
					"transition-all duration-200 data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0",
					className,
				)}
				{...props}
			>
				{children}
				<DialogPrimitive.Close
					aria-label="Close"
					className="absolute top-4 right-4 rounded-md p-1 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					<XIcon className="size-4" />
				</DialogPrimitive.Close>
			</DialogPrimitive.Popup>
		</DialogPrimitive.Portal>
	);
}

function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props) {
	return (
		<DialogPrimitive.Title
			className={cn("font-semibold text-[15px] text-foreground", className)}
			{...props}
		/>
	);
}

function DialogDescription({
	className,
	...props
}: DialogPrimitive.Description.Props) {
	return (
		<DialogPrimitive.Description
			className={cn("mt-0.5 text-muted-foreground text-xs", className)}
			{...props}
		/>
	);
}

function DialogFooter({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			className={cn("mt-5 flex justify-end gap-2", className)}
			{...props}
		/>
	);
}

export {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
	DialogTrigger,
};
```

- [ ] **Step 2: Create ConfirmDialog**

Create `frontend/src/components/ui/confirm-dialog.tsx`:

```tsx
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";

export function ConfirmDialog({
	open,
	onOpenChange,
	title,
	description,
	confirmLabel = "Delete",
	pending = false,
	onConfirm,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	title: string;
	description: string;
	confirmLabel?: string;
	pending?: boolean;
	onConfirm: () => void;
}) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-sm">
				<DialogTitle>{title}</DialogTitle>
				<DialogDescription>{description}</DialogDescription>
				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="destructive-solid"
						loading={pending}
						onClick={onConfirm}
					>
						{confirmLabel}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
```

- [ ] **Step 3: Create Popover**

Create `frontend/src/components/ui/popover.tsx`:

```tsx
import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import { cn } from "@/lib/utils";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;

function PopoverContent({
	className,
	sideOffset = 6,
	align = "end",
	...props
}: PopoverPrimitive.Popup.Props & {
	sideOffset?: number;
	align?: "start" | "center" | "end";
}) {
	return (
		<PopoverPrimitive.Portal>
			<PopoverPrimitive.Positioner sideOffset={sideOffset} align={align}>
				<PopoverPrimitive.Popup
					className={cn(
						"z-50 w-72 rounded-lg border border-border bg-popover p-3 text-popover-foreground shadow-[var(--shadow-popover)] outline-none",
						"transition-all duration-150 data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0",
						className,
					)}
					{...props}
				/>
			</PopoverPrimitive.Positioner>
		</PopoverPrimitive.Portal>
	);
}

export { Popover, PopoverContent, PopoverTrigger };
```

- [ ] **Step 4: Create Switch**

Create `frontend/src/components/ui/switch.tsx`:

```tsx
import { Switch as SwitchPrimitive } from "@base-ui/react/switch";
import { cn } from "@/lib/utils";

function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
	return (
		<SwitchPrimitive.Root
			className={cn(
				"relative h-[18px] w-8 shrink-0 rounded-full border border-transparent bg-secondary outline-none transition-colors",
				"focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
				"data-checked:bg-solid-success",
				className,
			)}
			{...props}
		>
			<SwitchPrimitive.Thumb className="block size-3.5 translate-x-0.5 rounded-full bg-white shadow-sm transition-transform data-checked:translate-x-[15px]" />
		</SwitchPrimitive.Root>
	);
}

export { Switch };
```

- [ ] **Step 5: Create Tooltip**

Create `frontend/src/components/ui/tooltip.tsx`:

```tsx
import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import type * as React from "react";

export function Tooltip({
	content,
	children,
	side = "top",
}: {
	content: React.ReactNode;
	children: React.ReactElement;
	side?: "top" | "bottom" | "left" | "right";
}) {
	return (
		<TooltipPrimitive.Root delay={300}>
			<TooltipPrimitive.Trigger render={children} />
			<TooltipPrimitive.Portal>
				<TooltipPrimitive.Positioner side={side} sideOffset={6}>
					<TooltipPrimitive.Popup className="z-50 rounded-md border border-border bg-popover px-2 py-1 text-popover-foreground text-xs shadow-[var(--shadow-popover)]">
						{content}
					</TooltipPrimitive.Popup>
				</TooltipPrimitive.Positioner>
			</TooltipPrimitive.Portal>
		</TooltipPrimitive.Root>
	);
}
```

> Base UI API note: this repo's `mobile-nav.tsx` already used `Dialog.Root/Trigger/Portal/Backdrop/Popup` from `@base-ui/react/dialog`, so the same component family (Popover/Switch/Tooltip) follows the identical Portal/Positioner/Popup shape. If `bun check-types` flags a prop (e.g. `delay` or `render`), open `node_modules/@base-ui/react` typings for the exact name — adjust the wrapper only, call-sites stay stable.

- [ ] **Step 6: Verify and commit**

```bash
cd frontend && bun check-types && bun check
git add src/components/ui
git commit -m "feat(frontend): dialog, confirm-dialog, popover, switch, tooltip primitives"
```

---

## Task 9: Query layer — latest jobs, results default, SSE rewrite

**Files:**
- Modify: `frontend/src/queries/queryKeys.ts`
- Modify: `frontend/src/queries/jobs.ts`
- Modify: `frontend/src/queries/scheduler.ts`
- Rewrite: `frontend/src/queries/sseProgress.ts`

- [ ] **Step 1: Add query keys**

In `frontend/src/queries/queryKeys.ts`, add to the `queryKeys` object:

```ts
	latestJobs: () => ["latest-jobs"] as const,
```

- [ ] **Step 2: Add `useLatestJobs`, fix `useResults`, invalidate latest jobs on trigger/cancel**

In `frontend/src/queries/jobs.ts`:

a) Add after `useJobs`:

```ts
export function useLatestJobs() {
	return useQuery({
		queryKey: queryKeys.latestJobs(),
		queryFn: () => client.checker.LatestJobs(),
		staleTime: 15_000,
	});
}
```

b) Replace `useResults` with (jobId `null` now means "latest completed" — the backend already supports an empty `JobID`):

```ts
export function useResults(subscriptionId: string, jobId: string | null) {
	return useQuery({
		queryKey: queryKeys.results(subscriptionId, jobId),
		queryFn: () =>
			client.checker.GetResults(subscriptionId, { JobID: jobId ?? "" }),
		enabled: !!subscriptionId,
		retry: (failureCount, err) => {
			// 404 = no completed checks yet; don't hammer.
			if (isApiError(err) && err.status === 404) return false;
			return failureCount < 2;
		},
	});
}
```

Add the import: `import { isApiError } from "../lib/client";`

c) In `useTriggerCheck`'s `onSuccess`, add a second invalidation:

```ts
			qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
```

d) In `useCancelCheck`'s `onSuccess`, add the same line.

- [ ] **Step 3: Add `useSetScheduleEnabled`**

In `frontend/src/queries/scheduler.ts`, append:

```ts
export function useSetScheduleEnabled() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (args: { id: string; enabled: boolean }) =>
			client.scheduler.SetEnabled(args.id, { enabled: args.enabled }),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.scheduler() }),
	});
}
```

- [ ] **Step 4: Rewrite `useSSEProgress` — reconnect status, batched log, latest-jobs invalidation**

Replace the entire contents of `frontend/src/queries/sseProgress.ts`:

```ts
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import type { NodeDebug } from "@/components/debug-panel";
import { queryKeys } from "./queryKeys";

export interface SSEProgress {
	progress?: number;
	total?: number;
	node_name?: string;
	alive?: boolean;
	latency_ms?: number;
	speed_kbps?: number;
	upload_speed_kbps?: number;
	done?: boolean;
	status?: string;
	debug?: NodeDebug;
}

export type SSEConnection = "idle" | "open" | "reconnecting" | "done";

interface UseSSEProgressOptions {
	jobId: string | null;
	subscriptionId: string;
	onDone?: () => void;
}

interface UseSSEProgressResult {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	debugData: NodeDebug[];
	connection: SSEConnection;
}

const MAX_LOG_ENTRIES = 500;
const FLUSH_INTERVAL_MS = 800;

// useSSEProgress subscribes to /api/check/:jobId/progress.
// - Per-node events are buffered and flushed every FLUSH_INTERVAL_MS so a
//   200-node burst doesn't render 200 times (spec: throttled live inserts).
// - EventSource reconnects automatically; we surface that as `connection:
//   "reconnecting"` instead of silently closing like the old version did.
// - On done: closes, invalidates jobs + latest-jobs + results so every list
//   refreshes, then fires onDone.
export function useSSEProgress({
	jobId,
	subscriptionId,
	onDone,
}: UseSSEProgressOptions): UseSSEProgressResult {
	const [progress, setProgress] = useState<SSEProgress | null>(null);
	const [logEntries, setLogEntries] = useState<SSEProgress[]>([]);
	const [debugData, setDebugData] = useState<NodeDebug[]>([]);
	const [connection, setConnection] = useState<SSEConnection>("idle");
	const qc = useQueryClient();
	const onDoneRef = useRef(onDone);
	onDoneRef.current = onDone;

	useEffect(() => {
		setLogEntries([]);
		setProgress(null);
		setDebugData([]);
		setConnection(jobId ? "reconnecting" : "idle");
	}, [jobId]);

	useEffect(() => {
		if (!jobId) return;

		const buffer: SSEProgress[] = [];
		const debugBuffer: NodeDebug[] = [];
		const flush = () => {
			if (buffer.length > 0) {
				const batch = buffer.splice(0, buffer.length);
				setLogEntries((prev) =>
					[...prev, ...batch].slice(-MAX_LOG_ENTRIES),
				);
			}
			if (debugBuffer.length > 0) {
				const batch = debugBuffer.splice(0, debugBuffer.length);
				setDebugData((prev) => [...prev, ...batch]);
			}
		};
		const timer = setInterval(flush, FLUSH_INTERVAL_MS);

		const es = new EventSource(
			`${window.location.origin}/api/check/${jobId}/progress`,
		);
		es.onopen = () => setConnection("open");
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			setProgress(data);
			if (data.debug) debugBuffer.push(data.debug);
			if (data.node_name) buffer.push(data);
			if (data.done) {
				flush();
				setConnection("done");
				es.close();
				qc.invalidateQueries({ queryKey: queryKeys.jobs(subscriptionId) });
				qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
				qc.invalidateQueries({
					queryKey: queryKeys.results(subscriptionId),
				});
				onDoneRef.current?.();
			}
		};
		// Do NOT close on error: EventSource retries on its own. If the job
		// finished while we were away, the next message is a done event
		// (checker re-sends terminal state to late subscribers).
		es.onerror = () => setConnection("reconnecting");

		return () => {
			clearInterval(timer);
			es.close();
		};
	}, [jobId, qc, subscriptionId]);

	return { progress, logEntries, debugData, connection };
}
```

- [ ] **Step 5: Verify and commit**

```bash
cd frontend && bun check-types && bun check && bun run test:unit
git add src/queries
git commit -m "feat(frontend): latest-jobs query, default-latest results, reconnecting batched SSE hook"
```

---

## Task 10: App shell — icon rail, mobile tab bar, root layout

**Files:**
- Create: `frontend/src/components/rail.tsx`
- Create: `frontend/src/components/mobile-tabbar.tsx`
- Modify: `frontend/src/routes/__root.tsx`
- Delete: `frontend/src/components/sidebar.tsx`, `frontend/src/components/mobile-nav.tsx`

> Transitional note: until Tasks 11–22 rewrite the pages, old pages render inside the new shell without the old `max-w-5xl` wrapper — functional but plain. Each page task restores its own layout.

- [ ] **Step 1: Create the icon rail**

Create `frontend/src/components/rail.tsx`:

```tsx
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { Clock, List, LogOut, Moon, Settings, Sun } from "lucide-react";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip } from "@/components/ui/tooltip";
import { clearToken } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { useMe } from "@/queries";

const NAV_ITEMS = [
	{ to: "/", label: "Subscriptions", icon: List, exact: true },
	{ to: "/scheduler", label: "Scheduler", icon: Clock, exact: true },
	{ to: "/settings", label: "Settings", icon: Settings, exact: false },
] as const;

function RailLink({
	to,
	label,
	icon: Icon,
	exact,
}: {
	to: string;
	label: string;
	icon: React.ElementType;
	exact: boolean;
}) {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	const isActive = exact ? pathname === to : pathname.startsWith(to);
	return (
		<Tooltip content={label} side="right">
			<Link
				to={to}
				aria-label={label}
				className={cn(
					"flex size-9 items-center justify-center rounded-lg border border-transparent text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground",
					isActive && "text-foreground",
				)}
				style={
					isActive
						? {
								background: "var(--color-active-bg)",
								borderColor: "var(--color-active-border)",
							}
						: undefined
				}
			>
				<Icon size={16} strokeWidth={1.75} />
			</Link>
		</Tooltip>
	);
}

export function Rail() {
	const navigate = useNavigate();
	const { theme, toggle } = useTheme();
	const meQuery = useMe();
	const username = meQuery.data?.username ?? "…";

	function logout() {
		clearToken();
		navigate({ to: "/login" });
	}

	return (
		<aside className="hidden h-screen w-14 shrink-0 flex-col items-center gap-1.5 border-border border-r bg-card py-3 md:flex">
			<Link
				to="/"
				aria-label="subs-check home"
				className="mb-2 flex size-8 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground text-sm"
			>
				S
			</Link>

			{NAV_ITEMS.map((item) => (
				<RailLink key={item.to} {...item} />
			))}

			<div className="flex-1" />

			<Tooltip
				content={theme === "dark" ? "Light mode" : "Dark mode"}
				side="right"
			>
				<button
					type="button"
					onClick={toggle}
					aria-label="Toggle theme"
					className="flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					{theme === "dark" ? (
						<Sun size={16} strokeWidth={1.75} />
					) : (
						<Moon size={16} strokeWidth={1.75} />
					)}
				</button>
			</Tooltip>

			<DropdownMenu>
				<DropdownMenuTrigger
					aria-label="Account menu"
					className="flex size-8 items-center justify-center rounded-full border border-border bg-secondary font-medium text-foreground text-xs outline-none transition-colors hover:bg-muted"
				>
					{username.slice(0, 1).toUpperCase()}
				</DropdownMenuTrigger>
				<DropdownMenuContent side="right" align="end" className="min-w-40">
					<div className="px-2 py-1.5 text-muted-foreground text-xs">
						Signed in as <span className="font-medium">{username}</span>
					</div>
					<DropdownMenuItem onClick={logout}>
						<LogOut size={14} /> Log out
					</DropdownMenuItem>
				</DropdownMenuContent>
			</DropdownMenu>
		</aside>
	);
}
```

> `dropdown-menu.tsx` already exists in `src/components/ui/` (from the old `packages/ui`). If its exported names differ (open the file), match them here — the shadcn convention is `DropdownMenu/Trigger/Content/Item`.

- [ ] **Step 2: Create the mobile tab bar**

Create `frontend/src/components/mobile-tabbar.tsx`:

```tsx
import { Link, useRouterState } from "@tanstack/react-router";
import { Clock, List, Settings } from "lucide-react";
import { cn } from "@/lib/utils";

const TABS = [
	{ to: "/", label: "Subs", icon: List, exact: true },
	{ to: "/scheduler", label: "Scheduler", icon: Clock, exact: true },
	{ to: "/settings", label: "Settings", icon: Settings, exact: false },
] as const;

export function MobileTabbar() {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	return (
		<nav className="flex shrink-0 border-border border-t bg-card pb-[env(safe-area-inset-bottom)] md:hidden">
			{TABS.map(({ to, label, icon: Icon, exact }) => {
				const active = exact ? pathname === to : pathname.startsWith(to);
				return (
					<Link
						key={to}
						to={to}
						className={cn(
							"flex min-h-12 flex-1 flex-col items-center justify-center gap-0.5 text-[10px] transition-colors",
							active ? "text-primary" : "text-muted-foreground",
						)}
					>
						<Icon size={18} strokeWidth={1.75} />
						{label}
					</Link>
				);
			})}
		</nav>
	);
}
```

- [ ] **Step 3: Rewrite the root layout**

In `frontend/src/routes/__root.tsx` (the post-migration version with `RootDocument`), replace the `RootComponent` function and its imports of `Sidebar`/`MobileNav` with:

```tsx
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Rail } from "@/components/rail";
```

```tsx
function RootComponent() {
	const { location } = useRouterState();
	const authed = isAuthenticated() && location.pathname !== "/login";

	return (
		<RootDocument>
			<QueryClientProvider client={queryClient}>
				{authed ? (
					<PlatformRulesProvider>
						<div className="flex h-dvh flex-col md:flex-row">
							<Rail />
							<main className="min-h-0 flex-1 overflow-hidden">
								<Outlet />
							</main>
							<MobileTabbar />
						</div>
					</PlatformRulesProvider>
				) : (
					<div className="flex min-h-screen items-center justify-center">
						<Outlet />
					</div>
				)}
				<Toaster richColors />
				<TanStackRouterDevtools position="bottom-left" />
			</QueryClientProvider>
		</RootDocument>
	);
}
```

Everything else in the file (QueryClient config, `beforeLoad`, `RootDocument`, head) stays exactly as the migration left it.

> The `main` is now `overflow-hidden`: the workbench manages its own internal scroll areas. Secondary pages (scheduler/settings) wrap themselves in `h-full overflow-y-auto` — their tasks do that.

- [ ] **Step 4: Delete the old navigation**

```bash
rm frontend/src/components/sidebar.tsx frontend/src/components/mobile-nav.tsx
```

`useRouterState`/`useNavigate` imports they held move with them. Run `bun check-types` — if any file still imports `Sidebar`/`MobileNav`/`SidebarNav`, this is the moment to notice (only `__root.tsx` referenced them after the migration).

- [ ] **Step 5: Verify in browser, commit**

```bash
cd frontend && bun check-types && bun check && bun dev
```

Open http://localhost:3001 (logged in): icon rail on desktop, bottom tabs at 375px (devtools). Old pages render inside.

```bash
git add -A frontend/src
git commit -m "feat(frontend): icon rail + mobile tab bar shell, drop sidebar"
```

---

## Task 11: Workbench route — three panes, list column, unlock strip

**Files:**
- Create: `frontend/src/components/workbench/sub-list.tsx`
- Create: `frontend/src/components/workbench/unlock-strip.tsx`
- Rewrite: `frontend/src/routes/index.tsx`

- [ ] **Step 1: Create the unlock strip (footer of the list column)**

Create `frontend/src/components/workbench/unlock-strip.tsx`:

```tsx
import { CheckCircle, Globe, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import type { PlatformKey } from "@/components/platform-icons";
import { PlatformIcon } from "@/components/platform-icons";
import type { checker } from "@/lib/client.gen";
import { useLocalUnlock } from "@/queries";

const PLATFORM_KEYS: (keyof checker.LocalUnlockResult)[] = [
	"openai",
	"claude",
	"gemini",
	"grok",
	"netflix",
	"youtube",
	"youtube_premium",
	"disney",
	"tiktok",
];

// Compact footer strip showing what the server's own IP can reach — the old
// Dashboard panel, demoted to a popover. Matters when reading node results:
// a platform blocked for the server itself shows as blocked on every node.
export function UnlockStrip() {
	const { data, isLoading, isFetching, refetch } = useLocalUnlock();

	const unlockCount = data
		? PLATFORM_KEYS.filter((k) => data[k] === true).length
		: 0;

	return (
		<Popover>
			<PopoverTrigger className="flex w-full items-center gap-2 border-border border-t px-4 py-2.5 text-left text-muted-foreground text-xs outline-none transition-colors hover:bg-secondary/50">
				<Globe size={13} strokeWidth={1.75} className="shrink-0" />
				{isLoading ? (
					<span>Checking server network…</span>
				) : data ? (
					<>
						<span className="truncate font-mono tabular-nums">
							{data.country ? `${data.country} ` : ""}
							{data.ip || "server"}
						</span>
						<span className="ml-auto shrink-0 font-medium text-success">
							{unlockCount} unlocks ›
						</span>
					</>
				) : (
					<span>Server network unlock ›</span>
				)}
			</PopoverTrigger>
			<PopoverContent align="start" className="w-80">
				<div className="mb-2 flex items-center justify-between">
					<p className="font-medium text-foreground text-xs">
						Server network unlock
					</p>
					<Button
						variant="ghost"
						size="xs"
						onClick={() => refetch()}
						disabled={isFetching}
					>
						<RefreshCw size={12} className={isFetching ? "animate-spin" : ""} />
						Refresh
					</Button>
				</div>
				<p className="mb-3 text-muted-foreground text-xs">
					Platforms reachable from this server's own IP.
				</p>
				<div className="flex flex-wrap gap-2">
					{PLATFORM_KEYS.map((k) => {
						const available = data ? data[k] === true : false;
						return (
							<span
								key={k}
								className="inline-flex items-center gap-1.5 rounded-md bg-secondary px-2 py-1"
								style={{ opacity: available ? 1 : 0.35 }}
							>
								<PlatformIcon
									platform={k as PlatformKey}
									size={14}
									showLabel
								/>
								{available ? (
									<CheckCircle size={10} className="text-success" />
								) : null}
							</span>
						);
					})}
				</div>
			</PopoverContent>
		</Popover>
	);
}
```

- [ ] **Step 2: Create the subscription list column**

Create `frontend/src/components/workbench/sub-list.tsx`:

```tsx
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusDot, type DotTone } from "@/components/ui/status-dot";
import { UnlockStrip } from "@/components/workbench/unlock-strip";
import type { checker, subscription } from "@/lib/client.gen";
import { cn } from "@/lib/utils";

type Subscription = subscription.Subscription;
type LatestJobSummary = checker.LatestJobSummary;

function relativeTime(iso: string): string {
	const ms = Date.now() - new Date(iso).getTime();
	const minutes = Math.floor(ms / 60_000);
	if (minutes < 1) return "just now";
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	return `${Math.floor(hours / 24)}d ago`;
}

function dotTone(sub: Subscription, latest?: LatestJobSummary): DotTone {
	if (!sub.enabled) return "neutral";
	if (!latest) return "neutral";
	if (latest.status === "running" || latest.status === "queued") return "info";
	if (latest.status === "failed") return "danger";
	return "success";
}

function SubListItem({
	sub,
	latest,
	selected,
	liveProgressPct,
	onSelect,
}: {
	sub: Subscription;
	latest?: LatestJobSummary;
	selected: boolean;
	// Set only for the selected subscription while its check runs (mirrors
	// the detail pane's SSE data — non-selected rows don't open streams).
	liveProgressPct: number | null;
	onSelect: () => void;
}) {
	const running =
		latest?.status === "running" || latest?.status === "queued";
	return (
		<button
			type="button"
			onClick={onSelect}
			className={cn(
				"w-full rounded-lg border border-transparent px-3 py-2.5 text-left transition-colors",
				selected ? "" : "hover:bg-secondary/50",
				!sub.enabled && "opacity-45",
			)}
			style={
				selected
					? {
							background: "var(--color-active-bg)",
							borderColor: "var(--color-active-border)",
						}
					: undefined
			}
		>
			<div className="flex items-center gap-2">
				<StatusDot tone={dotTone(sub, latest)} pulse={running} />
				<span className="min-w-0 flex-1 truncate font-medium text-foreground text-sm">
					{sub.name || sub.url}
				</span>
				<span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
					{running
						? liveProgressPct !== null
							? `${Math.round(liveProgressPct)}%`
							: "Running…"
						: latest?.finished_at
							? relativeTime(latest.finished_at)
							: ""}
				</span>
			</div>
			{running && liveProgressPct !== null ? (
				<div className="mt-1.5 pl-4">
					<Progress value={liveProgressPct} className="h-[3px]" />
				</div>
			) : latest && !running ? (
				<p className="mt-0.5 truncate pl-4 text-muted-foreground text-xs tabular-nums">
					{latest.status === "failed"
						? "last check failed"
						: `${latest.available}/${latest.total} alive`}
					{latest.status === "completed" && latest.avg_latency_ms > 0
						? ` · ⚡${latest.avg_latency_ms}ms avg`
						: ""}
				</p>
			) : null}
		</button>
	);
}

export function SubList({
	subs,
	latestJobs,
	loading,
	selectedId,
	liveProgressPct,
	onSelect,
	onAdd,
}: {
	subs: Subscription[];
	latestJobs: Record<string, LatestJobSummary>;
	loading: boolean;
	selectedId: string | null;
	liveProgressPct: number | null;
	onSelect: (id: string) => void;
	onAdd: () => void;
}) {
	return (
		<div className="flex h-full min-h-0 flex-col">
			<div className="flex shrink-0 items-center justify-between border-border border-b px-4 py-3">
				<h2 className="font-semibold text-foreground text-sm">
					Subscriptions
				</h2>
				<Button variant="outline" size="sm" onClick={onAdd}>
					<Plus size={13} /> Add
				</Button>
			</div>

			<div className="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
				{loading
					? Array.from({ length: 3 }).map((_, i) => (
							<div key={i} className="space-y-2 px-3 py-2.5">
								<Skeleton className="h-4 w-36" />
								<Skeleton className="h-3 w-24" />
							</div>
						))
					: subs.map((sub) => (
							<SubListItem
								key={sub.id}
								sub={sub}
								latest={latestJobs[sub.id]}
								selected={sub.id === selectedId}
								liveProgressPct={
									sub.id === selectedId ? liveProgressPct : null
								}
								onSelect={() => onSelect(sub.id)}
							/>
						))}
			</div>

			<UnlockStrip />
		</div>
	);
}
```

- [ ] **Step 3: Rewrite the index route as the workbench shell**

Replace the entire contents of `frontend/src/routes/index.tsx`:

```tsx
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Inbox } from "lucide-react";
import { useState } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { SubList } from "@/components/workbench/sub-list";
import { useLatestJobs, useSubscriptions } from "@/queries";

const searchSchema = z.object({
	sub: z.string().optional(),
});

export const Route = createFileRoute("/")({
	validateSearch: searchSchema,
	component: WorkbenchPage,
});

function WorkbenchPage() {
	const navigate = useNavigate({ from: "/" });
	const { sub: selectedFromUrl } = Route.useSearch();
	const [addOpen, setAddOpen] = useState(false);

	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};

	// Invalid/deleted ?sub falls back to no selection (spec: silent).
	const selected = subs.find((s) => s.id === selectedFromUrl);
	const selectedId = selected?.id ?? null;

	const select = (id: string | null) =>
		navigate({ search: id ? { sub: id } : {}, replace: false });

	return (
		<div className="flex h-full min-h-0">
			{/* List column: full-width on mobile (hidden when a sub is open) */}
			<div
				className={cn(
					"h-full w-full min-w-0 border-border md:w-[280px] md:shrink-0 md:border-r lg:w-[300px]",
					selectedId ? "hidden md:block" : "block",
				)}
			>
				<SubList
					subs={subs}
					latestJobs={latestJobs}
					loading={subsQuery.isLoading}
					selectedId={selectedId}
					liveProgressPct={null /* wired in Task 15 */}
					onSelect={(id) => select(id)}
					onAdd={() => setAddOpen(true)}
				/>
			</div>

			{/* Detail pane */}
			<div
				className={cn(
					"h-full min-w-0 flex-1",
					selectedId ? "block" : "hidden md:block",
				)}
			>
				{selected ? (
					<DetailPane key={selected.id} sub={selected} onBack={() => select(null)} />
				) : subs.length === 0 && !subsQuery.isLoading ? (
					<EmptyState
						icon={Inbox}
						title="No subscriptions yet"
						description="Add your first subscription URL to start checking nodes."
						action={
							<Button variant="success" onClick={() => setAddOpen(true)}>
								Add subscription
							</Button>
						}
					/>
				) : (
					<EmptyState
						icon={Inbox}
						title="Select a subscription"
						description="Pick a subscription on the left to see its nodes and run checks."
					/>
				)}
			</div>

			{/* Dialogs (Task 12) */}
			<SubscriptionDialog open={addOpen} onOpenChange={setAddOpen} />
		</div>
	);
}

// Placeholder until Task 13–15 build the real detail pane.
function DetailPane({
	sub,
	onBack,
}: {
	sub: { id: string; name: string; url: string };
	onBack: () => void;
}) {
	return (
		<div className="p-6">
			<button
				type="button"
				onClick={onBack}
				className="mb-2 text-muted-foreground text-xs md:hidden"
			>
				← Back
			</button>
			<p className="font-semibold text-foreground">{sub.name || sub.url}</p>
			<p className="text-muted-foreground text-xs">Detail pane lands in Task 13.</p>
		</div>
	);
}
```

Add these imports used above: `import { cn } from "@/lib/utils";` and `import { SubscriptionDialog } from "@/components/workbench/subscription-dialog";`.

> This task references `SubscriptionDialog` which Task 12 creates. **Do Task 12 immediately after; the two commit together** (Step 5 below is shared). If executing strictly task-by-task, create a minimal `subscription-dialog.tsx` exporting a `null`-rendering component first, then let Task 12 fill it.

- [ ] **Step 4: (bridging stub so this task compiles alone)**

Create `frontend/src/components/workbench/subscription-dialog.tsx`:

```tsx
export function SubscriptionDialog(_props: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	return null; // Replaced with the real dialog in Task 12.
}
```

- [ ] **Step 5: Verify and commit**

```bash
cd frontend && bun check-types && bun check
```

Browser: `/` shows list column with status dots + summaries (latest-jobs powered), unlock strip popover works, clicking a sub sets `?sub=` and shows the placeholder pane; at 375px list↔detail switch works.

```bash
git add frontend/src
git commit -m "feat(frontend): workbench shell with subscription list column and unlock strip"
```

---

## Task 12: Subscription add/edit dialog + delete confirm

**Files:**
- Rewrite: `frontend/src/components/workbench/subscription-dialog.tsx`

- [ ] **Step 1: Implement the real dialog**

Replace `frontend/src/components/workbench/subscription-dialog.tsx`:

```tsx
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { isApiError } from "@/lib/client";
import type { subscription } from "@/lib/client.gen";
import { useCreateSubscription, useUpdateSubscription } from "@/queries";

type Subscription = subscription.Subscription;

function isValidHttpUrl(value: string): boolean {
	try {
		const u = new URL(value);
		return u.protocol === "http:" || u.protocol === "https:";
	} catch {
		return false;
	}
}

// One dialog for both create (sub == null) and edit (sub set).
export function SubscriptionDialog({
	open,
	onOpenChange,
	sub,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	sub?: Subscription | null;
}) {
	const editing = !!sub;
	const [name, setName] = useState("");
	const [url, setUrl] = useState("");
	const [enabled, setEnabled] = useState(true);
	const [urlError, setUrlError] = useState<string | null>(null);

	// Re-seed form whenever the dialog opens for a different target.
	useEffect(() => {
		if (open) {
			setName(sub?.name ?? "");
			setUrl(sub?.url ?? "");
			setEnabled(sub?.enabled ?? true);
			setUrlError(null);
		}
	}, [open, sub]);

	const createMut = useCreateSubscription();
	const updateMut = useUpdateSubscription();
	const pending = createMut.isPending || updateMut.isPending;

	function submit() {
		if (!isValidHttpUrl(url)) {
			setUrlError("Must be a valid http(s) URL");
			return;
		}
		setUrlError(null);
		const onError = (e: unknown) =>
			toast.error(isApiError(e) ? e.message : "Request failed");
		if (editing && sub) {
			updateMut.mutate(
				{
					id: sub.id,
					params: {
						name,
						url,
						enabled,
						cron_expr: sub.cron_expr ?? "",
						clear_cron_expr: false,
					},
				},
				{
					onSuccess: () => {
						toast.success("Subscription updated");
						onOpenChange(false);
					},
					onError,
				},
			);
		} else {
			createMut.mutate(
				{ name, url, cron_expr: "" },
				{
					onSuccess: () => {
						toast.success("Subscription added");
						onOpenChange(false);
					},
					onError,
				},
			);
		}
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogTitle>
					{editing ? "Edit subscription" : "Add subscription"}
				</DialogTitle>
				<DialogDescription>
					Paste a Clash/V2Ray subscription URL.
				</DialogDescription>

				<div className="mt-4 space-y-3">
					<div className="space-y-1.5">
						<Label htmlFor="sub-name" className="text-xs">
							Name <span className="text-muted-foreground">(optional)</span>
						</Label>
						<Input
							id="sub-name"
							value={name}
							placeholder="My provider"
							onChange={(e) => setName(e.target.value)}
						/>
					</div>
					<div className="space-y-1.5">
						<Label htmlFor="sub-url" className="text-xs">
							URL
						</Label>
						<Input
							id="sub-url"
							value={url}
							placeholder="https://…"
							className="font-mono"
							aria-invalid={!!urlError}
							onChange={(e) => {
								setUrl(e.target.value);
								if (urlError) setUrlError(null);
							}}
						/>
						{urlError ? (
							<p className="text-danger text-xs">⚠ {urlError}</p>
						) : null}
					</div>
					{editing ? (
						<label className="flex cursor-pointer items-center gap-2 text-sm">
							<Checkbox
								checked={enabled}
								onCheckedChange={(v) => setEnabled(v === true)}
							/>
							Enabled
						</label>
					) : null}
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={pending}
						disabled={!url}
						onClick={submit}
					>
						{editing ? "Save" : "Add"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
```

> Delete confirmation lives with the detail header's `⋯` menu (Task 13) via the shared `ConfirmDialog` — the message there must name the subscription and warn about nodes + history, per spec.

- [ ] **Step 2: Verify and commit**

```bash
cd frontend && bun check-types && bun check
```

Browser: "+ Add" opens the dialog; invalid URL shows the inline error; adding refreshes the list (TanStack Query invalidation); mobile (375px) dialog is full-screen.

```bash
git add frontend/src/components/workbench/subscription-dialog.tsx
git commit -m "feat(frontend): subscription add/edit dialog with URL validation"
```

---

## Task 13: Detail header — run check, export, history, edit/delete

**Files:**
- Create: `frontend/src/components/copy-button.tsx`
- Create: `frontend/src/components/check-options-fields.tsx`
- Create: `frontend/src/components/workbench/run-check-button.tsx`
- Create: `frontend/src/components/workbench/export-popover.tsx`
- Create: `frontend/src/components/workbench/detail-header.tsx`

- [ ] **Step 1: CopyButton (shared)**

Create `frontend/src/components/copy-button.tsx`:

```tsx
import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";

export function CopyButton({
	text,
	className,
}: {
	text: string;
	className?: string;
}) {
	const [copied, setCopied] = useState(false);
	return (
		<button
			type="button"
			aria-label="Copy to clipboard"
			onClick={() => {
				navigator.clipboard.writeText(text);
				setCopied(true);
				setTimeout(() => setCopied(false), 1500);
			}}
			className={cn(
				"shrink-0 rounded p-1 transition-colors hover:bg-secondary",
				copied ? "text-success" : "text-muted-foreground",
				className,
			)}
		>
			{copied ? <Check size={12} /> : <Copy size={12} />}
		</button>
	);
}
```

- [ ] **Step 2: Check options fields (shared between Run Check popover and Schedule dialog)**

Create `frontend/src/components/check-options-fields.tsx`:

```tsx
import { Checkbox } from "@/components/ui/checkbox";
import type { PlatformKey } from "@/components/platform-icons";
import { PlatformIcon } from "@/components/platform-icons";
import type { CheckFormOptions } from "@/lib/checkOptions";
import { MEDIA_APPS } from "@/lib/checkOptions";
import { cn } from "@/lib/utils";

// Controlled fieldset for check options. Immutable updates only — parent owns
// the state (Run Check popover persists to localStorage, Schedule dialog
// submits to the scheduler).
export function CheckOptionsFields({
	value,
	onChange,
	showDebug = false,
}: {
	value: CheckFormOptions;
	onChange: (next: CheckFormOptions) => void;
	showDebug?: boolean;
}) {
	const toggleApp = (app: string) =>
		onChange({
			...value,
			media_apps: value.media_apps.includes(app)
				? value.media_apps.filter((a) => a !== app)
				: [...value.media_apps, app],
		});

	return (
		<div className="space-y-3">
			<div>
				<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Check options
				</p>
				<div className="space-y-1.5">
					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={value.speed_test}
							onCheckedChange={(v) =>
								onChange({ ...value, speed_test: v === true })
							}
						/>
						Speed test{" "}
						<span className="text-muted-foreground text-xs">(download)</span>
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
								onCheckedChange={(v) =>
									onChange({ ...value, debug: v === true })
								}
							/>
							Debug mode
						</label>
					) : null}
				</div>
			</div>

			<div>
				<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
					Media platforms
				</p>
				<div className="flex flex-wrap gap-1.5">
					{MEDIA_APPS.map((app) => {
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
								<PlatformIcon platform={app as PlatformKey} size={12} showLabel />
							</button>
						);
					})}
				</div>
			</div>
		</div>
	);
}
```

- [ ] **Step 3: Run Check split button**

Create `frontend/src/components/workbench/run-check-button.tsx`:

```tsx
import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, Play } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { CheckOptionsFields } from "@/components/check-options-fields";
import { queryKeys } from "@/queries";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	type CheckFormOptions,
	loadCheckOptions,
	saveCheckOptions,
} from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import { useTriggerCheck } from "@/queries";

// Split button: primary click re-runs with the last-used options
// (localStorage per subscription); the chevron opens the options popover.
export function RunCheckButton({
	subscriptionId,
	disabled,
	onStarted,
}: {
	subscriptionId: string;
	disabled?: boolean;
	onStarted: (jobId: string) => void;
}) {
	const [open, setOpen] = useState(false);
	const [opts, setOpts] = useState<CheckFormOptions>(() =>
		loadCheckOptions(subscriptionId),
	);
	const triggerMut = useTriggerCheck();
	const qc = useQueryClient();

	function start(withOpts: CheckFormOptions) {
		saveCheckOptions(subscriptionId, withOpts);
		triggerMut.mutate(
			{ subscriptionId, params: withOpts },
			{
				onSuccess: (resp) => {
					setOpen(false);
					toast.success("Check started");
					onStarted(resp.job_id);
				},
				onError: (e) => {
					toast.error(
						isApiError(e) ? e.message : "Failed to start check",
					);
					// 409/412 = a check is already running (manual or scheduled).
					// Refresh latest-jobs so the workbench effect attaches to it.
					if (isApiError(e) && (e.status === 409 || e.status === 412)) {
						qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
					}
				},
			},
		);
	}

	return (
		<div className="flex">
			<Button
				variant="success"
				className="rounded-r-none"
				loading={triggerMut.isPending}
				disabled={disabled}
				onClick={() => start(opts)}
			>
				<Play size={13} /> Run Check
			</Button>
			<Popover open={open} onOpenChange={setOpen}>
				<PopoverTrigger
					render={
						<Button
							variant="success"
							size="icon"
							aria-label="Check options"
							className="rounded-l-none border-white/20 border-l"
							disabled={disabled || triggerMut.isPending}
						/>
					}
				>
					<ChevronDown size={14} />
				</PopoverTrigger>
				<PopoverContent className="w-80">
					<CheckOptionsFields value={opts} onChange={setOpts} showDebug />
					<Button
						variant="success"
						className="mt-3 w-full"
						loading={triggerMut.isPending}
						onClick={() => start(opts)}
					>
						Start check
					</Button>
				</PopoverContent>
			</Popover>
		</div>
	);
}
```

> Base UI composition note: `PopoverTrigger` accepts a `render` prop to merge onto a custom element (same pattern as `TooltipPrimitive.Trigger render={…}` in Task 8). If the installed version exposes a different composition prop, check `node_modules/@base-ui/react` — only the trigger wiring changes.

- [ ] **Step 4: Export popover**

Create `frontend/src/components/workbench/export-popover.tsx`:

```tsx
import { Download } from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { useAPIKey } from "@/queries";

const FORMATS = ["clash", "base64", "routeros"] as const;

export function ExportPopover({ subscriptionId }: { subscriptionId: string }) {
	const apiKeyQuery = useAPIKey();
	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const base = `${window.location.origin}/api/export/${subscriptionId}`;

	return (
		<Popover>
			<PopoverTrigger
				render={<Button variant="outline" size="sm" />}
			>
				<Download size={13} /> Export
			</PopoverTrigger>
			<PopoverContent className="w-96">
				<p className="mb-2 font-medium text-foreground text-xs">
					Subscription URLs
				</p>
				{apiKeyQuery.isLoading ? (
					<Spinner />
				) : (
					<div className="space-y-1.5">
						{FORMATS.map((t) => {
							const url = `${base}?token=${apiKey}&target=${t}`;
							return (
								<div key={t} className="flex items-center gap-2">
									<span className="w-16 shrink-0 text-[11px] text-muted-foreground">
										{t}
									</span>
									<code className="min-w-0 flex-1 truncate rounded bg-secondary px-2 py-1 font-mono text-[11px] text-foreground">
										{url}
									</code>
									<CopyButton text={url} />
								</div>
							);
						})}
						<p className="pt-1 text-[11px] text-muted-foreground">
							All-subscriptions URLs live in Settings → Export API.
						</p>
					</div>
				)}
			</PopoverContent>
		</Popover>
	);
}
```

- [ ] **Step 5: Detail header**

Create `frontend/src/components/workbench/detail-header.tsx`:

```tsx
import { ArrowLeft, MoreHorizontal } from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { ExportPopover } from "@/components/workbench/export-popover";
import { RunCheckButton } from "@/components/workbench/run-check-button";
import type { checker, subscription } from "@/lib/client.gen";

type Subscription = subscription.Subscription;
type JobSummary = checker.JobSummary;

function jobLabel(j: JobSummary): string {
	const when = new Date(j.created_at).toLocaleString(undefined, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
	if (j.status === "running" || j.status === "queued") {
		return `${when} · running`;
	}
	if (j.status === "failed") return `${when} · failed`;
	return `${when} · ${j.available}/${j.total}`;
}

export function DetailHeader({
	sub,
	jobs,
	selectedJobId,
	activeJobId,
	onSelectJob,
	onRunStarted,
	onEdit,
	onToggleEnabled,
	onDelete,
	onBack,
}: {
	sub: Subscription;
	jobs: JobSummary[];
	selectedJobId: string | null; // null = latest completed
	activeJobId: string | null; // currently running job (SSE attached)
	onSelectJob: (jobId: string | null) => void;
	onRunStarted: (jobId: string) => void;
	onEdit: () => void;
	onToggleEnabled: () => void;
	onDelete: () => void;
	onBack: () => void;
}) {
	return (
		<div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-border border-b px-4 py-3 md:px-5">
			<button
				type="button"
				onClick={onBack}
				aria-label="Back to list"
				className="rounded-md p-1 text-muted-foreground hover:bg-secondary md:hidden"
			>
				<ArrowLeft size={16} />
			</button>

			<div className="min-w-0 flex-1 basis-48">
				<h1 className="truncate font-semibold text-[15px] text-foreground">
					{sub.name || sub.url}
				</h1>
				<div className="flex items-center gap-1">
					<p className="truncate font-mono text-[11px] text-muted-foreground">
						{sub.url}
					</p>
					<CopyButton text={sub.url} />
				</div>
			</div>

			<div className="flex items-center gap-2">
				{jobs.length > 0 ? (
					<Select
						value={selectedJobId ?? "latest"}
						onValueChange={(v: string) =>
							onSelectJob(v === "latest" ? null : v)
						}
					>
						<SelectTrigger
							size="sm"
							aria-label="Check history"
							className="max-w-52 text-xs"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="latest">Latest result</SelectItem>
							{jobs.map((j) => (
								<SelectItem key={j.id} value={j.id}>
									{jobLabel(j)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				) : null}

				<ExportPopover subscriptionId={sub.id} />

				<DropdownMenu>
					<DropdownMenuTrigger
						render={
							<Button
								variant="outline"
								size="icon-sm"
								aria-label="Subscription actions"
							/>
						}
					>
						<MoreHorizontal size={14} />
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuItem onClick={onEdit}>Edit</DropdownMenuItem>
						<DropdownMenuItem onClick={onToggleEnabled}>
							{sub.enabled ? "Disable" : "Enable"}
						</DropdownMenuItem>
						<DropdownMenuItem
							onClick={onDelete}
							className="text-danger focus:text-danger"
						>
							Delete
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>

				<RunCheckButton
					subscriptionId={sub.id}
					disabled={!sub.enabled || !!activeJobId}
					onStarted={onRunStarted}
				/>
			</div>
		</div>
	);
}
```

> `select.tsx` exists from the old `packages/ui`. Open it and match exports (`Select/SelectTrigger/SelectValue/SelectContent/SelectItem` is the shadcn standard; if `SelectTrigger` lacks a `size` prop, drop it and add `className="h-7"`). Same drill as dropdown-menu.

- [ ] **Step 6: Verify compile, commit**

```bash
cd frontend && bun check-types && bun check
git add frontend/src/components
git commit -m "feat(frontend): detail header cluster — run-check split button, export popover, history select"
```

---

## Task 14: Results section — chips, filter bar, sortable node table

**Files:**
- Create: `frontend/src/components/workbench/node-table.tsx`
- Create: `frontend/src/components/workbench/results-section.tsx`

- [ ] **Step 1: New NodeTable (sortable, tone-badged, responsive)**

Create `frontend/src/components/workbench/node-table.tsx`:

```tsx
import { ArrowDown, ArrowUp } from "lucide-react";
import { PlatformIcon, PlatformIconAny } from "@/components/platform-icons";
import { Badge } from "@/components/ui/badge";
import { Tooltip } from "@/components/ui/tooltip";
import type { checker } from "@/lib/client.gen";
import {
	latencyTone,
	type SortDir,
	type SortKey,
} from "@/lib/nodeFilters";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

function formatSpeed(kbps: number): string {
	return kbps >= 1024 ? `${(kbps / 1024).toFixed(1)} MB/s` : `${kbps} KB/s`;
}

const toneText: Record<string, string> = {
	success: "text-success",
	warning: "text-warning",
	danger: "text-danger",
};

function Latency({ r }: { r: NodeResult }) {
	if (!r.alive) return <span className="text-muted-foreground/60">—</span>;
	return (
		<span className={cn("font-medium", toneText[latencyTone(r.latency_ms)])}>
			{r.latency_ms}ms
		</span>
	);
}

function UnlockIcons({
	r,
	ruleByKey,
}: {
	r: NodeResult;
	ruleByKey: Record<string, PlatformRule>;
}) {
	return (
		<div className="flex flex-wrap items-center gap-1.5">
			{r.netflix && <PlatformIcon platform="netflix" />}
			{r.youtube && !r.youtube_premium && <PlatformIcon platform="youtube" />}
			{r.youtube_premium && <PlatformIcon platform="youtube_premium" />}
			{r.openai && <PlatformIcon platform="openai" />}
			{r.claude && <PlatformIcon platform="claude" />}
			{r.gemini && <PlatformIcon platform="gemini" />}
			{r.grok && <PlatformIcon platform="grok" />}
			{r.disney && <PlatformIcon platform="disney" />}
			{r.tiktok && <PlatformIcon platform="tiktok" />}
			{r.extra_platforms &&
				Object.entries(r.extra_platforms)
					.filter(([, v]) => v)
					.map(([key]) => {
						const rule = ruleByKey[key];
						return (
							<PlatformIconAny
								key={key}
								platformKey={key}
								icon={rule?.icon}
								label={rule?.name ?? key}
							/>
						);
					})}
		</div>
	);
}

function SortHeader({
	label,
	myKey,
	sortKey,
	sortDir,
	onSort,
	className,
}: {
	label: string;
	myKey: SortKey;
	sortKey: SortKey;
	sortDir: SortDir;
	onSort: (key: SortKey) => void;
	className?: string;
}) {
	const active = sortKey === myKey;
	return (
		<th className={cn("px-3 py-2 text-left", className)}>
			<button
				type="button"
				onClick={() => onSort(myKey)}
				className={cn(
					"inline-flex items-center gap-1 font-medium text-[11px] uppercase tracking-[0.4px] transition-colors",
					active ? "text-primary" : "text-muted-foreground hover:text-foreground",
				)}
			>
				{label}
				{active ? (
					sortDir === "asc" ? (
						<ArrowUp size={11} />
					) : (
						<ArrowDown size={11} />
					)
				) : null}
			</button>
		</th>
	);
}

function EnableToggle({
	r,
	onToggleEnabled,
}: {
	r: NodeResult;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}) {
	if (!onToggleEnabled) return null;
	return (
		<Tooltip content={r.enabled ? "Exclude from exports" : "Include in exports"}>
			<button
				type="button"
				aria-pressed={r.enabled}
				onClick={() => onToggleEnabled(r.node_id, !r.enabled)}
				className={cn(
					"rounded-full p-1 text-[13px] leading-none transition-colors hover:bg-secondary",
					r.enabled ? "text-success" : "text-muted-foreground/50",
				)}
			>
				{r.enabled ? "●" : "○"}
			</button>
		</Tooltip>
	);
}

export interface NodeTableProps {
	results: NodeResult[];
	rules?: PlatformRule[];
	sortKey: SortKey;
	sortDir: SortDir;
	onSort: (key: SortKey) => void;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}

export function NodeTable({
	results,
	rules = [],
	sortKey,
	sortDir,
	onSort,
	onToggleEnabled,
}: NodeTableProps) {
	const ruleByKey = Object.fromEntries(rules.map((r) => [r.key, r]));

	return (
		<>
			{/* Mobile: cards */}
			<div className="space-y-2 md:hidden">
				{results.map((r) => (
					<div
						key={r.node_id}
						className={cn(
							"rounded-lg border border-border bg-card p-3",
							!r.enabled && "opacity-50",
						)}
					>
						<div className="flex items-center gap-2">
							<span className="min-w-0 flex-1 truncate font-mono text-foreground text-xs">
								{r.node_name}
							</span>
							<Badge tone={r.alive ? "success" : "danger"}>
								{r.alive ? "alive" : "dead"}
							</Badge>
							<EnableToggle r={r} onToggleEnabled={onToggleEnabled} />
						</div>
						<div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs tabular-nums">
							<span>
								<Latency r={r} />
							</span>
							{r.alive && r.speed_kbps ? (
								<span className="text-foreground">
									↓ {formatSpeed(r.speed_kbps)}
								</span>
							) : null}
							{r.alive && r.upload_speed_kbps ? (
								<span className="text-muted-foreground">
									↑ {formatSpeed(r.upload_speed_kbps)}
								</span>
							) : null}
							{r.traffic_bytes > 0 ? (
								<span className="text-muted-foreground">
									{formatBytes(r.traffic_bytes)}
								</span>
							) : null}
							{r.country ? (
								<span className="text-muted-foreground">{r.country}</span>
							) : null}
						</div>
						<div className="mt-2">
							<UnlockIcons r={r} ruleByKey={ruleByKey} />
						</div>
					</div>
				))}
			</div>

			{/* Desktop: table */}
			<div className="hidden overflow-hidden rounded-lg border border-border md:block">
				<table className="w-full border-collapse text-[12.5px]">
					<thead>
						<tr className="border-border border-b bg-card">
							<th className="w-8 px-2 py-2" aria-label="Enabled" />
							<SortHeader
								label="Node"
								myKey="name"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<SortHeader
								label="Latency"
								myKey="latency"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<SortHeader
								label="↓ Speed"
								myKey="speed"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] lg:table-cell">
								↑ Upload
							</th>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] lg:table-cell">
								Traffic
							</th>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] xl:table-cell">
								Country
							</th>
							<th className="px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
								Unlocks
							</th>
						</tr>
					</thead>
					<tbody className="tabular-nums">
						{results.map((r) => (
							<tr
								key={r.node_id}
								className={cn(
									"border-secondary border-b transition-colors last:border-0 hover:bg-secondary/40",
									!r.enabled && "opacity-50",
								)}
							>
								<td className="px-2 py-1.5">
									<EnableToggle r={r} onToggleEnabled={onToggleEnabled} />
								</td>
								<td
									className={cn(
										"max-w-52 truncate px-3 py-1.5 font-mono text-[11px]",
										r.alive ? "text-foreground" : "text-muted-foreground/70",
									)}
								>
									{r.node_name}
								</td>
								<td className="px-3 py-1.5">
									<Latency r={r} />
								</td>
								<td className="px-3 py-1.5">
									{r.alive && r.speed_kbps ? (
										formatSpeed(r.speed_kbps)
									) : (
										<span className="text-muted-foreground/60">—</span>
									)}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground lg:table-cell">
									{r.alive && r.upload_speed_kbps
										? formatSpeed(r.upload_speed_kbps)
										: "—"}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground lg:table-cell">
									{formatBytes(r.traffic_bytes)}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground xl:table-cell">
									{r.country || "—"}
								</td>
								<td className="px-3 py-1.5">
									<UnlockIcons r={r} ruleByKey={ruleByKey} />
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</>
	);
}
```

> Unlock-icon tooltips (spec: "platform icons with tooltips"): `PlatformIcon`/`PlatformIconAny` come from the existing `platform-icons.tsx`. Verify each icon exposes a hover label (a `title` attribute or `showLabel`); if not, wrap each icon in the `Tooltip` component from Task 8 with the platform name from `PLATFORM_META` — change `UnlockIcons` only.

- [ ] **Step 2: Results section (chips + filter bar + table glue)**

Create `frontend/src/components/workbench/results-section.tsx`:

```tsx
import { ChevronDown, SearchX } from "lucide-react";
import { useMemo, useState } from "react";
import { PlatformIconAny } from "@/components/platform-icons";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { NodeTable } from "@/components/workbench/node-table";
import type { checker } from "@/lib/client.gen";
import {
	BUILTIN_PLATFORMS,
	filterNodes,
	nodeHasPlatform,
	sortNodes,
	type SortDir,
	type SortKey,
} from "@/lib/nodeFilters";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

function Chip({ children }: { children: React.ReactNode }) {
	return (
		<span className="inline-flex items-center gap-1 rounded-full border border-border bg-card px-3 py-1 text-xs tabular-nums">
			{children}
		</span>
	);
}

export function ResultsSection({
	results,
	rules = [],
	onToggleEnabled,
}: {
	results: NodeResult[];
	rules?: PlatformRule[];
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}) {
	const [text, setText] = useState("");
	const [aliveOnly, setAliveOnly] = useState(false);
	const [platforms, setPlatforms] = useState<string[]>([]);
	const [sortKey, setSortKey] = useState<SortKey>("latency");
	const [sortDir, setSortDir] = useState<SortDir>("asc");

	const handleSort = (key: SortKey) => {
		if (key === sortKey) {
			setSortDir((d) => (d === "asc" ? "desc" : "asc"));
		} else {
			setSortKey(key);
			setSortDir(key === "speed" ? "desc" : "asc");
		}
	};

	const alive = results.filter((r) => r.alive);
	const avgLatency =
		alive.length > 0
			? Math.round(
					alive.reduce((sum, r) => sum + r.latency_ms, 0) / alive.length,
				)
			: 0;
	const peakSpeed = alive.reduce((max, r) => Math.max(max, r.speed_kbps), 0);

	// Unlock counts for chips: top platforms with at least one unlocked node.
	const unlockCounts = useMemo(() => {
		const keys: string[] = [
			...BUILTIN_PLATFORMS,
			...rules.map((r) => r.key),
		];
		return keys
			.map((key) => ({
				key,
				count: alive.filter((r) => nodeHasPlatform(r, key)).length,
			}))
			.filter((e) => e.count > 0)
			.sort((a, b) => b.count - a.count)
			.slice(0, 4);
	}, [alive, rules]);

	const visible = useMemo(
		() =>
			sortNodes(
				filterNodes(results, { text, aliveOnly, platforms }),
				sortKey,
				sortDir,
			),
		[results, text, aliveOnly, platforms, sortKey, sortDir],
	);

	const togglePlatform = (key: string) =>
		setPlatforms((prev) =>
			prev.includes(key) ? prev.filter((p) => p !== key) : [...prev, key],
		);

	return (
		<div className="space-y-3">
			{/* Summary chips */}
			<div className="flex flex-wrap gap-2">
				<Chip>
					<b className="text-success">{alive.length}</b> alive
				</Chip>
				<Chip>
					<b className="text-danger">{results.length - alive.length}</b> dead
				</Chip>
				{avgLatency > 0 ? (
					<Chip>
						avg <b>{avgLatency}ms</b>
					</Chip>
				) : null}
				{peakSpeed > 0 ? (
					<Chip>
						⬇{" "}
						<b>
							{peakSpeed >= 1024
								? `${(peakSpeed / 1024).toFixed(1)} MB/s`
								: `${peakSpeed} KB/s`}
						</b>{" "}
						peak
					</Chip>
				) : null}
				{unlockCounts.map((e) => (
					<Chip key={e.key}>
						{e.key} <b className="text-success">{e.count}</b>
					</Chip>
				))}
			</div>

			{/* Filter bar */}
			<div className="flex flex-wrap items-center gap-2">
				<Input
					value={text}
					onChange={(e) => setText(e.target.value)}
					placeholder="Filter nodes…"
					className="h-7 w-44 text-xs"
				/>
				<button
					type="button"
					aria-pressed={aliveOnly}
					onClick={() => setAliveOnly((v) => !v)}
					className={cn(
						"rounded-full border px-3 py-1 font-medium text-xs transition-colors",
						aliveOnly
							? "border-info-line bg-info-muted text-info"
							: "border-border text-muted-foreground hover:bg-secondary",
					)}
				>
					Alive only
				</button>
				<Popover>
					<PopoverTrigger
						className={cn(
							"inline-flex items-center gap-1 rounded-full border px-3 py-1 font-medium text-xs outline-none transition-colors",
							platforms.length > 0
								? "border-info-line bg-info-muted text-info"
								: "border-border text-muted-foreground hover:bg-secondary",
						)}
					>
						Unlocks{platforms.length > 0 ? ` (${platforms.length})` : ""}
						<ChevronDown size={12} />
					</PopoverTrigger>
					<PopoverContent className="w-72">
						<p className="mb-2 font-medium text-foreground text-xs">
							Show nodes unlocking
						</p>
						<div className="flex flex-wrap gap-1.5">
							{[...BUILTIN_PLATFORMS, ...rules.map((r) => r.key)].map(
								(key) => (
									<button
										key={key}
										type="button"
										aria-pressed={platforms.includes(key)}
										onClick={() => togglePlatform(key)}
										className={cn(
											"rounded-full border px-2.5 py-1 text-xs transition-colors",
											platforms.includes(key)
												? "border-info-line bg-info-muted text-info"
												: "border-border text-muted-foreground hover:bg-secondary",
										)}
									>
										{key}
									</button>
								),
							)}
						</div>
					</PopoverContent>
				</Popover>
				<span className="ml-auto text-[11px] text-muted-foreground tabular-nums">
					{visible.length} of {results.length} shown
				</span>
			</div>

			{visible.length === 0 ? (
				<EmptyState
					icon={SearchX}
					title="No nodes match"
					description="Loosen the filters above."
				/>
			) : (
				<NodeTable
					results={visible}
					rules={rules}
					sortKey={sortKey}
					sortDir={sortDir}
					onSort={handleSort}
					onToggleEnabled={onToggleEnabled}
				/>
			)}
		</div>
	);
}
```

- [ ] **Step 3: Verify and commit**

```bash
cd frontend && bun check-types && bun check && bun run test:unit
git add frontend/src/components/workbench
git commit -m "feat(frontend): sortable filterable node table with summary chips"
```

---

## Task 15: Running state + detail pane composition

**Files:**
- Create: `frontend/src/components/workbench/progress-panel.tsx`
- Create: `frontend/src/components/workbench/detail-pane.tsx`
- Modify: `frontend/src/routes/index.tsx` (replace placeholder, wire SSE up to the list)

- [ ] **Step 1: Progress panel**

Create `frontend/src/components/workbench/progress-panel.tsx`:

```tsx
import { ChevronDown, ChevronRight, WifiOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Spinner } from "@/components/ui/spinner";
import type { SSEConnection, SSEProgress } from "@/queries";
import { cn } from "@/lib/utils";

function formatElapsed(startedAt: number): string {
	const s = Math.floor((Date.now() - startedAt) / 1000);
	return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
}

export function ProgressPanel({
	progress,
	logEntries,
	connection,
	cancelPending,
	onCancel,
}: {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	connection: SSEConnection;
	cancelPending: boolean;
	onCancel: () => void;
}) {
	const [logOpen, setLogOpen] = useState(false);
	const startedAtRef = useRef(Date.now());
	const logRef = useRef<HTMLDivElement | null>(null);
	const [, forceTick] = useState(0);

	// Re-render every second for the elapsed clock.
	useEffect(() => {
		const t = setInterval(() => forceTick((n) => n + 1), 1000);
		return () => clearInterval(t);
	}, []);

	// Auto-scroll the log container only (not the page).
	// biome-ignore lint/correctness/useExhaustiveDependencies: scroll on new entries
	useEffect(() => {
		const el = logRef.current;
		if (el) el.scrollTop = el.scrollHeight;
	}, [logEntries.length]);

	const done = progress?.total ?? 0;
	const current = progress?.progress ?? 0;
	const pct = done > 0 ? (current / done) * 100 : 0;
	const aliveSoFar = logEntries.filter((e) => e.alive).length;
	const reconnecting = connection === "reconnecting";

	// ETA from the average pace so far (spec: "elapsed + ETA").
	const elapsedSec = (Date.now() - startedAtRef.current) / 1000;
	const eta =
		current > 0 && done > current
			? Math.round(((done - current) * elapsedSec) / current)
			: null;
	const etaLabel =
		eta !== null
			? ` · ~${String(Math.floor(eta / 60)).padStart(2, "0")}:${String(eta % 60).padStart(2, "0")} left`
			: "";

	return (
		<div className="space-y-2.5 rounded-lg border border-info-line bg-info-muted/30 p-4">
			<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
				{reconnecting ? (
					<WifiOff size={14} className="text-warning" />
				) : (
					<Spinner className="size-3.5 text-info" />
				)}
				<span className="font-medium text-foreground text-sm">
					{reconnecting ? "Reconnecting…" : "Checking nodes…"}
				</span>
				<span className="text-muted-foreground text-xs tabular-nums">
					{current} / {done || "?"}
				</span>
				<span className="ml-auto text-xs tabular-nums">
					<b className="text-success">{aliveSoFar}</b>{" "}
					<span className="text-muted-foreground">alive so far</span>
				</span>
				<Button
					variant="outline"
					size="sm"
					loading={cancelPending}
					onClick={onCancel}
					className="text-danger"
				>
					Cancel
				</Button>
			</div>

			<Progress value={pct} />

			<div className="flex items-center justify-between text-[11px] text-muted-foreground">
				<button
					type="button"
					onClick={() => setLogOpen((v) => !v)}
					className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
				>
					{logOpen ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
					Live log ({logEntries.length})
				</button>
				<span className="tabular-nums">
					elapsed {formatElapsed(startedAtRef.current)}
					{etaLabel}
				</span>
			</div>

			{logOpen && logEntries.length > 0 ? (
				<div
					ref={logRef}
					className="max-h-44 overflow-y-auto rounded-md bg-background/60 p-2"
				>
					{logEntries.map((e, i) => (
						<div
							key={`${e.node_name}-${i}`}
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
		</div>
	);
}
```

- [ ] **Step 2: Detail pane composition**

Create `frontend/src/components/workbench/detail-pane.tsx`:

```tsx
import { PlayCircle } from "lucide-react";
import { toast } from "sonner";
import { DebugPanel, type NodeDebug } from "@/components/debug-panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { DetailHeader } from "@/components/workbench/detail-header";
import { ProgressPanel } from "@/components/workbench/progress-panel";
import { ResultsSection } from "@/components/workbench/results-section";
import { isApiError } from "@/lib/client";
import type { checker, subscription } from "@/lib/client.gen";
import type { SSEConnection, SSEProgress } from "@/queries";
import {
	useCancelCheck,
	useJobs,
	useResults,
	useRules,
	useSetNodeEnabled,
} from "@/queries";

type Subscription = subscription.Subscription;

export function DetailPane({
	sub,
	activeJobId,
	progress,
	logEntries,
	debugData,
	connection,
	selectedJobId,
	onSelectJob,
	onRunStarted,
	onEdit,
	onToggleEnabled,
	onDelete,
	onBack,
}: {
	sub: Subscription;
	activeJobId: string | null;
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	debugData: NodeDebug[];
	connection: SSEConnection;
	selectedJobId: string | null;
	onSelectJob: (jobId: string | null) => void;
	onRunStarted: (jobId: string) => void;
	onEdit: () => void;
	onToggleEnabled: () => void;
	onDelete: () => void;
	onBack: () => void;
}) {
	const jobsQuery = useJobs(sub.id, { Limit: 20 });
	const rulesQuery = useRules();
	const resultsQuery = useResults(sub.id, selectedJobId);
	const cancelMut = useCancelCheck(sub.id);
	const toggleNodeMut = useSetNodeEnabled(sub.id);

	const running = !!activeJobId && !progress?.done;
	const job = resultsQuery.data?.job;
	const results = resultsQuery.data?.results ?? [];
	const noChecksYet =
		resultsQuery.isError &&
		isApiError(resultsQuery.error) &&
		resultsQuery.error.status === 404;

	const handleCancel = () => {
		if (!activeJobId) return;
		cancelMut.mutate(activeJobId, {
			onSuccess: () => toast.success("Check cancelled"),
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Cancel failed"),
		});
	};

	const handleToggleNode = (nodeId: string, enabled: boolean) => {
		toggleNodeMut.mutate(
			{ nodeId, enabled },
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update node"),
			},
		);
	};

	return (
		<div className="flex h-full min-h-0 flex-col">
			<DetailHeader
				sub={sub}
				jobs={jobsQuery.data?.jobs ?? []}
				selectedJobId={selectedJobId}
				activeJobId={activeJobId}
				onSelectJob={onSelectJob}
				onRunStarted={onRunStarted}
				onEdit={onEdit}
				onToggleEnabled={onToggleEnabled}
				onDelete={onDelete}
				onBack={onBack}
			/>

			<div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 md:p-5">
				{running ? (
					<ProgressPanel
						progress={progress}
						logEntries={logEntries}
						connection={connection}
						cancelPending={cancelMut.isPending}
						onCancel={handleCancel}
					/>
				) : null}

				{debugData.length > 0 ? <DebugPanel data={debugData} /> : null}

				{!running && job ? (
					<div className="flex items-center gap-2 text-xs">
						<Badge
							tone={
								job.status === "completed"
									? "success"
									: job.status === "failed"
										? "danger"
										: "info"
							}
						>
							{job.status}
						</Badge>
						<span className="text-muted-foreground tabular-nums">
							{new Date(job.created_at).toLocaleString()}
						</span>
					</div>
				) : null}

				{running ? (
					// Spec §Running state: alive nodes stream into the table as
					// found. Partial rows (no unlocks/traffic/country yet, no
					// node_id) — the final refetch replaces them with full data.
					streamedResults.length > 0 ? (
						<ResultsSection results={streamedResults} rules={[]} />
					) : null
				) : resultsQuery.isLoading && !noChecksYet ? (
					<div className="space-y-2">
						<Skeleton className="h-8 w-2/3" />
						<Skeleton className="h-40 w-full" />
					</div>
				) : noChecksYet ? (
					<EmptyState
						icon={PlayCircle}
						title="No checks yet"
						description="Run your first check to see node availability, latency and unlocks."
					/>
				) : (
					<ResultsSection
						results={results}
						rules={rulesQuery.data?.rules}
						onToggleEnabled={handleToggleNode}
					/>
				)}

				{!running ? <ExportLogsDisclosure subscriptionId={sub.id} /> : null}
			</div>
		</div>
	);
}

// Compact disclosure for the per-subscription export request log (existing
// feature carried over from the old detail page).
function ExportLogsDisclosure({ subscriptionId }: { subscriptionId: string }) {
	const [open, setOpen] = useState(false);
	const logsQuery = useExportLogs(subscriptionId, { enabled: open });
	const logs = logsQuery.data?.logs ?? [];

	return (
		<div className="rounded-lg border border-border bg-card">
			<button
				type="button"
				onClick={() => setOpen((v) => !v)}
				className="flex w-full items-center justify-between px-4 py-2.5 text-muted-foreground text-xs transition-colors hover:text-foreground"
			>
				<span className="inline-flex items-center gap-2">
					<Download size={13} strokeWidth={1.5} /> Export requests
				</span>
				{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
			</button>
			{open ? (
				<div className="border-border border-t px-4 py-3">
					{logsQuery.isLoading ? (
						<Skeleton className="h-4 w-40" />
					) : logs.length === 0 ? (
						<p className="text-muted-foreground text-xs">
							No export requests yet.
						</p>
					) : (
						<div className="space-y-1">
							{logs.map((log) => (
								<div
									key={log.id}
									className="flex items-center justify-between py-0.5 font-mono text-[11px] tabular-nums"
								>
									<span className="text-foreground">
										{new Date(log.requested_at).toLocaleString()}
									</span>
									<span className="text-muted-foreground">
										{log.ip || "—"}
									</span>
								</div>
							))}
						</div>
					)}
				</div>
			) : null}
		</div>
	);
}
```

Additional imports for `detail-pane.tsx` (beyond those already listed at the top of Step 2): `useMemo`, `useState` from react; `ChevronDown`, `ChevronRight`, `Download` from lucide-react; `useExportLogs` from `@/queries`; and the streamed-rows memo inside `DetailPane` before the return:

```tsx
	const streamedResults: checker.NodeResult[] = useMemo(
		() =>
			logEntries
				.filter((e) => e.alive)
				.map((e, i) => ({
					node_id: `live-${i}`,
					node_name: e.node_name ?? "",
					node_type: "",
					enabled: true,
					alive: true,
					latency_ms: e.latency_ms ?? 0,
					speed_kbps: e.speed_kbps ?? 0,
					upload_speed_kbps: e.upload_speed_kbps ?? 0,
					country: "",
					ip: "",
					netflix: false,
					youtube: false,
					youtube_premium: false,
					openai: false,
					claude: false,
					gemini: false,
					grok: false,
					disney: false,
					tiktok: false,
					extra_platforms: {},
					traffic_bytes: 0,
				})),
		[logEntries],
	);
```

- [ ] **Step 3: Wire SSE state at the page level**

In `frontend/src/routes/index.tsx`:

a) Delete the placeholder `DetailPane` function and import the real one plus the extra pieces:

```tsx
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DetailPane } from "@/components/workbench/detail-pane";
import { isApiError } from "@/lib/client";
import {
	useDeleteSubscription,
	useLatestJobs,
	useSSEProgress,
	useSubscriptions,
	useUpdateSubscription,
} from "@/queries";
import { toast } from "sonner";
import type { subscription } from "@/lib/client.gen";
```

b) Replace the `WorkbenchPage` body with:

```tsx
function WorkbenchPage() {
	const navigate = useNavigate({ from: "/" });
	const { sub: selectedFromUrl } = Route.useSearch();
	const [addOpen, setAddOpen] = useState(false);
	const [editOpen, setEditOpen] = useState(false);
	const [deleteOpen, setDeleteOpen] = useState(false);
	const [activeJobId, setActiveJobId] = useState<string | null>(null);
	const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};

	const selected = subs.find((s) => s.id === selectedFromUrl);
	const selectedId = selected?.id ?? null;

	// Reset per-subscription view state when switching subscriptions.
	// biome-ignore lint/correctness/useExhaustiveDependencies: reset on id change only
	useEffect(() => {
		setActiveJobId(null);
		setSelectedJobId(null);
	}, [selectedId]);

	// If the selected subscription already has a running/queued job (page
	// reload, scheduled run, 409 on trigger), attach to it.
	const latestForSelected = selectedId ? latestJobs[selectedId] : undefined;
	useEffect(() => {
		if (
			latestForSelected &&
			(latestForSelected.status === "running" ||
				latestForSelected.status === "queued")
		) {
			setActiveJobId((cur) => cur ?? latestForSelected.id);
		}
	}, [latestForSelected]);

	const { progress, logEntries, debugData, connection } = useSSEProgress({
		jobId: activeJobId,
		subscriptionId: selectedId ?? "",
		onDone: () => {
			setActiveJobId(null);
			setSelectedJobId(null); // jump to the fresh latest result
		},
	});

	const liveProgressPct =
		activeJobId && progress?.total
			? ((progress.progress ?? 0) / progress.total) * 100
			: null;

	const updateMut = useUpdateSubscription();
	const deleteMut = useDeleteSubscription();

	const select = (id: string | null) =>
		navigate({ search: id ? { sub: id } : {} });

	const handleToggleEnabled = () => {
		if (!selected) return;
		updateMut.mutate(
			{
				id: selected.id,
				params: {
					name: selected.name,
					url: selected.url,
					enabled: !selected.enabled,
					cron_expr: selected.cron_expr ?? "",
					clear_cron_expr: false,
				},
			},
			{
				onSuccess: () =>
					toast.success(
						selected.enabled ? "Subscription disabled" : "Subscription enabled",
					),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);
	};

	const handleDelete = () => {
		if (!selected) return;
		deleteMut.mutate(selected.id, {
			onSuccess: () => {
				toast.success("Subscription deleted");
				setDeleteOpen(false);
				select(null);
			},
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

	return (
		<div className="flex h-full min-h-0">
			<div
				className={cn(
					"h-full w-full min-w-0 border-border md:w-[280px] md:shrink-0 md:border-r lg:w-[300px]",
					selectedId ? "hidden md:block" : "block",
				)}
			>
				<SubList
					subs={subs}
					latestJobs={latestJobs}
					loading={subsQuery.isLoading}
					selectedId={selectedId}
					liveProgressPct={liveProgressPct}
					onSelect={(id) => select(id)}
					onAdd={() => setAddOpen(true)}
				/>
			</div>

			<div
				className={cn(
					"h-full min-w-0 flex-1",
					selectedId ? "block" : "hidden md:block",
				)}
			>
				{selected ? (
					<DetailPane
						key={selected.id}
						sub={selected}
						activeJobId={activeJobId}
						progress={progress}
						logEntries={logEntries}
						debugData={debugData}
						connection={connection}
						selectedJobId={selectedJobId}
						onSelectJob={setSelectedJobId}
						onRunStarted={(jobId) => setActiveJobId(jobId)}
						onEdit={() => setEditOpen(true)}
						onToggleEnabled={handleToggleEnabled}
						onDelete={() => setDeleteOpen(true)}
						onBack={() => select(null)}
					/>
				) : subs.length === 0 && !subsQuery.isLoading ? (
					<EmptyState
						icon={Inbox}
						title="No subscriptions yet"
						description="Add your first subscription URL to start checking nodes."
						action={
							<Button variant="success" onClick={() => setAddOpen(true)}>
								Add subscription
							</Button>
						}
					/>
				) : (
					<EmptyState
						icon={Inbox}
						title="Select a subscription"
						description="Pick a subscription on the left to see its nodes and run checks."
					/>
				)}
			</div>

			<SubscriptionDialog open={addOpen} onOpenChange={setAddOpen} />
			<SubscriptionDialog
				open={editOpen}
				onOpenChange={setEditOpen}
				sub={selected ?? null}
			/>
			<ConfirmDialog
				open={deleteOpen}
				onOpenChange={setDeleteOpen}
				title={`Delete “${selected?.name || selected?.url || ""}”?`}
				description="This removes the subscription, all of its nodes and the entire check history. This cannot be undone."
				confirmLabel="Delete"
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
		</div>
	);
}
```

Add `useEffect` to the React import.

- [ ] **Step 4: Full check-flow verification in browser**

With `encore run` + `bun dev`:
1. Select a subscription → results render (latest completed), sorting + filters work.
2. Run Check → progress panel appears, list row shows live percent + mini bar, log collapsible, Cancel works.
3. Let one finish → panel disappears, fresh results load, list summary updates (latest-jobs invalidation).
4. History select switches to an older job's results; "Latest result" returns.
5. Kill `encore run` mid-check → panel shows "Reconnecting…"; restart → recovers or shows done.
6. 375px: list↔detail navigation, results as cards.

- [ ] **Step 5: Commit**

```bash
cd frontend && bun check-types && bun check
git add frontend/src
git commit -m "feat(frontend): live progress panel and full detail pane composition"
```

---

## Task 16: Route redirects + drop the old pages

**Files:**
- Rewrite: `frontend/src/routes/subscriptions/index.tsx`
- Rewrite: `frontend/src/routes/subscriptions/$id.tsx`
- Delete: `frontend/src/components/node-table.tsx` (old version)

- [ ] **Step 1: Redirect /subscriptions → /**

Replace the entire contents of `frontend/src/routes/subscriptions/index.tsx`:

```tsx
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/")({
	beforeLoad: () => {
		throw redirect({ to: "/" });
	},
});
```

- [ ] **Step 2: Redirect /subscriptions/$id → /?sub=$id**

Replace the entire contents of `frontend/src/routes/subscriptions/$id.tsx`:

```tsx
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/$id")({
	beforeLoad: ({ params }) => {
		throw redirect({ to: "/", search: { sub: params.id } });
	},
});
```

- [ ] **Step 3: Delete the superseded NodeTable**

```bash
rm frontend/src/components/node-table.tsx
cd frontend && bun check-types
```

Expected: clean — the only importer was the old `$id.tsx`.

- [ ] **Step 4: Verify redirects in browser, commit**

Visit `/subscriptions` → lands on `/`; `/subscriptions/<real-id>` → lands on `/?sub=<id>` with that subscription open.

```bash
git add -A frontend/src
git commit -m "refactor(frontend): redirect old subscription routes into the workbench"
```

---

## Task 17: Scheduler page — table, dialog, enable switch

**Files:**
- Create: `frontend/src/components/schedule-dialog.tsx`
- Rewrite: `frontend/src/routes/scheduler.tsx`

- [ ] **Step 1: Schedule create/edit dialog**

Create `frontend/src/components/schedule-dialog.tsx`:

```tsx
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { CheckOptionsFields } from "@/components/check-options-fields";
import { CronPicker } from "@/components/cron-picker";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	type CheckFormOptions,
	DEFAULT_CHECK_OPTIONS,
} from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import type { scheduler, subscription } from "@/lib/client.gen";
import { describeCron, nextRun } from "@/lib/cron";
import { useCreateScheduledJob } from "@/queries";

type ScheduledJob = scheduler.ScheduledJob;
type Subscription = subscription.Subscription;

const PRESETS = [
	{ label: "Every 6h", expr: "0 */6 * * *" },
	{ label: "Every 12h", expr: "0 */12 * * *" },
	{ label: "Daily 4:00", expr: "0 4 * * *" },
] as const;

// Create + edit share this dialog. scheduler.Create upserts on
// subscription_id, so "edit" is just Create with the same subscription.
export function ScheduleDialog({
	open,
	onOpenChange,
	subs,
	editing,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	subs: Subscription[];
	editing?: ScheduledJob | null;
}) {
	const [subId, setSubId] = useState("");
	const [cron, setCron] = useState("0 */6 * * *");
	const [opts, setOpts] = useState<CheckFormOptions>(DEFAULT_CHECK_OPTIONS);

	useEffect(() => {
		if (open) {
			setSubId(editing?.subscription_id ?? "");
			setCron(editing?.cron_expr ?? "0 */6 * * *");
			setOpts({
				...DEFAULT_CHECK_OPTIONS,
				speed_test: editing?.speed_test ?? true,
				media_apps: editing?.media_apps ?? [
					...DEFAULT_CHECK_OPTIONS.media_apps,
				],
			});
		}
	}, [open, editing]);

	const createMut = useCreateScheduledJob();
	const next = nextRun(cron);

	function submit() {
		createMut.mutate(
			{
				subscription_id: subId,
				cron_expr: cron,
				options: {
					speed_test: opts.speed_test,
					media_apps: opts.media_apps,
				},
			},
			{
				onSuccess: () => {
					toast.success(editing ? "Schedule updated" : "Schedule created");
					onOpenChange(false);
				},
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to save schedule"),
			},
		);
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				<DialogTitle>{editing ? "Edit schedule" : "New schedule"}</DialogTitle>
				<DialogDescription>
					Run automatic checks on a cron schedule.
				</DialogDescription>

				<div className="mt-4 space-y-4">
					<div className="space-y-1.5">
						<Label className="text-xs">Subscription</Label>
						<Select
							value={subId}
							onValueChange={(v: string) => setSubId(v)}
							disabled={!!editing}
						>
							<SelectTrigger className="w-full">
								<SelectValue placeholder="Choose a subscription…" />
							</SelectTrigger>
							<SelectContent>
								{subs.map((s) => (
									<SelectItem key={s.id} value={s.id}>
										{s.name || s.url}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>

					<div className="space-y-1.5">
						<Label className="text-xs">Schedule</Label>
						<div className="flex flex-wrap gap-1.5">
							{PRESETS.map((p) => (
								<button
									key={p.expr}
									type="button"
									aria-pressed={cron === p.expr}
									onClick={() => setCron(p.expr)}
									className={
										cron === p.expr
											? "rounded-full border border-info-line bg-info-muted px-3 py-1 font-medium text-info text-xs"
											: "rounded-full border border-border px-3 py-1 text-muted-foreground text-xs hover:bg-secondary"
									}
								>
									{p.label}
								</button>
							))}
						</div>
						<CronPicker value={cron} onChange={setCron} />
						<p className="text-muted-foreground text-xs">
							{describeCron(cron)}
							{next ? (
								<>
									{" · next: "}
									<span className="tabular-nums">
										{next.toLocaleString()}
									</span>
								</>
							) : (
								<span className="text-danger"> · invalid expression</span>
							)}
						</p>
					</div>

					<CheckOptionsFields value={opts} onChange={setOpts} />
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={createMut.isPending}
						disabled={!subId || !next}
						onClick={submit}
					>
						{editing ? "Save" : "Create"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
```

> `CronPicker` keeps its existing look for now — it's functional and self-contained. If its `allowDisable` checkbox shows, don't pass `allowDisable` here (a schedule always has a cron).

- [ ] **Step 2: Rewrite the scheduler page**

Replace the entire contents of `frontend/src/routes/scheduler.tsx`:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { CalendarClock, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ScheduleDialog } from "@/components/schedule-dialog";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { scheduler } from "@/lib/client.gen";
import { describeCron, formatUntil, nextRun } from "@/lib/cron";
import {
	useDeleteScheduledJob,
	useLatestJobs,
	useScheduledJobs,
	useSetScheduleEnabled,
	useSubscriptions,
} from "@/queries";

type ScheduledJob = scheduler.ScheduledJob;

export const Route = createFileRoute("/scheduler")({
	component: SchedulerPage,
});

function SchedulerPage() {
	const [dialogOpen, setDialogOpen] = useState(false);
	const [editing, setEditing] = useState<ScheduledJob | null>(null);
	const [deleting, setDeleting] = useState<ScheduledJob | null>(null);

	const jobsQuery = useScheduledJobs();
	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();

	const jobs = jobsQuery.data?.jobs ?? [];
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};
	const subName = (id: string) => {
		const s = subs.find((x) => x.id === id);
		return s ? s.name || s.url : id.slice(0, 8);
	};

	const toggleMut = useSetScheduleEnabled();
	const deleteMut = useDeleteScheduledJob();

	const handleToggle = (job: ScheduledJob, enabled: boolean) =>
		toggleMut.mutate(
			{ id: job.id, enabled },
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);

	const handleDelete = () => {
		if (!deleting) return;
		deleteMut.mutate(deleting.id, {
			onSuccess: () => {
				toast.success("Schedule deleted");
				setDeleting(null);
			},
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

	return (
		<div className="h-full overflow-y-auto">
			<div className="mx-auto max-w-4xl space-y-5 p-4 pb-8 md:p-6">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="font-semibold text-foreground text-lg">
							Scheduler
						</h1>
						<p className="mt-0.5 text-muted-foreground text-sm">
							Automatic checks on a cron schedule
						</p>
					</div>
					<Button
						variant="success"
						onClick={() => {
							setEditing(null);
							setDialogOpen(true);
						}}
					>
						<Plus size={14} /> New schedule
					</Button>
				</div>

				{jobsQuery.isLoading ? (
					<div className="space-y-2">
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
					</div>
				) : jobs.length === 0 ? (
					<div className="rounded-lg border border-border">
						<EmptyState
							icon={CalendarClock}
							title="No schedules yet"
							description="Create one to check a subscription automatically — every few hours or at a fixed time."
							action={
								<Button variant="success" onClick={() => setDialogOpen(true)}>
									New schedule
								</Button>
							}
						/>
					</div>
				) : (
					<div className="overflow-x-auto rounded-lg border border-border">
						<table className="w-full border-collapse text-[12.5px]">
							<thead>
								<tr className="border-border border-b bg-card text-left text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									<th className="px-3 py-2 font-medium">Subscription</th>
									<th className="px-3 py-2 font-medium">Schedule</th>
									<th className="px-3 py-2 font-medium">Next run</th>
									<th className="px-3 py-2 font-medium">Last check</th>
									<th className="px-3 py-2 text-right font-medium">Enabled</th>
									<th className="w-10 px-2 py-2" aria-label="Actions" />
								</tr>
							</thead>
							<tbody className="tabular-nums">
								{jobs.map((job) => {
									const next = job.enabled ? nextRun(job.cron_expr) : null;
									const latest = latestJobs[job.subscription_id];
									return (
										<tr
											key={job.id}
											className="border-secondary border-b last:border-0"
										>
											<td className="px-3 py-2.5 font-medium text-foreground">
												<button
													type="button"
													onClick={() => {
														setEditing(job);
														setDialogOpen(true);
													}}
													className="hover:underline"
												>
													{subName(job.subscription_id)}
												</button>
											</td>
											<td className="px-3 py-2.5">
												{describeCron(job.cron_expr)}{" "}
												<span className="font-mono text-[11px] text-muted-foreground">
													{job.cron_expr}
												</span>
											</td>
											<td className="px-3 py-2.5 text-info">
												{next ? formatUntil(next) : "—"}
											</td>
											<td className="px-3 py-2.5 text-muted-foreground">
												{latest ? (
													<>
														<span
															className={
																latest.status === "failed"
																	? "text-danger"
																	: latest.status === "completed"
																		? "text-success"
																		: "text-info"
															}
														>
															{latest.status === "completed"
																? "✓"
																: latest.status === "failed"
																	? "✕"
																	: "⟳"}
														</span>{" "}
														{new Date(
															latest.finished_at ?? latest.created_at,
														).toLocaleString(undefined, {
															month: "short",
															day: "numeric",
															hour: "2-digit",
															minute: "2-digit",
														})}
														{latest.status === "completed"
															? ` · ${latest.available}/${latest.total}`
															: ""}
													</>
												) : (
													"—"
												)}
											</td>
											<td className="px-3 py-2.5 text-right">
												<Switch
													checked={job.enabled}
													onCheckedChange={(v) => handleToggle(job, v === true)}
													disabled={toggleMut.isPending}
												/>
											</td>
											<td className="px-2 py-2.5 text-right">
												<button
													type="button"
													aria-label="Delete schedule"
													onClick={() => setDeleting(job)}
													className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-danger-muted hover:text-danger"
												>
													<Trash2 size={13} />
												</button>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>
				)}

				<ScheduleDialog
					open={dialogOpen}
					onOpenChange={setDialogOpen}
					subs={subs}
					editing={editing}
				/>
				<ConfirmDialog
					open={!!deleting}
					onOpenChange={(o) => !o && setDeleting(null)}
					title={`Delete schedule for “${deleting ? subName(deleting.subscription_id) : ""}”?`}
					description="Automatic checks for this subscription will stop. The subscription itself is not affected."
					pending={deleteMut.isPending}
					onConfirm={handleDelete}
				/>
			</div>
		</div>
	);
}
```

> `Switch`'s Base UI change handler may be `onCheckedChange(checked: boolean)`; if typings differ, adapt at the call site only.

- [ ] **Step 3: Verify and commit**

Browser: create a schedule (preset + custom cron), table shows description + next run countdown + last check; toggle pauses (DB `enabled=false`, no cron fire); edit by clicking the name (subscription select disabled, cron/options editable); delete confirms.

```bash
cd frontend && bun check-types && bun check
git add frontend/src
git commit -m "feat(frontend): scheduler table with next-run, last-check, enable switch and dialogs"
```

---

## Task 18: Settings — tab layout + General tab

**Files:**
- Create: `frontend/src/routes/settings.tsx`
- Create: `frontend/src/routes/settings/index.tsx`
- Rewrite: `frontend/src/routes/settings/general.tsx`

- [ ] **Step 1: Settings layout with tabs**

Create `frontend/src/routes/settings.tsx`:

```tsx
import { createFileRoute, Link, Outlet } from "@tanstack/react-router";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/settings")({
	component: SettingsLayout,
});

const TABS = [
	{ to: "/settings/general", label: "General" },
	{ to: "/settings/notify", label: "Notifications" },
	{ to: "/settings/platforms", label: "Platform Rules" },
	{ to: "/settings/export", label: "Export API" },
] as const;

function SettingsLayout() {
	return (
		<div className="h-full overflow-y-auto">
			<div className="mx-auto max-w-3xl space-y-5 p-4 pb-8 md:p-6">
				<h1 className="font-semibold text-foreground text-lg">Settings</h1>
				<nav className="flex gap-1 overflow-x-auto border-border border-b">
					{TABS.map((tab) => (
						<Link
							key={tab.to}
							to={tab.to}
							activeProps={{
								className: "border-primary font-medium text-foreground",
							}}
							inactiveProps={{
								className:
									"border-transparent text-muted-foreground hover:text-foreground",
							}}
							className={cn(
								"-mb-px shrink-0 border-b-2 px-3.5 py-2 text-sm transition-colors",
							)}
						>
							{tab.label}
						</Link>
					))}
				</nav>
				<Outlet />
			</div>
		</div>
	);
}
```

- [ ] **Step 2: Redirect /settings → /settings/general**

Create `frontend/src/routes/settings/index.tsx`:

```tsx
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/settings/")({
	beforeLoad: () => {
		throw redirect({ to: "/settings/general" });
	},
});
```

- [ ] **Step 3: Rewrite the General tab**

Replace the entire contents of `frontend/src/routes/settings/general.tsx`:

```tsx
import { zodResolver } from "@hookform/resolvers/zod";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { isApiError } from "@/lib/client";
import { useSettings, useUpdateSettings } from "@/queries";

export const Route = createFileRoute("/settings/general")({
	component: GeneralSettingsPage,
});

const httpUrl = z
	.string()
	.url("Must be a valid URL")
	.refine((u) => u.startsWith("http"), "Must be http(s)");

const formSchema = z.object({
	latency_test_url: httpUrl,
	speed_test_url: httpUrl,
	upload_test_url: z.union([httpUrl, z.literal("")]),
	smtp_host: z.string(),
	smtp_port: z.coerce.number().int().min(0).max(65535),
	smtp_user: z.string(),
	smtp_pass: z.string(),
	from: z.string(),
});

type FormValues = z.infer<typeof formSchema>;

function Section({
	title,
	description,
	children,
}: {
	title: string;
	description: string;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-lg border border-border bg-card p-4 md:p-5">
			<h2 className="font-semibold text-foreground text-sm">{title}</h2>
			<p className="mt-0.5 mb-4 text-muted-foreground text-xs">{description}</p>
			<div className="space-y-3">{children}</div>
		</section>
	);
}

function Field({
	label,
	error,
	children,
}: {
	label: string;
	error?: string;
	children: React.ReactNode;
}) {
	return (
		<div className="space-y-1.5">
			<Label className="text-xs">{label}</Label>
			{children}
			{error ? <p className="text-danger text-xs">⚠ {error}</p> : null}
		</div>
	);
}

function GeneralSettingsPage() {
	const settingsQuery = useSettings();
	const updateMut = useUpdateSettings();

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			latency_test_url: "",
			speed_test_url: "",
			upload_test_url: "",
			smtp_host: "",
			smtp_port: 587,
			smtp_user: "",
			smtp_pass: "",
			from: "",
		},
	});

	const loaded = settingsQuery.data;
	useEffect(() => {
		if (loaded) {
			form.reset({
				latency_test_url: loaded.latency_test_url,
				speed_test_url: loaded.speed_test_url,
				upload_test_url: loaded.upload_test_url,
				smtp_host: loaded.email_config.smtp_host,
				smtp_port: loaded.email_config.smtp_port,
				smtp_user: loaded.email_config.smtp_user,
				smtp_pass: loaded.email_config.smtp_pass,
				from: loaded.email_config.from,
			});
		}
	}, [loaded, form]);

	const onSubmit = (values: FormValues) => {
		updateMut.mutate(
			{
				latency_test_url: values.latency_test_url,
				speed_test_url: values.speed_test_url,
				upload_test_url: values.upload_test_url,
				email_config: {
					smtp_host: values.smtp_host,
					smtp_port: values.smtp_port,
					smtp_user: values.smtp_user,
					smtp_pass: values.smtp_pass,
					from: values.from,
					// Legacy global recipient — superseded by per-channel
					// to_email; pass through untouched.
					to: loaded?.email_config.to ?? "",
				},
			},
			{
				onSuccess: () => toast.success("Settings saved"),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to save"),
			},
		);
	};

	if (settingsQuery.isLoading) {
		return (
			<div className="space-y-3">
				<Skeleton className="h-40 w-full" />
				<Skeleton className="h-56 w-full" />
			</div>
		);
	}

	const errors = form.formState.errors;

	return (
		<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
			<Section
				title="Connectivity tests"
				description="Endpoints used to measure node latency and bandwidth during checks."
			>
				<Field label="Latency test URL" error={errors.latency_test_url?.message}>
					<Input
						{...form.register("latency_test_url")}
						className="font-mono"
						placeholder="https://www.gstatic.com/generate_204"
					/>
				</Field>
				<div className="grid gap-3 sm:grid-cols-2">
					<Field
						label="Download test URL"
						error={errors.speed_test_url?.message}
					>
						<Input {...form.register("speed_test_url")} className="font-mono" />
					</Field>
					<Field
						label="Upload test URL (optional)"
						error={errors.upload_test_url?.message}
					>
						<Input
							{...form.register("upload_test_url")}
							className="font-mono"
						/>
					</Field>
				</div>
			</Section>

			<Section
				title="Email (SMTP)"
				description="Used by email notification channels. Recipients are configured per channel in the Notifications tab."
			>
				<div className="grid gap-3 sm:grid-cols-2">
					<Field label="SMTP host" error={errors.smtp_host?.message}>
						<Input {...form.register("smtp_host")} placeholder="smtp.example.com" />
					</Field>
					<Field label="SMTP port" error={errors.smtp_port?.message}>
						<Input type="number" {...form.register("smtp_port")} />
					</Field>
					<Field label="Username" error={errors.smtp_user?.message}>
						<Input {...form.register("smtp_user")} autoComplete="off" />
					</Field>
					<Field label="Password" error={errors.smtp_pass?.message}>
						<Input
							type="password"
							{...form.register("smtp_pass")}
							autoComplete="new-password"
						/>
					</Field>
				</div>
				<Field label="From address" error={errors.from?.message}>
					<Input {...form.register("from")} placeholder="subs-check <noreply@example.com>" />
				</Field>
			</Section>

			<div className="flex justify-end">
				<Button type="submit" variant="success" loading={updateMut.isPending}>
					Save settings
				</Button>
			</div>
		</form>
	);
}
```

> Field names come from the generated `settings.UserSettings` / `EmailConfig` types; if `zodResolver` argument types complain under zod v4, use `standardSchemaResolver` from the same package — the repo already depends on `@hookform/resolvers` ^5.

- [ ] **Step 4: Verify and commit**

Browser: /settings redirects to General; tabs navigate; form loads existing values, validates bad URLs inline, saves with toast.

```bash
cd frontend && bun check-types && bun check
git add frontend/src/routes
git commit -m "feat(frontend): settings tab layout and rebuilt general tab with validation"
```

---

## Task 19: Settings — Export API tab

**Files:**
- Create: `frontend/src/routes/settings/export.tsx`

- [ ] **Step 1: Create the Export API tab**

Create `frontend/src/routes/settings/export.tsx`:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { isApiError } from "@/lib/client";
import { useAPIKey, useRegenerateAPIKey, useSubscriptions } from "@/queries";

export const Route = createFileRoute("/settings/export")({
	component: ExportSettingsPage,
});

const FORMATS = ["clash", "base64", "routeros"] as const;

function ExportSettingsPage() {
	const apiKeyQuery = useAPIKey();
	const subsQuery = useSubscriptions();
	const regenMut = useRegenerateAPIKey();
	const [confirmOpen, setConfirmOpen] = useState(false);
	const [subId, setSubId] = useState<string>("all");
	const [format, setFormat] = useState<(typeof FORMATS)[number]>("clash");

	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const subs = subsQuery.data?.subscriptions ?? [];
	const origin = typeof window !== "undefined" ? window.location.origin : "";
	const url =
		subId === "all"
			? `${origin}/api/export/all?token=${apiKey}&target=${format}`
			: `${origin}/api/export/${subId}?token=${apiKey}&target=${format}`;

	const handleRegenerate = () =>
		regenMut.mutate(undefined, {
			onSuccess: () => {
				toast.success("API key regenerated — old links stop working");
				setConfirmOpen(false);
			},
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Failed to regenerate"),
		});

	return (
		<div className="space-y-4">
			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="font-semibold text-foreground text-sm">API Key</h2>
				<p className="mt-0.5 mb-3 text-muted-foreground text-xs">
					Authenticates export URLs. Regenerating invalidates all existing
					links.
				</p>
				{apiKeyQuery.isLoading ? (
					<Skeleton className="h-8 w-full" />
				) : (
					<div className="flex items-center gap-2">
						<code className="min-w-0 flex-1 truncate rounded-md border border-border bg-background px-3 py-1.5 font-mono text-foreground text-xs">
							{apiKey || "—"}
						</code>
						<CopyButton text={apiKey} />
						<Button
							variant="outline"
							size="sm"
							className="text-danger"
							onClick={() => setConfirmOpen(true)}
						>
							<RefreshCw size={12} /> Regenerate
						</Button>
					</div>
				)}
			</section>

			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="font-semibold text-foreground text-sm">
					Subscription URLs
				</h2>
				<p className="mt-0.5 mb-3 text-muted-foreground text-xs">
					Use these as subscription links in your proxy client.
				</p>
				<div className="flex flex-wrap items-center gap-2">
					<Select value={subId} onValueChange={(v: string) => setSubId(v)}>
						<SelectTrigger className="w-44">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All subscriptions</SelectItem>
							{subs.map((s) => (
								<SelectItem key={s.id} value={s.id}>
									{s.name || s.url}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<Select
						value={format}
						onValueChange={(v: string) =>
							setFormat(v as (typeof FORMATS)[number])
						}
					>
						<SelectTrigger className="w-28">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{FORMATS.map((f) => (
								<SelectItem key={f} value={f}>
									{f}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<div className="mt-3 flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
					<code className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted-foreground">
						{url}
					</code>
					<CopyButton text={url} />
				</div>
			</section>

			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="mb-2 font-semibold text-foreground text-sm">
					Parameters
				</h2>
				<table className="w-full text-muted-foreground text-xs">
					<tbody>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">token</td>
							<td>Your API key (required)</td>
						</tr>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">target</td>
							<td>
								<code>clash</code> (default) · <code>base64</code> ·{" "}
								<code>routeros</code>
							</td>
						</tr>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">list</td>
							<td>
								RouterOS address-list name (default <code>clash_servers</code>)
							</td>
						</tr>
					</tbody>
				</table>
			</section>

			<ConfirmDialog
				open={confirmOpen}
				onOpenChange={setConfirmOpen}
				title="Regenerate API key?"
				description="Every existing export link stops working immediately. Proxy clients using the old key must be updated."
				confirmLabel="Regenerate"
				pending={regenMut.isPending}
				onConfirm={handleRegenerate}
			/>
		</div>
	);
}
```

- [ ] **Step 2: Verify and commit**

Browser: tab shows masked-ish key + copy; regenerate confirms and rotates; URL builder switches subscription/format and copies.

```bash
cd frontend && bun check-types && bun check
git add frontend/src/routes/settings/export.tsx
git commit -m "feat(frontend): export API settings tab with url builder"
```

---

## Task 20: Settings — Notifications tab

**Files:**
- Create: `frontend/src/components/notify-channel-dialog.tsx`
- Rewrite: `frontend/src/routes/settings/notify.tsx`

- [ ] **Step 1: Channel dialog**

Create `frontend/src/components/notify-channel-dialog.tsx`:

```tsx
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { CronPicker } from "@/components/cron-picker";
import type { PlatformKey } from "@/components/platform-icons";
import { PlatformIcon } from "@/components/platform-icons";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { MEDIA_APPS } from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import type { notify } from "@/lib/client.gen";
import { cn } from "@/lib/utils";
import { useCreateNotifyChannel, useUpdateNotifyChannel } from "@/queries";

type NotifyChannel = notify.NotifyChannel;

const TYPES = [
	{ value: "webhook", label: "Webhook" },
	{ value: "telegram", label: "Telegram" },
	{ value: "email", label: "Email" },
] as const;

// Config JSON keys must match services/notify/senders.go:
// webhook {url, method, headers} · telegram {bot_token, chat_id} · email {to_email}
interface ConfigState {
	url: string;
	bot_token: string;
	chat_id: string;
	to_email: string;
}

const EMPTY_CONFIG: ConfigState = {
	url: "",
	bot_token: "",
	chat_id: "",
	to_email: "",
};

function configFromChannel(ch: NotifyChannel | null | undefined): ConfigState {
	if (!ch) return { ...EMPTY_CONFIG };
	const cfg = (ch.config ?? {}) as Record<string, unknown>;
	return {
		url: typeof cfg.url === "string" ? cfg.url : "",
		bot_token: typeof cfg.bot_token === "string" ? cfg.bot_token : "",
		chat_id: typeof cfg.chat_id === "string" ? cfg.chat_id : "",
		to_email: typeof cfg.to_email === "string" ? cfg.to_email : "",
	};
}

function buildConfig(type: string, c: ConfigState): Record<string, unknown> {
	switch (type) {
		case "webhook":
			return { url: c.url, method: "POST", headers: {} };
		case "telegram":
			return { bot_token: c.bot_token, chat_id: c.chat_id };
		case "email":
			return { to_email: c.to_email };
		default:
			return {};
	}
}

export function NotifyChannelDialog({
	open,
	onOpenChange,
	editing,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	editing?: NotifyChannel | null;
}) {
	const [name, setName] = useState("");
	const [type, setType] = useState<string>("webhook");
	const [config, setConfig] = useState<ConfigState>(EMPTY_CONFIG);
	const [onCheckComplete, setOnCheckComplete] = useState(true);
	const [unlockCron, setUnlockCron] = useState("");
	const [platformAlerts, setPlatformAlerts] = useState<string[]>([]);

	useEffect(() => {
		if (open) {
			setName(editing?.name ?? "");
			setType(editing?.type ?? "webhook");
			setConfig(configFromChannel(editing));
			setOnCheckComplete(editing?.on_check_complete ?? true);
			setUnlockCron(editing?.unlock_cron ?? "");
			setPlatformAlerts(editing?.platform_alerts ?? []);
		}
	}, [open, editing]);

	const createMut = useCreateNotifyChannel();
	const updateMut = useUpdateNotifyChannel();
	const pending = createMut.isPending || updateMut.isPending;

	const configValid =
		(type === "webhook" && config.url.startsWith("http")) ||
		(type === "telegram" && !!config.bot_token && !!config.chat_id) ||
		(type === "email" && config.to_email.includes("@"));

	function submit() {
		const common = {
			name,
			config: buildConfig(type, config) as notify.JSONValue,
			on_check_complete: onCheckComplete,
			unlock_cron: unlockCron,
			platform_alerts: platformAlerts,
		};
		const onError = (e: unknown) =>
			toast.error(isApiError(e) ? e.message : "Failed to save channel");
		if (editing) {
			updateMut.mutate(
				{
					id: editing.id,
					params: { ...common, enabled: editing.enabled },
				},
				{
					onSuccess: () => {
						toast.success("Channel updated");
						onOpenChange(false);
					},
					onError,
				},
			);
		} else {
			createMut.mutate(
				{ ...common, type },
				{
					onSuccess: () => {
						toast.success("Channel created");
						onOpenChange(false);
					},
					onError,
				},
			);
		}
	}

	const togglePlatform = (app: string) =>
		setPlatformAlerts((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				<DialogTitle>{editing ? "Edit channel" : "Add channel"}</DialogTitle>
				<DialogDescription>
					Where and when to send check reports.
				</DialogDescription>

				<div className="mt-4 space-y-4">
					<div className="grid gap-3 sm:grid-cols-2">
						<div className="space-y-1.5">
							<Label className="text-xs">Name</Label>
							<Input
								value={name}
								placeholder="My alerts"
								onChange={(e) => setName(e.target.value)}
							/>
						</div>
						<div className="space-y-1.5">
							<Label className="text-xs">Type</Label>
							<Select
								value={type}
								onValueChange={(v: string) => setType(v)}
								disabled={!!editing}
							>
								<SelectTrigger className="w-full">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{TYPES.map((t) => (
										<SelectItem key={t.value} value={t.value}>
											{t.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
					</div>

					{type === "webhook" ? (
						<div className="space-y-1.5">
							<Label className="text-xs">Webhook URL</Label>
							<Input
								value={config.url}
								className="font-mono"
								placeholder="https://…"
								onChange={(e) =>
									setConfig({ ...config, url: e.target.value })
								}
							/>
						</div>
					) : null}
					{type === "telegram" ? (
						<div className="grid gap-3 sm:grid-cols-2">
							<div className="space-y-1.5">
								<Label className="text-xs">Bot token</Label>
								<Input
									value={config.bot_token}
									className="font-mono"
									onChange={(e) =>
										setConfig({ ...config, bot_token: e.target.value })
									}
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-xs">Chat ID</Label>
								<Input
									value={config.chat_id}
									className="font-mono"
									onChange={(e) =>
										setConfig({ ...config, chat_id: e.target.value })
									}
								/>
							</div>
						</div>
					) : null}
					{type === "email" ? (
						<div className="space-y-1.5">
							<Label className="text-xs">Recipients</Label>
							<Input
								value={config.to_email}
								placeholder="a@example.com, b@example.com"
								onChange={(e) =>
									setConfig({ ...config, to_email: e.target.value })
								}
							/>
							<p className="text-muted-foreground text-xs">
								SMTP server is configured in Settings → General.
							</p>
						</div>
					) : null}

					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={onCheckComplete}
							onCheckedChange={(v) => setOnCheckComplete(v === true)}
						/>
						Notify when a check completes
					</label>

					<div className="space-y-1.5">
						<Label className="text-xs">
							Platform alerts{" "}
							<span className="text-muted-foreground">
								(alert when a platform loses all unlocked nodes)
							</span>
						</Label>
						<div className="flex flex-wrap gap-1.5">
							{MEDIA_APPS.map((app) => {
								const active = platformAlerts.includes(app);
								return (
									<button
										key={app}
										type="button"
										aria-pressed={active}
										onClick={() => togglePlatform(app)}
										className={cn(
											"inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors",
											active
												? "border-info-line bg-info-muted text-info"
												: "border-border text-muted-foreground hover:bg-secondary",
										)}
									>
										<PlatformIcon
											platform={app as PlatformKey}
											size={12}
											showLabel
										/>
									</button>
								);
							})}
						</div>
					</div>

					<div className="space-y-1.5">
						<Label className="text-xs">
							Scheduled unlock report{" "}
							<span className="text-muted-foreground">(optional)</span>
						</Label>
						<CronPicker value={unlockCron} onChange={setUnlockCron} allowDisable />
					</div>
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={pending}
						disabled={!name || !configValid}
						onClick={submit}
					>
						{editing ? "Save" : "Create"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
```

> Open the generated `notify` namespace in `client.gen.ts` while implementing: if `NotifyChannel`'s entity fields differ from `UpdateChannelParams` (e.g. config exposed differently), adapt `configFromChannel` only. `JSONValue` cast matches the generated param type.

- [ ] **Step 2: Rewrite the Notifications tab**

Replace the entire contents of `frontend/src/routes/settings/notify.tsx`:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { BellOff, Plus, Send } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { NotifyChannelDialog } from "@/components/notify-channel-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { notify } from "@/lib/client.gen";
import {
	useDeleteNotifyChannel,
	useNotifyChannels,
	useTestNotifyChannel,
	useUpdateNotifyChannel,
} from "@/queries";

type NotifyChannel = notify.NotifyChannel;

export const Route = createFileRoute("/settings/notify")({
	component: NotifySettingsPage,
});

function NotifySettingsPage() {
	const [dialogOpen, setDialogOpen] = useState(false);
	const [editing, setEditing] = useState<NotifyChannel | null>(null);
	const [deleting, setDeleting] = useState<NotifyChannel | null>(null);

	const channelsQuery = useNotifyChannels();
	const channels = channelsQuery.data?.channels ?? [];

	const updateMut = useUpdateNotifyChannel();
	const deleteMut = useDeleteNotifyChannel();
	const testMut = useTestNotifyChannel();

	const handleToggle = (ch: NotifyChannel, enabled: boolean) =>
		updateMut.mutate(
			{
				id: ch.id,
				params: {
					name: ch.name,
					config: ch.config,
					enabled,
					on_check_complete: ch.on_check_complete,
					unlock_cron: ch.unlock_cron,
					platform_alerts: ch.platform_alerts,
				},
			},
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);

	const handleTest = (ch: NotifyChannel) =>
		testMut.mutate(
			{ id: ch.id, params: { report_type: "check" } },
			{
				onSuccess: () => toast.success(`Test sent to “${ch.name}”`),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Test failed"),
			},
		);

	const handleDelete = () => {
		if (!deleting) return;
		deleteMut.mutate(deleting.id, {
			onSuccess: () => {
				toast.success("Channel deleted");
				setDeleting(null);
			},
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<p className="text-muted-foreground text-xs">
					Channels receive check reports, platform alerts and scheduled unlock
					summaries.
				</p>
				<Button
					variant="success"
					size="sm"
					onClick={() => {
						setEditing(null);
						setDialogOpen(true);
					}}
				>
					<Plus size={13} /> Add channel
				</Button>
			</div>

			{channelsQuery.isLoading ? (
				<div className="space-y-2">
					<Skeleton className="h-14 w-full" />
					<Skeleton className="h-14 w-full" />
				</div>
			) : channels.length === 0 ? (
				<div className="rounded-lg border border-border">
					<EmptyState
						icon={BellOff}
						title="No channels yet"
						description="Add a webhook, Telegram bot or email recipient to get notified after checks."
						action={
							<Button variant="success" onClick={() => setDialogOpen(true)}>
								Add channel
							</Button>
						}
					/>
				</div>
			) : (
				<div className="space-y-2">
					{channels.map((ch) => (
						<div
							key={ch.id}
							className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-card px-4 py-3"
						>
							<Switch
								checked={ch.enabled}
								onCheckedChange={(v) => handleToggle(ch, v === true)}
								disabled={updateMut.isPending}
							/>
							<button
								type="button"
								onClick={() => {
									setEditing(ch);
									setDialogOpen(true);
								}}
								className="min-w-0 flex-1 text-left"
							>
								<span className="block truncate font-medium text-foreground text-sm hover:underline">
									{ch.name}
								</span>
								<span className="text-muted-foreground text-xs">
									{ch.on_check_complete ? "check reports" : ""}
									{ch.platform_alerts?.length
										? ` · ${ch.platform_alerts.length} platform alerts`
										: ""}
									{ch.unlock_cron ? " · scheduled report" : ""}
								</span>
							</button>
							<Badge tone="info">{ch.type}</Badge>
							<Button
								variant="outline"
								size="sm"
								loading={testMut.isPending && testMut.variables?.id === ch.id}
								onClick={() => handleTest(ch)}
							>
								<Send size={12} /> Test
							</Button>
							<Button
								variant="ghost"
								size="sm"
								className="text-danger"
								onClick={() => setDeleting(ch)}
							>
								Delete
							</Button>
						</div>
					))}
				</div>
			)}

			<NotifyChannelDialog
				open={dialogOpen}
				onOpenChange={setDialogOpen}
				editing={editing}
			/>
			<ConfirmDialog
				open={!!deleting}
				onOpenChange={(o) => !o && setDeleting(null)}
				title={`Delete channel “${deleting?.name ?? ""}”?`}
				description="This channel stops receiving all notifications. This cannot be undone."
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
		</div>
	);
}
```

- [ ] **Step 3: Verify and commit**

Browser: list renders existing channels with type badge + toggles; add each type (webhook/telegram/email) with validation; Test fires with toast result; delete confirms.

```bash
cd frontend && bun check-types && bun check
git add frontend/src
git commit -m "feat(frontend): notifications tab with channel dialog, test and enable toggle"
```

---

## Task 21: Platform Rules tab — restyle

**Files:**
- Modify: `frontend/src/routes/settings/platforms.tsx`
- Modify: `frontend/src/components/platforms/RuleCard.tsx`

Scope: chrome only (header button, toggle, edit/delete buttons, delete confirm). The editor dialog internals (`RuleEditorDialog`, Monaco, console/docs panels) keep their current styling — functional machinery, separately styled, out of scope per spec.

- [ ] **Step 1: Restyle the page header**

In `frontend/src/routes/settings/platforms.tsx`:

a) The page now renders inside the Settings layout — remove the `max-w-2xl` wrapper class (the layout owns width) and the `<h1>` heading line, replacing the header block:

```tsx
			<div className="flex items-center justify-between">
				<p className="text-muted-foreground text-xs">
					Rules run during each proxy check. Built-in rules are seeded on
					first visit. Custom keys store results in{" "}
					<code className="rounded bg-secondary px-1 font-mono">
						extra_platforms
					</code>
					.
				</p>
				<Button
					variant="success"
					size="sm"
					onClick={() => setAddOpen(true)}
					className="shrink-0"
				>
					<Plus size={13} /> Add Rule
				</Button>
			</div>
```

b) Replace the loading spinner block with two `Skeleton` rows, and the `No rules yet.` paragraph with:

```tsx
					<div className="rounded-lg border border-border">
						<EmptyState
							icon={Tv2}
							title="No rules yet"
							description="Add a detection rule to test custom platforms during checks."
						/>
					</div>
```

c) Imports: add `Button`, `EmptyState`, `Skeleton`, `Tv2` (lucide); drop `Loader2`; root div className becomes `"space-y-5"`.

- [ ] **Step 2: Restyle RuleCard**

In `frontend/src/components/platforms/RuleCard.tsx`:

a) Replace the hand-rolled toggle `<button>` (the one with `bg-green-500`) with:

```tsx
			<Switch
				checked={rule.enabled}
				onCheckedChange={(v) => handleToggle(v === true)}
				disabled={toggleMut.isPending}
			/>
```

b) Replace the Edit/Delete raw buttons block with:

```tsx
			<div className="flex items-center gap-1">
				<Button variant="ghost" size="sm" onClick={onEdit}>
					Edit
				</Button>
				<Button
					variant="ghost"
					size="icon-sm"
					aria-label="Delete rule"
					className="text-muted-foreground hover:text-danger"
					onClick={() => setConfirmOpen(true)}
				>
					<Trash2 size={13} />
				</Button>
			</div>

			<ConfirmDialog
				open={confirmOpen}
				onOpenChange={setConfirmOpen}
				title={`Delete rule “${rule.name}”?`}
				description="Nodes stop being tested against this platform on future checks."
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
```

c) Wire the confirm: add `const [confirmOpen, setConfirmOpen] = useState(false);`, change `handleDelete`'s `onSuccess` to also `setConfirmOpen(false)`, and add imports:

```tsx
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Switch } from "@/components/ui/switch";
```

(Drop the now-unused `Loader2` import.)

- [ ] **Step 3: Verify and commit**

Browser: Platform Rules tab inside Settings; toggle uses Switch; delete confirms; rule editor still opens and saves.

```bash
cd frontend && bun check-types && bun check
git add frontend/src
git commit -m "refactor(frontend): restyle platform rules chrome with unified components"
```

---

## Task 22: Login page — re-theme

**Files:**
- Rewrite: `frontend/src/routes/login.tsx`

- [ ] **Step 1: Rewrite with design tokens (kills the hardcoded GitHub-dark palette)**

Replace the entire contents of `frontend/src/routes/login.tsx`:

```tsx
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { setToken } from "@/lib/auth";
import { isApiError } from "@/lib/client";
import { useLogin, useRegister } from "@/queries";

export const Route = createFileRoute("/login")({
	component: LoginPage,
});

function LoginPage() {
	const navigate = useNavigate();
	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [remember, setRemember] = useState(false);

	const loginMut = useLogin();
	const registerMut = useRegister();
	const pending = loginMut.isPending || registerMut.isPending;

	function submit(e: React.FormEvent) {
		e.preventDefault();
		if (!username || !password) return;
		const onError = (err: unknown) =>
			toast.error(
				isApiError(err)
					? err.message
					: mode === "login"
						? "Login failed"
						: "Registration failed",
			);
		if (mode === "login") {
			loginMut.mutate(
				{ username, password, remember },
				{
					onSuccess: (resp) => {
						setToken(resp.token, remember);
						navigate({ to: "/" });
					},
					onError,
				},
			);
		} else {
			registerMut.mutate(
				{ username, password },
				{
					onSuccess: () => {
						toast.success("Account created — please sign in");
						setMode("login");
					},
					onError,
				},
			);
		}
	}

	return (
		<div className="w-full max-w-sm px-4">
			<form
				onSubmit={submit}
				className="rounded-xl border border-border bg-card p-6 shadow-[var(--shadow-popover)]"
			>
				<div className="mx-auto mb-3 flex size-10 items-center justify-center rounded-xl bg-primary font-bold text-lg text-primary-foreground">
					S
				</div>
				<h1 className="text-center font-semibold text-[15px] text-foreground">
					subs-check
				</h1>
				<p className="mt-0.5 mb-5 text-center text-muted-foreground text-xs">
					{mode === "login" ? "Sign in to your account" : "Create an account"}
				</p>

				<div className="space-y-3">
					<div className="space-y-1.5">
						<Label htmlFor="username" className="text-xs">
							Username
						</Label>
						<Input
							id="username"
							value={username}
							autoComplete="username"
							onChange={(e) => setUsername(e.target.value)}
						/>
					</div>
					<div className="space-y-1.5">
						<Label htmlFor="password" className="text-xs">
							Password
						</Label>
						<Input
							id="password"
							type="password"
							value={password}
							autoComplete={
								mode === "login" ? "current-password" : "new-password"
							}
							onChange={(e) => setPassword(e.target.value)}
						/>
					</div>
					{mode === "login" ? (
						<label className="flex cursor-pointer items-center gap-2 text-muted-foreground text-xs">
							<Checkbox
								checked={remember}
								onCheckedChange={(v) => setRemember(v === true)}
							/>
							Remember me
						</label>
					) : null}
				</div>

				<Button
					type="submit"
					variant="success"
					className="mt-5 w-full"
					loading={pending}
					disabled={!username || !password}
				>
					{mode === "login" ? "Sign in" : "Create account"}
				</Button>

				<button
					type="button"
					onClick={() => setMode(mode === "login" ? "register" : "login")}
					className="mt-4 w-full text-center text-primary text-xs hover:underline"
				>
					{mode === "login"
						? "Create an account"
						: "Already have an account? Sign in"}
				</button>
			</form>
		</div>
	);
}
```

- [ ] **Step 2: Verify and commit**

Browser (logged out): login card follows light/dark theme; bad credentials toast; register flow switches back to login; remember-me persists across browser restart (localStorage vs sessionStorage).

```bash
cd frontend && bun check-types && bun check
git add frontend/src/routes/login.tsx
git commit -m "fix(frontend): re-theme login page with design tokens"
```

---

## Task 23: Token cleanup + full verification

**Files:**
- Modify: `frontend/src/styles.css`
- Modify: `frontend/src/components/cron-picker.tsx`, `frontend/src/components/debug-panel.tsx`, `frontend/src/components/platforms/*` (only if grep hits below)

- [ ] **Step 1: Inventory remaining legacy-token users**

```bash
cd frontend
grep -rn -- "--color-btn-success\|--color-badge-\|--legacy-success\|--legacy-warning" src --include="*.tsx" --include="*.ts"
```

For every hit, replace with the new system: `var(--color-btn-success)` → `<Button variant="success">` or class `bg-solid-success`; badge vars → `<Badge tone="…">` or `text-success`/`bg-success-muted` classes; `var(--legacy-success)` → `text-success` class (or `var(--success)` where a CSS value is required). Expected hit areas: `cron-picker.tsx`, `debug-panel.tsx`, `platforms/` internals — mechanical swaps, no behavior change.

- [ ] **Step 2: Delete dead tokens**

In `frontend/src/styles.css`, delete from BOTH theme blocks: `--legacy-success`, `--legacy-warning`, `--color-btn-success`, `--color-badge-info-bg`, `--color-badge-info`, `--color-badge-success-bg`, `--color-badge-success`, `--color-badge-danger-bg`, `--color-badge-danger`, `--color-badge-ai-bg`, `--color-badge-ai`. Keep: `--color-dimmed`, `--color-code`, `--color-active-bg`, `--color-active-border`, `--color-progress` (still used by rail/list selection, progress bars, and code text).

Re-run the grep from Step 1 — expected: no output.

- [ ] **Step 3: Raw-button / inline-color audit (spec hard rule)**

```bash
cd frontend
# Raw <button> outside ui primitives and allowed text-button cases:
grep -rn "<button" src/routes src/components --include="*.tsx" | grep -v "src/components/ui/" | grep -v "src/components/platforms/"
# Inline color styles:
grep -rn 'style={{' src/routes src/components --include="*.tsx" | grep -v "src/components/ui/" | grep -v "src/components/platforms/" | grep -iv "width\|--color-active\|--color-progress\|--shadow"
```

Review every hit: pill/toggle buttons built in Tasks 13–14 (aria-pressed pattern) and icon-only buttons with hover classes are fine; anything carrying hardcoded color styles is not — convert to Button/Badge/tokens. `platforms/` internals are excluded by scope (Task 21 note).

- [ ] **Step 4: Full automated gate**

```bash
cd frontend
bun check-types && bun check && bun run test:unit && bun run build
cd ..
encore test ./services/...
```

Expected: all green, production build succeeds.

- [ ] **Step 5: Browser walkthrough (definition of done, spec §Verification)**

With `encore run` + `bun dev`, at **1440 / 768 / 375 px** each:

1. Login (light + dark) → workbench.
2. Add subscription → appears in list; invalid URL rejected inline.
3. Run Check with options → progress panel, live log, list-row percent; Cancel once; re-run with remembered options (popover pre-filled).
4. Completed → chips/filters/sorting (latency asc default, click toggles), platform filter, alive-only, node enable/disable persists.
5. History select → older job; "Latest result" returns.
6. Export popover URLs copy; Settings → Export API builder works; key regenerate confirm.
7. Scheduler: create (preset + custom), next-run countdown, enable toggle off/on, edit, delete.
8. Notifications: add webhook channel, Test, toggle, delete.
9. Platform Rules: toggle, edit dialog opens, delete confirm.
10. /subscriptions and /subscriptions/:id redirect correctly.
11. Mobile: bottom tabs, list↔detail, full-screen dialogs.

- [ ] **Step 6: Final commit**

```bash
git add -A frontend/src
git commit -m "chore(frontend): remove legacy tokens, enforce component-only buttons"
```

---

## Execution order summary

```
Phase 0  TanStack Start migration (existing plan, separate PR)
Task 1   Spec amendment + plan commit
Task 2-3 Backend endpoints (TDD) + client regen        ← needs encore run/test
Task 4   Frontend deps + vitest
Task 5   Design tokens
Task 6   Logic utils (TDD)
Task 7-8 UI primitives
Task 9   Query layer
Task 10  App shell (rail + tabbar)
Task 11-12 Workbench list + dialogs
Task 13-15 Detail pane (header → results → running)
Task 16  Route redirects
Task 17  Scheduler
Task 18-20 Settings (layout/general → export → notifications)
Task 21  Platform rules restyle
Task 22  Login
Task 23  Cleanup + full verification
```




