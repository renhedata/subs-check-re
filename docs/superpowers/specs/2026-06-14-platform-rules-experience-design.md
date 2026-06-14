# Platform Rules Experience — Rule-Driven Unlock, De-Hardcoded Icons, First-Class Page

**Date:** 2026-06-14
**Status:** Approved
**Branch:** `feat/platform-rules-experience` (off `feat/verge-streaming-parity`; depends on the `platforms` map, the 15 default rules, and the icon consumers introduced there)

## Goal

Make "Platform Rules" a first-class, professionally-designed part of the app and make all platform detection/display **rule-driven instead of hardcoded**:

1. **Network-unlock by enabled rules** — the server-side `/network-unlock` probe runs the user's *enabled* rules, not a hardcoded default set.
2. **De-hardcoded icons** — every platform icon comes from its rule's `icon` field (Iconify ID / data URL / http URL / emoji), not the hardcoded `PLATFORM_META` brand glyphs.
3. **Richer icon picker** — surface Iconify's full multi-set search/browse (already wired) **plus SVG/PNG upload** (stored inline as a data URL).
4. **Platform Rules → top-level sidebar entry** with a focused professional redesign (action bar + clean sortable list + right-side editor sheet + inline test).

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| Custom-icon storage | **A — data URL inline** in the existing `icon` text field; cap ≤ 32 KB, SVG preferred |
| Icon library | Iconify (150+ sets / 200k+ icons) is already wired; surface it better + add upload. No new icon npm deps. |
| De-hardcoding extent | Remove `PLATFORM_META` entirely; all icons driven by the rule's `icon`/`name`; letter-badge fallback |
| UI redesign ambition | **方案 1 (focused professional)** — action bar, sortable list, right-side editor sheet, inline test panel |
| Sidebar | Move out of Settings to a top-level `/rules` entry (rail + mobile tabbar) |
| Scope | One spec (the whole Platform-Rules / detection experience) |

---

## Section 1 — Backend: network-unlock runs enabled rules

`services/checker/local_check.go`:

- `GetLocalUnlock` (auth'd, has `userID` via claims) loads the user's rules with `loadUserRules(ctx, userID)`, filters to `enabled`, and runs them against a plain (no-proxy) client.
- Replace `runDefaultRulesAgainst` (which iterates the hardcoded `defaultRules`) with `runRulesAgainst(ctx, client, rules []*PlatformRule) map[string]PlatformOutcome` that runs each enabled rule **concurrently** (preserve the old goroutine fan-out — ~15 network probes must not run serially) and collects `{key: PlatformOutcome}`.
- `LocalUnlockResult` is unchanged (`{Platforms map[string]PlatformOutcome, IP, Country}`).
- Remove the now-unused `runDefaultRulesAgainst` and `ruleResult` helper (the new helper supersedes them). Keep `getProxyInfo` for IP/country.

Result: "enable a rule → it's probed; disable it → it isn't," consistent with `checkNode`.

## Section 2 — De-hardcoded, unified icon renderer

`frontend/src/components/platform-icons.tsx` is rewritten:

- **Remove** `PLATFORM_META`, the `react-icons/si` imports, the custom `DisneyPlusIcon`/`GrokIcon`/`BahamutIcon`/`PrimeVideoIcon` components, `PlatformIcon`, and `PlatformIconAny`.
- **Add** one component `PlatformIcon({ icon, label, size?, showLabel? })` that renders from the `icon` string:
  - Iconify ID (`isIconifyId`) → `<IconifyIcon>` wrapped so a **non-resolving** id falls back to a letter badge (use `@iconify/react`'s `loadIcon`/`iconExists` in an effect; show letter badge until/unless it resolves).
  - `http(s)://` or `data:` → `<img>`.
  - emoji/other text → `<span>`.
  - empty/missing → first-letter badge of `label`.
- **Add** a display hook `usePlatformDisplay(key) → { icon, label }` that reads the matching rule from the existing `usePlatformRules()` context (`icon` from `rule.icon`, `label` from `rule.name`), falling back to `{ icon: "", label: key }` when no rule exists.
- Brand `color` tinting is dropped (icons render via their own colors / `currentColor`).
- `PlatformKey` union is removed; `nodeFilters.BUILTIN_PLATFORMS` stays (it's the canonical builtin key list used for export-tag ordering — data, not icons).

Default rules already carry Iconify IDs; any that don't resolve (e.g. niche ones) show the letter badge. As part of this section, set any non-resolving default-rule `icon` to a valid Iconify id or emoji (audit the 15 during implementation; e.g. `bahamut` → an emoji or `mdi:television-classic`).

## Section 3 — Icon picker (search + browse + upload + emoji)

Rework `frontend/src/components/platforms/IconPicker.tsx` (and `IconPickerInput`) into a popover with:

- **Search** — Iconify `api.iconify.design/search` (existing, keep; debounced grid).
- **Quick** — a small curated set of popular collections for one-click pick (brands: `simple-icons`, `logos`; generic: `lucide`, `tabler`). Implemented via Iconify search scoped by `prefix=`.
- **Upload** — file input accepting `image/svg+xml,image/png,image/jpeg,image/webp`; `FileReader.readAsDataURL`; **reject > 32 KB**; on success write the data URL to `icon`. (Uploaded SVGs render via `<img src="data:image/svg+xml,…">`, which does **not** execute embedded scripts, so only a size/mime check is needed — no heavy sanitization.)
- **Direct input** — keep the raw text field (emoji / paste id / paste URL).
- **Live preview** of the selected icon via the new `PlatformIcon`.

## Section 4 — Sidebar move + focused page redesign (方案 1)

**Move to top-level:**
- New route `frontend/src/routes/rules.tsx` at `/rules` (move the page logic out of `routes/settings/platforms.tsx`; delete the settings route + remove the "Platform Rules" entry from `routes/settings.tsx` `TABS`).
- Add a nav entry in `rail.tsx` `NAV_ITEMS` and `mobile-tabbar.tsx` (between Scheduler and Settings), icon `Radar` (lucide), `matchPrefix: "/rules"`.

**Redesign (focused/professional):**
- **Action bar:** title, "Add rule" button, a search input (filter by name/key), filter chips (enabled-only, rule type).
- **Rule list:** clean rows — drag handle, icon (new renderer), name, `key` badge, rule-type badge, `enabled` Switch, edit/delete. **Drag-to-reorder** updates `sort_order` (persist via `useUpdateRule`). Add `@dnd-kit/core` + `@dnd-kit/sortable` (not currently deps; standard, small).
- **Editor:** a **right-side sheet** (slide-over). No Sheet primitive exists today — add a lightweight `frontend/src/components/ui/sheet.tsx` (right slide-over built on the same Base UI / portal pattern as `ui/dialog.tsx`). Contents: name, `key` (read-only on edit), the **new IconPicker**, rule-type selector, the type-specific editor (reuse `ConditionEditor` / the script editor), and an **inline Test panel** (reuse `useTestRule` + the test-node picker + `ConsolePanel`). Save/delete actions.
- Built-in vs custom rules are visually distinguished (a small "default" tag) but edited the same way (note: editing a built-in rule is overwritten by the default-rule sync on the next `ListRules` — surface a hint, consistent with the verge spec's documented behavior).

## Section 5 — Consumers + testing

**Consumers updated to the unified renderer + `usePlatformDisplay`:**
- `workbench/node-table.tsx` `UnlockIcons`, `workbench/node-detail-dialog.tsx` platform matrix, `routes/settings/export-tags.tsx`, `components/notify-channel-dialog.tsx` (alert chips), `components/check-options-fields.tsx` (media selector), `workbench/unlock-strip.tsx`.
- `unlock-strip.tsx`: drop the hardcoded `PLATFORM_KEYS`; render from the returned `platforms` map keys (which reflect the enabled rules from Section 1); optionally show `status`/`region` per platform.
- Remove all `as PlatformKey` casts.

**Testing:**
- Backend: `GetLocalUnlock` unit test — seed enabled + disabled rules for a user, assert only enabled keys appear in `Platforms` (mock the HTTP client like the rule tests do).
- Frontend unit tests: `PlatformIcon` rendering branches (iconify / url / data / emoji / empty→letter-badge); upload→data-URL helper (type + 32 KB cap); existing `countryToFlag`/`nodeFilters` stay green.
- `bun check-types` and `bun run build` green; no new `bun check` errors in touched files.

## Out of Scope

- Object storage for icons (data URL only).
- Migrating the `feat/verge-streaming-parity` branch to main.
- Icon-set CDN bundling (Iconify stays API-driven).
- Per-rule icon color theming.

## Risks & Mitigations

- **Removing `PLATFORM_META` touches ~6 consumers** — broad but mechanical; `bun check-types` is the backstop (the build won't pass until every consumer is migrated).
- **Large data-URL icons bloat `ListRules`** — 32 KB cap + SVG-first guidance; most icons are Iconify ids (tiny).
- **Iconify ids that don't resolve** — letter-badge fallback + audit default-rule icons.
- **New deps (`@dnd-kit/core`+`@dnd-kit/sortable`) and a new `ui/sheet.tsx`** — both small/standard; confirmed not present today.
- **`local_check` serial slowness** — preserve concurrent fan-out in `runRulesAgainst`.

## Verification (definition of done)

1. `encore test ./services/checker/` green incl. the new `GetLocalUnlock` enabled-rules test.
2. `bun check-types` + `bun run build` green; new icon-renderer + upload unit tests pass.
3. Browser: Platform Rules is a top-level sidebar item with the redesigned page (sortable list, right sheet editor, icon picker with search/quick/upload/emoji + preview, inline test). A node's unlock icons + the network-unlock strip render from rule icons (change a rule's icon → UI reflects it). Disable a rule → it disappears from the network-unlock probe and from per-node detection.
