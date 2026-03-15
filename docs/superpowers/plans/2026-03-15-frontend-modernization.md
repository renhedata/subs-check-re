# Frontend Modernization Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the entire web app with a GitHub-style dark theme, replace the top-nav header with a 220px fixed sidebar, and add skeleton loading states + spinner buttons.

**Architecture:** Override CSS custom properties in `index.css` to GitHub's dark palette, always force `dark` class on `<html>`. Build a new `sidebar.tsx` component that replaces `header.tsx`. Rewrite each page component for visual consistency — no backend changes.

**Tech Stack:** React 19, TanStack Router (Link activeProps), shadcn/ui (Skeleton, Select), Lucide icons, Tailwind v4 (CSS custom property overrides), Biome

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `frontend/apps/web/index.html` | Add `class="dark"` to `<html>` |
| Modify | `frontend/apps/web/src/index.css` | GitHub palette CSS var overrides |
| Create | `frontend/apps/web/src/components/sidebar.tsx` | Sidebar nav with Lucide icons + logout |
| Modify | `frontend/apps/web/src/routes/__root.tsx` | Conditional layout: sidebar+content or bare outlet |
| Delete | `frontend/apps/web/src/components/header.tsx` | Replaced by sidebar |
| Delete | `frontend/apps/web/src/components/mode-toggle.tsx` | Always dark now |
| Delete | `frontend/apps/web/src/components/theme-provider.tsx` | Always dark now |
| Modify | `frontend/apps/web/src/routes/index.tsx` | Dashboard stat cards redesign |
| Modify | `frontend/apps/web/src/routes/subscriptions/index.tsx` | Subscription list redesign + skeleton |
| Modify | `frontend/apps/web/src/components/node-table.tsx` | New badge styles + latency color coding |
| Modify | `frontend/apps/web/src/routes/subscriptions/$id.tsx` | Gradient progress bar + job summary |
| Run cmd | `frontend/packages/ui/` | `bunx shadcn add select` |
| Modify | `frontend/apps/web/src/routes/scheduler.tsx` | shadcn Select + show sub names |
| Modify | `frontend/apps/web/src/routes/login.tsx` | Lock icon + link toggle style |
| Modify | `frontend/apps/web/src/routes/settings/general.tsx` | Consistent card styling |
| Modify | `frontend/apps/web/src/routes/settings/notify.tsx` | Consistent card styling + shadcn Select |

---

## Chunk 1: CSS Foundation + Always-Dark

### Task 1: Force dark mode via HTML class

**Files:**
- Modify: `frontend/apps/web/index.html`

- [ ] **Step 1: Add `class="dark"` to `<html>` tag**

Replace the opening `<html lang="en">` with:

```html
<html lang="en" class="dark">
```

- [ ] **Step 2: Verify dev server still starts**

```bash
cd frontend && bun dev:web
```

Expected: Vite starts, no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/index.html
git commit -m "feat(frontend): force dark mode via html class"
```

---

### Task 2: Override CSS variables with GitHub palette

**Files:**
- Modify: `frontend/apps/web/src/index.css`

- [ ] **Step 1: Replace `index.css` contents**

The current file only imports globals. Override the dark CSS variables to GitHub's exact palette:

```css
@import "@frontend/ui/globals.css";

/* GitHub dark palette — overrides the shared dark vars */
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
}

@layer base {
	html {
		color-scheme: dark;
	}
	body {
		background-color: #0d1117;
	}
}
```

- [ ] **Step 2: Start dev server and visually confirm background color**

```bash
cd frontend && bun dev:web
```

Open http://localhost:3001 in a browser. Expected: page background is `#0d1117` (near-black), not white.

- [ ] **Step 3: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/index.css
git commit -m "feat(frontend): apply GitHub dark palette via CSS var overrides"
```

---

## Chunk 2: Sidebar + Layout Shell

### Task 3: Create sidebar component

**Files:**
- Create: `frontend/apps/web/src/components/sidebar.tsx`

- [ ] **Step 1: Write sidebar component**

```tsx
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import {
	Bell,
	Clock,
	LayoutDashboard,
	List,
	LogOut,
	Settings,
	User,
} from "lucide-react";
import { clearToken } from "@/lib/auth";

const NAV_ITEMS = [
	{ to: "/", label: "Dashboard", icon: LayoutDashboard, exact: true },
	{ to: "/subscriptions", label: "Subscriptions", icon: List, exact: false },
	{ to: "/scheduler", label: "Scheduler", icon: Clock, exact: true },
	{ to: "/settings/notify", label: "Notify", icon: Bell, exact: false },
] as const;

const BOTTOM_ITEMS = [
	{ to: "/settings/general", label: "Settings", icon: Settings, exact: false },
] as const;

function NavItem({
	to,
	label,
	icon: Icon,
	exact,
}: {
	to: string;
	label: string;
	icon: React.ElementType;
	exact: boolean;
}) {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	const isActive = exact ? pathname === to : pathname.startsWith(to);

	return (
		<Link
			to={to}
			className={[
				"flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
				isActive
					? "border text-[#f0f6fc] font-medium"
					: "text-[#8b949e] hover:bg-white/5 hover:text-[#e6edf3] border border-transparent",
			].join(" ")}
			style={
				isActive
					? {
							background: "rgba(31,111,235,0.13)",
							borderColor: "rgba(31,111,235,0.27)",
						}
					: undefined
			}
		>
			<Icon size={14} strokeWidth={1.5} />
			{label}
		</Link>
	);
}

export function Sidebar() {
	const navigate = useNavigate();

	function logout() {
		clearToken();
		navigate({ to: "/login" });
	}

	return (
		<aside
			className="flex h-screen w-[220px] flex-shrink-0 flex-col border-r"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			{/* Logo */}
			<div
				className="flex items-center gap-2.5 border-b px-4 py-3"
				style={{ borderColor: "#30363d" }}
			>
				<div
					className="flex h-6 w-6 items-center justify-center rounded-md text-xs font-bold"
					style={{ background: "#58a6ff", color: "#0d1117" }}
				>
					S
				</div>
				<span className="text-sm font-semibold text-[#f0f6fc]">subs-check</span>
			</div>

			{/* Primary nav */}
			<nav className="flex flex-col gap-0.5 p-2 flex-1">
				{NAV_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}
			</nav>

			{/* Bottom nav */}
			<div className="flex flex-col gap-0.5 border-t p-2" style={{ borderColor: "#30363d" }}>
				{BOTTOM_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} />
				))}
				<button
					type="button"
					onClick={logout}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-sm text-[#8b949e] hover:bg-white/5 hover:text-[#e6edf3] transition-colors"
				>
					<div
						className="flex h-5 w-5 items-center justify-center rounded-full"
						style={{ background: "#30363d" }}
					>
						<User size={10} strokeWidth={1.5} className="text-[#8b949e]" />
					</div>
					{/* TODO: replace "admin" with actual username once user-profile API is available */}
					<span className="flex-1 text-left">admin</span>
					<LogOut size={12} strokeWidth={1.5} />
				</button>
			</div>
		</aside>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

> **Note:** Do NOT commit `sidebar.tsx` in isolation — it will have an unused import error until `__root.tsx` references it. Proceed immediately to Task 4 and commit both files together in Task 4, Step 5.

---

### Task 4: Rewrite layout shell, delete old components

**Files:**
- Modify: `frontend/apps/web/src/routes/__root.tsx`
- Delete: `frontend/apps/web/src/components/header.tsx`
- Delete: `frontend/apps/web/src/components/mode-toggle.tsx`
- Delete: `frontend/apps/web/src/components/theme-provider.tsx`

- [ ] **Step 1: Rewrite `__root.tsx`**

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@frontend/ui/components/sonner";
import { HeadContent, Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";

import { Sidebar } from "@/components/sidebar";
import { isAuthenticated } from "@/lib/auth";

import "../index.css";

export interface RouterAppContext {}

const queryClient = new QueryClient({
	defaultOptions: {
		queries: { staleTime: 30_000, retry: 2 },
	},
});

export const Route = createRootRouteWithContext<RouterAppContext>()({
	component: RootComponent,
	head: () => ({
		meta: [
			{ title: "subs-check" },
			{ name: "description", content: "Proxy subscription checker" },
		],
		links: [{ rel: "icon", href: "/favicon.ico" }],
	}),
});

function RootComponent() {
	const authed = isAuthenticated();

	return (
		<QueryClientProvider client={queryClient}>
			<HeadContent />
			{authed ? (
				<div className="flex h-screen overflow-hidden">
					<Sidebar />
					<main className="flex-1 overflow-y-auto px-6 py-6">
						<div className="mx-auto max-w-5xl">
							<Outlet />
						</div>
					</main>
				</div>
			) : (
				<div className="flex min-h-screen items-center justify-center">
					<Outlet />
				</div>
			)}
			<Toaster richColors />
			<TanStackRouterDevtools position="bottom-left" />
		</QueryClientProvider>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors. (If errors reference deleted files, they disappear once `__root.tsx` no longer imports them.)

- [ ] **Step 3: Lint**

```bash
cd frontend && bun check
```

Expected: no lint errors (Biome auto-fixes where possible).

- [ ] **Step 3b: Verify `loader.tsx` has no references to deleted files**

```bash
grep -n "theme-provider\|mode-toggle\|header" frontend/apps/web/src/components/loader.tsx
```

Expected: no output. If matches found, remove the imports.

- [ ] **Step 4: Visual smoke-test**

```bash
cd frontend && bun dev:web
```

Open http://localhost:3001. Expected: sidebar renders on the left (220px, dark `#161b22` background), main content area on the right. Navigate to `/login` — sidebar should NOT appear (unauthenticated layout).

- [ ] **Step 5: Commit** (stages sidebar, root, and deletes old components in one atomic commit)

```bash
git add frontend/apps/web/src/components/sidebar.tsx \
        frontend/apps/web/src/routes/__root.tsx
git rm frontend/apps/web/src/components/header.tsx \
      frontend/apps/web/src/components/mode-toggle.tsx \
      frontend/apps/web/src/components/theme-provider.tsx
git commit -m "feat(frontend): sidebar nav, conditional layout shell, remove header"
```

---

## Chunk 3: Dashboard + Subscription List

### Task 5: Rewrite Dashboard page

**Files:**
- Modify: `frontend/apps/web/src/routes/index.tsx`

- [ ] **Step 1: Rewrite `index.tsx`**

```tsx
import { createFileRoute, redirect } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle, Clock, FileText } from "lucide-react";
import { Skeleton } from "@frontend/ui/components/skeleton";

import { api, type Subscription } from "@/lib/api";
import { isAuthenticated } from "@/lib/auth";

export const Route = createFileRoute("/")({
	beforeLoad: () => {
		if (!isAuthenticated()) throw redirect({ to: "/login" });
	},
	component: DashboardPage,
});

function StatCard({
	label,
	value,
	icon: Icon,
	valueColor,
	sub,
	loading,
}: {
	label: string;
	value: number;
	icon: React.ElementType;
	valueColor?: string;
	sub?: string;
	loading: boolean;
}) {
	return (
		<div
			className="rounded-lg border p-4"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			<div className="mb-2 flex items-center gap-1.5">
				<Icon size={13} strokeWidth={1.5} className="text-[#8b949e]" />
				<span
					className="text-[11px] font-medium uppercase tracking-[0.4px]"
					style={{ color: "#8b949e" }}
				>
					{label}
				</span>
			</div>
			{loading ? (
				<Skeleton className="h-8 w-12" />
			) : (
				<p
					className="text-[28px] font-bold leading-none"
					style={{ color: valueColor ?? "#f0f6fc" }}
				>
					{value}
				</p>
			)}
			{sub && !loading && (
				<p className="mt-1 text-[11px]" style={{ color: "#8b949e" }}>
					{sub}
				</p>
			)}
		</div>
	);
}

function DashboardPage() {
	const { data, isLoading } = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
	});

	const subs = data?.subscriptions ?? [];
	const enabled = subs.filter((s) => s.enabled).length;
	const scheduled = subs.filter((s) => s.cron_expr).length;

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-lg font-semibold text-[#f0f6fc]">Dashboard</h1>
				<p className="mt-0.5 text-sm" style={{ color: "#8b949e" }}>
					Overview of your proxy subscriptions
				</p>
			</div>

			<div className="grid gap-3 sm:grid-cols-3">
				<StatCard
					label="Subscriptions"
					icon={FileText}
					value={subs.length}
					loading={isLoading}
				/>
				<StatCard
					label="Active"
					icon={CheckCircle}
					value={enabled}
					valueColor="#3fb950"
					sub={`of ${subs.length} total`}
					loading={isLoading}
				/>
				<StatCard
					label="Scheduled"
					icon={Clock}
					value={scheduled}
					sub="cron jobs"
					loading={isLoading}
				/>
			</div>
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/index.tsx
git commit -m "feat(frontend): dashboard stat cards with GitHub styling and skeletons"
```

---

### Task 6: Rewrite Subscriptions list page

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/index.tsx`

- [ ] **Step 1: Rewrite `subscriptions/index.tsx`**

```tsx
import { useState } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Clock, Loader2, Play, Plus, Trash2 } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Skeleton } from "@frontend/ui/components/skeleton";

import { api, ApiError, type Subscription } from "@/lib/api";

export const Route = createFileRoute("/subscriptions/")({
	component: SubscriptionsPage,
});

function SubscriptionsPage() {
	const navigate = useNavigate();
	const qc = useQueryClient();
	const [name, setName] = useState("");
	const [url, setUrl] = useState("");
	const [adding, setAdding] = useState(false);

	const { data, isLoading } = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => api.delete(`/subscriptions/${id}`),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["subscriptions"] });
			toast.success("Deleted");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Delete failed"),
	});

	const triggerMut = useMutation({
		mutationFn: (id: string) => api.post<{ job_id: string }>(`/check/${id}`),
		onSuccess: (resp, id) => {
			toast.success("Check started");
			navigate({ to: "/subscriptions/$id", params: { id }, search: { job: resp.job_id } });
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed to start check"),
	});

	const createMut = useMutation({
		mutationFn: () => api.post<Subscription>("/subscriptions", { name, url }),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["subscriptions"] });
			setName("");
			setUrl("");
			setAdding(false);
			toast.success("Subscription added");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed to add"),
	});

	const subs = data?.subscriptions ?? [];

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="text-lg font-semibold text-[#f0f6fc]">Subscriptions</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add
				</button>
			</div>

			{adding && (
				<div
					className="rounded-lg border p-4 space-y-3"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Name (optional)</Label>
						<Input
							placeholder="My Sub"
							value={name}
							onChange={(e) => setName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Subscription URL</Label>
						<Input
							placeholder="https://..."
							value={url}
							onChange={(e) => setUrl(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="flex gap-2">
						<Button
							size="sm"
							onClick={() => createMut.mutate()}
							disabled={!url || createMut.isPending}
							style={{ background: "#238636", color: "#fff" }}
							className="border-0"
						>
							{createMut.isPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Add"
							)}
						</Button>
						<Button size="sm" variant="outline" onClick={() => setAdding(false)}>
							Cancel
						</Button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{isLoading
					? Array.from({ length: 3 }).map((_, i) => (
							<div
								key={i}
								className="rounded-lg border p-4"
								style={{ background: "#161b22", borderColor: "#30363d" }}
							>
								<Skeleton className="h-4 w-48 mb-2" />
								<Skeleton className="h-3 w-72" />
							</div>
						))
					: subs.map((sub) => <SubRow key={sub.id} sub={sub} deleteMut={deleteMut} triggerMut={triggerMut} />)}

				{!isLoading && subs.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No subscriptions yet. Add one above.
					</p>
				)}
			</div>
		</div>
	);
}

function SubRow({
	sub,
	deleteMut,
	triggerMut,
}: {
	sub: Subscription;
	deleteMut: { mutate: (id: string) => void; isPending: boolean };
	triggerMut: { mutate: (id: string) => void; isPending: boolean };
}) {
	return (
		<div
			className="flex items-center gap-3 rounded-lg border px-4 py-3"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			{/* Status dot */}
			<div
				className="h-2 w-2 flex-shrink-0 rounded-full"
				style={{ background: sub.last_run_at ? "#3fb950" : "#30363d" }}
			/>

			{/* Info */}
			<div className="flex-1 min-w-0">
				<Link
					to="/subscriptions/$id"
					params={{ id: sub.id }}
					className="text-sm font-medium hover:underline"
					style={{ color: "#58a6ff" }}
				>
					{sub.name || sub.url}
				</Link>
				{sub.name && (
					<p
						className="mt-0.5 truncate font-mono text-xs"
						style={{ color: "#8b949e" }}
					>
						{sub.url}
					</p>
				)}
				{sub.cron_expr && (
					<p className="mt-0.5 flex items-center gap-1 text-xs" style={{ color: "#6e7681" }}>
						<Clock size={10} strokeWidth={1.5} />
						{sub.cron_expr}
					</p>
				)}
			</div>

			{/* Actions */}
			<div className="flex items-center gap-2 flex-shrink-0">
				<button
					type="button"
					onClick={() => triggerMut.mutate(sub.id)}
					disabled={triggerMut.isPending}
					className="flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs transition-colors hover:bg-white/5 disabled:opacity-50"
					style={{ borderColor: "#30363d", color: "#8b949e" }}
				>
					{triggerMut.isPending ? (
						<Loader2 size={11} className="animate-spin" />
					) : (
						<Play size={11} strokeWidth={1.5} />
					)}
					Check
				</button>
				<button
					type="button"
					onClick={() => deleteMut.mutate(sub.id)}
					disabled={deleteMut.isPending}
					className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
					style={{ color: "#6e7681" }}
				>
					<Trash2 size={13} strokeWidth={1.5} />
				</button>
			</div>
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/index.tsx
git commit -m "feat(frontend): subscriptions list GitHub styling + skeleton loading"
```

---

## Chunk 4: Node Table + Subscription Detail

### Task 7: Rewrite NodeTable component

**Files:**
- Modify: `frontend/apps/web/src/components/node-table.tsx`

- [ ] **Step 1: Rewrite `node-table.tsx`**

```tsx
import type { NodeResult } from "@/lib/api";

interface Props {
	results: NodeResult[];
}

function latencyColor(ms: number): string {
	if (ms < 50) return "#3fb950";
	if (ms <= 200) return "#d29922";
	return "#f85149";
}

function UnlockBadge({ label, style }: { label: string; style: "media" | "ai" | "other" }) {
	const styles = {
		media: { background: "#3d1a1a", color: "#f85149" },
		ai: { background: "#1a3a1a", color: "#3fb950" },
		other: { background: "#1a2a3a", color: "#58a6ff" },
	};
	return (
		<span
			className="rounded px-1.5 py-0.5 text-[10px] font-semibold"
			style={styles[style]}
		>
			{label}
		</span>
	);
}

function StatusBadge({ alive }: { alive: boolean }) {
	return alive ? (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
			style={{ background: "#1a4731", color: "#3fb950" }}
		>
			<span className="h-1.5 w-1.5 rounded-full" style={{ background: "#3fb950" }} />
			alive
		</span>
	) : (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
			style={{ background: "#3d1a1a", color: "#f85149" }}
		>
			<span className="h-1.5 w-1.5 rounded-full" style={{ background: "#f85149" }} />
			dead
		</span>
	);
}

export function NodeTable({ results }: Props) {
	const alive = results.filter((r) => r.alive);
	const dead = results.filter((r) => !r.alive);
	const sorted = [...alive, ...dead];

	if (sorted.length === 0) {
		return (
			<p className="text-sm" style={{ color: "#8b949e" }}>
				No results yet.
			</p>
		);
	}

	return (
		<div className="overflow-x-auto rounded-lg border" style={{ borderColor: "#30363d" }}>
			<table className="w-full border-collapse">
				<thead>
					<tr style={{ borderBottom: "1px solid #30363d" }}>
						{["Node", "Status", "Latency", "Speed", "Country", "Unlocks"].map((h) => (
							<th
								key={h}
								className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.4px]"
								style={{ color: "#8b949e" }}
							>
								{h}
							</th>
						))}
					</tr>
				</thead>
				<tbody>
					{sorted.map((r) => (
						<tr
							key={r.node_id}
							className="transition-colors hover:bg-white/[0.02]"
							style={{ borderBottom: "1px solid #21262d" }}
						>
							<td
								className="max-w-[180px] truncate px-3 py-2 font-mono text-[11px]"
								style={{ color: r.alive ? "#f0f6fc" : "#6e7681" }}
							>
								{r.node_name}
							</td>
							<td className="px-3 py-2">
								<StatusBadge alive={r.alive} />
							</td>
							<td className="px-3 py-2 text-xs font-medium">
								{r.alive ? (
									<span style={{ color: latencyColor(r.latency_ms) }}>
										{r.latency_ms}ms
									</span>
								) : (
									<span style={{ color: "#6e7681" }}>—</span>
								)}
							</td>
							<td className="px-3 py-2 text-xs">
								{r.alive && r.speed_kbps ? (
									<span style={{ color: "#58a6ff" }}>
										{r.speed_kbps >= 1024
											? `${(r.speed_kbps / 1024).toFixed(1)} MB/s`
											: `${r.speed_kbps} KB/s`}
									</span>
								) : (
									<span style={{ color: "#6e7681" }}>—</span>
								)}
							</td>
							<td
								className="px-3 py-2 text-xs"
								style={{ color: r.alive ? "#f0f6fc" : "#6e7681" }}
							>
								{r.country || "—"}
							</td>
							<td className="px-3 py-2">
								<div className="flex flex-wrap gap-1">
									{/* netflix/openai/claude/gemini/disney are boolean; youtube/tiktok are string (unlock region or empty) */}
									{r.netflix && <UnlockBadge label="NF" style="media" />}
									{r.youtube && <UnlockBadge label="YT" style="media" />}
									{r.openai && <UnlockBadge label="GPT" style="ai" />}
									{r.claude && <UnlockBadge label="CL" style="ai" />}
									{r.gemini && <UnlockBadge label="GM" style="ai" />}
									{r.disney && <UnlockBadge label="D+" style="other" />}
									{r.tiktok && <UnlockBadge label="TK" style="other" />}
								</div>
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

> **Note:** `node-table.tsx` is self-contained — proceed immediately to Task 8. Both files are committed together in Task 8, Step 3.

---

### Task 8: Rewrite Subscription Detail page

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx`

- [ ] **Step 1: Rewrite `$id.tsx`**

```tsx
import { useEffect, useRef, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { z } from "zod";

import { api, type CheckJob, type NodeResult, type Subscription } from "@/lib/api";
import { NodeTable } from "@/components/node-table";

const searchSchema = z.object({
	job: z.string().optional(),
});

export const Route = createFileRoute("/subscriptions/$id")({
	validateSearch: searchSchema,
	component: SubscriptionDetailPage,
});

interface SSEProgress {
	progress?: number;
	total?: number;
	node_name?: string;
	alive?: boolean;
	latency_ms?: number;
	speed_kbps?: number;
	done?: boolean;
	status?: string;
}

function latencyColor(ms: number): string {
	if (ms < 50) return "#3fb950";
	if (ms <= 200) return "#d29922";
	return "#f85149";
}

function JobStatusBadge({ status }: { status: CheckJob["status"] }) {
	const map: Record<CheckJob["status"], { bg: string; color: string }> = {
		queued: { bg: "#1a2a3a", color: "#58a6ff" },
		running: { bg: "#1a2a3a", color: "#58a6ff" },
		completed: { bg: "#1a4731", color: "#3fb950" },
		failed: { bg: "#3d1a1a", color: "#f85149" },
	};
	const s = map[status];
	return (
		<span
			className="rounded-full px-2 py-0.5 text-[10px] font-medium"
			style={{ background: s.bg, color: s.color }}
		>
			{status}
		</span>
	);
}

function SubscriptionDetailPage() {
	const { id } = Route.useParams();
	const { job: jobIdFromSearch } = Route.useSearch();
	const [jobId, setJobId] = useState<string | null>(jobIdFromSearch ?? null);
	const [progress, setProgress] = useState<SSEProgress | null>(null);
	const esRef = useRef<EventSource | null>(null);

	const resultsQuery = useQuery({
		queryKey: ["results", id],
		queryFn: () =>
			api.get<{ job: CheckJob; results: NodeResult[] }>(`/check/${id}/results`),
		retry: false,
		staleTime: 0,
	});

	// Resolve subscription name from cache
	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
		staleTime: 30_000,
	});
	const sub = subsQuery.data?.subscriptions.find(
		(s) => s.id === (resultsQuery.data?.job.subscription_id ?? id),
	);

	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally narrow deps to avoid re-running on every render
	useEffect(() => {
		const job = resultsQuery.data?.job;
		if (job && (job.status === "running" || job.status === "queued") && !jobId) {
			setJobId(job.id);
		}
	}, [resultsQuery.data?.job?.id, resultsQuery.data?.job?.status, jobId]);

	useEffect(() => {
		if (!jobId) return;
		const es = new EventSource(`/api/check/${jobId}/progress`);
		esRef.current = es;
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			setProgress(data);
			if (data.done) {
				es.close();
				resultsQuery.refetch();
			}
		};
		es.onerror = () => es.close();
		return () => es.close();
	}, [jobId]);

	const job = resultsQuery.data?.job;
	const results = resultsQuery.data?.results ?? [];
	const progressPct =
		progress?.total ? ((progress.progress ?? 0) / progress.total) * 100 : 0;

	return (
		<div className="space-y-5">
			{/* Header */}
			<div>
				<h1 className="text-lg font-semibold text-[#f0f6fc]">
					{sub?.name || sub?.url || "Subscription Detail"}
				</h1>
				<p className="mt-0.5 font-mono text-xs" style={{ color: "#6e7681" }}>
					{id.slice(0, 8)}…
				</p>
			</div>

			{/* Progress bar */}
			{progress && !progress.done && (
				<div className="space-y-1.5">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-1.5 text-sm" style={{ color: "#f0f6fc" }}>
							<RefreshCw size={13} strokeWidth={1.5} className="animate-spin" style={{ color: "#58a6ff" }} />
							Checking nodes…
						</div>
						<span className="text-xs" style={{ color: "#8b949e" }}>
							{progress.progress ?? 0} / {progress.total ?? "?"}
						</span>
					</div>
					<div
						className="h-[3px] w-full overflow-hidden rounded-sm"
						style={{ background: "#21262d" }}
					>
						<div
							className="h-full rounded-sm transition-[width] duration-300 ease-out"
							style={{
								width: `${progressPct}%`,
								background: "linear-gradient(90deg, #1f6feb, #58a6ff)",
							}}
						/>
					</div>
					{progress.node_name && (
						<p className="font-mono text-[11px]" style={{ color: "#8b949e" }}>
							↳ {progress.node_name}
							{progress.alive && progress.latency_ms ? (
								<span
									className="ml-2 font-medium"
									style={{ color: latencyColor(progress.latency_ms) }}
								>
									{progress.latency_ms}ms
								</span>
							) : null}
							{progress.alive && progress.speed_kbps ? (
								<span className="ml-1.5 font-medium" style={{ color: "#58a6ff" }}>
									{progress.speed_kbps >= 1024
										? `${(progress.speed_kbps / 1024).toFixed(1)}MB/s`
										: `${progress.speed_kbps}KB/s`}
								</span>
							) : null}
							{progress.alive === false ? (
								<span className="ml-2" style={{ color: "#f85149" }}>dead</span>
							) : null}
						</p>
					)}
				</div>
			)}

			{/* Job summary */}
			{job && (
				<div className="flex items-center gap-3 text-sm">
					<JobStatusBadge status={job.status} />
					<span style={{ color: "#8b949e" }}>
						{results.filter((r) => r.alive).length} / {job.total} alive
					</span>
				</div>
			)}

			{resultsQuery.isLoading && (
				<p className="text-sm" style={{ color: "#8b949e" }}>Loading results…</p>
			)}
			{resultsQuery.isError && (
				<p className="text-sm" style={{ color: "#8b949e" }}>No check results yet.</p>
			)}

			<NodeTable results={results} />
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/components/node-table.tsx \
        frontend/apps/web/src/routes/subscriptions/\$id.tsx
git commit -m "feat(frontend): node table badges + latency colors, detail page progress bar"
```

---

## Chunk 5: Remaining Pages

### Task 9: Install Select + rewrite Scheduler page

**Files:**
- Run cmd in: `frontend/packages/ui/`
- Modify: `frontend/apps/web/src/routes/scheduler.tsx`

- [ ] **Step 1: Install shadcn Select component**

```bash
cd frontend/packages/ui && bunx shadcn add select
```

Expected: `src/components/select.tsx` created, `package.json` updated if needed.

- [ ] **Step 2: Rewrite `scheduler.tsx`**

```tsx
import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Clock, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";

import { api, ApiError, type ScheduledJob, type Subscription } from "@/lib/api";

export const Route = createFileRoute("/scheduler")({
	component: SchedulerPage,
});

function SchedulerPage() {
	const qc = useQueryClient();
	const [adding, setAdding] = useState(false);
	const [subId, setSubId] = useState("");
	const [cronExpr, setCronExpr] = useState("");

	const jobsQuery = useQuery({
		queryKey: ["scheduler"],
		queryFn: () => api.get<{ jobs: ScheduledJob[] }>("/scheduler"),
	});

	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
	});

	const createMut = useMutation({
		mutationFn: () => api.post("/scheduler", { subscription_id: subId, cron_expr: cronExpr }),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			setAdding(false);
			setSubId("");
			setCronExpr("");
			toast.success("Schedule created");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => api.delete(`/scheduler/${id}`),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			toast.success("Removed");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
	});

	const jobs = jobsQuery.data?.jobs ?? [];
	const subs = subsQuery.data?.subscriptions ?? [];

	function subName(subId: string) {
		const s = subs.find((s) => s.id === subId);
		return s ? s.name || s.url : subId.slice(0, 8) + "…";
	}

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="text-lg font-semibold text-[#f0f6fc]">Scheduler</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Schedule
				</button>
			</div>

			{adding && (
				<div
					className="rounded-lg border p-4 space-y-3"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Subscription</Label>
						<Select value={subId} onValueChange={setSubId}>
							<SelectTrigger className="h-8 text-sm">
								<SelectValue placeholder="Select subscription…" />
							</SelectTrigger>
							<SelectContent>
								{subs.map((s) => (
									<SelectItem key={s.id} value={s.id}>
										{s.name || s.url}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Cron Expression</Label>
						<Input
							placeholder="0 */6 * * *  (every 6 hours)"
							value={cronExpr}
							onChange={(e) => setCronExpr(e.target.value)}
							className="h-8 text-sm font-mono"
						/>
					</div>
					<div className="flex gap-2">
						<Button
							size="sm"
							onClick={() => createMut.mutate()}
							disabled={!subId || !cronExpr || createMut.isPending}
							style={{ background: "#238636", color: "#fff" }}
							className="border-0"
						>
							{createMut.isPending ? <Loader2 size={13} className="animate-spin" /> : "Save"}
						</Button>
						<Button size="sm" variant="outline" onClick={() => setAdding(false)}>
							Cancel
						</Button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{jobs.map((job) => (
					<div
						key={job.id}
						className="flex items-center justify-between rounded-lg border px-4 py-3"
						style={{ background: "#161b22", borderColor: "#30363d" }}
					>
						<div className="flex items-center gap-3">
							<Clock size={13} strokeWidth={1.5} style={{ color: "#8b949e" }} />
							<div>
								<p className="font-mono text-sm text-[#f0f6fc]">{job.cron_expr}</p>
								<p className="text-xs mt-0.5" style={{ color: "#8b949e" }}>
									{subName(job.subscription_id)}
								</p>
							</div>
						</div>
						<button
							type="button"
							onClick={() => deleteMut.mutate(job.id)}
							disabled={deleteMut.isPending}
							className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
							style={{ color: "#6e7681" }}
						>
							<Trash2 size={13} strokeWidth={1.5} />
						</button>
					</div>
				))}
				{!jobsQuery.isLoading && jobs.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No scheduled jobs.
					</p>
				)}
			</div>
		</div>
	);
}
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
# shadcn add may also update components.json — stage it if modified
git add frontend/packages/ui/src/components/select.tsx \
        frontend/packages/ui/package.json \
        frontend/packages/ui/components.json \
        frontend/bun.lock \
        frontend/apps/web/src/routes/scheduler.tsx
git commit -m "feat(frontend): scheduler page with shadcn Select, show sub names"
```

---

### Task 10: Rewrite Login page

**Files:**
- Modify: `frontend/apps/web/src/routes/login.tsx`

- [ ] **Step 1: Rewrite `login.tsx`**

```tsx
import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Lock, Loader2 } from "lucide-react";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";

import { api, ApiError } from "@/lib/api";
import { setToken } from "@/lib/auth";

export const Route = createFileRoute("/login")({
	component: LoginPage,
});

function LoginPage() {
	const navigate = useNavigate();
	const [mode, setMode] = useState<"login" | "register">("login");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [loading, setLoading] = useState(false);

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		setLoading(true);
		try {
			if (mode === "register") {
				await api.post("/auth/register", { username, password });
				toast.success("Account created — please log in");
				setMode("login");
			} else {
				const resp = await api.post<{ token: string }>("/auth/login", { username, password });
				setToken(resp.token);
				navigate({ to: "/" });
			}
		} catch (err) {
			toast.error(err instanceof ApiError ? err.message : "Something went wrong");
		} finally {
			setLoading(false);
		}
	}

	return (
		<div
			className="w-full max-w-sm rounded-lg border p-6"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			<div className="mb-6 flex items-center gap-2">
				<Lock size={16} strokeWidth={1.5} style={{ color: "#58a6ff" }} />
				<h1 className="text-base font-semibold text-[#f0f6fc]">
					{mode === "login" ? "Sign in" : "Create account"}
				</h1>
			</div>

			<form onSubmit={handleSubmit} className="space-y-4">
				<div className="space-y-1.5">
					<Label htmlFor="username" className="text-xs text-[#8b949e]">Username</Label>
					<Input
						id="username"
						value={username}
						onChange={(e) => setUsername(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				<div className="space-y-1.5">
					<Label htmlFor="password" className="text-xs text-[#8b949e]">Password</Label>
					<Input
						id="password"
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						required
						className="h-8 text-sm"
					/>
				</div>
				<button
					type="submit"
					disabled={loading}
					className="flex w-full items-center justify-center gap-2 rounded-md py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
					style={{ background: "#238636" }}
				>
					{loading ? (
						<Loader2 size={14} className="animate-spin" />
					) : mode === "login" ? (
						"Sign in"
					) : (
						"Register"
					)}
				</button>

				<p className="text-center text-xs" style={{ color: "#8b949e" }}>
					{mode === "login" ? "No account? " : "Have an account? "}
					<button
						type="button"
						className="underline hover:text-[#f0f6fc] transition-colors"
						style={{ color: "#58a6ff" }}
						onClick={() => setMode(mode === "login" ? "register" : "login")}
					>
						{mode === "login" ? "Register" : "Sign in"}
					</button>
				</p>
			</form>
		</div>
	);
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/login.tsx
git commit -m "feat(frontend): login page GitHub styling with Lock icon"
```

---

### Task 11: Apply consistent styling to Settings pages

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/general.tsx`
- Modify: `frontend/apps/web/src/routes/settings/notify.tsx`

- [ ] **Step 1: Rewrite `settings/general.tsx`**

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { useEffect } from "react";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { api, type UserSettings } from "@/lib/api";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";

export const Route = createFileRoute("/settings/general")({
	component: GeneralSettingsPage,
});

const DEFAULT_SPEED_TEST_URL = "https://speed.cloudflare.com/__down?bytes=204800";

function GeneralSettingsPage() {
	const qc = useQueryClient();

	const settingsQuery = useQuery({
		queryKey: ["settings"],
		queryFn: () => api.get<UserSettings>("/settings"),
	});

	const { register, handleSubmit, reset } = useForm<UserSettings>({
		defaultValues: { speed_test_url: "" },
	});

	useEffect(() => {
		if (settingsQuery.data) reset(settingsQuery.data);
	}, [settingsQuery.data]);

	const saveMutation = useMutation({
		mutationFn: (data: UserSettings) => api.put<UserSettings>("/settings", data),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["settings"] });
			toast.success("Settings saved");
		},
		onError: () => toast.error("Failed to save settings"),
	});

	return (
		<div className="space-y-5 max-w-lg">
			<h1 className="text-lg font-semibold text-[#f0f6fc]">General Settings</h1>

			<div
				className="rounded-lg border p-5"
				style={{ background: "#161b22", borderColor: "#30363d" }}
			>
				<form onSubmit={handleSubmit((d) => saveMutation.mutate(d))} className="space-y-4">
					<div className="space-y-1.5">
						<Label htmlFor="speed_test_url" className="text-xs text-[#8b949e]">
							Speed Test URL
						</Label>
						<Input
							id="speed_test_url"
							placeholder={DEFAULT_SPEED_TEST_URL}
							{...register("speed_test_url")}
							className="h-8 text-sm font-mono"
						/>
						<p className="text-xs" style={{ color: "#6e7681" }}>
							URL used to measure download speed. Leave blank to use default.
						</p>
					</div>

					<button
						type="submit"
						disabled={saveMutation.isPending}
						className="flex items-center gap-2 rounded-md px-4 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
						style={{ background: "#238636" }}
					>
						{saveMutation.isPending ? <Loader2 size={13} className="animate-spin" /> : "Save"}
					</button>
				</form>
			</div>
		</div>
	);
}
```

- [ ] **Step 2: Rewrite `settings/notify.tsx`**

```tsx
import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";

import { api, ApiError, type NotifyChannel } from "@/lib/api";

export const Route = createFileRoute("/settings/notify")({
	component: NotifyPage,
});

function NotifyPage() {
	const qc = useQueryClient();
	const [adding, setAdding] = useState(false);
	const [type, setType] = useState<"webhook" | "telegram">("webhook");
	const [name, setName] = useState("");
	const [webhookUrl, setWebhookUrl] = useState("");
	const [botToken, setBotToken] = useState("");
	const [chatId, setChatId] = useState("");

	const channelsQuery = useQuery({
		queryKey: ["notify-channels"],
		queryFn: () => api.get<{ channels: NotifyChannel[] }>("/notify/channels"),
	});

	const createMut = useMutation({
		mutationFn: () => {
			const config =
				type === "webhook"
					? { url: webhookUrl, method: "POST" }
					: { bot_token: botToken, chat_id: chatId };
			return api.post("/notify/channels", { name, type, config });
		},
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["notify-channels"] });
			setAdding(false);
			setName("");
			setWebhookUrl("");
			setBotToken("");
			setChatId("");
			toast.success("Channel added");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => api.delete(`/notify/channels/${id}`),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["notify-channels"] });
			toast.success("Removed");
		},
	});

	const channels = channelsQuery.data?.channels ?? [];

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="text-lg font-semibold text-[#f0f6fc]">Notification Channels</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Channel
				</button>
			</div>

			{adding && (
				<div
					className="rounded-lg border p-4 space-y-3"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<div className="space-y-1.5">
						<Label className="text-xs text-[#8b949e]">Name</Label>
						<Input
							placeholder="My Channel"
							value={name}
							onChange={(e) => setName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="space-y-1.5">
						<Label className="text-xs text-[#8b949e]">Type</Label>
						<Select value={type} onValueChange={(v) => setType(v as "webhook" | "telegram")}>
							<SelectTrigger className="h-8 text-sm">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="webhook">Webhook</SelectItem>
								<SelectItem value="telegram">Telegram</SelectItem>
							</SelectContent>
						</Select>
					</div>
					{type === "webhook" && (
						<div className="space-y-1.5">
							<Label className="text-xs text-[#8b949e]">URL</Label>
							<Input
								placeholder="https://..."
								value={webhookUrl}
								onChange={(e) => setWebhookUrl(e.target.value)}
								className="h-8 text-sm"
							/>
						</div>
					)}
					{type === "telegram" && (
						<>
							<div className="space-y-1.5">
								<Label className="text-xs text-[#8b949e]">Bot Token</Label>
								<Input
									placeholder="123456:ABC..."
									value={botToken}
									onChange={(e) => setBotToken(e.target.value)}
									className="h-8 text-sm font-mono"
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-xs text-[#8b949e]">Chat ID</Label>
								<Input
									placeholder="-1001234567890"
									value={chatId}
									onChange={(e) => setChatId(e.target.value)}
									className="h-8 text-sm font-mono"
								/>
							</div>
						</>
					)}
					<div className="flex gap-2">
						<button
							type="button"
							onClick={() => createMut.mutate()}
							disabled={createMut.isPending}
							className="flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
							style={{ background: "#238636" }}
						>
							{createMut.isPending ? <Loader2 size={13} className="animate-spin" /> : "Save"}
						</button>
						<button
							type="button"
							onClick={() => setAdding(false)}
							className="rounded-md border px-3 py-1.5 text-sm transition-colors hover:bg-white/5"
							style={{ borderColor: "#30363d", color: "#8b949e" }}
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{channels.map((ch) => (
					<div
						key={ch.id}
						className="flex items-center justify-between rounded-lg border px-4 py-3"
						style={{ background: "#161b22", borderColor: "#30363d" }}
					>
						<div>
							<p className="text-sm font-medium text-[#f0f6fc]">{ch.name || ch.id}</p>
							<p
								className="mt-0.5 text-[11px] uppercase tracking-[0.4px]"
								style={{ color: "#8b949e" }}
							>
								{ch.type}
							</p>
						</div>
						<button
							type="button"
							onClick={() => deleteMut.mutate(ch.id)}
							className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149]"
							style={{ color: "#6e7681" }}
						>
							<Trash2 size={13} strokeWidth={1.5} />
						</button>
					</div>
				))}
				{!channelsQuery.isLoading && channels.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No channels configured.
					</p>
				)}
			</div>
		</div>
	);
}
```

- [ ] **Step 3: Type-check and lint**

```bash
cd frontend && bun check-types && bun check
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/settings/general.tsx \
        frontend/apps/web/src/routes/settings/notify.tsx
git commit -m "feat(frontend): settings pages consistent GitHub styling"
```

---

### Task 12: Final verification pass

- [ ] **Step 1: Full type-check**

```bash
cd frontend && bun check-types
```

Expected: 0 errors.

- [ ] **Step 2: Full lint**

```bash
cd frontend && bun check
```

Expected: 0 lint errors. Fix any Biome issues (tabs, trailing commas) if flagged.

- [ ] **Step 3: Build check**

> **Note:** Run `bun dev:web` briefly first (a few seconds) if you haven't recently — this triggers TanStack Router's Vite plugin to regenerate `routeTree.gen.ts` from the current route files. Then stop it and run the build.

```bash
cd frontend && bun build
```

Expected: Build completes with no errors.

- [ ] **Step 4: Commit if any lint fixes were auto-applied**

```bash
git add -p  # stage only lint-fixed files
git commit -m "style(frontend): biome lint fixes"
```
