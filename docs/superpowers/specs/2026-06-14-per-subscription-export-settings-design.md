# Per-Subscription Export Settings (include-dead + sort order)

**Date:** 2026-06-14
**Status:** Approved
**Branch:** `feat/node-details-export-tags` (continues the export work; stacked on `feat/ui-redesign` / PR #3)

## Goal

Two per-subscription export settings, configured in the Add/Edit Subscription dialog, applied to that subscription's clash/base64 export link:

- **`export_include_dead`** (bool, default `false`): include `alive=false` nodes in the export.
- **`export_sort`** (enum `speed_desc` | `latency_asc`, default `speed_desc`): node order in the export.

Defaults reproduce today's behavior exactly. **RouterOS export is unaffected** (it lists enabled node servers via `latestServerAddresses`, independent of alive/sort).

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| Where to configure | A — the Add/Edit Subscription dialog |
| Interaction with per-node `enabled` | include-dead only relaxes the `alive` filter; manually-disabled (`enabled=false`) nodes stay excluded |
| Sort options | only two: download speed high→low (default), latency low→high |
| RouterOS | unaffected |
| Branch | continue on `feat/node-details-export-tags` |

## Data Model (subscription service)

Migration `services/subscription/migrations/2_add_export_settings.up.sql`:
```sql
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_include_dead BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_sort TEXT NOT NULL DEFAULT 'speed_desc';
```
`.down.sql` drops both columns.

`Subscription` struct gains:
```go
ExportIncludeDead bool   `json:"export_include_dead"`
ExportSort        string `json:"export_sort"`
```

A normalizer guards the enum:
```go
func normalizeExportSort(s string) string {
	if s == "latency_asc" { return "latency_asc" }
	return "speed_desc" // default + fallback for unknown/empty
}
```

### Endpoints (`services/subscription/subscription.go`)

- **List / GetSubscription / GetSubscriptionByID**: add `export_include_dead, export_sort` to each SELECT column list and scan target. (`GetSubscriptionByID` is the internal lookup the checker export path will use.)
- **CreateParams** gains `ExportIncludeDead bool` + `ExportSort string`. `Create` inserts them (`export_sort` via `normalizeExportSort`; a zero-value request yields `false` + `speed_desc`). The returned struct includes them.
- **UpdateParams** gains `ExportIncludeDead *bool` + `ExportSort *string` (pointers, matching the existing `Enabled *bool` COALESCE partial-update pattern). `Update` sets:
  ```sql
  export_include_dead = COALESCE($7, export_include_dead),
  export_sort         = COALESCE($8, export_sort)
  ```
  with `$8` pre-normalized in Go when non-nil (`nil` → no change).

## Export Path (checker service)

`exportPrefs` carries the two settings into the loaders:
```go
type exportPrefs struct {
	IncludeDead bool
	Sort        string // "speed_desc" | "latency_asc"
}
```

- **`loadExportProxies`** (`export.go`) resolves prefs per subscription:
  - single sub: `sub, err := subsvc.GetSubscriptionByID(ctx, &subsvc.GetByIDParams{ID: subID})`; on error fall back to `exportPrefs{Sort: "speed_desc"}` (include-dead false); pass into `latestUsableProxies`.
  - all subs: prefs are resolved per-subscription inside `latestUsableProxiesAcrossAllSubs` (see below).
- **`loadJobProxies`** signature gains `prefs exportPrefs`:
  - **WHERE**: the `cr.alive = true` predicate becomes conditional. With `IncludeDead`, drop it; the `COALESCE(n.enabled, true) = true` predicate is always kept (disabled nodes always excluded).
  - **ORDER BY** by `prefs.Sort`:
    - `speed_desc` → `speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST` (current)
    - `latency_asc` → `latency_ms ASC NULLS LAST, speed_kbps DESC NULLS LAST`
  - The alive predicate and order clause are built from validated/boolean inputs only (no string interpolation of user data — `Sort` is one of two constants), so no injection risk.
  - The post-query Go `sort.Slice` is parameterized to match `prefs.Sort` (kept consistent with the SQL order).
- **`latestUsableProxies(ctx, subID, userID, cfg, prefs)`** — passes prefs to `loadJobProxies`.
- **`latestUsableProxiesAcrossAllSubs(ctx, userID, cfg)`** — in its per-job loop, fetch each subscription via `subsvc.GetSubscriptionByID` (gives both the name for the prefix and the prefs); skip the job on lookup error (mirrors the current "name empty → skip"). This replaces the existing `GetSubscriptionNames` batch call. Pass each sub's prefs to `loadJobProxies`.
- **RouterOS** (`latestServerAddresses`) is untouched.

> Dead nodes still carry a valid proxy config (`COALESCE(n.config, cr.node_config)`), so they render fine. They have `speed_kbps=0` (sink to the bottom under `speed_desc`; `latency_ms` NULL/0 sorts last under `latency_asc` via NULLS LAST) and pick up no platform/speed export tags.

## Frontend

`SubscriptionDialog` (`components/workbench/subscription-dialog.tsx`) — add two controls, shown in **both** add and edit modes:
- Checkbox **"Include dead nodes in export"** → `export_include_dead`.
- Select **"Export order"**: `Download speed (high→low)` = `speed_desc`, `Latency (low→high)` = `latency_asc` → `export_sort`.

Seed both from the subscription on open (defaults `false` / `speed_desc` for add). On submit, include them in `CreateParams` (add) / `UpdateParams` (edit) alongside the existing fields. Regenerate the typed client so `Subscription`/`CreateParams`/`UpdateParams` carry the new fields.

> The dialog already passes `cron_expr`/`clear_cron_expr` on update; the two new fields join that payload. The workbench's `handleToggleEnabled` (which builds an `UpdateParams` for the enable toggle) must also pass the current `export_include_dead`/`export_sort` so toggling enable doesn't reset them — or send them as `null`/omitted (pointer params → COALESCE no-ops). Use the omit/undefined path so enable-toggle never touches export settings.

## Out of Scope

- Sort options beyond the two chosen (no name/country/upload/traffic sort)
- Per-node export ordering or manual reordering
- Changing RouterOS export behavior
- Applying include-dead/sort to the in-app results table (that has its own independent sort/filter)

## Risks & Mitigations

- **`export_sort` injection** → never interpolate the raw value; map through `normalizeExportSort` to one of two constants before building the ORDER BY.
- **Backward compatibility** → column defaults (`false`, `speed_desc`) + `loadExportProxies` error-fallback reproduce current export output byte-for-byte; untouched subscriptions behave identically.
- **All-subs lookup cost** → one `GetSubscriptionByID` per subscription in the aggregate export (small N; same order as the prior `GetSubscriptionNames` + acceptable).
- **Enable-toggle clobbering export settings** → enable toggle sends pointer params with the export fields omitted (COALESCE no-op), preserving them.

## Verification (definition of done)

1. `encore test ./services/...` green — new tests: subscription Create/Update round-trips the two fields + `normalizeExportSort`; checker export honors include-dead (dead node present only when true) and sort order (`latency_asc` reorders).
2. `bun check-types`, `bun check`, `bun run test:unit`, `bun run build` green.
3. Browser: Add/Edit subscription dialog shows + persists the checkbox and select; toggling enable does not reset them.
4. End-to-end: seed a job with one alive + one dead node; default export excludes the dead node and orders by speed; set include-dead → dead node appears; set `latency_asc` → order changes; RouterOS export unchanged across all of it.
5. Default (untouched) subscription export output matches pre-feature byte-for-byte.
