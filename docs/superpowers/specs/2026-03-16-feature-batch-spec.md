# subs-check-re Feature Batch Spec

**Date:** 2026-03-16
**Status:** Draft

## Overview

This spec covers six independent but related improvements to the subs-check-re platform:

1. Speed test bug fix
2. Selective check options (speed / media toggles per job)
3. Job history API + frontend
4. Notify channel update endpoint
5. Node export API (subscription link generation)
6. Dashboard API documentation

---

## 1. Speed Test Bug Fix

### Problem

`measureSpeed` in `services/checker/mihomo.go` reuses `pc.Client`, which has `Timeout: 10s`. That timeout starts from `Do()` and includes TCP + TLS handshake. For slow proxies, the handshake alone may consume 2–5 s, leaving insufficient time to download 200 KB. Worse, `io.Copy` returns `(n, err)` where `err != nil` when the timeout fires mid-download — the code then returns 0, discarding the partial measurement.

### Fix

- Add `speedTestTimeout = 30 * time.Second` constant.
- In `measureSpeed`, create a **dedicated** `*http.Client` using the proxy's existing `Transport` but with `Timeout: speedTestTimeout`. Do not reuse `pc.Client`.
- Record `startDownload` only **after** reading the response headers (i.e., after `resp` is returned, before `io.Copy`). This excludes connection and handshake overhead from the speed calculation.
- If `io.Copy` returns `(n > 0, err != nil)` (partial download due to timeout), **still compute speed** from `n` and the elapsed time since `startDownload`.

```go
const speedTestTimeout = 30 * time.Second

func measureSpeed(ctx context.Context, transport http.RoundTripper, speedTestURL string) int {
    client := &http.Client{Timeout: speedTestTimeout, Transport: transport}
    resp, err := get(ctx, client, speedTestURL)
    if err != nil {
        return 0
    }
    defer resp.Body.Close()
    startDownload := time.Now()
    n, _ := io.Copy(io.Discard, resp.Body)   // ignore err — partial is fine
    if n == 0 {
        return 0
    }
    elapsed := time.Since(startDownload).Seconds()
    if elapsed == 0 {
        return 0
    }
    return int(float64(n) / 1024 / elapsed)
}
```

Callers pass `pc.Client.Transport` (the embedded `*http.Client` has a `.Transport` field):

```go
result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)
```

---

## 2. Selective Check Options

### Goal

Users should be able to trigger a check with some checks disabled to save time:
- Skip speed test (for quick connectivity/unlock-only checks)
- Skip all media checks (for speed-only checks)
- Select a subset of media platforms to test

### API Change

`POST /check/:subscriptionID` accepts an optional JSON body:

```json
{
  "speed_test": true,
  "media_apps": ["netflix", "youtube", "openai", "claude", "gemini", "disney", "tiktok"]
}
```

| Field | Type | Default | Meaning |
|-------|------|---------|---------|
| `speed_test` | bool | `true` | Whether to run speed test |
| `media_apps` | []string | all platforms | Platforms to test. Empty array = skip all media checks. |

Valid `media_apps` values: `openai`, `claude`, `gemini`, `netflix`, `youtube`, `disney`, `tiktok`.

### Storage

Add `options_json JSONB` column to `check_jobs` (new migration). The resolved options (with defaults applied) are **always** inserted as non-null JSON — never NULL:

```json
{"speed_test": true, "media_apps": ["openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"]}
```

When `TriggerCheck` is called with no body (or missing fields), defaults are used before inserting.

### Go types

```go
type CheckOptions struct {
    SpeedTest  bool     `json:"speed_test"`
    MediaApps  []string `json:"media_apps"`
}

func defaultCheckOptions() CheckOptions {
    return CheckOptions{
        SpeedTest: true,
        MediaApps: []string{"openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"},
    }
}
```

### Implementation

- `checkNode` signature gains an `opts CheckOptions` parameter.
- `measureSpeed` is called only when `opts.SpeedTest == true`.
- Each platform check is called only when that platform name is in `opts.MediaApps`.

### Internal trigger

`TriggerInternalParams` gains an `Options CheckOptions` field (zero-value struct means "all disabled", so callers must explicitly set defaults). The scheduler call site passes `defaultCheckOptions()`. Alternatively, the handler can check if `Options` is zero-value and apply defaults — specify this in the handler, not in the struct tag.

The safe pattern: in `TriggerCheckInternal`, after unmarshalling, call `applyOptionDefaults(&p.Options)` which sets `SpeedTest = true` and fills `MediaApps` if they are zero/empty.

---

## 3. Job History

### Backend

#### New endpoint: list jobs

```
GET /check/:subscriptionID/jobs
```

**Auth:** required (JWT)
**Query params:** `limit` (default 20), `offset` (default 0)

**Response:**
```json
{
  "jobs": [
    {
      "id": "...",
      "subscription_id": "...",
      "status": "completed",
      "total": 120,
      "available": 47,
      "speed_test": true,
      "media_apps": ["netflix", "openai"],
      "created_at": "...",
      "finished_at": "..."
    }
  ],
  "total": 15
}
```

`available` is read from the new `check_jobs.available` column (populated when job completes).

`speed_test` and `media_apps` are read by deserialising `options_json` from the DB into a `CheckOptions` struct, then flattened onto the response object. The Go response type:

```go
type JobSummary struct {
    ID             string     `json:"id"`
    SubscriptionID string     `json:"subscription_id"`
    Status         string     `json:"status"`
    Total          int        `json:"total"`
    Available      int        `json:"available"`
    SpeedTest      bool       `json:"speed_test"`
    MediaApps      []string   `json:"media_apps"`
    CreatedAt      time.Time  `json:"created_at"`
    FinishedAt     *time.Time `json:"finished_at,omitempty"`
}
```

When scanning the row, scan `options_json` into `[]byte`, then `json.Unmarshal` into `CheckOptions` to populate `SpeedTest` and `MediaApps`.

#### SQL changes in `runJob`

When marking the job complete, also persist `available`:
```sql
UPDATE check_jobs
SET status = 'completed', finished_at = $2, available = $3
WHERE id = $1
```
(Replace the existing `UPDATE check_jobs SET status='completed', finished_at=$2 WHERE id=$1` at the bottom of `runJob`.)

#### Modified: get results

```
GET /check/:subscriptionID/results?job_id=<optional>
```

Without `job_id`: returns latest **completed** job's results. Update the existing SQL to add `AND status = 'completed'`:
```sql
SELECT ... FROM check_jobs
WHERE subscription_id = $1 AND user_id = $2 AND status = 'completed'
ORDER BY created_at DESC LIMIT 1
```

With `job_id`: verify ownership before returning results:
```sql
SELECT id FROM check_jobs
WHERE id = $1 AND subscription_id = $2 AND user_id = $3
```
Return 404 if no row found. Then query `check_results` for that `job_id`.

### Frontend

**`/subscriptions/$id` page changes:**

- Fetch jobs list on mount (`GET /check/:id/jobs`).
- Show a **job history selector** (a compact row of pills above the results table): each pill shows `created_at` date + available/total. Clicking a pill loads that job's results via `?job_id=`.
- The active/running job (if any) is highlighted in the selector.
- When no jobs exist: show an empty state ("No checks run yet") instead of showing the node table.
- When a job is selected and it has results: show the node table normally.

---

## 4. Notify Channel Update

### New endpoint

```
PUT /notify/channels/:id
```

**Auth:** required
**Body:**
```json
{
  "name": "My Webhook",
  "config": { "url": "https://...", "method": "POST", "headers": {} },
  "enabled": true
}
```

All fields are optional (partial update). Use `*bool` for `enabled` so an omitted field does not reset it to `false`. Enforces ownership (user_id check). Returns updated `Channel`.

Go type:
```go
type UpdateChannelParams struct {
    Name    *string         `json:"name"`
    Config  json.RawMessage `json:"config"`
    Enabled *bool           `json:"enabled"`
}
```
SQL: `SET name = COALESCE($2, name), config = COALESCE($3, config), enabled = COALESCE($4, enabled)` — pass `nil` for unset pointer fields.

### Frontend

Settings → Notify page: add an edit button per channel that opens an inline edit form (same fields as create form, pre-populated with current values).

---

## 5. Node Export API

### Purpose

Generate a ready-to-use subscription link from the latest completed check results for a subscription. Intended to be used directly as a subscription URL in proxy clients (Clash, v2ray, etc.).

### Service ownership

The export endpoint lives in the **`checker` service** (it queries `check_results` and `nodes`, both in the checker DB). It cannot directly query `user_settings` (owned by the `settings` service). To resolve the API key to a `user_id`, a new private Encore API is added to the `settings` service:

```go
//encore:api private method=GET path=/internal/settings/api-key/:apiKey
func GetUserIDByAPIKey(ctx context.Context, apiKey string) (*UserIDResponse, error)
```

The checker service calls `settingssvc.GetUserIDByAPIKey(ctx, token)` at the start of the export handler.

### Endpoint

```
GET /export/:subscriptionID?token=<api_key>&target=clash
```

**Auth:** query param `token` (API key, not JWT). Marked `//encore:api public raw` so Encore's auth middleware is bypassed; the handler validates the token manually via the internal call above. This endpoint is public-accessible so proxy clients can fetch it directly.

**Parameters:**

| Param | Required | Default | Values |
|-------|----------|---------|--------|
| `token` | yes | — | User's API key |
| `target` | no | `clash` | `clash`, `base64` |

### Behavior

1. Validate `token` → call `settingssvc.GetUserIDByAPIKey(ctx, token)` to get `user_id`.
2. Find latest **completed** `check_job` for the given `subscriptionID` owned by that user.
3. Query `check_results JOIN nodes` for that job where `alive = true`.
4. For each node, append platform tags to the node name:
   - `|NF` if `netflix = true`
   - `|GPT` if `openai = true`
   - `|GM` if `gemini = true`
   - `|CL` if `claude = true`
   - `|YT-<region>` if `youtube != ''`
   - `|D+` if `disney = true`
   - `|TK-<region>` if `tiktok != ''`
5. Sort by `speed_kbps DESC`, then `latency_ms ASC`.
6. Render output in `target` format.

**`target=clash` output:**
YAML with `proxies:` block. Node config is stored as JSONB in `nodes.config`; serialize directly. Set `Content-Type: text/yaml`.

**`target=base64` output:**
One URI per line (`vmess://...`, `ss://...`, `trojan://...`, etc.), then base64-encode the entire block. Set `Content-Type: text/plain`. URI reconstruction is protocol-specific (see implementation notes below).

**Error responses:**
- 401 if token invalid or missing
- 404 if no completed job found for the subscription
- 200 with empty proxies list if alive nodes = 0

### API Key Management

- `api_key TEXT` column added to `user_settings` (new migration, nullable initially).
- New endpoint: `GET /settings/api-key` — returns existing key or generates a new UUID v4 on first call (upserts). Returns `{"api_key": "..."}`.
- New endpoint: `POST /settings/api-key/regenerate` — generates a new UUID v4, overwrites old one, returns new key.

### URI Reconstruction (base64 target)

Protocol-specific reconstruction from stored JSONB config:

| Type | URI format |
|------|-----------|
| `ss` | `ss://<base64(method:password)>@<server>:<port>#<name>` |
| `vmess` | `vmess://<base64(JSON v2 config)>` |
| `trojan` | `trojan://<password>@<server>:<port>?<params>#<name>` |
| `vless` | `vless://<uuid>@<server>:<port>?<params>#<name>` |
| `ssr` | `ssr://<base64(...)>` |
| others | Skip (omit from output) |

---

## 6. Dashboard API Documentation

### Changes to `/` (Dashboard page)

Add a new **"Export API"** section below the stat cards:

**Section content:**
- Brief explanation: "Use these URLs as subscription links directly in your proxy client."
- API Key display: masked value with a copy button, and a "Regenerate" button.
- A list of all user subscriptions, each with a generated export URL:
  ```
  https://<host>/api/export/<subscriptionID>?token=<api_key>&target=clash
  ```
  Two copy buttons per subscription: one for `clash`, one for `base64`.
- Parameter reference table:

| Parameter | Values | Description |
|-----------|--------|-------------|
| `token` | your API key | Authentication |
| `target` | `clash` (default), `base64` | Output format |

---

## Database Migrations

### `checker` service

**Migration 5** — add options and available to check_jobs:
```sql
ALTER TABLE check_jobs ADD COLUMN options_json JSONB;
ALTER TABLE check_jobs ADD COLUMN available INT NOT NULL DEFAULT 0;
```

### `settings` service

**Migration 2** — add api_key:
```sql
ALTER TABLE user_settings ADD COLUMN api_key TEXT UNIQUE;
```

### `notify` service

No schema changes needed.

---

## Implementation Order

1. **Speed test fix** — isolated, no schema change, low risk
2. **Notify update endpoint** — trivial, backend only
3. **Selective check options** — backend (migration + logic), then frontend trigger UI
4. **Job history** — backend (migration + endpoints), then frontend
5. **API key + Export endpoint** — backend (migration + new endpoint)
6. **Dashboard API docs** — frontend only

---

## Out of Scope

- Rate limiting on the export endpoint
- Export link expiry / revocation by subscription
- Multiple API keys per user
- `min_speed` filter on export
- Country-based node filtering
