# Frontend Modernization Design

**Date:** 2026-03-15
**Scope:** Full visual + interaction overhaul of the React web app

---

## Goals

- Replace the current minimal UI with a GitHub-style dark theme
- Introduce a persistent sidebar navigation replacing the top navbar
- Improve interaction feedback: loading states, progress animation, status badges
- No emoji anywhere — use Lucide icons throughout

## Visual Design System

### Color Palette

| Token | Value | Usage |
|-------|-------|-------|
| `--bg-canvas` | `#0d1117` | Page background |
| `--bg-surface` | `#161b22` | Sidebar, cards |
| `--bg-overlay` | `#21262d` | Table rows, hover states |
| `--border` | `#30363d` | All borders |
| `--border-muted` | `#21262d` | Table row dividers |
| `--text-primary` | `#f0f6fc` | Headings, body text |
| `--text-secondary` | `#8b949e` | Labels, metadata |
| `--text-muted` | `#6e7681` | Disabled, dead nodes |
| `--accent-blue` | `#58a6ff` | Links, active nav, primary actions |
| `--accent-green` | `#3fb950` | Alive status, success |
| `--accent-red` | `#f85149` | Dead status, errors |
| `--accent-yellow` | `#d29922` | Medium latency (50–200ms) |
| `--btn-primary-bg` | `#238636` | Primary "Add/Save" buttons |

### Typography

- Font stack: `system-ui, -apple-system, sans-serif`
- Monospace (node names, URLs, cron): `ui-monospace, SFMono-Regular, monospace`
- Node table text: `12px`
- Section headings: `18px`, `font-weight: 600`
- Stat card numbers: `28px`, `font-weight: 700`
- Labels/meta: `11px`, uppercase, `letter-spacing: 0.4px`

### Component Patterns

**Cards / Panels:** `background: #161b22`, `border: 1px solid #30363d`, `border-radius: 8px`

**Status badges (pill):**
- Alive: `background: #1a4731`, `color: #3fb950`, inline dot + text
- Dead: `background: #3d1a1a`, `color: #f85149`

**Unlock badges (compact):**
- NF/YT: `background: #3d1a1a`, `color: #f85149`
- GPT/Claude/Gemini: `background: #1a3a1a`, `color: #3fb950` (Claude is green here, overriding the previous orange treatment — all AI platform unlocks use the same green)
- D+/TikTok: `background: #1a2a3a`, `color: #58a6ff`

**Latency color coding:**
- `< 50ms` → `#3fb950` (green)
- `50–200ms` → `#d29922` (yellow)
- `> 200ms` → `#f85149` (red)

**Icons:** Lucide only. No emoji. Size `14px` in nav, `13px` in table/cards, `stroke-width: 1.5`.

---

## Layout

### Shell (`__root.tsx`)

Replace the current `grid grid-rows-[auto_1fr]` + `<Header>` layout with a two-column flex layout:

```
┌───────────────────────────────────────────────────────────┐
│ Sidebar (220px fixed)  │ Main content (flex-1, scrollable) │
│  ┌──────────────────┐  │                                   │
│  │ Logo             │  │  <Outlet />                       │
│  │ ───────────────  │  │                                   │
│  │ Dashboard        │  │                                   │
│  │ Subscriptions    │  │                                   │
│  │ Scheduler        │  │                                   │
│  │ Notify           │  │                                   │
│  │ ───────────────  │  │                                   │
│  │ Settings         │  │                                   │
│  │ [user] Logout    │  │                                   │
│  └──────────────────┘  │                                   │
└───────────────────────────────────────────────────────────┘
```

**Sidebar dimensions:** `width: 220px`, fixed (no collapse). Full viewport height (`h-screen`). `flex-shrink: 0`.

**Login/unauthenticated routes:** The `/login` route does NOT render the sidebar. Implement this by conditionally rendering the layout: if `isAuthenticated()` is false, render `<Outlet />` alone (centered, full viewport); if authenticated, render the two-column sidebar + content layout. The `isAuthenticated()` check already exists in `src/lib/auth.ts`.

**ThemeProvider / ModeToggle:** Remove `<ThemeProvider>` from `__root.tsx` entirely. The app is always dark — set `dark` class on `<html>` via `index.css` (`html { color-scheme: dark; }`) and Tailwind's dark mode class. Delete `src/components/mode-toggle.tsx` and `src/components/theme-provider.tsx`. Delete `src/components/header.tsx`.

Active nav item: `background: rgba(31,111,235,0.13)`, `border: 1px solid rgba(31,111,235,0.27)`, text `#f0f6fc`.
Inactive: text `#8b949e`, hover: background `rgba(255,255,255,0.05)`, text `#e6edf3`.

### Sidebar Nav Icons (Lucide)

| Item | Icon |
|------|------|
| Dashboard | `LayoutDashboard` |
| Subscriptions | `List` |
| Scheduler | `Clock` |
| Notify | `Bell` |
| Settings | `Settings` |
| Logout | `LogOut` |
| User avatar | `User` |

---

## Pages

### Dashboard (`/`)

- Page subtitle: "Overview of your proxy subscriptions"
- Three stat cards in a 3-column grid:
  - **Subscriptions** (icon: `FileText`) — total count from `GET /subscriptions`
  - **Active** (icon: `CheckCircle`) — count of `enabled: true` subscriptions, green number
  - **Scheduled** (icon: `Clock`) — count of subscriptions with `cron_expr` set
- No "Recent Checks" panel. The three stat cards are sufficient. (The current dashboard already has exactly these three stats — this is a visual-only upgrade.)

### Subscriptions (`/subscriptions`)

- Header row: title + green "Add" button (`Plus` icon, `#238636` bg)
- Add form: inline card that slides in (currently toggle-shows), unchanged logic
- Subscription rows as cards:
  - Left: status dot (green=has results, grey=never checked), name as blue link, URL in monospace below
  - Right: node count badge, `▶ Check` button (outline), `Trash2` delete icon (ghost, hover red)
  - If `cron_expr`: show `Clock` icon + expression in muted text

### Subscription Detail (`/subscriptions/$id`)

- Page title: subscription name — `CheckJob` has no name field, so resolve it by looking up the subscription from the cached `useQuery(["subscriptions"])` result by `job.subscription_id`
- Progress bar (while running):
  - Thin `3px` bar, `background: linear-gradient(90deg, #1f6feb, #58a6ff)`, `border-radius: 2px`
  - Container: `height: 3px`, `background: #21262d`
  - `RefreshCw` icon (Lucide, `animate-spin`) beside "Checking nodes… N / M"
  - Current node name + latency + speed in monospace below, color-coded by latency rules
- Job summary row: status badge, total/available counts
- Node table: see table spec above

### Scheduler (`/scheduler`)

- Add form: replace raw `<select>` with shadcn `Select` component. **Note:** `Select` is not yet in the UI package — run `bunx shadcn add select` from `frontend/packages/ui/` before implementing this page.
- Job rows: show subscription name (resolved from subscriptions query, not raw UUID), `Clock` icon + cron expression, delete button

### Notify Settings (`/settings/notify`)

- Unchanged logic, apply consistent card/input styling

### General Settings (`/settings/general`)

- Unchanged logic, apply consistent styling

### Login (`/login`)

- Centered card, unchanged logic
- Replace plain toggle button with a styled link button
- Add a subtle `Shield` or `Lock` icon near the title

---

## Interaction Improvements

### Loading States

- Replace bare `<p>Loading...</p>` with a skeleton shimmer using shadcn `Skeleton` (already in ui package)
- Subscription list: 3 skeleton card rows while loading
- Dashboard stats: skeleton numbers

### Transitions / Feedback

- Nav active state: instant (no animation needed)
- Progress bar: CSS `transition: width 0.3s ease`
- Add form expand: no animation required (keep toggle)
- Toast notifications: already using Sonner with `richColors`, keep as-is

### Button States

- Disabled: `opacity: 0.5`, no pointer
- Loading (pending mutation): show spinner inline — replace button text with `<Loader2 className="animate-spin" />`

---

## Implementation Scope

**Files to create:**
- `src/components/sidebar.tsx` — new sidebar component

**Files to significantly modify:**
- `src/routes/__root.tsx` — replace Header with Sidebar, new layout shell
- `src/components/header.tsx` — delete (replaced by sidebar)
- `src/routes/index.tsx` — dashboard redesign + recent checks
- `src/routes/subscriptions/index.tsx` — visual overhaul
- `src/routes/subscriptions/$id.tsx` — progress bar + job summary improvements
- `src/routes/scheduler.tsx` — replace select, show sub names
- `src/routes/login.tsx` — minor polish
- `src/routes/settings/general.tsx` — styling consistency
- `src/routes/settings/notify.tsx` — styling consistency
- `src/components/node-table.tsx` — new badge styles, latency color coding

**CSS:** Update `src/index.css` with GitHub palette CSS variables; apply `bg-[#0d1117]` as body background via Tailwind or direct CSS.

**No backend changes required.**

---

## Out of Scope

- Mobile/responsive layout (sidebar collapses) — future work
- Sorting/filtering on the node table — future work
- Dark/light mode toggle — removed (always dark)
- Animation library — no new dependencies
