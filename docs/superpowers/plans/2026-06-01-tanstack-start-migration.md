# TanStack Start Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Turborepo Bun monorepo + nginx with a single TanStack Start app whose Nitro server proxies `/api/*` to the Encore backend.

**Architecture:** Flatten `frontend/apps/web` + `packages/*` into a single `frontend/` package. Replace Vite SPA + nginx with TanStack Start (full, not static SPA mode) + Nitro — Nitro runs as a Node.js server that serves the app shell and proxies all `/api/*` requests to Encore at the URL in `ENCORE_URL`. No SSR data fetching is added; all pages remain client-side.

**Tech Stack:** TanStack Start (`@tanstack/react-start`), Nitro v3, TanStack Router + Query, React 19, Tailwind v4, shadcn/ui, Bun, Biome.

---

## File Map

**Create:**
- `frontend/src/router.tsx` — TanStack Start `createRouter()` (replaces inline router in main.tsx)
- `frontend/src/entry-client.tsx` — `StartClient` hydration entry (replaces main.tsx)
- `frontend/src/styles.css` — merged CSS (globals.css tailwind directives + index.css custom vars)
- `frontend/server/routes/api/[...path].ts` — Nitro catch-all proxy to Encore

**Rewrite:**
- `frontend/package.json` — single package, merged deps, no workspaces
- `frontend/tsconfig.json` — standalone (no workspace extends)
- `frontend/vite.config.ts` — TanStack Start + Nitro plugins
- `frontend/components.json` — updated paths for flat structure
- `frontend/src/routes/__root.tsx` — add `<html>/<body>/<Scripts>` + theme script
- `frontend/Dockerfile` — Node.js Nitro image (remove nginx stage)
- `docker-compose.yml` — remove nginx service, add `ENCORE_URL` to frontend

**Move (content unchanged):**
- `apps/web/src/components/**` → `src/components/`
- `apps/web/src/lib/**` → `src/lib/`
- `apps/web/src/queries/**` → `src/queries/`
- `apps/web/src/routes/**` → `src/routes/`
- `packages/ui/src/components/*.tsx` → `src/components/ui/`
- `packages/ui/src/lib/utils.ts` → `src/lib/utils.ts`
- `packages/env/src/web.ts` → `src/env/web.ts`

**Delete (after verification):**
- `frontend/apps/` — entire directory
- `frontend/packages/` — entire directory
- `frontend/turbo.json`
- `frontend/apps/web/src/main.tsx` — replaced by entry-client.tsx

---

## Task 1: Rewrite package.json

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Replace the workspace root package.json with a single-package manifest**

Replace the entire contents of `frontend/package.json` with:

```json
{
  "name": "web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "start": "node .output/server/index.mjs",
    "check-types": "tsc --noEmit",
    "check": "biome check --write .",
    "gen:client": "encore gen client subs-check-uqti --lang=typescript --output=./src/lib/client.gen.ts"
  },
  "dependencies": {
    "@base-ui/react": "^1.0.0",
    "@hookform/resolvers": "^5.1.1",
    "@iconify/react": "^6.0.2",
    "@monaco-editor/react": "^4.7.0",
    "@t3-oss/env-core": "^0.13.1",
    "@tanstack/react-query": "^5.90.21",
    "@tanstack/react-router": "^1.141.1",
    "@tanstack/react-start": "^1.141.1",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "cronstrue": "^3.14.0",
    "dotenv": "^17.2.2",
    "lucide-react": "^0.546.0",
    "next-themes": "^0.4.6",
    "react": "^19.2.3",
    "react-dom": "^19.2.3",
    "react-hook-form": "^7.71.2",
    "react-icons": "^5.6.0",
    "shadcn": "^3.6.2",
    "sonner": "^2.0.5",
    "tailwind-merge": "^3.3.1",
    "tw-animate-css": "^1.3.4",
    "zod": "^4.1.13"
  },
  "devDependencies": {
    "@biomejs/biome": "^2.2.0",
    "@tailwindcss/vite": "^4.1.18",
    "@tanstack/react-query-devtools": "^5.90.21",
    "@tanstack/react-router-devtools": "^1.141.1",
    "@types/node": "^22.13.14",
    "@types/react": "^19.2.10",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^4.3.4",
    "lefthook": "^2.0.13",
    "nitro": "3.0.260522-beta",
    "postcss": "^8.5.3",
    "tailwindcss": "^4.1.18",
    "typescript": "^5",
    "vite": "^6.2.2"
  },
  "packageManager": "bun@1.3.10"
}
```

> Note: `@tanstack/react-start` ships the Vite plugin at `@tanstack/react-start/plugin/vite`. `turbo` and all `@frontend/*` workspace deps are removed.

---

## Task 2: Create tsconfig.json and vite.config.ts

**Files:**
- Modify: `frontend/tsconfig.json`
- Modify: `frontend/apps/web/vite.config.ts` → will become `frontend/vite.config.ts` in Task 3

- [ ] **Step 1: Replace frontend/tsconfig.json with a standalone config**

Replace `frontend/tsconfig.json` (currently the workspace root tsconfig) with:

```json
{
  "compilerOptions": {
    "strict": true,
    "esModuleInterop": true,
    "jsx": "react-jsx",
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "verbatimModuleSyntax": true,
    "skipLibCheck": true,
    "types": ["vite/client"],
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src", "server"]
}
```

> This is the merged version of `apps/web/tsconfig.json` without the `@frontend/ui` path alias (we'll use `@/components/ui/` directly). `server` is included so the Nitro proxy file type-checks.

- [ ] **Step 2: Create frontend/vite.config.ts (at the root, not apps/web/)**

Create `frontend/vite.config.ts`:

```ts
import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { nitro } from "nitro/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [
		tailwindcss(),
		tanstackStart(),
		viteReact(),
		nitro(),
	],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
	server: {
		port: 3001,
	},
});
```

> No `server.proxy` here — proxying is handled by the Nitro server route in Task 6.

---

## Task 3: Move source files

All commands run from `frontend/`. The source files move but their content does not change yet — import fixes come in Task 5.

**Files:**
- Move: `apps/web/src/**` → `src/**`
- Move: `packages/ui/src/components/` → `src/components/ui/`
- Move: `packages/ui/src/lib/utils.ts` → `src/lib/utils.ts`
- Move: `packages/env/src/web.ts` → `src/env/web.ts`

- [ ] **Step 1: Move the web app source**

```bash
cd /path/to/project/frontend
cp -r apps/web/src/. src/
```

> This copies components/, lib/, queries/, routes/, routeTree.gen.ts, main.tsx, and index.css into `src/`.

- [ ] **Step 2: Move shared UI components**

```bash
cp apps/web/src/components/ui/* src/components/ui/ 2>/dev/null || true
cp packages/ui/src/components/*.tsx src/components/ui/
cp packages/ui/src/lib/utils.ts src/lib/utils.ts
```

> The shadcn components (button, input, label, etc.) move from `packages/ui/src/components/` to `src/components/ui/`.

- [ ] **Step 3: Move env package**

```bash
mkdir -p src/env
cp packages/env/src/web.ts src/env/web.ts
```

- [ ] **Step 4: Copy public assets if any exist**

```bash
cp -r apps/web/public/. public/ 2>/dev/null || true
```

---

## Task 4: Create merged styles.css

**Files:**
- Create: `frontend/src/styles.css`

The current `src/index.css` (copied in Task 3) imports `@frontend/ui/globals.css`. We merge both into one file and fix the `@source` path for Tailwind v4.

- [ ] **Step 1: Create src/styles.css with merged content**

Create `frontend/src/styles.css`:

```css
@import "tailwindcss";
@import "tw-animate-css";
@import "shadcn/tailwind.css";

@source "../**/*.{ts,tsx}";

@custom-variant dark (&:is(.dark *));

/* ── Light theme (GitHub-inspired) ─────────────────────────────────── */
:root {
	--background: #ffffff;
	--foreground: #1f2328;
	--card: #f6f8fa;
	--card-foreground: #1f2328;
	--popover: #ffffff;
	--popover-foreground: #1f2328;
	--primary: #0969da;
	--primary-foreground: #ffffff;
	--secondary: #eaeef2;
	--secondary-foreground: #1f2328;
	--muted: #eaeef2;
	--muted-foreground: #656d76;
	--accent: #eaeef2;
	--accent-foreground: #1f2328;
	--destructive: #d1242f;
	--border: #d0d7de;
	--input: #d0d7de;
	--ring: #0969da;
	--radius: 0.5rem;

	--color-dimmed: #6e7781;
	--color-code: #1f2328;
	--color-btn-success: #1f883d;
	--color-success: #1a7f37;
	--color-warning: #9a6700;
	--color-badge-info-bg: #ddf4ff;
	--color-badge-info: #0969da;
	--color-badge-success-bg: #dafbe1;
	--color-badge-success: #1a7f37;
	--color-badge-danger-bg: #ffd7d9;
	--color-badge-danger: #d1242f;
	--color-badge-ai-bg: #dafbe1;
	--color-badge-ai: #1a7f37;
	--color-active-bg: rgba(9, 105, 218, 0.1);
	--color-active-border: rgba(9, 105, 218, 0.4);
	--color-progress: linear-gradient(90deg, #0550ae, #0969da);
}

/* ── Dark theme (GitHub-inspired) ──────────────────────────────────── */
.dark {
	--background: #0d1117;
	--foreground: #f0f6fc;
	--card: #161b22;
	--card-foreground: #f0f6fc;
	--popover: #161b22;
	--popover-foreground: #f0f6fc;
	--primary: #58a6ff;
	--primary-foreground: #0d1117;
	--secondary: #21262d;
	--secondary-foreground: #f0f6fc;
	--muted: #21262d;
	--muted-foreground: #8b949e;
	--accent: #21262d;
	--accent-foreground: #f0f6fc;
	--destructive: #f85149;
	--border: #30363d;
	--input: #30363d;
	--ring: #58a6ff;
	--radius: 0.5rem;

	--color-dimmed: #6e7681;
	--color-code: #c9d1d9;
	--color-btn-success: #238636;
	--color-success: #3fb950;
	--color-warning: #d29922;
	--color-badge-info-bg: #1a2a3a;
	--color-badge-info: #58a6ff;
	--color-badge-success-bg: #1a4731;
	--color-badge-success: #3fb950;
	--color-badge-danger-bg: #3d1a1a;
	--color-badge-danger: #f85149;
	--color-badge-ai-bg: #1a3a1a;
	--color-badge-ai: #3fb950;
	--color-active-bg: rgba(31, 111, 235, 0.13);
	--color-active-border: rgba(31, 111, 235, 0.27);
	--color-progress: linear-gradient(90deg, #1f6feb, #58a6ff);
}

@layer base {
	html {
		color-scheme: light;
	}
	html.dark {
		color-scheme: dark;
	}
	body {
		background-color: var(--background);
		color: var(--foreground);
	}
	input,
	textarea,
	select {
		color: var(--foreground);
	}
	input[type="checkbox"],
	input[type="radio"] {
		accent-color: var(--primary);
	}
	input:-webkit-autofill,
	input:-webkit-autofill:hover,
	input:-webkit-autofill:focus {
		-webkit-text-fill-color: var(--foreground);
		-webkit-box-shadow: 0 0 0px 1000px var(--card) inset;
		transition: background-color 600000s 0s, color 600000s 0s;
	}
}
```

---

## Task 5: Rewrite TanStack Start entry points

**Files:**
- Create: `frontend/src/router.tsx`
- Create: `frontend/src/entry-client.tsx`
- Modify: `frontend/src/routes/__root.tsx`

- [ ] **Step 1: Create src/router.tsx**

Create `frontend/src/router.tsx`:

```tsx
import Loader from "@/components/loader";
import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";
import type { RouterAppContext } from "./routes/__root";

export function createRouter() {
	return createTanStackRouter({
		routeTree,
		defaultPreload: "intent",
		defaultPendingComponent: () => <Loader />,
		context: {} satisfies RouterAppContext,
	});
}

declare module "@tanstack/react-router" {
	interface Register {
		router: ReturnType<typeof createRouter>;
	}
}
```

- [ ] **Step 2: Create src/entry-client.tsx**

Create `frontend/src/entry-client.tsx`:

```tsx
import { StartClient } from "@tanstack/react-start/client";
import { StrictMode } from "react";
import { hydrateRoot } from "react-dom/client";
import { createRouter } from "./router";

const router = createRouter();

hydrateRoot(
	document,
	<StrictMode>
		<StartClient router={router} />
	</StrictMode>,
);
```

> This replaces `main.tsx`. TanStack Start's Vite plugin discovers `entry-client.tsx` automatically. The old `main.tsx` will be deleted in Task 9.

- [ ] **Step 3: Rewrite src/routes/__root.tsx**

Replace the entire contents of `frontend/src/routes/__root.tsx`:

```tsx
/// <reference types="vite/client" />
import { Toaster } from "@/components/ui/sonner";
import {
	MutationCache,
	QueryCache,
	QueryClient,
	QueryClientProvider,
} from "@tanstack/react-query";
import {
	HeadContent,
	Outlet,
	Scripts,
	createRootRouteWithContext,
	redirect,
	useRouterState,
} from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import type { ReactNode } from "react";

import { PlatformRulesProvider } from "@/components/platform-rules-context";
import { Sidebar } from "@/components/sidebar";
import { isAuthenticated } from "@/lib/auth";
import { handleUnauthorized, isApiError } from "@/lib/client";
import appCss from "../styles.css?url";

// biome-ignore lint/complexity/noBannedTypes: intentionally empty context for TanStack Router
export type RouterAppContext = {};

const queryClient = new QueryClient({
	queryCache: new QueryCache({
		onError: (err) => handleUnauthorized(err),
	}),
	mutationCache: new MutationCache({
		onError: (err) => handleUnauthorized(err),
	}),
	defaultOptions: {
		queries: {
			staleTime: 30_000,
			retry: (failureCount, err) => {
				if (isApiError(err) && err.status === 401) return false;
				return failureCount < 2;
			},
		},
	},
});

export const Route = createRootRouteWithContext<RouterAppContext>()({
	beforeLoad: ({ location }) => {
		const authed = isAuthenticated();
		const isLoginPage = location.pathname === "/login";
		if (!authed && !isLoginPage) {
			throw redirect({ to: "/login" });
		}
		if (authed && isLoginPage) {
			throw redirect({ to: "/" });
		}
	},
	head: () => ({
		meta: [
			{ charSet: "utf-8" },
			{ name: "viewport", content: "width=device-width, initial-scale=1" },
			{ title: "subs-check" },
			{ name: "description", content: "Proxy subscription checker" },
		],
		links: [
			{ rel: "stylesheet", href: appCss },
			{ rel: "icon", href: "/favicon.ico" },
		],
	}),
	component: RootComponent,
});

function RootDocument({ children }: { children: ReactNode }) {
	return (
		<html lang="en">
			<head>
				{/* Inline theme detection — must run before first paint */}
				<script
					// biome-ignore lint/security/noDangerouslySetInnerHtml: intentional inline script for theme flash prevention
					dangerouslySetInnerHTML={{
						__html: `(()=>{var s=localStorage.getItem("theme"),t=s==="light"||s==="dark"?s:window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.classList.toggle("dark",t==="dark")})()`,
					}}
				/>
				<HeadContent />
			</head>
			<body>
				{children}
				<Scripts />
			</body>
		</html>
	);
}

function RootComponent() {
	const { location } = useRouterState();
	const authed = isAuthenticated() && location.pathname !== "/login";

	return (
		<RootDocument>
			<QueryClientProvider client={queryClient}>
				{authed ? (
					<PlatformRulesProvider>
						<div className="flex h-screen overflow-hidden">
							<Sidebar />
							<main className="flex-1 overflow-y-auto px-6 py-6">
								<div className="mx-auto max-w-5xl">
									<Outlet />
								</div>
							</main>
						</div>
					</PlatformRulesProvider>
				) : (
					<div className="flex min-h-screen items-center justify-center">
						<Outlet />
					</div>
				)}
				<Toaster richColors />
				<TanStackRouterDevtools position="bottom-left" />
			</QueryClientProvider>
		</RootDocument>
	);
}
```

> Key changes from the old version: removed `import "../index.css"`, added `RootDocument` wrapper with `<html>/<head>/<body>`, inline theme script, `HeadContent`, `Scripts`, and CSS loaded via `appCss?url` in `head()`.

---

## Task 6: Fix all import paths

**Files:**
- Modify: all `.tsx`/`.ts` files under `frontend/src/`

Four substitutions cover all cross-package imports. Run from `frontend/src/`.

- [ ] **Step 1: Replace @frontend/ui/components/ → @/components/ui/**

```bash
find /path/to/project/frontend/src -type f \( -name "*.tsx" -o -name "*.ts" \) \
  -exec sed -i '' 's|@frontend/ui/components/|@/components/ui/|g' {} +
```

- [ ] **Step 2: Replace @frontend/ui/lib/ → @/lib/**

```bash
find /path/to/project/frontend/src -type f \( -name "*.tsx" -o -name "*.ts" \) \
  -exec sed -i '' 's|@frontend/ui/lib/|@/lib/|g' {} +
```

- [ ] **Step 3: Replace @frontend/ui/globals.css → (remove — now merged into styles.css)**

The old `src/index.css` starts with `@import "@frontend/ui/globals.css"`. This line is no longer needed since `styles.css` now contains the merged CSS. Delete `src/index.css`:

```bash
rm /path/to/project/frontend/src/index.css
```

- [ ] **Step 4: Replace @frontend/env/web → @/env/web**

```bash
find /path/to/project/frontend/src -type f \( -name "*.tsx" -o -name "*.ts" \) \
  -exec sed -i '' 's|@frontend/env/web|@/env/web|g' {} +
```

- [ ] **Step 5: Verify no @frontend references remain**

```bash
grep -r "@frontend/" /path/to/project/frontend/src
```

Expected: no output.

---

## Task 7: Create Nitro proxy handler

**Files:**
- Create: `frontend/server/routes/api/[...path].ts`

- [ ] **Step 1: Create the server directory and proxy handler**

```bash
mkdir -p /path/to/project/frontend/server/routes/api
```

Create `frontend/server/routes/api/[...path].ts`:

```ts
export default defineEventHandler((event) => {
	const base = process.env.ENCORE_URL ?? "http://localhost:4000";
	const target = base + event.path.replace(/^\/api/, "");
	return proxyRequest(event, target);
});
```

> `defineEventHandler` and `proxyRequest` are H3 utilities auto-imported by Nitro — no explicit imports needed. `event.path` is the full incoming path (e.g. `/api/subscriptions`). After stripping `/api`, the request forwards to `http://<ENCORE_URL>/subscriptions`. This matches the current nginx `proxy_pass` + `rewrite` behavior. The SSE endpoint (`/api/check/:id/progress`) streams correctly because `proxyRequest` does not buffer the response body.

---

## Task 8: Update components.json

**Files:**
- Modify: `frontend/apps/web/components.json` → move to `frontend/components.json`

- [ ] **Step 1: Move and update components.json**

```bash
cp /path/to/project/frontend/apps/web/components.json /path/to/project/frontend/components.json
```

Then edit `frontend/components.json` to:

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "base-nova",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/styles.css",
    "baseColor": "neutral",
    "cssVariables": true,
    "prefix": ""
  },
  "iconLibrary": "lucide",
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  },
  "menuColor": "default",
  "menuAccent": "subtle",
  "registries": {}
}
```

---

## Task 9: Install and verify dev server

**Files:** none (verification only)

- [ ] **Step 1: Install dependencies**

```bash
cd /path/to/project/frontend
bun install
```

Expected: packages install, no workspace-related errors.

- [ ] **Step 2: Start dev server**

```bash
bun dev
```

Expected: Nitro dev server starts on port 3001, output like:
```
  ➜  Local:   http://localhost:3001/
```
The TanStack Router plugin will auto-regenerate `src/routeTree.gen.ts` on first run.

- [ ] **Step 3: Open the app and verify login page loads**

Open `http://localhost:3001` in a browser. Expected: the login page renders (redirect from `/` since no auth).

- [ ] **Step 4: Verify API proxy (requires Encore running on :4000)**

With `encore run` running in another terminal, log in and confirm a page that fetches data (e.g. `/subscriptions`) loads without network errors. The proxy strips `/api` and forwards to `:4000`.

- [ ] **Step 5: Commit working state**

```bash
git add frontend/src frontend/server frontend/package.json frontend/tsconfig.json frontend/vite.config.ts frontend/components.json
git commit -m "feat(frontend): migrate to TanStack Start + Nitro, flatten monorepo"
```

---

## Task 10: Delete old monorepo directories

**Files:**
- Delete: `frontend/apps/`
- Delete: `frontend/packages/`
- Delete: `frontend/turbo.json`

Only do this after Task 9 passes.

- [ ] **Step 1: Remove old directories**

```bash
rm -rf /path/to/project/frontend/apps
rm -rf /path/to/project/frontend/packages
rm -f /path/to/project/frontend/turbo.json
```

- [ ] **Step 2: Verify dev server still works**

```bash
cd /path/to/project/frontend
bun dev
```

Expected: starts cleanly, no import errors from the deleted packages.

- [ ] **Step 3: Commit cleanup**

```bash
git add -A frontend/
git commit -m "chore(frontend): remove Turborepo monorepo artifacts"
```

---

## Task 11: Update Dockerfile

**Files:**
- Modify: `frontend/Dockerfile`

- [ ] **Step 1: Replace the Dockerfile**

Replace the entire contents of `frontend/Dockerfile`:

```dockerfile
FROM oven/bun:1.3.10-alpine AS builder

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

> `bun run build` runs `vite build` which produces `.output/` (Nitro's output directory). The production stage runs the Nitro Node.js server — no nginx.

- [ ] **Step 2: Verify the Docker build locally**

```bash
cd /path/to/project/frontend
docker build -t subs-check-frontend:test .
```

Expected: build completes, image ~200MB.

- [ ] **Step 3: Smoke-test the container (optional — requires Encore running)**

```bash
docker run -e ENCORE_URL=http://host.docker.internal:4000 -p 3001:3000 subs-check-frontend:test
```

Open `http://localhost:3001` — login page should load.

---

## Task 12: Update docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Remove nginx service and update frontend service**

Replace the `frontend` and `nginx` sections in `docker-compose.yml`. The complete new services block (keeping `migrator`, `nsq`, `backend` unchanged):

```yaml
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    environment:
      ENCORE_URL: http://backend:8080
    ports:
      - "18080:3000"
    depends_on:
      - backend
    restart: unless-stopped
```

Delete the entire `nginx:` service block.

> The `18080:3000` mapping replaces `nginx`'s `18080:80`. `ENCORE_URL=http://backend:8080` tells Nitro where to forward `/api/*` — `backend` is the Docker Compose service name, port 8080 is Encore's production port.

- [ ] **Step 2: Verify docker-compose config is valid**

```bash
docker compose config
```

Expected: prints resolved config with no errors, no `nginx` service present.

- [ ] **Step 3: Commit**

```bash
git add frontend/Dockerfile docker-compose.yml
git commit -m "feat(deploy): replace nginx with Nitro server, wire ENCORE_URL"
```

---

## Task 13: Final production build check

- [ ] **Step 1: Run full stack with docker compose**

```bash
docker compose up --build frontend
```

Expected: frontend container starts, logs show Nitro server listening on port 3000.

- [ ] **Step 2: Verify proxy in production container**

With backend running, open `http://localhost:18080`. The login page should load and API calls should succeed through the Nitro proxy.

- [ ] **Step 3: Delete deploy/nginx.conf**

```bash
rm /path/to/project/deploy/nginx.conf
```

- [ ] **Step 4: Final commit**

```bash
git add deploy/nginx.conf
git commit -m "chore: remove nginx.conf (replaced by Nitro proxy)"
```
