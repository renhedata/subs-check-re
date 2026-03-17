# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`subs-check-re` is a proxy subscription node checker platform — a multi-user web app that tests proxy nodes for availability, latency, speed, and streaming unlock status (Netflix, YouTube, OpenAI, etc.). It rewrites [subs-check](https://github.com/beck-8/subs-check) as a full web platform.

**Backend:** Go + [Encore](https://encore.dev) framework (`services/`)
**Frontend:** React 19 + Vite, lives in `frontend/` (Turborepo monorepo managed with Bun)

## Development Commands

### Backend (Go / Encore)

```bash
encore run          # Start backend dev server (port 4000), auto-manages PostgreSQL via Docker
encore db migrate   # Run database migrations
encore db shell     # Open psql shell to local dev DB
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts  # Regenerate frontend API client
```

### Frontend (run from `frontend/`)

```bash
cd frontend
bun dev             # Start all frontend apps (web on port 3001)
bun dev:web         # Web app only
bun build           # Production build
bun check-types     # TypeScript type checking
bun check           # Lint + format with Biome (auto-fix)
```

### Both together

```bash
# Terminal 1
encore run
# Terminal 2
cd frontend && bun dev
```

## Architecture

### Backend — Encore Services (`services/`)

Each subdirectory is an independent Encore service (Go package). Services communicate via Encore's typed API calls or PubSub — never direct imports.

| Service | Responsibility |
|---------|----------------|
| `auth` | JWT auth (register/login), `encore.dev/beta/auth` middleware |
| `subscription` | CRUD for subscription URLs and node listings |
| `checker` | Core: fetches subscription, replaces nodes, runs mihomo checks, SSE progress |
| `scheduler` | Manages `robfig/cron` jobs; re-registers from DB on startup |
| `notify` | Consumes PubSub events from checker, sends webhook/telegram/email |

**Key dependency:** `github.com/metacubex/mihomo` is used as a Go library (not subprocess) for proxy protocol handling and platform unlock detection.

**Auth:** JWT with claims `{ sub: user_id, exp: now+24h }`. All non-`/auth/*` endpoints require it. Multi-tenancy is enforced by filtering on `user_id` in each service.

**Check job states:** `queued → running → completed | failed`
A subscription can only have one `running` job at a time (409 if attempted).

### Frontend — Turborepo Monorepo (`frontend/`)

```
frontend/
├── apps/web/          # React app (TanStack Router file-based routes in src/routes/)
├── packages/ui/       # Shared shadcn/ui components
├── packages/env/      # Zod-validated env schemas
└── packages/config/   # Shared tsconfig
```

**Data flow:** `TanStack Query → Encore generated client → /api/* → Vite proxy → Encore :4000`

The Vite dev server proxies `/api` to `http://localhost:4000`. Production: nginx routes `/api/` to backend container (same domain, no CORS needed).

**API client:** Uses Encore-generated type-safe client.
- Generated file: `src/lib/client.gen.ts` — **do not edit manually**, committed to git
- Singleton initializer: `src/lib/client.ts` — hand-written, handles auth token and baseURL
- BaseURL: `window.location.origin + '/api'` (works with both Vite proxy and nginx)
- Regenerate after backend API changes: `encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts`
- Type naming: response/entity fields are snake_case (JSON tags), query parameter structs use PascalCase Go field names (e.g., `{ JobID: jobId }`)

**SSE constraint:** `/check/:jobId/progress` is `public raw` — must stay public because `EventSource` cannot send auth headers. Do not add auth to this endpoint.

**Routing:** TanStack Router with file-based routing. `routeTree.gen.ts` is auto-generated — do not edit manually.

**Linting/formatting:** Biome (not ESLint/Prettier). Run `bun check` from `frontend/`. Tab indentation enforced.

**Adding UI components:** Use `bunx shadcn add <component>` from `frontend/packages/ui/` for shared components, or directly in `frontend/apps/web/src/components/` for app-specific ones.

### Database Schema (PostgreSQL, Encore-managed)

Core tables: `users`, `subscriptions`, `nodes`, `check_jobs`, `check_results`, `notify_channels`

- `nodes` are fully replaced (DELETE + INSERT in transaction) each time a check job starts
- `check_results` references `check_jobs.id` for grouping results by run
- `subscriptions.cron_expr` (nullable) drives scheduled checks; scheduler re-registers all on startup

### Real-time Progress (SSE)

`POST /check/:subscriptionId` → returns `{ job_id }` → client connects `GET /check/:jobId/progress` (SSE stream) → receives `{"progress":N,"total":M,"node_name":"..."}` → final `{"done":true,"available":N}`.

## Design Specs

Full architecture decisions and rationale: `docs/superpowers/specs/`

## Reference Implementation

`~/tmp/subs-check` (local path, adjust as needed) — the original project. Key files to reference:
- `check/check.go` — mihomo-based concurrent node checker
- `check/platform/` — per-platform unlock detection logic (Netflix, YouTube, OpenAI, etc.)
- `proxy/get.go` — subscription URL fetching and parsing
