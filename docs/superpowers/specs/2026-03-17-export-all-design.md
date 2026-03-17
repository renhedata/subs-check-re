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
3. Query all subscriptions owned by `user_id` from `subscriptions` table
4. For each subscription, find its latest completed check job (`status='completed' ORDER BY created_at DESC LIMIT 1`)
5. For each job, query alive nodes (`cr.alive = true`) with their config and unlock flags — no historical speed fallback (use `cr.speed_kbps` directly, 0 if untested)
6. Prefix each node name with the subscription name before tagging: `{subName}|{taggedName}`
7. Merge all nodes from all subscriptions, sort by `speed_kbps DESC, latency_ms ASC`
8. Render with existing `renderClash` or `renderBase64`
9. Log to `export_logs` with `subscription_id = "all"`

### Node Naming

```
{subscriptionName}|{taggedName(originalName, ...unlocks, speedKbps)}
```

Example: `ProxyA|HK-01|NF|GPT|1.5MB`

The subscription name is fetched alongside the node query (JOIN `subscriptions`).

### Query Pattern

```sql
-- For each subscription owned by user, find latest completed job
SELECT cj.id AS job_id, s.name AS sub_name
FROM check_jobs cj
JOIN subscriptions s ON s.id = cj.subscription_id
WHERE cj.user_id = $1
  AND cj.status = 'completed'
  AND cj.id = (
      SELECT id FROM check_jobs
      WHERE subscription_id = cj.subscription_id AND user_id = $1 AND status = 'completed'
      ORDER BY created_at DESC LIMIT 1
  )
```

Then for each `(job_id, sub_name)`, query alive nodes from `check_results` (same columns as `Export`).

**Simpler alternative** (use in implementation): use `DISTINCT ON` or a single query with a lateral join to get the latest job per subscription, then join check_results.

### Implementation location

Add `ExportAll` function to `services/checker/export.go`. No new migrations needed.

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
