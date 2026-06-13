# UI Redesign Design

**Date:** 2026-06-12
**Status:** Approved

## Goal

Full restructure of the frontend UI: a check-centric information architecture (three-pane workbench), consistent interaction patterns (dialogs, confirmations, loading/empty states), and a unified visual system (GitHub palette with Linear-grade detail polish). UI language stays English.

## Sequencing Decision

The approved **TanStack Start migration** (`docs/superpowers/specs/2026-06-01-tanstack-start-migration-design.md`) executes **first**, then this redesign lands on the migrated flat structure (`frontend/src/...`). Two separate phases, two separate PRs. All paths in this spec use post-migration layout.

Work branches from `origin/main` (which already contains the mobile-responsive work from PR #1 and detection-stability fixes from PR #2). The local `fix/detection-stability` branch is stale on frontend files and must not be the base.

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| Scope | Full restructure — navigation and page organization may change |
| Primary workflow | Check-centric: trigger checks, watch progress, inspect node results |
| Visual style | GitHub (Primer) palette and restraint as the base, plus Linear-grade detail: tabular numerals, refined dark mode, crisp badges/progress, subtle 150ms transitions |
| Information architecture | Master-detail workbench (option 2 of 3) |
| UI language | English |
| Ordering vs. TanStack Start migration | Migration first, then UI redesign |

## Information Architecture

### Before → After

| Current | After redesign |
|---------|----------------|
| `/` Dashboard (stats + server unlock + export API) | **Deleted.** Stats fold into the workbench list column; server unlock becomes a status strip; export API moves to Settings and the detail header |
| `/subscriptions` list page | Merged into `/` workbench (left column) |
| `/subscriptions/$id` detail page | Merged into `/` workbench (right pane) |
| `/scheduler` | Stays, redesigned as a table |
| `/settings/general`, `/settings/notify`, `/settings/platforms` | One Settings section with four tabs: General / Notifications / Platform Rules / Export API |
| `/login` | Stays, re-themed |

### Routes

- `/` — workbench. Selected subscription is a search param: `/?sub=<id>` (shareable, bookmarkable). An invalid or deleted `?sub` id silently clears the selection (list view, no error page).
- `/scheduler`
- `/settings/general`, `/settings/notify`, `/settings/platforms`, `/settings/export`
- `/login`
- Redirects: `/subscriptions` → `/`, `/subscriptions/$id` → `/?sub=$id`.

## Workbench (the main screen)

Three panes, left to right:

### 1. Icon rail (56px)

- Logo, then nav icons: Workbench (list icon), Scheduler (clock), Settings (gear). Active item gets the `active-bg`/`active-border` treatment.
- Bottom: theme toggle, user avatar with dropdown (username, logout).
- Tooltips on hover (icon-only rail).

### 2. Subscription list column (280px)

- Header: "Subscriptions" + `＋ Add` button (opens dialog).
- One row per subscription:
  - **Status dot**: green = last check completed, blue with subtle pulse = check running (row also shows a 3px mini progress bar), red = last check failed, gray = disabled (row at reduced opacity).
  - Name, relative time of last check, summary line: `86/120 alive · ⚡42ms avg`.
  - Running rows: pulsing dot + "Running…" label. Only the *selected* subscription's row mirrors live percent/mini-progress (data already in memory from the detail pane's SSE) — non-selected rows do not open extra SSE connections; their summary refreshes when the jobs list invalidates.
- Selected row: `active-bg` + `active-border` (same tokens as nav).
- Footer status strip: server network unlock summary — flag/IP + `N unlocks ›`. Click opens a popover with the full per-platform grid and a refresh action (content of the old Dashboard panel).
- Empty state: icon + "No subscriptions yet" + primary Add button.

### 3. Detail pane (flex)

**Header row:**
- Subscription name + truncated URL with copy button.
- History dropdown (`Latest · 2h ago · 86/120`) replacing the old pills row — selecting an entry loads that job's results.
- `Export ▾` popover: format picker (clash / base64 / routeros) + copyable URL for this subscription.
- `⋯` menu: Edit (dialog), Enable/Disable, Delete (confirm dialog).
- **`▶ Run Check ▾`** primary (green) split button — see Interactions.

**Results area (job completed):**
- Summary chips: alive count, dead count, avg latency, peak speed, unlock counts per selected platform.
- Filter bar: text filter, `Alive only` toggle pill, `Unlocks ▾` platform filter, right-aligned `N of M shown`.
- Node table (sortable):
  - Columns: Node (flag + name), Latency (default sort, ascending), Speed, Upload (when tested), Traffic, Unlocks (platform icons with tooltips), per-node enable toggle (`●`/`○`).
  - Numbers in tabular-nums. Latency color-coded via tokens (success/warning/danger). Disabled nodes grayed with an `off` chip.
  - Mobile: rows collapse to cards (keep the merged PR #1 card pattern, restyled).

**Running state:** progress panel pinned above the table:
- Spinner, `Checking nodes…`, `144 / 200` counter, `61 alive so far`, elapsed + ETA, red-text `Cancel` button.
- Collapsible live log (collapsed by default).
- Alive nodes stream into the table below as they are found (inserted per current sort).
- SSE disconnect → panel shows `Reconnecting…` and retries automatically; if the job finished while disconnected, refetch results.

**Empty states:** no subscription selected → centered hint; subscription never checked → "Run your first check" + button.

### Mobile (<768px)

- Icon rail becomes a bottom tab bar (Workbench / Scheduler / Settings).
- No `?sub` param → list column full-screen; with `?sub` → detail full-screen with a back button.
- Dialogs go full-screen (existing Base UI pattern).

## Interactions

1. **All create/edit flows use dialogs** (Base UI Dialog) — no more inline expanding forms. Add/Edit subscription dialog: Name, URL (validated http(s), inline error), Enabled checkbox.
2. **All deletes use a confirm dialog** stating consequences ("This removes the subscription, its 45 nodes and all check history."), red destructive button, loading state while pending.
3. **Run Check split button**: main click starts a check with the last-used options (persisted in localStorage per subscription); `▾` opens options: Speed test, Upload test, Media platforms (pill multi-select). `Start check` inside the panel.
4. **Every mutating button** has loading (spinner) and disabled states.
5. **409 on concurrent check**: toast "A check is already running for this subscription" and switch the view to the running job.

## Secondary Pages

### Scheduler

- Table: Subscription · Schedule (human-readable + raw cron in mono) · Next run (computed client-side from cron, e.g. "in 2h 14m") · Last check (✓/✕ + time + `86/120` or failure reason; this is the subscription's latest job whether manual or scheduled — sourced from `latest_job`) · Enabled (Switch).
- `＋ New schedule` and row edit open a dialog: subscription select, cron presets (Every 6h / Every 12h / Daily 4:00 / Custom cron input with validation + next-3-runs preview), check options (same component as Run Check options).
- Delete via confirm dialog. Empty state with explainer + button.

### Settings (four tabs)

- **General**: latency test URL, speed/upload test URLs, SMTP — grouped cards, save button with loading state, react-hook-form + zod validation.
- **Notifications**: channel list (name, type badge, enabled). Add/Edit via dialog with type-specific fields (webhook URL / Telegram token + chat id / email recipients). `Test` button per channel with success/failure toast. Delete via confirm dialog.
- **Platform Rules**: existing rules feature (RuleCard grid, RuleEditorDialog, console/docs panels) restyled with the unified components; no functional changes.
- **Export API**: API key card (masked display, Copy, Regenerate with confirm), Subscription URLs card with two selects (subscription: All/each; format: clash/base64/routeros) + copyable URL, parameter reference table (token/target/list).

### Login

- Re-themed with design tokens (currently hardcoded GitHub-dark hex values — breaks light mode). Centered card, logo, username/password, Remember me, loading state on submit, mode switch to Register. No functional auth changes.

## Design System

### Tokens (`frontend/src/styles.css`)

- Keep the GitHub light/dark palettes already defined.
- Consolidate ad-hoc tokens (`--color-btn-success`, `--color-badge-*`) into a semantic scale: `success / warning / danger / info`, each with `fg / bg / border`.
- Add: focus ring (2px, `--ring`, offset 2), shadow scale (`--shadow-popover`, `--shadow-dialog`), motion (`--transition-fast: 150ms ease`), `tabular-nums` utility class.
- Monospace for URLs, cron expressions, API keys, code.

### Component library (`frontend/src/components/ui/`)

Extend/add (shadcn conventions):

| Component | Notes |
|-----------|-------|
| Button | Add `success`, `danger`, `ghost` variants + `loading` prop (spinner, auto-disable) |
| Badge | `success / danger / warning / info / neutral` |
| Dialog | Base UI; standard + `ConfirmDialog` wrapper (danger variant); full-screen on mobile |
| Tabs | Settings navigation |
| Switch | Scheduler/subscription enabled toggles |
| Tooltip | Platform icons, icon rail, truncated text |
| Progress | Check progress bar (gradient fill via token) |
| EmptyState | Icon + title + description + optional action |
| StatusDot | Color by status, optional pulse for running |
| SortableTable | Header-click sorting helpers for the node table |

**Hard rule:** page code must not contain raw `<button>` elements or inline `style={{ background: … }}` color styling. Everything goes through components and tokens. (Exception: dynamic values that are genuinely data-driven, e.g. progress width.)

### States

- Initial page loads → Skeleton blocks matching final layout.
- Mutations → button spinner; no full-page blocking.
- Lists → EmptyState with a guiding action.
- Form validation → react-hook-form + zod, inline errors.
- API errors → toast with the server message; 401 → existing redirect-to-login flow.

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

## Out of Scope

- i18n / Chinese UI (decided: English only)
- New check features, node operations beyond existing enable/disable
- Multi-user/admin features
- Charts or historical trend visualizations
- Backend changes beyond the two endpoints above

## Risks & Mitigations

- **Workbench width pressure** (rail 56 + list 280 + table): the node table drops to card layout under 768px; between 768–1024px the list column narrows to 240px and table hides the Traffic column. Columns beyond Node/Latency/Unlocks hide progressively rather than squeeze.
- **Live insert into a sorted table** can cause row jumping while the user reads: throttle inserts to batches (e.g. flush every 1s) and pause insertion sort while the pointer hovers the table (append to a "+N new" buffer chip instead).
- **History dropdown** must handle the running job: while running, the dropdown pins a "Running now…" entry at top.

## Verification (definition of done)

1. `bun check-types`, `bun check` (Biome), production build — all green.
2. Browser walkthrough at 1440 / 768 / 375 px: login → add subscription → run check (watch progress, cancel, re-run) → sort/filter results → scheduler CRUD → all four settings tabs → export URL copy → light/dark toggle.
3. Visual walkthrough against the brainstorm mockups (`.superpowers/brainstorm/97101-1781269467/content/`).
4. No raw `<button>`/inline color styles in page code (grep check).
