# Mobile Responsive Support — Design

**Date:** 2026-06-09
**Status:** Approved
**Scope:** Frontend only (`frontend/apps/web`, `frontend/packages/ui`)

## Goal

Make the web app a complete, usable mobile experience on phones and portrait tablets,
without changing the existing desktop layout. "Complete" means: collapsible navigation,
no horizontal overflow, data tables reflow to cards, the rule-editor modal works on a
phone, and touch targets are comfortable.

## Constraints & Decisions

- **Breakpoint:** Tailwind `md` (768px) is the single divider. `< md` = mobile layout;
  `>= md` = current desktop layout, unchanged.
- **Navigation pattern:** hamburger + left slide-in drawer on mobile. Desktop keeps its
  fixed 220px sidebar.
- **Drawer primitive:** Base UI `Dialog` (`@base-ui/react@1.3.0`, already a dependency via
  the existing dropdown/select/checkbox components). It provides focus trap, ESC-to-close,
  scroll lock, and a backdrop for free. No new dependency.
- Tailwind v4 default breakpoints are used as-is (no theme override needed).

## Architecture

### 1. Responsive shell

**`routes/__root.tsx`** — the authed layout switches by breakpoint:

- Desktop (`>= md`): current row layout — `<DesktopSidebar>` + `<main>` — unchanged.
- Mobile (`< md`): vertical stack — `<MobileNav>` top bar + scrollable `<main>`.
- `<main>` padding becomes responsive: `px-4 py-4 md:px-6 md:py-6`. The
  `mx-auto max-w-5xl` content wrapper stays.
- The desktop sidebar is wrapped `hidden md:flex`; the mobile top bar is `md:hidden`.

### 2. Shared nav content

**`components/sidebar.tsx`** is split so the nav definition is not duplicated:

- Extract the inner content (logo, `NAV_ITEMS`, `BOTTOM_ITEMS`, theme toggle, logout)
  into a `SidebarNav` component that takes an optional `onNavigate?: () => void`
  callback (used by the drawer to close itself on link click).
- `Sidebar` (desktop `aside`, 220px, unchanged visually) renders `<SidebarNav />`.
- The drawer renders the same `<SidebarNav onNavigate={close} />`.

### 3. Mobile nav + drawer

**New `components/mobile-nav.tsx`:**

- Top bar (`md:hidden`, ~48px tall, `border-b`, `bg-card`): hamburger button (left) +
  "subs-check" logo. Sticky at top of the scroll container.
- Hamburger is the `Dialog.Trigger`. Drawer = `Dialog.Portal` > `Dialog.Backdrop`
  (dimmed) > `Dialog.Popup` anchored left, ~280px wide, full height, slide-in from left.
- `Dialog.Popup` renders `<SidebarNav onNavigate={() => setOpen(false)} />`.
- Open state is local (`useState`). The drawer closes on: link navigation (via
  `onNavigate`), backdrop click, and ESC (both handled by Base UI Dialog).
- Slide animation via Tailwind data-attribute states that Base UI sets on the popup
  (`data-[starting-style]` / `data-[ending-style]`), translating X from `-100%` to `0`.

### 4. Per-page reflow

- **Dashboard (`routes/index.tsx`):** stat grid already `sm:grid-cols-3`. Make the API-key
  action row wrap on narrow screens (`flex-wrap`); keep code blocks truncated. Export URL
  rows already truncate — verify no overflow.
- **Subscriptions list (`routes/subscriptions/index.tsx`):** the row is name (`flex-1`) +
  four action controls. On `< sm` the actions wrap to a second line under the name; ensure
  each control is at least ~36px tall for touch. The inline "Check options" and "Edit"
  panels already stack and are fine.
- **Subscription detail (`routes/subscriptions/$id.tsx`):**
  - Job-history pills already `flex-wrap` — fine.
  - **`NodeTable` mobile card view:** below `md`, render one stacked card per node instead
    of the 9-column table. Card layout: top row = node name (truncate) + status badge +
    enable/disable toggle; middle = a 2-column mini-grid of latency / down speed / up speed
    / traffic / country (omit empty); bottom = unlock-icon row (`flex-wrap`). At `>= md`,
    the existing `overflow-x-auto` table renders unchanged. Both views are driven by the
    same `results`/`rules` props.
  - Live-log rows already truncate; acceptable as-is on mobile.
- **Rule editor (`components/platforms/RuleEditorDialog.tsx`):** the hand-rolled
  `fixed inset-0` modal becomes full-screen on mobile.
  - Outer: `p-0 md:p-4`; inner panel `max-w-none rounded-none md:max-w-5xl md:rounded-xl`,
    `h-full md:h-auto`, `max-h-screen md:max-h-[94vh]`.
  - Header toolbar already `flex-wrap` — keep.
  - Body changes from side-by-side to `flex-col md:flex-row`. On mobile the `DocsPanel`
    becomes a collapsible overlay/section rather than a permanent side column; the Monaco
    editor area gets a sensible min-height so it remains usable.
- **Scheduler / settings (`routes/scheduler.tsx`, `routes/settings/general.tsx`,
  `routes/settings/notify.tsx`):** form pages. Verify inputs are full-width on mobile and
  the `CronPicker` does not overflow; apply minor `flex-wrap` / width fixes as needed.
  No structural redesign expected.

### 5. Touch targets

The densest interactive elements (the `text-[10px]` on/off toggles, `p-1.5` icon buttons)
get a mobile-only minimum size (e.g. `min-h-8` and slightly larger padding under a mobile
utility) while desktop stays compact via `md:` overrides.

## Components Touched

| File | Change |
|------|--------|
| `routes/__root.tsx` | Breakpoint-aware shell (top bar vs sidebar), responsive `main` padding |
| `components/sidebar.tsx` | Extract `SidebarNav`; desktop `aside` wraps `hidden md:flex` |
| `components/mobile-nav.tsx` | **New** — top bar + Base UI Dialog drawer |
| `components/node-table.tsx` | Add `< md` stacked card view alongside existing table |
| `components/platforms/RuleEditorDialog.tsx` | Full-screen on mobile; body stacks vertically |
| `routes/subscriptions/index.tsx` | Row action controls wrap on narrow screens |
| `routes/index.tsx` | API-key action row wraps |
| `routes/scheduler.tsx`, `routes/settings/*.tsx` | Minor input/width verification & fixes |

## Out of Scope

- No backend changes.
- No navigation information-architecture change (no bottom tab bar).
- No new component library or design-token changes.
- Desktop layout is not modified.

## Testing / Verification

- Run `encore run` + `cd frontend && bun dev`. Drive each route in a browser at 375px
  (phone) and 768px (tablet) viewports via Playwright:
  - Drawer opens from hamburger, closes on link nav / backdrop / ESC; focus is trapped.
  - No horizontal page overflow on any route.
  - `NodeTable` renders the card view < md and the table >= md.
  - Rule-editor modal is full-screen and usable on a phone (editor + docs reachable).
- `cd frontend && bun check-types` passes.
- `cd frontend && bun check` (Biome lint/format) passes.

## Risks

- Base UI `Dialog` animation wiring (data-attribute states) is the only unfamiliar piece;
  if slide animation proves fiddly, fall back to a simple fade — functionality is
  unaffected.
- `RuleEditorDialog` is the most complex reflow (Monaco + side panels); budget extra time
  for the docs-panel-as-overlay behavior.
