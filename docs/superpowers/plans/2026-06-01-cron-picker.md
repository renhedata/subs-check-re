# Cron Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fixed cron preset selectors in Notify and Scheduler pages with a visual, user-friendly cron builder backed by `react-js-cron` and `cronstrue`.

**Architecture:** A shared `CronPicker` component wraps `react-js-cron` and injects the project's existing shadcn `Select` components via `customSelectRenderer`, keeping antd CSS out of the bundle. The component handles an optional enable/disable toggle (Notify page) and a description line from `cronstrue`. Both `notify.tsx` and `scheduler.tsx` import the same component.

**Tech Stack:** `react-js-cron` v5 (visual builder), `cronstrue` (human-readable description), shadcn `Select` + `Checkbox`, Tailwind CSS, Bun

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `frontend/apps/web/src/components/cron-picker.tsx` | Shared visual cron builder |
| Modify | `frontend/apps/web/src/routes/settings/notify.tsx` | Replace `CRON_PRESETS` Select with CronPicker |
| Modify | `frontend/apps/web/src/routes/scheduler.tsx` | Replace preset buttons with CronPicker |

---

## Task 1: Install dependencies

**Files:**
- Modify: `frontend/package.json` (via bun add)

- [ ] **Step 1: Install packages**

Run from the repo root:

```bash
cd frontend && bun add react-js-cron cronstrue
```

Expected: packages added, no errors, `bun.lock` updated.

- [ ] **Step 2: Smoke-check types**

```bash
cd frontend && bun check-types 2>&1 | head -30
```

Expected: output contains no errors referencing `react-js-cron` or `cronstrue` (pre-existing unrelated errors are fine to ignore).

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/bun.lock
git commit -m "chore: add react-js-cron and cronstrue"
```

---

## Task 2: Build CronPicker component

**Files:**
- Create: `frontend/apps/web/src/components/cron-picker.tsx`

- [ ] **Step 1: Create the file**

Create `frontend/apps/web/src/components/cron-picker.tsx` with the following content:

```tsx
import { Checkbox } from "@frontend/ui/components/checkbox";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";
import { Cron } from "react-js-cron";
import "react-js-cron/dist/styles.css";
import cronstrue from "cronstrue";

interface CronPickerProps {
	value: string;
	onChange: (v: string) => void;
	/** When true, shows an enable/disable checkbox. Unchecked → onChange(""). */
	allowDisable?: boolean;
}

const DEFAULT_CRON = "0 * * * *";

function describe(cron: string): string {
	try {
		return cronstrue.toString(cron, { use24HourTimeFormat: true });
	} catch {
		return "";
	}
}

export function CronPicker({ value, onChange, allowDisable }: CronPickerProps) {
	const enabled = !allowDisable || value !== "";

	function handleToggle(checked: boolean) {
		onChange(checked ? DEFAULT_CRON : "");
	}

	function handleCronChange(next: string | ((prev: string) => string)) {
		onChange(typeof next === "function" ? next(value || DEFAULT_CRON) : next);
	}

	return (
		<div className="space-y-2">
			{allowDisable && (
				<label className="flex cursor-pointer select-none items-center gap-2">
					<Checkbox
						checked={enabled}
						onCheckedChange={(v) => handleToggle(v === true)}
					/>
					<span className="text-muted-foreground text-xs">Enable</span>
				</label>
			)}

			{enabled && (
				<>
					<div className="cron-picker-wrap text-sm text-foreground">
						<Cron
							value={value || DEFAULT_CRON}
							setValue={handleCronChange}
							humanizeLabels
							clearButton={false}
							customSelectRenderer={(props, defaultRenderer) => {
								if (props.multiple) return defaultRenderer(props);
								return (
									<Select
										key={props.key}
										value={String(props.value)}
										onValueChange={(v) => props.onChange(v)}
									>
										<SelectTrigger className="inline-flex h-7 w-auto text-xs">
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											{props.options.map((opt) => (
												<SelectItem
													key={String(opt.value)}
													value={String(opt.value)}
												>
													{String(opt.label)}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								);
							}}
						/>
					</div>
					<p className="text-[11px] text-muted-foreground/70">
						{describe(value || DEFAULT_CRON)}
					</p>
				</>
			)}
		</div>
	);
}
```

- [ ] **Step 2: Type-check the new file**

```bash
cd frontend && bun check-types 2>&1 | grep cron-picker
```

Expected: no output (no errors from this file).

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/components/cron-picker.tsx
git commit -m "feat: add CronPicker component"
```

---

## Task 3: Integrate CronPicker into notify.tsx

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/notify.tsx`

The file currently has:
- `CRON_PRESETS` constant at lines 41–48 — remove it
- `cronToLabel` at lines 62–65 — replace body to use cronstrue
- `ReportSettings` uses a `<Select>` for `unlockCron` — replace with `<CronPicker>`
- The existing `Select` import must be **kept** (it is also used for the channel Type dropdown)

- [ ] **Step 1: Add CronPicker import and cronstrue to notify.tsx**

At the top of `frontend/apps/web/src/routes/settings/notify.tsx`, add after the existing imports:

```ts
import cronstrue from "cronstrue";
import { CronPicker } from "@/components/cron-picker";
```

- [ ] **Step 2: Remove CRON_PRESETS and update cronToLabel**

Delete the `CRON_PRESETS` block (lines 41–48):

```ts
const CRON_PRESETS = [
	{ label: "Disabled", value: "" },
	{ label: "Every hour", value: "0 * * * *" },
	{ label: "Every 2 hours", value: "0 */2 * * *" },
	{ label: "Every 6 hours", value: "0 */6 * * *" },
	{ label: "Every 12 hours", value: "0 */12 * * *" },
	{ label: "Once a day", value: "0 0 * * *" },
] as const;
```

Replace the `cronToLabel` function body:

```ts
function cronToLabel(cron: string): string {
	if (!cron) return "disabled";
	try {
		return cronstrue.toString(cron, { use24HourTimeFormat: true });
	} catch {
		return cron;
	}
}
```

- [ ] **Step 3: Replace the Select in ReportSettings**

In `ReportSettings`, find and remove the entire `<div className="space-y-1.5">` block that contains the Select for `unlockCron` (the block starting with the `<Clock>` icon and ending after the `<p>` description). Replace it with:

```tsx
<div className="space-y-1.5">
	<div className="flex items-center gap-2">
		<Clock
			size={12}
			strokeWidth={1.5}
			className="text-muted-foreground"
		/>
		<Label className="text-muted-foreground text-xs">
			Scheduled network unlock report
		</Label>
	</div>
	<CronPicker allowDisable value={unlockCron} onChange={setUnlockCron} />
	<p className="text-[10px] text-muted-foreground/70">
		Periodically reports which streaming platforms are accessible from
		this server's network.
	</p>
</div>
```

- [ ] **Step 4: Type-check**

```bash
cd frontend && bun check-types 2>&1 | grep notify
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/settings/notify.tsx
git commit -m "feat(notify): replace cron preset select with CronPicker"
```

---

## Task 4: Integrate CronPicker into scheduler.tsx

**Files:**
- Modify: `frontend/apps/web/src/routes/scheduler.tsx`

The file currently has:
- `SCHEDULE_PRESETS` constant at lines 51–58 — remove it
- `cronToLabel` at lines 60–63 — replace body to use cronstrue
- `ScheduleForm` renders preset buttons — replace with `<CronPicker>`
- `selectedCron` initialises to `""` — change to `"0 * * * *"` and remove the `!selectedCron` check from the Save button

- [ ] **Step 1: Add imports**

In `scheduler.tsx`, add to the import block:

```ts
import cronstrue from "cronstrue";
import { CronPicker } from "@/components/cron-picker";
```

- [ ] **Step 2: Remove SCHEDULE_PRESETS and update cronToLabel**

Delete the `SCHEDULE_PRESETS` block (lines 51–58):

```ts
const SCHEDULE_PRESETS = [
	{ label: "1h", cron: "0 * * * *", desc: "Every hour" },
	{ label: "2h", cron: "0 */2 * * *", desc: "Every 2 hours" },
	{ label: "6h", cron: "0 */6 * * *", desc: "Every 6 hours" },
	{ label: "12h", cron: "0 */12 * * *", desc: "Every 12 hours" },
	{ label: "Daily", cron: "0 0 * * *", desc: "Once a day" },
	{ label: "Weekly", cron: "0 0 * * 0", desc: "Once a week" },
] as const;
```

Replace `cronToLabel`:

```ts
function cronToLabel(cron: string): string {
	if (!cron) return "Not scheduled";
	try {
		return cronstrue.toString(cron, { use24HourTimeFormat: true });
	} catch {
		return cron;
	}
}
```

- [ ] **Step 3: Change initial cron state**

In `SchedulerPage`, change:

```ts
const [selectedCron, setSelectedCron] = useState("");
```

to:

```ts
const [selectedCron, setSelectedCron] = useState("0 * * * *");
```

- [ ] **Step 4: Replace preset buttons in ScheduleForm and fix Save disabled state**

In `ScheduleForm`, remove the entire `{/* Schedule presets */}` block:

```tsx
{/* Schedule presets */}
<div className="space-y-1.5">
	<p className="text-muted-foreground text-xs">Schedule</p>
	<div className="flex flex-wrap gap-2">
		{SCHEDULE_PRESETS.map((preset) => {
			const active = selectedCron === preset.cron;
			return (
				<button
					key={preset.cron}
					type="button"
					onClick={() => setSelectedCron(preset.cron)}
					title={preset.desc}
					className="rounded-md border px-3 py-1 text-sm transition-colors"
					style={{
						borderColor: active ? "var(--primary)" : "var(--border)",
						color: active ? "var(--primary)" : "var(--muted-foreground)",
						background: active
							? "var(--color-badge-info-bg)"
							: "transparent",
					}}
				>
					{preset.label}
				</button>
			);
		})}
	</div>
	{selectedCron && (
		<p className="text-[11px]" style={{ color: "var(--color-dimmed)" }}>
			{SCHEDULE_PRESETS.find((p) => p.cron === selectedCron)?.desc}
		</p>
	)}
</div>
```

Replace with:

```tsx
<div className="space-y-1.5">
	<p className="text-muted-foreground text-xs">Schedule</p>
	<CronPicker value={selectedCron} onChange={setSelectedCron} />
</div>
```

In the same `ScheduleForm`, find the Save `<Button>` disabled prop:

```tsx
disabled={(!hideSubSelector && !subId) || !selectedCron || isPending}
```

Change to:

```tsx
disabled={(!hideSubSelector && !subId) || isPending}
```

- [ ] **Step 5: Type-check and lint**

```bash
cd frontend && bun check-types && bun check
```

Expected: no type errors. Fix any Biome lint issues if reported.

- [ ] **Step 6: Commit**

```bash
git add frontend/apps/web/src/routes/scheduler.tsx
git commit -m "feat(scheduler): replace cron preset buttons with CronPicker"
```

---

## Task 5: Visual verification

- [ ] **Step 1: Start servers**

```bash
# Terminal 1
encore run
# Terminal 2 (from repo root)
cd frontend && bun dev
```

- [ ] **Step 2: Verify notify page**

Open `http://localhost:3001/settings/notify`.

- Click **Add Channel**, fill in name and type.
- In Report Types, "Scheduled network unlock report" now shows a checkbox (unchecked = disabled).
- Check the box → builder appears with default "Every hour at minute 0".
- Change the period to "day" → contextual hour/minute selects appear → description updates.
- Save the channel and reopen edit → the saved cron is correctly pre-filled.
- The channel row tag (e.g. "At 09:00 AM, every day") renders via `cronToLabel`.

- [ ] **Step 3: Verify scheduler page**

Open `http://localhost:3001/scheduler`.

- Click **Add Schedule**, select a subscription.
- The schedule section shows the CronPicker builder (no preset buttons).
- Default is "Every hour at minute 0" — Save is enabled immediately.
- Changing the period updates the description and cron string.
- Existing job rows show human-readable cron labels (via cronstrue in `cronToLabel`).

- [ ] **Step 4: Commit any style fixes**

If visual adjustments were needed:

```bash
git add -p
git commit -m "fix(cron-picker): visual style adjustments"
```
