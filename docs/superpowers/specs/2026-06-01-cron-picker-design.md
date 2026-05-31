# Cron Picker — Design Spec

**Date:** 2026-06-01  
**Scope:** `frontend/apps/web/src/routes/settings/notify.tsx`, `frontend/apps/web/src/routes/scheduler.tsx`

## Problem

Both the Notify settings page (`unlock_cron`) and the Scheduler page (`cron_expr`) currently use a fixed list of preset options. Users cannot enter a custom cron expression, limiting scheduling flexibility.

## Goal

Replace the fixed presets with a visual cron builder that:
- Lets users construct any valid cron expression without knowing cron syntax
- Shows a human-readable description in real time (e.g. "Every day at 9:00 AM")
- Fits the existing dark-theme shadcn/ui design

## Library

**`react-js-cron`** — visual cron builder with `cronstrue` bundled for human-readable output.

Integration strategy: use the `components` prop to pass in the project's existing shadcn `Select` components so no antd styles are imported and the UI matches the existing design system.

## Shared Component

**File:** `frontend/apps/web/src/components/cron-picker.tsx`

```ts
interface CronPickerProps {
  value: string             // cron string; empty string = disabled
  onChange: (v: string) => void
  allowDisable?: boolean    // show enable/disable toggle (notify page)
}
```

### Behaviour

| Scenario | Behaviour |
|----------|-----------|
| `allowDisable=true`, toggle off | `onChange("")` called, builder hidden |
| `allowDisable=true`, toggle on | builder shown, initialises to `"0 * * * *"` if value was empty |
| `allowDisable=false` | builder always visible, no toggle |

### Layout (top to bottom)

1. **Toggle** (only when `allowDisable=true`) — enable/disable label + switch
2. **Builder** — react-js-cron component with shadcn Select injected via `components` prop
3. **Description** — `cronstrue.toString(value)` in `text-[11px] text-muted-foreground/70`; hidden when disabled

## Integration

### `notify.tsx` — `ReportSettings`

Replace the `<Select>` for `unlock_cron` with:

```tsx
<CronPicker allowDisable value={unlockCron} onChange={setUnlockCron} />
```

Remove `CRON_PRESETS` constant and the `cronToLabel` helper (no longer needed in this file).

### `scheduler.tsx` — `ScheduleForm`

Replace the preset-buttons block (`SCHEDULE_PRESETS.map(...)`) with:

```tsx
<CronPicker value={selectedCron} onChange={setSelectedCron} />
```

Keep `SCHEDULE_PRESETS` in `cronToLabel` only if it is still used in the job row display; otherwise remove it too.

## Styling

- Wrap react-js-cron in a container with `className="space-y-1.5"` consistent with surrounding form fields
- Pass shadcn `Select`, `SelectTrigger`, `SelectContent`, `SelectItem` via the `components` prop
- No antd CSS import; use only existing Tailwind/CSS-var styles

## Out of Scope

- Backend changes (cron strings are already stored and validated server-side)
- Cron validation error display (react-js-cron handles invalid state internally)
- "Next run" time preview (human-readable description is sufficient)
