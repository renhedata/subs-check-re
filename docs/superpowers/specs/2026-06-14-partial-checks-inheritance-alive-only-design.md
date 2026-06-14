# Partial Checks with Inherited Results + Alive-Only Mode

**Date:** 2026-06-14
**Status:** Approved
**Branch:** `feat/partial-checks-inheritance` (new). Depends on the rule-driven
infrastructure (`useRules()` / the `ListRules` endpoint) introduced on
`feat/platform-rules-experience`; base this branch on that work (or on `main`
once it has landed there), not on a `main` that lacks it.

## Goal

Make a check measure only the dimensions the user selects, and have every
**unselected** dimension fall back to the node's most recent known value
(matched by `node_name` within the subscription) when results are displayed or
exported. Add an explicit **"Alive only"** mode: re-probe connectivity fast
while preserving the last-known speed, country, and streaming columns.

The motivating use case: quickly re-verify which nodes are still up without
re-running the expensive speed + streaming tests, and without losing those
columns from the results view or the exported subscription.

## Behavior Model

| Dimension | Behavior |
|-----------|----------|
| **alive + latency** | **Always measured.** Always uses this run's fresh value; never inherited. |
| download speed | Measured only if selected; otherwise inherits the most recent `>0` value. (Existing behavior — kept.) |
| upload speed | Measured only if selected; otherwise inherits. (Existing behavior — kept.) |
| country | Fetched alongside platform tests; otherwise inherits the most recent non-empty value. (**New.**) |
| streaming platforms | Each platform selected individually; unselected platforms inherit **per-key** from the most recent result that tested that platform. (**New.**) |

"Alive only" = speed off + upload off + platforms empty → measures alive, inherits everything else.

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| Is `alive` a skippable dimension? | **No (Model A).** Alive is always measured — it is the cheap gate every live test depends on, and keeping it fresh keeps exports trustworthy. |
| Streaming granularity | **Per-platform.** Selecting a subset tests only those; the rest inherit per-key. |
| Speed when not tested | Inherits previous value (existing logic, kept). |
| Where inheritance happens | **Read-time merge (Approach 1).** No schema change; consistent with the existing speed fallback; `check_results` stays a truthful record of what each job actually measured. |
| UI for "Alive only" | **Preset button** in the Run Check popover + a read-only note that connectivity is always tested (Option X). |
| Platform list source | **Rule-driven** via the existing `useRules()` — not the hardcoded 15. Custom rules become selectable and filterable. |
| Country inheritance | **Included** — otherwise an alive-only run blanks the country tag in exports. |

---

## Why rule filtering is a precondition

Today `MediaApps` is only an on/off gate. `checkNode` (`services/checker/mihomo.go`)
runs **all** enabled rules whenever `len(opts.MediaApps) > 0` — it does **not**
filter to the selected subset. So selecting 3 platforms still tests every enabled
platform, and an unselected platform is always re-measured, never inherited.

Per-platform inheritance is therefore impossible until selection actually filters
which rules run. Making `MediaApps` a real filter is part of this work.

---

## Backend

### (a) Filter rules by selection — `services/checker/jobrunner.go`

In `jobRunner.run`, after `loadUserRules`, filter the slice to rules whose `Key`
is in `opts.MediaApps` before fanning out to workers. `checkNode` then evaluates
only the selected rules. Empty `MediaApps` → no rules run → empty `platforms`
written → all platforms inherit at read time.

Filtering once per job (not per node) keeps it cheap. The `runUserRulesWithDebug`
loop in `engine.go` already skips `!rule.Enabled`, so the effective tested set is
`enabled ∩ selected`.

### (b) Make "Alive only" expressible — `services/checker/checker.go`

`applyOptionDefaults` currently coerces "speed off + empty media" into a full
default check, making alive-only impossible to express. Change it to:

- Default `MediaApps` to the built-in list **only when the field is `nil`** (omitted).
- An explicit empty `[]` stays empty (= test no platforms).
- Drop the "reset to full" branch.

`TriggerCheck` (user path) already builds options from defaults + pointer
overrides and does not coerce; the frontend sends `media_apps: []` for alive-only.
Verify the scheduler persists a complete options object so scheduled alive-only
checks survive the round-trip.

### (c) Country fetch stays in the media block — `services/checker/mihomo.go`

`getProxyInfo` (IP + country) remains gated on the media block so alive-only stays
fast (no extra request). When skipped, country inherits at read time (below).
`alive` and `latency` are always measured and never inherit.

### (d) Read-time inheritance — `GetResults` (`checker.go`) + `loadJobProxies` (`export_data.go`)

Both read paths compute each node's "latest known state" by `node_name` within
the subscription. `alive`/`latency` come from the current job row (fresh); the
inherited dimensions come from history:

- **Speed / upload:** existing `CASE WHEN cr.speed_kbps > 0 … ELSE (latest >0)` subqueries — kept as-is. (Export already inherits speed; export does not surface upload, so no change there.)
- **Country (new):** add a "latest non-empty country by `node_name`" fallback, same shape as the speed subquery.
- **Platforms (new), per-key merge:** a reusable CTE —

  ```sql
  WITH expanded AS (
    SELECT cr.node_name, kv.key, kv.value, cr.checked_at
    FROM check_results cr
    JOIN check_jobs cj ON cj.id = cr.job_id
    CROSS JOIN LATERAL jsonb_each(cr.platforms) AS kv(key, value)
    WHERE cj.subscription_id = $sub AND cr.platforms <> '{}'::jsonb
  ),
  latest_per_key AS (
    SELECT DISTINCT ON (node_name, key) node_name, key, value
    FROM expanded
    ORDER BY node_name, key, checked_at DESC
  )
  SELECT node_name, jsonb_object_agg(key, value) AS platforms
  FROM latest_per_key GROUP BY node_name
  ```

  Join this per-node merged blob into the result row and use it instead of the
  raw current-row `platforms`. Because the current job's rows are the most recent,
  any key it measured automatically wins; keys it didn't measure fall back to the
  newest prior measurement of that key.

In `loadJobProxies` the `cr.alive = true` filter now reflects **fresh** liveness
(alive is always measured), so alive-only exports still drop genuinely dead nodes
while restoring the inherited platform/country/speed tags.

No migration. No new endpoint. Regenerate the typed client only if a response
shape changes (it should not — `platforms`/`country` fields already exist).

---

## Frontend

### (a) Rule-driven platform list — `components/check-options-fields.tsx`, `lib/checkOptions.ts`

Replace the hardcoded `MEDIA_APPS` toggles with the user's enabled rules from
`useRules()`, keyed by `rule.key`, reusing `RulePlatformIcon`. Selected keys →
`media_apps`.

- "Select all" / first-run default = all enabled rule keys.
- A persisted selection is intersected with the currently-available rule keys
  (a deleted/renamed rule key drops out cleanly).
- `DEFAULT_CHECK_OPTIONS.media_apps` keeps a static built-in list as the
  no-rules-loaded fallback, but the live default derives from loaded rules.

### (b) "Alive only" preset + note — Run Check popover

- A **"Alive only"** button that sets `{ speed_test: false, upload_speed_test: false, media_apps: [] }`.
- A read-only line atop the options: **"Connectivity + latency: always tested."**
- The Schedule dialog reuses `CheckOptionsFields`, so scheduled alive-only checks
  come for free.

Live SSE progress shows `speed=0` etc. for inherited dimensions during the run
(they are not measured this run); the final results view fills them via the
read-time fallback. No staleness indicator in this iteration (out of scope).

---

## Edge Cases & Known Trade-offs

- Inheritance keys on `node_name`; duplicate names within a subscription merge —
  **pre-existing** limitation of the speed fallback, no new risk.
- "Most recent" is global (no time bound), matching the existing speed fallback.
  Viewing an old job may surface a newer inherited value; acceptable.
- A brand-new node, or a platform never tested, shows empty/0 (no history).
- A node judged dead this run still shows inherited speed/platforms in the results
  table (exports drop it via the alive filter). Acceptable.
- Bare API callers that omit `options` get the built-in platform default; custom
  rules won't run unless options are sent explicitly. The UI always sends complete
  keys, so it is unaffected.
- Speed fallback triggers on `>0`, so "tested, got 0" and "not tested" are
  indistinguishable — unchanged from today, acceptable.

---

## Testing

**Backend — unit**
- Rule filtering: only rules with `Key ∈ MediaApps` run; empty `MediaApps` runs none.
- Alive-only: writes empty `platforms`, fresh `alive`/`latency`, zero speed.
- `applyOptionDefaults`: explicit alive-only is preserved; `nil` media still defaults.

**Backend — integration (results + export)**
- Job A tests NF+YT → Job B alive-only → B's results inherit NF+YT.
- Job C tests NF only → NF fresh, YT still inherited from A (per-key recency).
- Country and speed fall back when not measured.
- `loadJobProxies` after an alive-only run preserves platform + country tags and
  filters on fresh liveness.

**Frontend**
- `CheckOptionsFields` renders rule-driven toggles from `useRules()`.
- "Alive only" preset clears speed + upload + platforms.
- Persisted selection intersects current rule keys.
- Update `lib/checkOptions.test.ts`.

Follow the existing test style. Do **not** run with `-race` (known harness hang,
per project memory).
