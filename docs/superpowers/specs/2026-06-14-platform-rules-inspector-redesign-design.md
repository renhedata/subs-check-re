# Platform Rules — Master–Detail Inspector Redesign

**Date:** 2026-06-14
**Status:** Approved (visual direction validated via browser mockups — direction B)
**Branch:** `feat/platform-rules-experience` (continues; **replaces** the prior page/editor from `2026-06-14-platform-rules-page.md`)

## Goal

Completely discard the current `/rules` UI (a centered list + slide-over `Sheet` editor) and rebuild it as a **professional master–detail inspector** — the pattern dev tools use (Postman / Insomnia / Linear). One full-width screen: a rule list on the left, a live inspector/editor on the right. No modal, no sheet, no separate dialog.

This is a **frontend-only** redesign — every backend endpoint and the data model already exist (`useRules`, `useCreateRule`, `useUpdateRule`, `useDeleteRule`, `useTestRule`, `useTestNodes`). It builds on the rule-driven icon system and `/rules` route already shipped in this branch.

## Decisions From Brainstorming (validated against live mockups)

| Decision | Choice |
|----------|--------|
| Overall layout | **B — master–detail**: left list pane + right inspector, full-width, no dialog/sheet |
| Editor housing | Inline inspector (the prior `RuleEditorDialog`/`Sheet` is removed entirely) |
| Left list | Search + `+`; **Built-in / Custom** group dividers; rows = drag-handle (hover) · icon · name · enabled dot; drag-to-reorder persists `sort_order`; click selects |
| Inspector | Identity header (icon→picker · name · key chip · **type segmented control** · enabled switch · ⋯/delete) → Definition area → docked Test panel → Save/Delete footer |
| Definition by type | `condition` → structured **form**; `js`/`ts`/`tengo`/`lua` → **code editor** (existing Monaco) |
| Test | Docked at inspector bottom: node picker → Run → verdict (status pill · region flag · latency) + console output |
| Icon picker | **Popover** anchored to the inspector icon: Search (Iconify) · Brands/Logos/Generic quick tabs · Emoji · **Upload SVG/PNG (data URL ≤32 KB)** + preview |
| Empty states | no selection → "Select a rule"; no rules → "No rules yet + New rule" |
| New rule | left `+` → inspector enters a blank editable **draft** (same UI), Save → `useCreateRule` |
| Theme | Use the app's existing design tokens (`primary`/`success`/`muted`/`border`/`--color-active-bg`…), **not** the mockup's raw hex. Accent = existing primary; enabled/pass = `success`. |
| Mobile | Single-pane: list by default; selecting a rule slides the inspector over full-screen (back button returns to list) |

Mockups persisted under `.superpowers/brainstorm/…/content/` (`direction.html`, `b-refined.html`, `states.html`).

---

## Architecture & Components

A clean decomposition replaces the monolithic `RuleEditorDialog`. New/changed files under `frontend/src/`:

| File | Responsibility |
|------|----------------|
| `routes/rules.tsx` | **Rewrite.** Page shell: holds `selectedId` + `draft` state; lays out the two panes (responsive); renders list + inspector/empty. No data fetching beyond `useRules`. |
| `components/platforms/RuleListPane.tsx` | **New.** Left pane: search box, `+` new, Built-in/Custom grouped **sortable** list (reuses `@dnd-kit`), selection highlight. Props: `rules`, `selectedId`, `onSelect`, `onNew`. Persists reorder via `useUpdateRule`. (Adapts the logic from the now-removed `SortableRuleList.tsx`.) |
| `components/platforms/RuleInspector.tsx` | **New.** Right pane for one rule (existing or draft). Owns local edit form state; identity header, type segmented control, the definition editor (delegates by type), docked test panel, Save/Delete footer. Props: `rule` (or draft), `isDraft`, `onSaved`, `onDeleted`, `onClose`. |
| `components/platforms/RuleDefinitionEditor.tsx` | **New (thin).** Switches on `rule_type`: `condition` → `ConditionEditor` (existing); else → the Monaco code editor (existing engine setup). Single `{ value, onChange }` interface so the inspector doesn't branch internally. |
| `components/platforms/RuleTestPanel.tsx` | **New.** Docked test runner: `useTestNodes` picker + `useTestRule` Run + verdict row + `ConsolePanel` (existing) output. Props: `ruleType`, `definition`. |
| `components/platforms/IconPickerPopover.tsx` | **New / refactor of `IconPicker.tsx`.** The richer popover (Search/Brands/Logos/Generic/Emoji tabs + grid + Upload). Reuses the `iconUpload` helper + `RuleIcon` preview already in the branch. Anchored to the inspector icon tile. |
| `components/platforms/RuleEditorDialog.tsx` | **Delete.** Replaced by `RuleInspector`. |
| `components/platforms/SortableRuleList.tsx` | **Delete** — its dnd/reorder logic is folded into `RuleListPane` (grouped Built-in/Custom + selection), so the standalone component is no longer used. |
| `components/ui/sheet.tsx` | **Delete** — no longer used (inspector is inline; picker is a popover). Confirm no other importers first. |
| `components/platforms/RuleCard.tsx` | **Delete if unused** after the rewrite. |

Reused as-is: `RuleIcon`/`usePlatformDisplay`, `ConditionEditor`, `ConsolePanel`, `engine`/`useMonacoSetup`, `iconUpload`, all `queries/rules.ts` hooks, `ui/*` primitives, `popover`.

### Page shell (`routes/rules.tsx`) data flow

- `useRules()` → `rules`. `const [selectedId, setSelectedId] = useState<string|null>(null)`. `const [draft, setDraft] = useState<DraftRule|null>(null)`.
- `+ New` → `setDraft(emptyDraft())`, `setSelectedId(null)`.
- Select a row → `setSelectedId(id)`, `setDraft(null)`.
- Right pane = `draft ? <RuleInspector isDraft draft={draft} …/> : selected ? <RuleInspector rule={selected} …/> : <EmptyState/>`.
- After `onSaved`/`onDeleted`, clear draft / adjust selection and let React Query refetch.

### Inspector edit model

`RuleInspector` keeps a local working copy (`name`, `key`, `icon`, `rule_type`, `definition`, `enabled`, `sort_order`) initialized from the rule/draft, with a dirty flag. **Save** calls `useUpdateRule` (existing) or `useCreateRule` (draft). `key` is editable only for a draft (read-only chip otherwise). Switching `rule_type` adapts the definition editor; the definition is preserved per-type where possible (keep last condition def and last script separately in local state so toggling doesn't destroy work — minor nicety, optional).

### Responsive

- `≥ lg`: two panes side by side (list ~252px fixed, inspector flex).
- `< lg`: one pane. Default shows `RuleListPane` full-width; selecting a rule (or `+`) shows `RuleInspector` full-width with a back chevron in its header (`onClose` → clear selection/draft → back to list). No `Sheet`/modal — just conditional render.

### Theme

Map the mockup palette to existing tokens: panels/borders via `bg-card`/`border-border`; selected row via `--color-active-bg`/`--color-active-border` (as the rail already does); enabled dot + pass pill via `text-success`/`success-muted`; type chips via `secondary`/`muted`; accent buttons via `primary`/`success` button variants. The code editor keeps its Monaco theme. No new color tokens.

## Error handling

- Mutations surface failures via `toast.error(isApiError(e) ? e.message : "…")` (existing pattern).
- Test failures render in the verdict row (red pill) + console, not a toast.
- Delete uses the existing `confirm-dialog` primitive before calling `useDeleteRule`.
- Built-in rule edit hint: a small inline note ("Built-in — edits are overwritten on upgrade; clone to a custom key to keep") — consistent with the documented default-rule sync behavior.

## Testing

- The redesign is presentational; rely on `bun check-types` + `bun run build` as the structural gate.
- Keep/port any existing unit tests that still apply; add a focused unit test for the draft→create vs existing→update branch selection in `RuleInspector` if it's cleanly extractable (pure function for "which mutation").
- Browser walkthrough (definition of done) covers the interactive behavior.

## Section 6 — Customizable built-in rules + Reset (added)

Today `syncDefaultRules` upserts every `is_default` rule on each `ListRules`, so user edits to a built-in rule are silently overwritten (they never persist). To make built-in rules editable **and** resettable:

- **`customized` flag** — new `platform_rules.customized boolean DEFAULT false` column. `syncDefaultRules` only restores built-ins **where `customized=false`**, so a user-edited built-in is preserved. The sync also stops resetting `sort_order` (only sets it on insert), so reordering built-ins persists too.
- **Marking** — `UpdateRule` sets `customized=true` for an `is_default` rule **only when its content (name/icon/rule_type/definition) actually differs from the seed** (`defaultRuleByKey`). Toggling `enabled` or reordering alone does **not** mark it customized (those already persist independently). Reverting content back to the seed clears the flag.
- **Reset** — new `POST /platform-rules/:ruleId/reset`: for an `is_default` rule, restore `name/icon/rule_type/definition/sort_order` from the seed and clear `customized`. 404 for non-existent / non-default rules.
- **UI (Inspector)** — for an `is_default` rule, show a small **"Modified"** badge + a **"Reset to default"** button when `customized`. Reset calls the endpoint and the rule reverts (refetch).

This **changes** the verge-documented "built-in edits are overwritten on upgrade" behavior — by design, per this request. The inspector's old "edits are overwritten" hint is replaced by the Modified/Reset affordance.

## Out of Scope

- Other backend changes (the rest of the redesign is frontend-only — only this Section 6 touches the backend).
- The icon system internals (`RuleIcon`/`usePlatformDisplay`/`iconUpload`) — already shipped; only the picker's *presentation* is reworked.
- Merging the branch to `main`.
- Multi-select / bulk actions on rules.

## Risks & Mitigations

- **Large rewrite of the rules page** — mitigated by the clean component split (each new file is small and single-purpose) and by reusing all data hooks + editors unchanged.
- **Removing `sheet.tsx`/`RuleEditorDialog.tsx`/`SortableRuleList.tsx`** — grep for importers before deleting; the build is the backstop.
- **Mobile two-pane** — handled by conditional single-pane render, not a separate layout.
- **Monaco in a non-modal inline pane** — `useMonacoSetup` already initializes it; ensure the editor sizes to the flex pane (height:100%/min-h-0 flex chain).

## Verification (definition of done)

1. `bun check-types` + `bun run build` green; no dead imports to the deleted files.
2. Browser (`bun dev` + `encore run`): `/rules` shows the master–detail layout; left list searches, groups Built-in/Custom, drag-reorders (persists across refresh), selection drives the inspector; inspector edits + saves; type segmented control swaps condition-form ↔ code editor; icon tile opens the picker (search/quick/emoji/upload-with-preview) and changing it updates the icon live; inline Test runs against a node and shows verdict + console; New-rule draft creates; delete confirms + removes; disabling a rule drops it from the server network-unlock strip. Mobile width collapses to single-pane with back navigation.
3. Visual quality matches the approved `b-refined.html` / `states.html` mockups (hierarchy, spacing, professional feel) using the app's theme tokens.
