# Export All Subscriptions Design

**Date:** 2026-03-17
**Status:** Approved

## Goal

Add a single export URL on the dashboard that combines alive nodes from all of the user's subscriptions into one subscription link.

## Context

The existing `/export/:subscriptionID?token=...&target=...` endpoint exports nodes from a single subscription's latest completed check job. Users currently need separate links for each subscription. A combined link is useful for proxy clients that only support a single subscription URL.

## Backend — New Endpoint

```
GET /export/all?token={apiKey}&target={clash|base64}
```

Registered as `//encore:api public raw method=GET path=/export/all`.

This does **not** conflict with `/export/:subscriptionID` because Encore routes are matched by literal path segments first — `all` is a fixed literal, UUID subscription IDs cannot equal `"all"`.

### Logic

1. Parse `token` → `user_id` via `settingssvc.GetUserIDByAPIKey` (same as `Export`)
2. Parse `target` (default `"clash"`)
3. Query the checker DB for the latest completed job per subscription owned by `user_id` using `DISTINCT ON`:
   ```sql
   SELECT DISTINCT ON (subscription_id) id AS job_id, subscription_id
   FROM check_jobs
   WHERE user_id = $1 AND status = 'completed'
   ORDER BY subscription_id, created_at DESC
   ```
4. Collect the distinct `subscription_id` values and call the new private endpoint `subsvc.GetSubscriptionNames` to resolve their names (see below). Build a `map[string]string` of `subscriptionID → name`.
5. For each `(job_id, subscription_id)`, query alive nodes from `check_results` — same columns as `Export`, no historical speed fallback.
6. For each node, prefix the name: `{subName}|{taggedName(originalName, ...unlocks, speedKbps)}`
7. Collect all nodes from all subscriptions and sort by `speed_kbps DESC, latency_ms ASC`
8. Render with existing `renderClash` or `renderBase64`
9. Log to `export_logs` with `subscription_id = "all"`

### Cross-Service: New Private Endpoint in Subscription Service

The `subscriptions` table lives in the subscription service's own database. The checker cannot query it directly. A new `private` endpoint is added to `services/subscription/subscription.go` following the same pattern as `settingssvc.GetUserIDByAPIKey`:

```go
// GetSubscriptionNamesParams is the request for the internal name-lookup endpoint.
type GetSubscriptionNamesParams struct {
    UserID string   `json:"user_id"`
    IDs    []string `json:"ids"`
}

// GetSubscriptionNamesResponse maps subscription ID → name.
type GetSubscriptionNamesResponse struct {
    Names map[string]string `json:"names"`
}

// GetSubscriptionNames is a private endpoint for internal service calls.
//
//encore:api private method=POST path=/internal/subscriptions/names
func GetSubscriptionNames(ctx context.Context, p *GetSubscriptionNamesParams) (*GetSubscriptionNamesResponse, error) {
    // Query subscriptions WHERE id = ANY($1) AND user_id = $2
    // Return id→name map
}
```

Called from `ExportAll` as:
```go
namesResp, err := subsvc.GetSubscriptionNames(ctx, &subsvc.GetSubscriptionNamesParams{
    UserID: userID,
    IDs:    subscriptionIDs,
})
```

### Node Naming

```
{subscriptionName}|{taggedName(originalName, ...unlocks, speedKbps)}
```

Example: `ProxyA|HK-01|NF|GPT|1.5MB`

`taggedName` is called with the original node name. The subscription prefix is prepended to the result.

### Implementation locations

- `services/subscription/subscription.go` — add `GetSubscriptionNames` private endpoint
- `services/checker/export.go` — add `ExportAll` function

No new migrations needed.

## Frontend — Dashboard

In `frontend/apps/web/src/routes/index.tsx`, the export section currently maps over `subs` to show per-subscription export URLs. Add an "All Subscriptions" card **above** the per-subscription list, using the same visual style (copy buttons for clash and base64 targets).

URL constructed as:
```typescript
`${window.location.origin}/api/export/all?token=${apiKey}&target=clash`
`${window.location.origin}/api/export/all?token=${apiKey}&target=base64`
```

No backend API type changes — `ExportAll` is a raw endpoint, no Encore-generated client update needed.

## Out of Scope

- Deduplication of nodes with identical configs across subscriptions
- Filtering by subscription (include/exclude toggles)
- Separate export log per subscription for the "all" combined export
