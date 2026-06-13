# Node Detail View + Global Export Tag Scheme

**Date:** 2026-06-13
**Status:** Approved
**Branch:** `feat/node-details-export-tags` (based on `feat/ui-redesign`; stacks on PR #3 — depends on the redesigned workbench/Settings UI, which is not yet in `main`)

## Goal

Two independent, node-related features:

- **F1 — Node detail view (read-only):** from the workbench results table, open a per-node dialog showing full identity (name, protocol, server:port, IP, country), performance metrics, the full platform unlock matrix, and the raw proxy config.
- **F2 — Global export tag scheme:** a Settings tab to customize the tags appended to node names in exports — a country tag, per-platform tags (toggle + label text), and the speed tag — applied across **all** subscriptions. Defaults preserve today's behavior.

Per-node base-name renaming is **out of scope** (decided: A1/B1 — customization is global tag rules, not per-node names).

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| F1 detail content | Everything available, incl. raw config + protocol type |
| F1 surface | Dialog opened by clicking a node row in the results table |
| Per-node base-name rename | Not done (customization is global, not per-node) |
| What "tag customization" means | A1 — global tag label/rule scheme (e.g. `NF`→`Netflix`), which tags show, speed format |
| Where it's edited | B1 — a new global tab under Settings |
| Country tag | Add the detected country as an optional export tag |
| Branch | New branch off `feat/ui-redesign` |

---

## F1 — Node Detail View

### Backend

`NodeResult` (`services/checker/checker.go`) gains three fields:

```go
Server string `json:"server"`
Port   int    `json:"port"`
Config string `json:"config"` // raw proxy config as a JSON string
```

`GetResults` (`services/checker/checker.go`) — the CTE already `LEFT JOIN nodes n` and selects `COALESCE(n.name, cr.node_name)` etc. Add to the SELECT/scan:

- `COALESCE(n.server, '')` → `Server`
- `COALESCE(n.port, 0)` → `Port`
- `COALESCE(n.config, cr.node_config)::text` → `Config` (scanned as `[]byte`, stored as string)

No new endpoint. Regenerate the typed client afterwards.

### Frontend

New component `components/workbench/node-detail-dialog.tsx`:
- Props: `result: NodeResult | null`, `rules: PlatformRule[]`, `open`, `onOpenChange`.
- Uses the existing `Dialog` primitive (full-screen on mobile).
- Sections:
  1. **Identity** — node name (header), protocol `type`, `server:port` (mono), exit `ip` (mono), `country`.
  2. **Performance** — latency (tone-colored via `latencyTone`), download (`speed_kbps`), upload (`upload_speed_kbps`), traffic (`formatBytes`). Each "—" when absent.
  3. **Platforms** — every built-in platform (`BUILTIN_PLATFORMS`) plus every key in `extra_platforms`, each shown with a ✓ (unlocked, success tone) or ✗ (muted). Built-in labels from `PLATFORM_META`; extra-platform labels from the matching `rules` entry (`rule.name`) falling back to the key. (Note: a platform the job didn't test shows ✗ — NodeResult carries no per-node "tested" set; acceptable for v1.)
  4. **Raw config** — collapsible `<details>`/disclosure with a `<pre>` of pretty-printed `JSON.parse(result.config)` (guarded) + a `CopyButton` for the raw string.

Wiring in `components/workbench/node-table.tsx`:
- The node-name cell (desktop row) and the mobile card become clickable to open the dialog. The existing enable-toggle (`●`/`○`) keeps its own `onClick` with `e.stopPropagation()` so toggling never opens the dialog.
- `node-table.tsx` owns `const [detail, setDetail] = useState<NodeResult | null>(null)` and renders `<NodeDetailDialog result={detail} rules={rules} open={!!detail} onOpenChange={(o) => !o && setDetail(null)} />`. `rules` is already passed into NodeTable.

No change to the table's columns or the SSE/streaming path. Streamed (live) rows have synthetic `node_id` and partial data; the dialog still opens and simply shows "—" for missing fields — acceptable, and the final refetch replaces them.

---

## F2 — Global Export Tag Scheme

### Data model (settings service)

Extend `UserSettings` (`services/settings/settings.go`):

```go
type PlatformTag struct {
	Key     string `json:"key"`     // netflix, openai, gemini, claude, grok, youtube, disney, tiktok
	Label   string `json:"label"`   // default short tag, user-editable
	Enabled bool   `json:"enabled"`
}

type ExportTagConfig struct {
	ShowCountry bool          `json:"show_country"` // default false (preserves current export names)
	ShowSpeed   bool          `json:"show_speed"`   // default true
	Platforms   []PlatformTag `json:"platforms"`    // ordered; drives tag order in export
}

type UserSettings struct {
	SpeedTestURL   string          `json:"speed_test_url"`
	UploadTestURL  string          `json:"upload_test_url"`
	LatencyTestURL string          `json:"latency_test_url"`
	EmailConfig    EmailConfig     `json:"email_config"`
	ExportTags     ExportTagConfig `json:"export_tags"`
}
```

Persistence mirrors `email_config`: a new `export_tags JSONB` column on `user_settings` (migration), read with `COALESCE(export_tags, 'null'::jsonb)` in `GetSettings`, written in `UpdateSettings`' upsert.

**Defaults** (when stored value is null/empty) come from a `defaultExportTags()` helper matching today's `taggedName`:

| key | label | enabled |
|-----|-------|---------|
| netflix | NF | ✓ |
| openai | GPT | ✓ |
| gemini | GM | ✓ |
| claude | CL | ✓ |
| grok | GK | ✓ |
| youtube | YT | ✓ |
| disney | D+ | ✓ |
| tiktok | TK | ✓ |

`ShowSpeed: true`, `ShowCountry: false`. YouTube Premium keeps its special rule in code: when premium is unlocked the youtube tag renders as `<label>+` (e.g. `YT+`).

> `GetSettings` must merge stored platforms with `defaultExportTags()` so that a config saved before a new platform key existed still tags that platform. Merge rule: start from defaults, override label/enabled for any key present in the stored list, preserve stored order for known keys.

### Export path (checker service)

1. **Settings lookup:** add a private endpoint to the settings service:
   ```go
   //encore:api private method=POST path=/internal/settings/export-tags
   func GetExportTags(ctx, *GetExportTagsParams{UserID string}) (*ExportTagConfig, error)
   ```
   returning the user's merged `ExportTagConfig` (defaults when no row). Mirrors the existing `GetUserIDByAPIKey` internal pattern that `export.go` already uses.
2. **Thread the config through:** `dispatchExport` already resolves `userID`. Fetch the config once there (or in `loadExportProxies`) and pass it into `latestUsableProxies` / `latestUsableProxiesAcrossAllSubs` → `loadJobProxies`.
3. **Query:** `loadJobProxies` adds `cr.country` to its SELECT/scan (needed for the country tag).
4. **`taggedName` rewrite** (`services/checker/export_data.go`): the function gains a `country string`, a `cfg ExportTagConfig`, and keeps the existing per-node unlock booleans (`netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok`) and `speedKbps` (exact param grouping — individual args vs a small struct — is the implementer's call).
   It builds the tag chain in this order: **country** (if `ShowCountry` and country non-empty) → **platforms** (in `cfg.Platforms` order, each enabled tag whose flag is true; youtube uses `label+"+"` when premium) → **speed** (if `ShowSpeed` and `speedKbps>0`, current format). Joined with `|`, appended to base name. No tags → base name unchanged (current behavior).

Separator stays `|`. `extra_platforms` (custom-rule platforms) are **not** added to export tags in this version — only the eight built-ins + country + speed.

### Frontend

New Settings tab **"Export Tags"** — fifth tab in `routes/settings.tsx` (`/settings/export-tags`), route `routes/settings/export-tags.tsx`.

- Loads via `useSettings()`; saves via `useUpdateSettings()` (sends the full `UserSettings` incl. existing fields, like the General tab does).
- UI:
  - **Country** row: `Switch` (Show detected country tag).
  - **Speed** row: `Switch` (Show speed tag).
  - **Platforms** list: one row per platform — `Switch` (enabled) + `Input` (label text, e.g. edit `NF`→`Netflix`). Platform display names from `PLATFORM_META`.
  - **Live preview** line: renders an example export name from the current form state using a sample node (name `HK-01`, country `HK`, a couple of unlocked platforms, a sample speed), e.g. `HK-01｜HK｜Netflix｜ChatGPT｜10.5MB`, so the effect is visible before saving. (The real export uses whatever `cr.country` holds — plain text, no flag rendering.)
  - Save button with `loading` state + success toast (General-tab pattern).
- Add the tab to the `TABS` array in `routes/settings.tsx`.

---

## Out of Scope

- Per-node base-name renaming
- Per-node custom tags (tags are a global scheme)
- Tagging `extra_platforms` (custom-rule platforms) in exports
- Configurable separator or drag-reorder of tags (fixed `|`, fixed order)
- Showing tested-vs-untested platform distinction in F1 (no per-node tested set available)

## Risks & Mitigations

- **Old saved configs missing a platform key** → `GetSettings` merges stored over `defaultExportTags()` so new platforms still tag by default.
- **Export is `public raw` (token auth)** → tag config fetched via the new private settings endpoint keyed by the token-resolved `userID`; no auth-context dependency.
- **Backward compatibility** → defaults (`ShowCountry:false`, `ShowSpeed:true`, built-in short labels) reproduce current export names exactly; users who never open the tab see no change.
- **Raw config exposes credentials** (password/uuid) → it's the user's own data behind auth; shown as-is with a copy button. No masking (YAGNI).

## Verification (definition of done)

1. `encore test ./services/...` green (new tests: `taggedName` honors config incl. country/disabled/custom-label/premium; `GetSettings` default-merge).
2. `bun check-types`, `bun check`, `bun run test:unit`, `bun run build` green.
3. Browser: click a node → detail dialog shows identity/perf/platform-matrix/raw-config + copy; mobile full-screen.
4. Settings → Export Tags: toggle country on, rename `NF`→`Netflix`, disable a platform, toggle speed; preview updates; save; then fetch an export URL and confirm node names reflect the config (country present, renamed/omitted tags correct, premium shows `+`).
5. Default (untouched) export names byte-match pre-feature output.
