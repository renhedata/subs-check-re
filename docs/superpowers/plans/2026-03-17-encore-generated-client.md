# Encore Generated Client Migration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hand-written `api.ts` fetch wrapper with Encore's generated type-safe TypeScript client, eliminating manual type mirroring.

**Architecture:** Generate `client.gen.ts` via `encore gen client`, write a thin `client.ts` singleton initializer, then migrate all 7 route files one by one from `api.get/post/put/delete()` to typed service methods. Keep `api.ts` alive until every file compiles, then delete it.

**Tech Stack:** Encore CLI (`encore gen client`), TypeScript, React 19, TanStack Query, Vite proxy (unchanged)

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `frontend/apps/web/src/lib/client.gen.ts` | Generated Encore client — do not edit manually |
| Create | `frontend/apps/web/src/lib/client.ts` | Singleton initializer with baseURL + auth |
| Modify | `frontend/apps/web/package.json` | Add `gen:client` script |
| Modify | `frontend/apps/web/src/routes/login.tsx` | auth service |
| Modify | `frontend/apps/web/src/routes/settings/general.tsx` | settings service |
| Modify | `frontend/apps/web/src/routes/subscriptions/index.tsx` | subscription + checker service |
| Modify | `frontend/apps/web/src/routes/scheduler.tsx` | scheduler + checker service |
| Modify | `frontend/apps/web/src/routes/subscriptions/$id.tsx` | checker service + SSE |
| Modify | `frontend/apps/web/src/routes/settings/notify.tsx` | notify service |
| Modify | `frontend/apps/web/src/routes/index.tsx` | settings + subscription + checker service |
| Modify | `frontend/apps/web/src/components/node-table.tsx` | type import update |
| Delete | `frontend/apps/web/src/lib/api.ts` | Replaced by generated client |

---

## Chunk 1: Generate Client + Singleton

### Task 1: Generate `client.gen.ts`

**Files:**
- Create: `frontend/apps/web/src/lib/client.gen.ts`

- [ ] **Step 1: Run generation command from project root**

```bash
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts
```

Expected: file created, no errors.

- [ ] **Step 2: Verify URL construction**

Open `frontend/apps/web/src/lib/client.gen.ts`. Find the `BaseClient` class (near the bottom). Locate where it concatenates the stored base URL with the request path — regardless of the exact variable names used. Confirm that the path being appended starts with `/`.

For example, if the generated code reads something like `baseURL + "/subscriptions"`, then `baseURL = "http://localhost:3001/api"` + `"/subscriptions"` = `"http://localhost:3001/api/subscriptions"` — which the Vite proxy rewrites to `localhost:4000/subscriptions`. ✅

- [ ] **Step 3: Add `gen:client` script to `frontend/apps/web/package.json`**

Add inside `"scripts"`:
```json
"gen:client": "encore gen client subs-check-uqti --lang=typescript --output=./src/lib/client.gen.ts"
```

Run from `frontend/apps/web/` to avoid path ambiguity.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/lib/client.gen.ts frontend/apps/web/package.json
git commit -m "feat(frontend): add encore generated typescript client"
```

---

### Task 2: Write `client.ts` singleton

**Files:**
- Create: `frontend/apps/web/src/lib/client.ts`

- [ ] **Step 1: Create the file**

```typescript
// frontend/apps/web/src/lib/client.ts
import { getToken } from "./auth";
import Client from "./client.gen";

export const client = new Client(
	`${window.location.origin}/api`,
	// auth is called per-request (lazy), not captured at construction time
	{ auth: () => getToken() ?? "" },
);

export function isApiError(
	err: unknown,
): err is { code: string; message: string; status: number } {
	return (
		typeof err === "object" &&
		err !== null &&
		"code" in err &&
		"message" in err &&
		"status" in err &&
		typeof (err as Record<string, unknown>).status === "number"
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors. (api.ts still exists — no routes migrated yet.)

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/lib/client.ts
git commit -m "feat(frontend): add client singleton with isApiError guard"
```

---

## Chunk 2: Migrate Simple Files

### Task 3: Migrate `login.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/login.tsx`

Current calls:
- `api.post("/auth/register", { username, password })` → `client.auth.Register({ username, password })`
- `api.post<{ token: string }>("/auth/login", { username, password })` → `client.auth.Login({ username, password })` (returns `{ token, user_id }`)
- `err instanceof ApiError ? err.message : ...` → `isApiError(err) ? err.message : ...`

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { ApiError, api } from "@/lib/api";
```
With:
```typescript
import { client, isApiError } from "@/lib/client";
```

- [ ] **Step 2: Replace register call**

Replace:
```typescript
await api.post("/auth/register", { username, password });
```
With:
```typescript
await client.auth.Register({ username, password });
```

- [ ] **Step 3: Replace login call**

Replace:
```typescript
const resp = await api.post<{ token: string }>("/auth/login", {
    username,
    password,
});
```
With:
```typescript
const resp = await client.auth.Login({ username, password });
```

- [ ] **Step 4: Replace error check**

Replace `err instanceof ApiError` with `isApiError(err)`.

- [ ] **Step 5: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/apps/web/src/routes/login.tsx
git commit -m "feat(frontend): migrate login.tsx to generated client"
```

---

### Task 4: Migrate `settings/general.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/general.tsx`

Current calls:
- `api.get<UserSettings>("/settings")` → `client.settings.GetSettings()`
- `api.put<UserSettings>("/settings", data)` → `client.settings.UpdateSettings(data)`

Type `UserSettings` now comes from `settings.UserSettings` in the generated client.

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { api, type UserSettings } from "@/lib/api";
```
With:
```typescript
import { client } from "@/lib/client";
import type { settings } from "@/lib/client.gen";
type UserSettings = settings.UserSettings;
```

> **Note:** `settings.UserSettings` in the generated client only contains `speed_test_url: string`. The hand-written interface had an extra `api_key?: string` field — that field is **not** present in the generated type. `settings/general.tsx` does not use `api_key` from `UserSettings`, so this is fine.

- [ ] **Step 2: Replace get call**

```typescript
// old
queryFn: () => api.get<UserSettings>("/settings"),
// new
queryFn: () => client.settings.GetSettings(),
```

- [ ] **Step 3: Replace put call**

```typescript
// old
api.put<UserSettings>("/settings", data),
// new
client.settings.UpdateSettings(data),
```

- [ ] **Step 4: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/settings/general.tsx
git commit -m "feat(frontend): migrate settings/general.tsx to generated client"
```

---

## Chunk 3: Migrate Subscription + Scheduler Files

### Task 5: Migrate `subscriptions/index.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/index.tsx`

Current calls:
- `api.get<{ subscriptions: Subscription[] }>("/subscriptions")` → `client.subscription.List()` (returns `{ subscriptions }`)
- `api.delete(\`/subscriptions/${id}\`)` → `client.subscription.Delete(id)`
- `api.put<Subscription>(\`/subscriptions/${id}\`, data)` → `client.subscription.Update(id, data)`
- `api.post<{ job_id: string }>(\`/check/${id}\`, opts)` → `client.checker.TriggerCheck(id, opts)`
- `api.post<Subscription>("/subscriptions", { name, url })` → `client.subscription.Create({ name, url })`

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { ApiError, api, type Subscription } from "@/lib/api";
```
With:
```typescript
import { client, isApiError } from "@/lib/client";
import type { subscription } from "@/lib/client.gen";
type Subscription = subscription.Subscription;
```

- [ ] **Step 2: Replace all api calls**

```typescript
// List
queryFn: () => client.subscription.List(),

// Delete
mutationFn: (id: string) => client.subscription.Delete(id),

// Update
mutationFn: ({ id, data }: ...) => client.subscription.Update(id, data),

// Trigger check
mutationFn: ({ id, opts }: ...) => client.checker.TriggerCheck(id, opts),

// Create
mutationFn: () => client.subscription.Create({ name, url }),
```

- [ ] **Step 3: Replace all `err instanceof ApiError`**

Replace with `isApiError(err)`.

- [ ] **Step 4: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/index.tsx
git commit -m "feat(frontend): migrate subscriptions/index.tsx to generated client"
```

---

### Task 6: Migrate `scheduler.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/scheduler.tsx`

Current calls:
- `api.get<{ jobs: ScheduledJob[] }>("/scheduler")` → `client.scheduler.List()` (returns `{ jobs }`)
- `api.get<{ subscriptions: Subscription[] }>("/subscriptions")` → `client.subscription.List()`
- `api.post("/scheduler", { subscription_id, cron_expr, options })` → `client.scheduler.Create({ subscription_id, cron_expr, options })`
- `api.delete(\`/scheduler/${id}\`)` → `client.scheduler.Delete(id)`
- `api.get<{ jobs: CheckJob[]; total: number }>(\`/check/${id}/jobs?limit=8\`)` → `client.checker.ListJobs(id, { Limit: 8, Offset: 0 })`

Note: `ListJobs` query params use PascalCase: `{ Limit: 8, Offset: 0 }`.

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { ApiError, api, type CheckJob, type ScheduledJob, type Subscription } from "@/lib/api";
```
With:
```typescript
import { client, isApiError } from "@/lib/client";
import type { checker, scheduler, subscription } from "@/lib/client.gen";
type CheckJob = checker.JobSummary;
type ScheduledJob = scheduler.ScheduledJob;
type Subscription = subscription.Subscription;
```

> **Note:** `checker.JobSummary.status` is typed as `string` in the generated client (not a union literal like `"queued" | "running" | "completed" | "failed"`). The `statusColor` function uses `CheckJob["status"]` as a parameter type — this widens to `string`, which still compiles. No action needed, but don't be surprised if TypeScript loses exhaustiveness checking on `status` comparisons.

- [ ] **Step 2: Replace api calls**

```typescript
// List scheduled jobs
queryFn: () => client.scheduler.List(),

// List subscriptions
queryFn: () => client.subscription.List(),

// Create scheduled job
mutationFn: (params) => client.scheduler.Create({
    subscription_id: params.subscription_id,
    cron_expr: params.cron_expr,
    options: { speed_test: params.speed_test, media_apps: params.media_apps },
}),

// Delete scheduled job
mutationFn: (id: string) => client.scheduler.Delete(id),

// List check jobs history
queryFn: () => client.checker.ListJobs(job.subscription_id, { Limit: 8, Offset: 0 }),
```

- [ ] **Step 3: Fix data access**

`client.scheduler.List()` returns `{ jobs: ScheduledJob[] }` — access as `data.jobs`.
`client.subscription.List()` returns `{ subscriptions }` — access as `data.subscriptions`.

- [ ] **Step 4: Replace `err instanceof ApiError`**

Replace with `isApiError(err)`.

- [ ] **Step 5: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/apps/web/src/routes/scheduler.tsx
git commit -m "feat(frontend): migrate scheduler.tsx to generated client"
```

---

## Chunk 4: Migrate Complex Files

### Task 7: Migrate `subscriptions/$id.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx`

Current calls:
- `api.get<{ jobs: CheckJob[]; total: number }>(\`/check/${id}/jobs?limit=10\`)` → `client.checker.ListJobs(id, { Limit: 10, Offset: 0 })`
- `api.get<{ job: CheckJob; results: NodeResult[] }>(\`/check/${id}/results${qs}\`)` → `client.checker.GetResults(id, { JobID: selectedJobId ?? "" })` — note PascalCase
- `api.get<{ subscriptions: Subscription[] }>("/subscriptions")` → `client.subscription.List()`
- `api.delete(\`/check/${jid}\`)` → `client.checker.CancelCheck(jid)`
- `api.get<{ logs: ExportLog[] }>(\`/export-logs/${subscriptionId}\`)` → `client.checker.GetExportLogs(subscriptionId)`
- `new EventSource(\`/api/check/${jobId}/progress\`)` → `new EventSource(\`${window.location.origin}/api/check/${jobId}/progress\`)`

- [ ] **Step 1: Update imports**

Replace:
```typescript
import {
    ApiError,
    api,
    type CheckJob,
    type ExportLog,
    type NodeResult,
    type Subscription,
} from "@/lib/api";
```
With:
```typescript
import { client, isApiError } from "@/lib/client";
import type { checker, subscription } from "@/lib/client.gen";
type CheckJob = checker.JobSummary;
type NodeResult = checker.NodeResult;
type ExportLog = checker.ExportLog;
type Subscription = subscription.Subscription;
```

- [ ] **Step 2: Replace ListJobs call**

```typescript
// old
api.get<{ jobs: CheckJob[]; total: number }>(`/check/${id}/jobs?limit=10`),
// new
client.checker.ListJobs(id, { Limit: 10, Offset: 0 }),
```

- [ ] **Step 3: Replace GetResults call**

```typescript
// old
const qs = selectedJobId ? `?job_id=${selectedJobId}` : "";
return api.get<{ job: CheckJob; results: NodeResult[] }>(`/check/${id}/results${qs}`);
// new
return client.checker.GetResults(id, { JobID: selectedJobId ?? "" });
```

Note: `GetResults` returns `{ job: checker.Job, results: checker.NodeResult[] }`. `job` type is `checker.Job` (not `JobSummary`), adjust variable usage if needed.

- [ ] **Step 4: Replace other api calls**

```typescript
// subscriptions list
queryFn: () => client.subscription.List(),

// cancel check
mutationFn: (jid: string) => client.checker.CancelCheck(jid),

// export logs
queryFn: () => client.checker.GetExportLogs(subscriptionId),
```

- [ ] **Step 5: Update SSE URL**

```typescript
// old
const es = new EventSource(`/api/check/${jobId}/progress`);
// new
const es = new EventSource(`${window.location.origin}/api/check/${jobId}/progress`);
```

- [ ] **Step 6: Replace `err instanceof ApiError`**

Replace with `isApiError(err)`.

- [ ] **Step 7: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 8: Commit**

```bash
git add "frontend/apps/web/src/routes/subscriptions/\$id.tsx"
git commit -m "feat(frontend): migrate subscriptions/\$id.tsx to generated client"
```

---

### Task 8: Migrate `settings/notify.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/notify.tsx`

Current calls:
- `api.get<{ channels: NotifyChannel[] }>("/notify/channels")` → `client.notify.ListChannels()` (returns `{ channels }`)
- `api.post("/notify/channels", { name, type, config })` → `client.notify.CreateChannel({ name, type, config })`
- `api.delete(\`/notify/channels/${id}\`)` → `client.notify.DeleteChannel(id)`
- `api.put(\`/notify/channels/${id}\`, data)` → `client.notify.UpdateChannel(id, data)`
- `api.post<{ ok: boolean; error?: string }>(\`/notify/channels/${id}/test\`)` → `client.notify.TestChannel(id)` (returns `{ ok, error? }`)

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { ApiError, api, type NotifyChannel } from "@/lib/api";
```
With:
```typescript
import { client, isApiError } from "@/lib/client";
import type { notify } from "@/lib/client.gen";
type NotifyChannel = notify.Channel;
```

- [ ] **Step 2: Replace all api calls**

```typescript
// List
queryFn: () => client.notify.ListChannels(),

// Create
mutationFn: () => client.notify.CreateChannel({ name, type, config }),

// Delete
mutationFn: (id: string) => client.notify.DeleteChannel(id),

// Update
mutationFn: ({ id, data }) => client.notify.UpdateChannel(id, data),

// Test
mutationFn: (id: string) => client.notify.TestChannel(id),
```

- [ ] **Step 3: Replace `err instanceof ApiError`**

Replace with `isApiError(err)`.

- [ ] **Step 4: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/settings/notify.tsx
git commit -m "feat(frontend): migrate settings/notify.tsx to generated client"
```

---

## Chunk 5: Migrate Dashboard + Node Table

### Task 9: Migrate `routes/index.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/index.tsx`

Current calls:
- `api.get<{ subscriptions: Subscription[] }>("/subscriptions")` → `client.subscription.List()`
- `api.get<{ api_key: string }>("/settings/api-key")` → `client.settings.GetAPIKey()` (returns `{ api_key }`)
- `api.post<{ api_key: string }>("/settings/api-key/regenerate")` → `client.settings.RegenerateAPIKey()`
- `api.get<LocalUnlockResult>("/network-unlock")` → `client.checker.GetLocalUnlock()`

- [ ] **Step 1: Update imports**

Replace:
```typescript
import { api, type LocalUnlockResult, type Subscription } from "@/lib/api";
```
With:
```typescript
import { client } from "@/lib/client";
import type { checker, subscription } from "@/lib/client.gen";
type LocalUnlockResult = checker.LocalUnlockResult;
type Subscription = subscription.Subscription;
```

- [ ] **Step 2: Replace all api calls**

```typescript
// subscriptions
queryFn: () => client.subscription.List(),

// api key
queryFn: () => client.settings.GetAPIKey(),

// regenerate api key
mutationFn: () => client.settings.RegenerateAPIKey(),

// local unlock
queryFn: () => client.checker.GetLocalUnlock(),
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/index.tsx
git commit -m "feat(frontend): migrate routes/index.tsx to generated client"
```

---

### Task 10: Update `node-table.tsx` type import

**Files:**
- Modify: `frontend/apps/web/src/components/node-table.tsx`

- [ ] **Step 1: Update import**

Replace:
```typescript
import type { NodeResult } from "@/lib/api";
```
With:
```typescript
import type { checker } from "@/lib/client.gen";
type NodeResult = checker.NodeResult;
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors. No remaining imports from `@/lib/api`.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/components/node-table.tsx
git commit -m "feat(frontend): migrate node-table.tsx to generated client types"
```

---

## Chunk 6: Cleanup + Final Verification

### Task 11: Delete `api.ts` and final checks

**Files:**
- Delete: `frontend/apps/web/src/lib/api.ts`

- [ ] **Step 1: Confirm no remaining imports from `@/lib/api`**

```bash
grep -r "@/lib/api" frontend/apps/web/src/
```

Expected: no output. If any files still import from `api`, fix them before continuing.

- [ ] **Step 2: Delete `api.ts`**

```bash
rm frontend/apps/web/src/lib/api.ts
```

- [ ] **Step 3: Full type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 4: Lint**

```bash
cd frontend && bun check
```

Expected: 0 errors, auto-fix applied.

- [ ] **Step 5: Build check**

```bash
cd frontend && bun build
```

Expected: build succeeds with no errors.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "feat(frontend): complete generated client migration, delete api.ts"
```
