# Verge Streaming Parity — Region/Status Unlock + Engine Extensions + Latency

**Date:** 2026-06-14
**Status:** Approved
**Branch:** `feat/verge-streaming-parity` (off `main`)

## Goal

Bring the checker's media-unlock detection and latency probe in line with
[clash-verge-rev](https://github.com/clash-verge-rev/clash-verge-rev)'s
`media_unlock_checker`, **while keeping `platform_rules` as the mechanism**:

1. **Rewrite all default platform rules** to verge's detection logic.
2. **Add the platforms verge has and we lack**: 哔哩哔哩大陆, 哔哩哔哩港澳台, 巴哈姆特动画疯, Spotify, Prime Video, and split ChatGPT into Web + iOS. Keep our YouTube(plain) and Grok (verge lacks them).
3. **Upgrade rule output from boolean → `{unlocked, status, region}`** so verge's multi-state + region survives, stored in a single unified `platforms jsonb` column.
4. **Change latency to verge's** single Cloudflare probe.

This is the **Option B** scope chosen during brainstorming: engine + DB + frontend all change.

## Decisions From Brainstorming

| Question | Decision |
|----------|----------|
| Scope of "match verge" | **B** — full parity incl. region + multi-status (engine + DB + frontend) |
| Storage model | **Unified** — one `platforms jsonb`, drop the 9 bool columns + `extra_platforms` |
| Engine extension | Add `http_post`/`http_request`, response headers, per-run cookie jar; JS rules may return an object |
| Status vocabulary | Store verge's raw status strings **plus** a normalized `unlocked` bool (canonical for filter/sort/export/alerts) |
| Region | 2-letter country code stored; UI renders flag emoji |
| ChatGPT | Keep `openai` key = ChatGPT **Web**; add new `chatgpt_ios` |
| YouTube(plain) / Grok | **Keep** (verge lacks them) |
| Disney+ | Full bamgrid `POST` chain via new `http_post`; main-page regex fallback |
| Bahamut | Multi-step GET with per-run cookie jar |
| Netflix region | via fast.com CDN API (GET JSON), not the `Location` header |
| Latency | Single GET `http://cp.cloudflare.com/generate_204`, 10s; merges alive+latency |
| Claude/Gemini | verge's "exit-IP country vs blocklist" approach (semantics change vs our current real-probe; accepted) |

A full read/write inventory of every consumer of the platform booleans was produced during brainstorming and drives the "Backend/Frontend changes" checklists below.

---

## Section 1 — Rule Engine Contract & Extensions

### Output type

```go
// services/checker/ (new shared type, e.g. in rules.go)
type PlatformOutcome struct {
	Unlocked bool   `json:"unlocked"`
	Status   string `json:"status"`           // verge-style: "Yes","No","Failed","Soon","Originals Only","IP Banned",...
	Region   string `json:"region,omitempty"` // 2-letter country code, "" if none
}
```

- `runRule` / `runUserRules` / `runUserRulesWithDebug` return `map[string]PlatformOutcome` (was `map[string]bool`).
- **JS rules** may `return { unlocked, status, region }`. A bare `return true/false` stays valid and normalizes to `{Unlocked:b, Status: b?"Yes":"No", Region:""}`. Detection: inspect the goja return value — object → read fields; otherwise `ToBoolean()`.
- **condition rules** still produce a bool → normalized the same way (`Region:""`). No region/status from condition rules; platforms needing region become `js`.

### New JS builtins (`services/checker/engine_script.go`)

- `http_post(url, {headers, body})` and a general `http_request({method, url, headers, body})`. `body` is a string (callers `JSON.stringify` or form-encode themselves).
- Response object gains `headers` (map of lower-cased name → value): `{ status, body, final_url, headers }`.
- **Per-execution cookie jar**: build the goja VM's `http.Client` with a `cookiejar.New` so multi-step flows (Bahamut: `getdeviceid` → `token.php`) share cookies. The jar lives for one rule evaluation only.
- `httpGetResult` struct extended with `Headers map[string]string`; `trackedHTTPRequest` already returns status/body/final_url and records headers into the debug recorder — surface them to JS too.

> The proxy client used for media checks (`mihomo.go` `mediaClient`) must carry the cookie jar when rules run. Today `mediaClient` is a bare `http.Client{Transport: pc.Transport, Timeout: 8s}`; add `Jar: cookiejar.New(nil)` per node-check (or per rule run). Confirm during implementation whether one jar per node or per rule is cleaner; **per rule** is safest for isolation.

### Backward-compat for existing rules

Existing user-authored `js`/`condition` rules returning bool keep working unchanged (normalized). No migration of user rules needed.

---

## Section 2 — Data Model & Migration

### New column

`check_results.platforms jsonb NOT NULL DEFAULT '{}'`:

```json
{
  "netflix": {"unlocked": true,  "status": "Yes",            "region": "US"},
  "disney":  {"unlocked": false, "status": "Soon",           "region": "JP"},
  "bilibili_hkmctw": {"unlocked": false, "status": "No",     "region": ""}
}
```

Covers **all** platforms — built-in and custom — replacing `extra_platforms`.

### Migration (checker service, new numbered migration)

`up`:
1. `ALTER TABLE check_results ADD COLUMN platforms jsonb NOT NULL DEFAULT '{}';`
2. Backfill from existing data: for each builtin bool column, write `{"unlocked": <bool>, "status": <bool?'Yes':'No'>, "region": ""}` into `platforms[<key>]`; merge each existing `extra_platforms` entry as `{"unlocked": <bool>, "status": <bool?'Yes':'No'>, "region": ""}`. (Single `UPDATE` using `jsonb_build_object` + `jsonb_object_agg` over `extra_platforms`.)
3. `ALTER TABLE check_results DROP COLUMN openai, DROP COLUMN netflix, ... DROP COLUMN extra_platforms;` (all 9 bools + extra_platforms).

`down`: recreate the 9 bool columns + `extra_platforms`; backfill bools from `platforms[key]->>'unlocked'`; drop `platforms`. Best-effort (region/status are lost on down — documented).

### API DTO

`NodeResult` (`checker.go`) and the internal `nodeCheckResult` (`mihomo.go`) and `LocalUnlockResult` (`local_check.go`):
- Remove the 9 bool fields + `ExtraPlatforms`.
- Add `Platforms map[string]PlatformOutcome `json:"platforms"``.

`PlatformUnlockSummary` (`checker.go`) counts are computed by iterating `platforms` and counting `unlocked` per key (dynamic, not 9 fixed fields).

### `builtinKeys` (`rules.go`)

Expand to the full 15-key roster (below). Still used to distinguish built-in vs custom for icon/label/export-default purposes.

---

## Section 3 — Latency (verge parity)

`services/checker/mihomo.go`:

- Change default constant: `aliveTestURL = "http://cp.cloudflare.com/generate_204"` (was gstatic).
- Replace the two-request flow in `checkNode` (`isAlive` then `measureLatency`) with **one** request: a `probeLatency(ctx, client, url) (alive bool, ms int)` that does a single GET, returns `alive = (err==nil && 200<=status<400)` and `ms = elapsed`. On `!alive`, node is dead → skip the rest (unchanged downstream logic).
- Per-user `latency_test_url` setting still overrides the default (unchanged threading).
- `isAlive` / `measureLatency` are removed (or `measureLatency` folded into `probeLatency`). Proxy client timeout stays 10s (`proxyTimeout`), matching verge.

---

## Platform Roster (15 rules, all `js`, output `{unlocked, status, region}`)

Ported from `clash-verge-rev/src-tauri/src/cmd/media_unlock_checker/*.rs`. `unlocked` is set explicitly by each rule.

| key | name | method | logic | region |
|---|---|---|---|---|
| `netflix` | Netflix | GET | fast.com `api.fast.com/netflix/speedtest/v2?...&urlCount=5`: 403→`No (IP Banned)`; JSON `targets[0].location.country`→`Yes`. Fallback titles `81280792`/`70143836`: 404+404→`Originals Only`(unlocked=false), 403→`No`, 200/301→`Yes` | fast.com country |
| `youtube` | YouTube | GET | `youtube.com`: 200 & body `youtube` & not地区否定词→`Yes` (keep current) | optional body `GL` |
| `youtube_premium` | YouTube Premium | GET | `youtube.com/premium?hl=en`: body contains "not available in your country"→`No`; 2xx & (`youtube premium`/`ad-free`/`"browseId":"SPunlimited"`)→`Yes` | body regex (4 patterns) |
| `openai` | ChatGPT Web | GET | `api.openai.com/compliance/cookie_requirements`: body `unsupported_country`→`No (Unsupported Country)` else `Yes` | `chat.openai.com/cdn-cgi/trace` `loc` |
| `chatgpt_ios` | ChatGPT iOS | GET | `ios.chat.openai.com/`: body `disallowed ISP`→`Disallowed ISP`; `request is not allowed`→`Yes`; `blocked`→`Blocked` | trace `loc` |
| `claude` | Claude | GET | `claude.ai/cdn-cgi/trace` `loc`; blocklist `[AF,BY,CN,CU,HK,IR,KP,MO,RU,SY]`→`No` else `Yes` | `loc` |
| `gemini` | Gemini | GET | `gemini.google.com` body marker `,2,1,200,"`→3-char country; blocklist `[CHN,RUS,BLR,CUB,IRN,PRK,SYR,HKG,MAC]`→`No` else `Yes` | parsed code |
| `grok` | Grok | GET | `api.x.ai/v1/models`: 401/200→`Yes` (keep current; verge lacks) | "" |
| `disney` | Disney+ | **POST** chain | bamgrid `devices`→assertion→`token`→refresh_token→`graphql`: `inSupportedLocation` + `countryCode`→`Yes`/`Soon`(false+code)/`No`; `forbidden-location`/403→`No (IP Banned)`; fallback GET `disneyplus.com` regex `region":"XX"`→`Yes` | graphql / main page |
| `tiktok` | TikTok | GET | `tiktok.com/cdn-cgi/trace`: status 403/451→`No`; 2xx & not (`access denied`/地区否定词)→`Yes`; fallback `tiktok.com/` | trace / body `"region":"XX"` |
| `bilibili_cn` | 哔哩哔哩大陆 | GET | playurl API (`avid=82846771&ep_id=307247`...) JSON `code`: 0→`Yes`, -10403→`No`, else `Failed` | "" |
| `bilibili_hkmctw` | 哔哩哔哩港澳台 | GET | playurl API (`avid=18281381&ep_id=183799`...) JSON `code`: 0→`Yes`, -10403→`No`, else `Failed` | "" |
| `bahamut` | 巴哈姆特动画疯 | GET + cookie | `ani.gamer.com.tw/ajax/getdeviceid.php`→regex `deviceid`→`ajax/token.php?...&device=<id>` contains `animeSn`→`Yes` else `No`; homepage `data-geo` for region | `data-geo` |
| `spotify` | Spotify | GET | `spotify.com/api/content/v1/country-selector?platform=web&format=json`: 403/451 or body `not available in your country`→`No` else `Yes` | final_url path / body `countryCode` |
| `prime_video` | Prime Video | GET | `primevideo.com`: body `isServiceRestricted`→`No`; regex `"currentTerritory":"XX"`→`Yes`+region | body |

Exact request bodies/headers for Disney (auth bearer constant `ZGlzbmV5...`, device JSON, token form, graphql mutation) are copied verbatim from `disney_plus.rs`.

`rules_defaults.go` `defaultRules` is rewritten to these 15 entries (each `ScriptDef{Code: ...}`); `seedDefaultRules` mechanism unchanged.

---

## Default-Rule Sync (compatibility — REQUIRED)

`seedDefaultRules` uses `ON CONFLICT (user_id, key) DO NOTHING`, so **existing users keep their old rules** and never receive the new logic or the 6 new platforms. Add a **sync** that runs where defaults are currently seeded (first `ListRules`, see `rules.go`):

- For each entry in `defaultRules`, **upsert** rows where `is_default = true`: update `definition, rule_type, icon, name, sort_order` and insert any missing `key`. Do **not** touch user-authored rules (`is_default=false`) or user-customized copies.
- Open question handled by decision: a default rule a user *edited* keeps `is_default=true` today (the one observed edited Claude/Gemini row is still `is_default=true`), so a naive upsert would overwrite user edits. **Mitigation:** add an `is_default` AND `not user-modified` guard — simplest is to only upsert when the stored `definition` still matches a known prior default (hash check) OR add a `customized boolean` flag set when a default rule is edited. **Chosen:** add nothing to schema; upsert all `is_default=true` rows unconditionally and document that editing a built-in rule is overwritten on upgrade (users wanting permanent edits should clone to a custom key). Revisit if too aggressive.

---

## Backend Changes (checklist, from inventory)

**checker service:**
- `engine_script.go` — http_post/http_request, response headers, cookie jar, object return (Section 1).
- `rule_eval.go`, `rules.go` — `PlatformOutcome`, `map[string]PlatformOutcome`, `builtinKeys` 15-key, default-rule sync.
- `rules_defaults.go` — 15 rewritten rules.
- `mihomo.go` — `nodeCheckResult.Platforms`; `checkNode` stores **every evaluated rule's** outcome into `Platforms` (rules still filtered by `opts.MediaApps`; drop the per-field `hasApp`→bool mapping entirely); `probeLatency` (Section 3); `mediaClient` cookie jar.
- `checker.go` — `NodeResult.Platforms`, `PlatformUnlockSummary` dynamic counts, `GetResults` SELECT/scan `platforms`, `defaultCheckOptions.MediaApps` default = all 15 keys.
- `jobstore.go` — `insertResult` writes `platforms` jsonb.
- `export_data.go` — `unlockFlags`/`builtinUnlocked`/`taggedName`/`loadJobProxies` read from `platforms` map.
- `local_check.go` — `LocalUnlockResult.Platforms`; `GetLocalUnlock` builds the map.
- New migration — unify columns (Section 2).

**notify service:**
- `notify_types.go` — `PlatformCounts`/`LocalUnlockReport` → maps; `fromCheckerSummary`/`fromCheckerLocalUnlock` iterate `platforms`.
- `alerts.go` — `checkPlatformAlerts` builds `current` from the platforms map (dynamic keys); `subscription_platform_state` / `platform_alerts` unchanged (keyed by string, auto-supports new platforms).
- `formatters.go` — add display names for the 6 new platforms.

**settings service:**
- `settings.go` — `defaultExportTags` add new platform tags (e.g. `spotify→SP`, `prime_video→PV`, `bahamut→BH`, `bilibili_cn→BL`, `bilibili_hkmctw→BL港`, `chatgpt_ios→iOS`); `mergeExportTags` builtin set expands.

---

## Frontend Changes (checklist)

- Regenerate `client.gen.ts` (`encore gen client ...`) — `NodeResult.platforms`.
- `components/platform-icons.tsx` — `PlatformKey` union + `PLATFORM_META` add `chatgpt_ios, bilibili_cn, bilibili_hkmctw, bahamut, spotify, prime_video` (icon/color/label).
- `lib/nodeFilters.ts` — `BUILTIN_PLATFORMS` expand; `NodeLike` uses `platforms: Record<string, {unlocked,status,region}>`; `nodeHasPlatform` reads `platforms[k]?.unlocked`; `filterNodes` updated.
- `components/workbench/node-table.tsx` — `UnlockIcons` iterates `node.platforms`, renders icon where `unlocked`; region code in `title` (stay compact).
- `components/workbench/node-detail-dialog.tsx` — `platformRows` from `platforms` map; each row = icon + label + **status pill** (green `Yes` / red `No` / amber `Soon`·`Originals Only` / grey `Failed`) + **flag emoji + region code**.
- `routes/settings/export-tags.tsx` — driven by expanded `PLATFORM_META`; preview includes new platforms.
- `components/notify-channel-dialog.tsx` — platform alert list driven by `PLATFORM_META` keys (auto-includes new).
- New `lib/countryToFlag.ts` — country code → flag emoji (regional-indicator math, matching verge's `alpha2_to_emoji`).

---

## Out of Scope

- Per-platform region columns (we use jsonb).
- SSE, scheduler, auth changes.
- Migrating user-authored rules (they keep working as bool).
- Preserving user edits to built-in default rules across upgrade (documented: cloned to custom key to persist).

## Risks & Mitigations

- **Disney POST chain + cookie engine** is the most complex piece; the bamgrid token flow can be brittle. Mitigation: keep the main-page regex fallback; cover with `httptest` mocks for each branch.
- **Destructive migration** (dropping bool columns). Mitigation: backfill in the same `up`; test on a copy; provide `down`; recommend a DB snapshot before deploy.
- **Default-rule sync overwrites user edits to built-ins** (see decision). Mitigation documented; revisit if reported.
- **Claude/Gemini semantics change** (exit-IP-vs-blocklist instead of real probe). Accepted per "match verge"; note in release notes.
- **`encore test -race` hangs** in this repo (known) — run tests without `-race`.

## Verification (definition of done)

1. `encore test ./services/...` (no `-race`) green. New tests: engine `http_post`/headers/cookie + object return; each default rule against `httptest` mocks asserting `{unlocked,status,region}` per branch (Yes/No/Soon/IP Banned/Originals Only/Failed); migration backfill (bool+extra → platforms); `taggedName` with `platforms` map; notify alert diff from map; `probeLatency` alive/dead/ms.
2. `bun check-types`, `bun check`, `bun run build` green; regenerated client committed.
3. Browser: run a check → node-detail platform matrix shows status pills + region flags; node-table shows unlocked icons; export tags + notify dialog list the new platforms; latency populated via Cloudflare probe.
4. Migration applied on a populated dev DB: existing results still show their unlock state (region empty for pre-migration rows); new checks populate region/status.
