# Mobile Responsive Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the web frontend a complete mobile experience (hamburger drawer nav, no horizontal overflow, table-to-card reflow, full-screen rule editor) without changing the desktop layout.

**Architecture:** Single `md` (768px) breakpoint. Below `md` the fixed sidebar is hidden and replaced by a top bar + left slide-in drawer (Base UI `Dialog`); data table reflows to stacked cards; the rule-editor modal goes full-screen. At/above `md` the current desktop UI is untouched.

**Tech Stack:** React 19, TanStack Router, Tailwind CSS v4, Base UI (`@base-ui/react@1.3.0`), lucide-react. Spec: `docs/superpowers/specs/2026-06-09-mobile-responsive-design.md`.

---

## Verification approach (read first)

This frontend has **no unit/E2E test harness** (Playwright was removed; only `tsc` + Biome run). The change is presentational (CSS classes / JSX structure). Standing up vitest + RTL to assert `className` strings would be brittle and low-value, so verification for every task is:

1. **Type-check:** `cd frontend && bun check-types` → expect no errors.
2. **Lint/format:** `cd frontend && bun check` → expect pass (auto-fixes formatting).
3. **Browser check** (the real gate for layout): with `encore run` (terminal 1) and `cd frontend && bun dev` (terminal 2) running, use the Playwright MCP browser tools to open `http://localhost:3001`, resize to **375×812** (phone) and **768×1024** (tablet), and visually confirm the task's acceptance criteria via `browser_snapshot` / `browser_take_screenshot`.

> Browser routes require auth — the app redirects to `/login`. Log in once (register a user via the login page) so authed routes render. If a running app is unavailable in the execution environment, rely on type-check + lint and note that browser confirmation is pending.

Run all three after each task. Commit only when type-check and lint pass.

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `frontend/apps/web/src/components/sidebar.tsx` | Nav content (`SidebarNav`) + desktop `aside` (`Sidebar`) | Rewrite (split) |
| `frontend/apps/web/src/components/mobile-nav.tsx` | Mobile top bar + drawer | Create |
| `frontend/apps/web/src/routes/__root.tsx` | App shell layout | Modify |
| `frontend/apps/web/src/components/node-table.tsx` | Results table + mobile card view | Rewrite |
| `frontend/apps/web/src/routes/subscriptions/index.tsx` | Subscription rows | Modify |
| `frontend/apps/web/src/routes/index.tsx` | Dashboard API-key row | Modify |
| `frontend/apps/web/src/components/platforms/RuleEditorDialog.tsx` | Rule editor modal | Modify |
| `frontend/apps/web/src/routes/settings/general.tsx` | SMTP form grids | Modify |
| `frontend/apps/web/src/routes/settings/notify.tsx` | Channel row | Modify |

---

### Task 1: Split `Sidebar` into shared `SidebarNav` + desktop `Sidebar`

**Files:**
- Modify: `frontend/apps/web/src/components/sidebar.tsx` (full rewrite)

- [ ] **Step 1: Rewrite `sidebar.tsx`**

Extract the inner content into an exported `SidebarNav` that accepts an `onNavigate` callback (fired on link click / logout so the drawer can close). `Sidebar` becomes a thin desktop-only wrapper (`hidden md:flex`). Behaviour and styling are otherwise identical to today.

```tsx
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import {
	Bell,
	Clock,
	LayoutDashboard,
	List,
	LogOut,
	Moon,
	Settings,
	Sun,
	Tv2,
	User,
} from "lucide-react";
import { clearToken } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { useMe } from "@/queries";

const NAV_ITEMS = [
	{ to: "/", label: "Dashboard", icon: LayoutDashboard, exact: true },
	{ to: "/subscriptions", label: "Subscriptions", icon: List, exact: false },
	{ to: "/scheduler", label: "Scheduler", icon: Clock, exact: true },
	{ to: "/settings/notify", label: "Notify", icon: Bell, exact: false },
	{ to: "/settings/platforms", label: "Platforms", icon: Tv2, exact: false },
] as const;

const BOTTOM_ITEMS = [
	{ to: "/settings/general", label: "Settings", icon: Settings, exact: false },
] as const;

function NavItem({
	to,
	label,
	icon: Icon,
	exact,
	onNavigate,
}: {
	to: string;
	label: string;
	icon: React.ElementType;
	exact: boolean;
	onNavigate?: () => void;
}) {
	const pathname = useRouterState({ select: (s) => s.location.pathname });
	const isActive = exact ? pathname === to : pathname.startsWith(to);

	return (
		<Link
			to={to}
			onClick={onNavigate}
			className={[
				"flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
				isActive
					? "border font-medium text-foreground"
					: "border border-transparent text-muted-foreground hover:bg-white/5 hover:text-foreground",
			].join(" ")}
			style={
				isActive
					? {
							background: "var(--color-active-bg)",
							borderColor: "var(--color-active-border)",
						}
					: undefined
			}
		>
			<Icon size={14} strokeWidth={1.5} />
			{label}
		</Link>
	);
}

export function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
	const navigate = useNavigate();
	const { theme, toggle } = useTheme();
	const meQuery = useMe();
	const username = meQuery.data?.username ?? "…";

	function logout() {
		clearToken();
		onNavigate?.();
		navigate({ to: "/login" });
	}

	return (
		<div className="flex h-full flex-col">
			{/* Logo */}
			<div className="flex items-center gap-2.5 border-border border-b px-4 py-3">
				<div
					className="flex h-6 w-6 items-center justify-center rounded-md font-bold text-xs"
					style={{
						background: "var(--primary)",
						color: "var(--primary-foreground)",
					}}
				>
					S
				</div>
				<span className="font-semibold text-foreground text-sm">
					subs-check
				</span>
			</div>

			{/* Primary nav */}
			<nav className="flex flex-1 flex-col gap-0.5 p-2">
				{NAV_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} onNavigate={onNavigate} />
				))}
			</nav>

			{/* Bottom nav */}
			<div className="flex flex-col gap-0.5 border-border border-t p-2">
				{BOTTOM_ITEMS.map((item) => (
					<NavItem key={item.to} {...item} onNavigate={onNavigate} />
				))}

				{/* Theme toggle */}
				<button
					type="button"
					onClick={toggle}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-muted-foreground text-sm transition-colors hover:bg-white/5 hover:text-foreground"
					title={
						theme === "dark" ? "Switch to light mode" : "Switch to dark mode"
					}
				>
					{theme === "dark" ? (
						<Sun size={14} strokeWidth={1.5} />
					) : (
						<Moon size={14} strokeWidth={1.5} />
					)}
					{theme === "dark" ? "Light mode" : "Dark mode"}
				</button>

				<button
					type="button"
					onClick={logout}
					className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-muted-foreground text-sm transition-colors hover:bg-white/5 hover:text-foreground"
				>
					<div className="flex h-5 w-5 items-center justify-center rounded-full bg-secondary">
						<User size={10} strokeWidth={1.5} />
					</div>
					<span className="flex-1 truncate text-left">{username}</span>
					<LogOut size={12} strokeWidth={1.5} />
				</button>
			</div>
		</div>
	);
}

export function Sidebar() {
	return (
		<aside className="hidden h-screen w-[220px] flex-shrink-0 flex-col border-border border-r bg-card md:flex">
			<SidebarNav />
		</aside>
	);
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && bun check-types`
Expected: no errors. (`__root.tsx` still imports `Sidebar`, which still exists.)

- [ ] **Step 3: Lint**

Run: `cd frontend && bun check`
Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/components/sidebar.tsx
git commit -m "refactor(frontend): extract SidebarNav for reuse in mobile drawer"
```

---

### Task 2: Create the mobile top bar + drawer

**Files:**
- Create: `frontend/apps/web/src/components/mobile-nav.tsx`

- [ ] **Step 1: Write `mobile-nav.tsx`**

Top bar is `md:hidden`. The hamburger is the Base UI `Dialog.Trigger`; the drawer is a left-anchored `Dialog.Popup` rendering the shared `SidebarNav`, closing itself on navigation via `onNavigate`. Base UI provides focus trap, ESC, scroll lock, and backdrop. Slide/fade use Base UI's `data-starting-style` / `data-ending-style` transition attributes.

```tsx
import { Dialog } from "@base-ui/react/dialog";
import { Menu } from "lucide-react";
import { useState } from "react";
import { SidebarNav } from "@/components/sidebar";

export function MobileNav() {
	const [open, setOpen] = useState(false);

	return (
		<Dialog.Root open={open} onOpenChange={setOpen}>
			<header className="flex h-12 flex-shrink-0 items-center gap-3 border-border border-b bg-card px-4 md:hidden">
				<Dialog.Trigger
					aria-label="Open navigation menu"
					className="-ml-1 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-white/5 hover:text-foreground"
				>
					<Menu size={18} strokeWidth={1.5} />
				</Dialog.Trigger>
				<div className="flex items-center gap-2">
					<div
						className="flex h-6 w-6 items-center justify-center rounded-md font-bold text-xs"
						style={{
							background: "var(--primary)",
							color: "var(--primary-foreground)",
						}}
					>
						S
					</div>
					<span className="font-semibold text-foreground text-sm">
						subs-check
					</span>
				</div>
			</header>

			<Dialog.Portal>
				<Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
				<Dialog.Popup className="fixed inset-y-0 left-0 z-50 w-[280px] max-w-[85vw] border-border border-r bg-card shadow-xl transition-transform duration-200 data-[ending-style]:-translate-x-full data-[starting-style]:-translate-x-full">
					<Dialog.Title className="sr-only">Navigation</Dialog.Title>
					<SidebarNav onNavigate={() => setOpen(false)} />
				</Dialog.Popup>
			</Dialog.Portal>
		</Dialog.Root>
	);
}
```

- [ ] **Step 2: Type-check**

Run: `cd frontend && bun check-types`
Expected: no errors. If the import path `@base-ui/react/dialog` errors, confirm the subpath: `ls node_modules/.bun/@base-ui+react@1.3.0+*/node_modules/@base-ui/react/dialog` (it exists — `Root`, `Trigger`, `Portal`, `Backdrop`, `Popup`, `Title` are exported).

- [ ] **Step 3: Lint**

Run: `cd frontend && bun check`
Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/components/mobile-nav.tsx
git commit -m "feat(frontend): add mobile top bar with slide-in nav drawer"
```

---

### Task 3: Wire the responsive shell in `__root.tsx`

**Files:**
- Modify: `frontend/apps/web/src/routes/__root.tsx:77-84`

- [ ] **Step 1: Add the `MobileNav` import**

After the existing `import { Sidebar } from "@/components/sidebar";` (line 18), add:

```tsx
import { MobileNav } from "@/components/mobile-nav";
```

- [ ] **Step 2: Replace the authed layout block**

Replace this (lines 77-84):

```tsx
					<div className="flex h-screen overflow-hidden">
						<Sidebar />
						<main className="flex-1 overflow-y-auto px-6 py-6">
							<div className="mx-auto max-w-5xl">
								<Outlet />
							</div>
						</main>
					</div>
```

with:

```tsx
					<div className="flex h-screen overflow-hidden">
						<Sidebar />
						<div className="flex min-w-0 flex-1 flex-col">
							<MobileNav />
							<main className="flex-1 overflow-y-auto px-4 py-4 md:px-6 md:py-6">
								<div className="mx-auto max-w-5xl">
									<Outlet />
								</div>
							</main>
						</div>
					</div>
```

- [ ] **Step 3: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 4: Browser check (first real mobile view)**

With `encore run` + `bun dev` running, open `http://localhost:3001`, log in, then:
- Resize to **768×1024**: desktop sidebar visible, no top bar.
- Resize to **375×812**: sidebar hidden, top bar visible. Tap hamburger → drawer slides in from left with all nav items. Tap a nav item → navigates and drawer closes. Reopen, press ESC / tap backdrop → closes. No horizontal page scroll.

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/__root.tsx
git commit -m "feat(frontend): responsive app shell with mobile top bar"
```

---

### Task 4: Add mobile card view to `NodeTable`

**Files:**
- Modify: `frontend/apps/web/src/components/node-table.tsx` (full rewrite)

- [ ] **Step 1: Rewrite `node-table.tsx`**

Adds a `formatSpeed` helper and `UnlockIcons` subcomponent (shared by table + card, DRY), a `NodeCard` for `< md`, and renders cards (`md:hidden`) plus the existing table (`hidden md:block`). Table speed cells now call `formatSpeed`; the Unlocks cell uses `UnlockIcons`. Sorting/empty-state behaviour is unchanged.

```tsx
import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";

import { PlatformIcon, PlatformIconAny } from "./platform-icons";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

interface Props {
	results: NodeResult[];
	rules?: PlatformRule[];
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}

function latencyColor(ms: number): string {
	if (ms < 50) return "var(--color-success)";
	if (ms <= 200) return "var(--color-warning)";
	return "var(--destructive)";
}

function formatSpeed(kbps: number): string {
	return kbps >= 1024 ? `${(kbps / 1024).toFixed(1)} MB/s` : `${kbps} KB/s`;
}

function StatusBadge({ alive }: { alive: boolean }) {
	return alive ? (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium text-[10px]"
			style={{
				background: "var(--color-badge-success-bg)",
				color: "var(--color-badge-success)",
			}}
		>
			<span
				className="h-1.5 w-1.5 rounded-full"
				style={{ background: "var(--color-success)" }}
			/>
			alive
		</span>
	) : (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium text-[10px]"
			style={{
				background: "var(--color-badge-danger-bg)",
				color: "var(--color-badge-danger)",
			}}
		>
			<span
				className="h-1.5 w-1.5 rounded-full"
				style={{ background: "var(--destructive)" }}
			/>
			dead
		</span>
	);
}

function UnlockIcons({
	r,
	ruleByKey,
}: {
	r: NodeResult;
	ruleByKey: Record<string, PlatformRule>;
}) {
	return (
		<div className="flex flex-wrap gap-1.5">
			{r.netflix && <PlatformIcon platform="netflix" />}
			{r.youtube && !r.youtube_premium && <PlatformIcon platform="youtube" />}
			{r.youtube_premium && <PlatformIcon platform="youtube_premium" />}
			{r.openai && <PlatformIcon platform="openai" />}
			{r.claude && <PlatformIcon platform="claude" />}
			{r.gemini && <PlatformIcon platform="gemini" />}
			{r.grok && <PlatformIcon platform="grok" />}
			{r.disney && <PlatformIcon platform="disney" />}
			{r.tiktok && <PlatformIcon platform="tiktok" />}
			{r.extra_platforms &&
				Object.entries(r.extra_platforms)
					.filter(([, v]) => v)
					.map(([key]) => {
						const rule = ruleByKey[key];
						return (
							<PlatformIconAny
								key={key}
								platformKey={key}
								icon={rule?.icon}
								label={rule?.name ?? key}
							/>
						);
					})}
		</div>
	);
}

function CardField({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div className="flex items-center justify-between gap-2">
			<span className="text-[10px] text-muted-foreground uppercase tracking-[0.4px]">
				{label}
			</span>
			<span className="text-xs">{children}</span>
		</div>
	);
}

function NodeCard({
	r,
	ruleByKey,
	onToggleEnabled,
}: {
	r: NodeResult;
	ruleByKey: Record<string, PlatformRule>;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}) {
	const dim = "var(--color-dimmed)";
	return (
		<div
			className="rounded-lg border border-border bg-card p-3"
			style={{ opacity: r.enabled ? 1 : 0.55 }}
		>
			<div className="flex items-center gap-2">
				<span
					className="min-w-0 flex-1 truncate font-mono text-xs"
					style={{ color: r.alive ? "var(--foreground)" : dim }}
				>
					{r.node_name}
				</span>
				<StatusBadge alive={r.alive} />
				{onToggleEnabled && (
					<button
						type="button"
						onClick={() => onToggleEnabled(r.node_id, !r.enabled)}
						title={r.enabled ? "Disable node" : "Enable node"}
						className="min-h-7 rounded px-2 py-1 font-medium text-[10px] transition-colors"
						style={{
							background: r.enabled
								? "var(--color-badge-success-bg)"
								: "var(--color-badge-danger-bg)",
							color: r.enabled
								? "var(--color-badge-success)"
								: "var(--color-badge-danger)",
						}}
					>
						{r.enabled ? "on" : "off"}
					</button>
				)}
			</div>

			<div className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1">
				<CardField label="Latency">
					{r.alive ? (
						<span style={{ color: latencyColor(r.latency_ms) }}>
							{r.latency_ms}ms
						</span>
					) : (
						<span style={{ color: dim }}>—</span>
					)}
				</CardField>
				<CardField label="Traffic">
					<span className="text-muted-foreground">
						{formatBytes(r.traffic_bytes)}
					</span>
				</CardField>
				<CardField label="↓ Speed">
					{r.alive && r.speed_kbps ? (
						<span style={{ color: "var(--primary)" }}>
							{formatSpeed(r.speed_kbps)}
						</span>
					) : (
						<span style={{ color: dim }}>—</span>
					)}
				</CardField>
				<CardField label="↑ Upload">
					{r.alive && r.upload_speed_kbps ? (
						<span style={{ color: "var(--color-warning)" }}>
							{formatSpeed(r.upload_speed_kbps)}
						</span>
					) : (
						<span style={{ color: dim }}>—</span>
					)}
				</CardField>
				<CardField label="Country">
					<span style={{ color: r.alive ? "var(--foreground)" : dim }}>
						{r.country || "—"}
					</span>
				</CardField>
			</div>

			<div className="mt-2">
				<UnlockIcons r={r} ruleByKey={ruleByKey} />
			</div>
		</div>
	);
}

export function NodeTable({ results, rules = [], onToggleEnabled }: Props) {
	const ruleByKey = Object.fromEntries(rules.map((r) => [r.key, r]));
	const alive = results.filter((r) => r.alive);
	const dead = results.filter((r) => !r.alive);
	const sorted = [...alive, ...dead];

	if (sorted.length === 0) {
		return <p className="text-muted-foreground text-sm">No results yet.</p>;
	}

	return (
		<>
			{/* Mobile: stacked cards */}
			<div className="space-y-2 md:hidden">
				{sorted.map((r) => (
					<NodeCard
						key={r.node_id}
						r={r}
						ruleByKey={ruleByKey}
						onToggleEnabled={onToggleEnabled}
					/>
				))}
			</div>

			{/* Desktop: table */}
			<div className="hidden overflow-x-auto rounded-lg border border-border md:block">
				<table className="w-full border-collapse">
					<thead>
						<tr style={{ borderBottom: "1px solid var(--border)" }}>
							{[
								"",
								"Node",
								"Status",
								"Latency",
								"↓ Speed",
								"↑ Upload",
								"Traffic",
								"Country",
								"Unlocks",
							].map((h) => (
								<th
									key={h}
									className="px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]"
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
								style={{ borderBottom: "1px solid var(--secondary)" }}
							>
								<td className="px-2 py-2">
									{onToggleEnabled && (
										<button
											type="button"
											onClick={() => onToggleEnabled(r.node_id, !r.enabled)}
											title={r.enabled ? "Disable node" : "Enable node"}
											className="rounded px-1.5 py-0.5 text-[10px] transition-colors"
											style={{
												background: r.enabled
													? "var(--color-badge-success-bg)"
													: "var(--color-badge-danger-bg)",
												color: r.enabled
													? "var(--color-badge-success)"
													: "var(--color-badge-danger)",
											}}
										>
											{r.enabled ? "on" : "off"}
										</button>
									)}
								</td>
								<td
									className="max-w-[180px] truncate px-3 py-2 font-mono text-[11px]"
									style={{
										color: r.enabled
											? r.alive
												? "var(--foreground)"
												: "var(--color-dimmed)"
											: "var(--color-dimmed)",
										opacity: r.enabled ? 1 : 0.5,
									}}
								>
									{r.node_name}
								</td>
								<td className="px-3 py-2">
									<StatusBadge alive={r.alive} />
								</td>
								<td className="px-3 py-2 font-medium text-xs">
									{r.alive ? (
										<span style={{ color: latencyColor(r.latency_ms) }}>
											{r.latency_ms}ms
										</span>
									) : (
										<span style={{ color: "var(--color-dimmed)" }}>—</span>
									)}
								</td>
								<td className="px-3 py-2 text-xs">
									{r.alive && r.speed_kbps ? (
										<span style={{ color: "var(--primary)" }}>
											{formatSpeed(r.speed_kbps)}
										</span>
									) : (
										<span style={{ color: "var(--color-dimmed)" }}>—</span>
									)}
								</td>
								<td className="px-3 py-2 text-xs">
									{r.alive && r.upload_speed_kbps ? (
										<span style={{ color: "var(--color-warning)" }}>
											{formatSpeed(r.upload_speed_kbps)}
										</span>
									) : (
										<span style={{ color: "var(--color-dimmed)" }}>—</span>
									)}
								</td>
								<td className="px-3 py-2 text-muted-foreground text-xs">
									{formatBytes(r.traffic_bytes)}
								</td>
								<td
									className="px-3 py-2 text-xs"
									style={{
										color: r.alive ? "var(--foreground)" : "var(--color-dimmed)",
									}}
								>
									{r.country || "—"}
								</td>
								<td className="px-3 py-2">
									<UnlockIcons r={r} ruleByKey={ruleByKey} />
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</>
	);
}
```

- [ ] **Step 2: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 3: Browser check**

Open a subscription detail page that has results (`/subscriptions/$id`). At **375px** confirm: stacked cards (node name + status + on/off, a 2-col metric grid, unlock icons), no horizontal scroll. At **768px** confirm: the original 9-column table renders and horizontally scrolls if needed. Toggle a node's on/off in card view — it persists.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/components/node-table.tsx
git commit -m "feat(frontend): mobile card view for node results table"
```

---

### Task 5: Subscription rows wrap actions + larger touch targets

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/index.tsx` (inside `SubRow`)

- [ ] **Step 1: Stack the row on mobile**

Replace (line 314):

```tsx
				<div className="flex items-center gap-3 px-4 py-3">
```

with:

```tsx
				<div className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:gap-3">
```

- [ ] **Step 2: Align the action cluster when stacked**

Replace (line 354):

```tsx
					<div className="flex flex-shrink-0 items-center gap-2">
```

with:

```tsx
					<div className="flex flex-shrink-0 items-center gap-2 self-end sm:self-auto">
```

- [ ] **Step 3: Enlarge the icon-button touch targets on mobile**

The edit (Pencil) and delete (Trash2) buttons use `p-1.5`. Give them a larger mobile tap area that collapses on desktop.

Replace the Pencil button's className (line 388):

```tsx
							className="rounded-md p-1.5 transition-colors hover:bg-white/5"
```

with:

```tsx
							className="rounded-md p-2 transition-colors hover:bg-white/5 sm:p-1.5"
```

Replace the Trash2 button's className (line 397):

```tsx
							className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
```

with:

```tsx
							className="rounded-md p-2 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50 sm:p-1.5"
```

- [ ] **Step 4: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 5: Browser check**

At **375px** on `/subscriptions`: each row shows the name/url block on top and the action buttons on a second line aligned right; nothing is clipped; the inline "Check options" and "Edit" panels still expand correctly. At **768px**: row is single-line as before.

- [ ] **Step 6: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/index.tsx
git commit -m "feat(frontend): wrap subscription row actions on mobile"
```

---

### Task 6: Dashboard API-key row wraps

**Files:**
- Modify: `frontend/apps/web/src/routes/index.tsx:136-137`

- [ ] **Step 1: Allow wrapping and force the code onto its own line on mobile**

Replace (line 136):

```tsx
						<div className="flex items-center gap-2">
							<code className="flex-1 truncate rounded bg-background px-3 py-1.5 font-mono text-foreground text-xs">
```

with:

```tsx
						<div className="flex flex-wrap items-center gap-2">
							<code className="min-w-0 flex-1 basis-full truncate rounded bg-background px-3 py-1.5 font-mono text-foreground text-xs sm:basis-auto">
```

(The closing `</div>` and the Copy/Regenerate buttons are unchanged. On mobile the code spans the full first line and the two buttons wrap beneath it; at `sm` and up it returns to a single inline row.)

- [ ] **Step 2: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 3: Browser check**

At **375px** on `/` (Dashboard): the API key code box is full-width with Copy/Regenerate below it; no overflow. At **768px**: unchanged single-row layout.

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/index.tsx
git commit -m "feat(frontend): wrap dashboard API key row on mobile"
```

---

### Task 7: Rule editor modal goes full-screen on mobile

**Files:**
- Modify: `frontend/apps/web/src/components/platforms/RuleEditorDialog.tsx:133-136`, `:239`, `:272-273`

- [ ] **Step 1: Full-bleed overlay + panel on mobile**

Replace the outer overlay (line 133):

```tsx
		<div className="fixed inset-0 z-50 flex items-stretch justify-center bg-black/70 p-4">
```

with:

```tsx
		<div className="fixed inset-0 z-50 flex items-stretch justify-center bg-black/70 p-0 md:p-4">
```

Replace the panel container (lines 134-137):

```tsx
			<div
				className="flex w-full max-w-5xl flex-col rounded-xl border border-border bg-card shadow-2xl"
				style={{ maxHeight: "94vh" }}
			>
```

with:

```tsx
			<div className="flex h-full max-h-screen w-full max-w-none flex-col rounded-none border border-border bg-card shadow-2xl md:h-auto md:max-h-[94vh] md:max-w-5xl md:rounded-xl">
```

(The inline `maxHeight` style is replaced by responsive `max-h-*` classes.)

- [ ] **Step 2: Stack the editor body vertically on mobile**

Replace the body row (line 239):

```tsx
				<div className="flex min-h-0 flex-1 overflow-hidden">
```

with:

```tsx
				<div className="flex min-h-0 flex-1 flex-col overflow-hidden md:flex-row">
```

- [ ] **Step 3: Give the editor column a minimum height on mobile**

Replace the editor column opener (line 240):

```tsx
					<div className="flex min-h-0 flex-1 flex-col overflow-hidden">
```

with:

```tsx
					<div className="flex min-h-0 flex-1 flex-col overflow-hidden max-md:min-h-[280px]">
```

- [ ] **Step 4: Make the docs panel a bottom section on mobile**

Replace the docs panel wrapper (lines 272-273):

```tsx
					{showDocs && (
						<div className="w-72 flex-shrink-0 overflow-y-auto border-border border-l bg-background/50">
```

with:

```tsx
					{showDocs && (
						<div className="max-h-[40vh] w-full flex-shrink-0 overflow-y-auto border-border border-t bg-background/50 md:max-h-none md:w-72 md:border-t-0 md:border-l">
```

- [ ] **Step 5: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 6: Browser check**

On `/settings/platforms`, tap "Add Rule" at **375px**: the dialog fills the screen; the header toolbar wraps its controls; the Monaco/condition editor is usable with a visible height; tapping "Docs" shows the docs as a scrollable section at the bottom (not a clipped side column); Test/Save/close all reachable. At **768px**: the dialog is the centered `max-w-5xl` panel with docs as a right-hand column, as before.

- [ ] **Step 7: Commit**

```bash
git add frontend/apps/web/src/components/platforms/RuleEditorDialog.tsx
git commit -m "feat(frontend): full-screen rule editor on mobile"
```

---

### Task 8: Settings forms reflow on mobile

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/general.tsx:137`, `:160`
- Modify: `frontend/apps/web/src/routes/settings/notify.tsx:516`, `:543`

- [ ] **Step 1: Stack the SMTP host/port grid**

In `general.tsx` replace (line 137):

```tsx
							<div className="grid grid-cols-3 gap-3">
```

with:

```tsx
							<div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
```

- [ ] **Step 2: Stack the SMTP user/password grid**

In `general.tsx` replace (line 160):

```tsx
							<div className="grid grid-cols-2 gap-3">
```

with:

```tsx
							<div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
```

- [ ] **Step 3: Stack the notify channel row on mobile**

In `notify.tsx` replace the `ChannelRow` header row (line 516):

```tsx
				<div className="flex items-center justify-between px-4 py-3">
```

with:

```tsx
				<div className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
```

- [ ] **Step 4: Let the channel action cluster wrap**

In `notify.tsx` replace the right-hand action cluster opener (line 543):

```tsx
					<div className="flex items-center gap-1">
```

with:

```tsx
					<div className="flex flex-wrap items-center gap-1 self-end sm:self-auto">
```

- [ ] **Step 5: Type-check + lint**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 6: Browser check**

At **375px**:
- `/settings/general`: SMTP host/port and user/password fields stack one-per-row; no overflow; Save button reachable.
- `/settings/notify`: each channel row shows name/meta on top and the Test/edit/delete controls wrapped on a line below, right-aligned; the inline edit panel (including `CronPicker`) fits without horizontal scroll.
- `/scheduler`: confirm job rows and the `ScheduleForm` (`CronPicker`, media-app checkboxes) fit at 375px with no horizontal scroll. No code change is expected here; if the `CronPicker` overflows, add `flex-wrap` to its row container in `frontend/apps/web/src/components/cron-picker.tsx` and note it.

At **768px**: general grids are 3-col / 2-col and notify rows are single-line, as before.

- [ ] **Step 7: Commit**

```bash
git add frontend/apps/web/src/routes/settings/general.tsx frontend/apps/web/src/routes/settings/notify.tsx
git commit -m "feat(frontend): responsive settings forms on mobile"
```

---

### Task 9: Full responsive sweep + final verification

**Files:** none (verification only; commit any fixes the sweep requires under the relevant task above)

- [ ] **Step 1: Type-check + lint (whole frontend)**

Run: `cd frontend && bun check-types && bun check`
Expected: pass.

- [ ] **Step 2: Route-by-route mobile sweep at 375×812**

With the app running and logged in, open each route and confirm no horizontal page overflow, readable content, reachable controls, and a working hamburger drawer:
- `/` (Dashboard) — stat cards stack, API key row wraps, export rows truncate.
- `/subscriptions` — rows wrap actions; add/edit/check panels usable.
- `/subscriptions/$id` — node cards; job pills wrap; live-log readable.
- `/scheduler` — schedule form + job rows fit.
- `/settings/notify` — channel rows + edit panel fit.
- `/settings/platforms` — rule list; full-screen editor dialog usable.
- `/settings/general` — SMTP grids stacked.
- `/login` — centered card unaffected.

- [ ] **Step 3: Tablet sweep at 768×1024**

Confirm every route shows the **desktop** layout (sidebar visible, no top bar, tables/grids/dialog in their original form).

- [ ] **Step 4: Production build sanity**

Run: `cd frontend && bun build`
Expected: build succeeds.

- [ ] **Step 5: Final commit (only if the sweep produced fixes)**

```bash
git add -A frontend/apps/web/src
git commit -m "fix(frontend): mobile responsive polish from full-route sweep"
```

---

## Self-Review

**Spec coverage:**
- Responsive shell (breakpoint `md`, top bar vs sidebar, responsive `main` padding) → Tasks 1-3. ✓
- Shared nav content (`SidebarNav`) → Task 1. ✓
- Mobile nav + drawer (Base UI Dialog, close on nav/backdrop/ESC) → Tasks 2-3. ✓
- Dashboard API-key row wrap → Task 6. ✓
- Subscriptions list row wrap + touch targets → Task 5. ✓
- NodeTable mobile card view → Task 4. ✓
- Rule editor full-screen + stacked body + docs-as-section → Task 7. ✓
- Scheduler / settings reflow (general grids, notify row, CronPicker check) → Task 8 + Task 9 sweep. ✓
- Touch targets → Task 5 (icon buttons), Task 4 (card on/off `min-h-7`). ✓
- Verification (type-check, lint, browser at 375/768) → every task + Task 9. ✓
- Out of scope (backend, IA change, new libs, desktop changes) → respected. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code. The only conditional ("if CronPicker overflows, add flex-wrap") is a guarded contingency with an exact action, not a placeholder.

**Type consistency:** `SidebarNav({ onNavigate })` defined in Task 1 is consumed identically in Task 2. `MobileNav` (no props) defined in Task 2, imported in Task 3. `formatSpeed`, `UnlockIcons`, `CardField`, `NodeCard` defined and used within Task 4. `NodeTable` public props (`results`, `rules`, `onToggleEnabled`) unchanged, so `subscriptions/$id.tsx` callers still compile.
