# TanStack Start Migration Design

**Date:** 2026-06-01
**Status:** Approved

## Goal

Replace the Turborepo Bun monorepo + nginx deployment with a single TanStack Start app running in SPA mode. Nitro (TanStack Start's server runtime) replaces nginx: it serves the static SPA shell and proxies `/api/*` to the Encore backend at runtime.

## Constraints

- SPA mode (CSR only) — no SSR, no auth changes
- Keep Bun as package manager
- Keep Biome for linting/formatting
- Keep the Encore-generated TypeScript client and all existing routes unchanged

## Architecture After Migration

```
frontend/                        ← single package, was Turborepo workspace root
├── src/
│   ├── components/
│   │   ├── ui/                  ← moved from packages/ui/src/components/
│   │   ├── cron-picker.tsx
│   │   ├── sidebar.tsx
│   │   └── ...                  ← all existing app-specific components
│   ├── env/
│   │   └── web.ts               ← moved from packages/env/src/web.ts
│   ├── lib/
│   │   ├── client.gen.ts        ← unchanged (Encore generated)
│   │   ├── client.ts            ← unchanged
│   │   ├── auth.ts
│   │   ├── format.ts
│   │   └── theme.ts
│   ├── queries/                 ← unchanged
│   ├── routes/                  ← unchanged (file-based TanStack Router)
│   │   ├── __root.tsx           ← updated for TanStack Start root
│   │   └── ...
│   ├── router.tsx               ← new: createRouter for TanStack Start
│   ├── main.tsx                 ← updated entry point
│   └── styles.css               ← merged from index.css + globals.css
├── server/
│   └── routes/
│       └── api/
│           └── [...path].ts     ← Nitro catch-all proxy to Encore
├── public/                      ← static assets
├── package.json                 ← merged deps from all packages
├── tsconfig.json
├── vite.config.ts               ← TanStack Start + Nitro plugins
├── components.json              ← shadcn config
└── Dockerfile
```

## Key Changes Per Layer

### 1. Monorepo → Single Package

Remove: `turbo.json`, `packages/ui/`, `packages/env/`, `packages/config/`, workspace `package.json`.

Merge into one `package.json`:
- All deps from `apps/web/package.json`, `packages/ui/package.json`, `packages/env/package.json`
- Add `@tanstack/react-start` and `nitro` (replaces `@tanstack/router-plugin`)
- Remove workspace protocol deps (`@frontend/ui`, `@frontend/env`, `@frontend/config`)

Import path rewrites (automated via find+replace):
- `@frontend/ui/components/` → `@/components/ui/`
- `@frontend/ui/lib/` → `@/lib/`
- `@frontend/env/web` → `@/env/web`

### 2. TanStack Start (SPA Mode)

**`vite.config.ts`** replaces the current Vite SPA config:

```ts
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import { nitro } from 'nitro/vite'
import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    tanstackStart({ ssr: false }),
    nitro({ /* proxy config — see section 3 */ }),
    viteReact(),
    tailwindcss(),
  ],
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
})
```

No separate Vite `server.proxy` needed — Nitro handles proxying in both dev and prod.

**`src/__root.tsx`** updated to TanStack Start format: uses `createRootRoute` from `@tanstack/react-start/router`, wraps with `<html>`/`<body>` and includes `<Scripts />` and `<ScrollRestoration />` from TanStack Start.

**`src/router.tsx`** new file: exports `createRouter()` configured for CSR (no SSR).

**`src/main.tsx`** updated entry: uses `StartClient` from `@tanstack/react-start/client` instead of `RouterProvider` directly.

### 3. Nitro Proxy (Replaces nginx)

A Nitro catch-all server route handles all `/api/*` traffic:

**`server/routes/api/[...path].ts`**:
```ts
export default defineEventHandler((event) => {
  const base = process.env.ENCORE_URL ?? 'http://localhost:4000'
  const target = base + event.path.replace(/^\/api/, '')
  return proxyRequest(event, target)
})
```

- Strips the `/api` prefix before forwarding (matching current nginx behavior)
- Reads `ENCORE_URL` at request time — runtime-configurable, no rebuild needed
- `proxyRequest` from H3 streams the response, so the SSE progress endpoint (`/api/check/:id/progress`) works correctly without extra configuration

Dev default: `ENCORE_URL=http://localhost:4000`
Docker prod: `ENCORE_URL=http://backend:8080`

### 4. Docker

**`frontend/Dockerfile`** (new):
```dockerfile
FROM oven/bun:1.3-alpine AS builder
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/.output ./.output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
```

**`docker-compose.yml`** changes:
- Remove `nginx` service entirely
- `frontend` service gains:
  - `environment: ENCORE_URL: http://backend:8080`
  - `ports: - "18080:3000"` (moved from nginx)
  - `depends_on: - backend`
- `frontend.build.context` points to `./frontend` (same path, now a single package)

## What Does NOT Change

- All route files (`src/routes/**`) — logic is identical
- The Encore-generated client (`client.gen.ts`, `client.ts`) — unchanged
- Auth flow (JWT in localStorage, injected as Authorization header)
- The `@/` path alias convention
- Biome config, lefthook
- All query files, component files, lib utilities

## Out of Scope

- SSR / server functions / server loaders
- Better Auth (auth stays in Go/Encore)
- Any backend changes
