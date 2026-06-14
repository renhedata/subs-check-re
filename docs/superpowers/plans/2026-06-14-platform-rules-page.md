# Platform Rules — Sidebar Entry + Page Redesign (Plan 2 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote Platform Rules from a Settings tab to a top-level `/rules` sidebar entry, and give it a focused professional redesign: an action bar, a drag-sortable rule list, and a right-side **Sheet** editor (with the new icon picker + type editor + inline test).

**Architecture:** Add a `Sheet` UI primitive (right slide-over on Base UI Dialog) and `@dnd-kit` for reorder. Move the page logic to a new `/rules` route, wire it into the rail + mobile tabbar, and remove the Settings tab. The editor reuses the existing `RuleEditorDialog` form, re-housed in a Sheet.

**Tech Stack:** React + TanStack Router (file-based; `routeTree.gen.ts` is generated — do not hand-edit), Base UI Dialog, `@dnd-kit/core`+`@dnd-kit/sortable` (new deps), Biome (tabs).

**Depends on:** Plan 1 (`2026-06-14-platform-rules-icons.md`) — the new icon picker (upload/quick) and `RuleIcon` are used here. **Spec:** `docs/superpowers/specs/2026-06-14-platform-rules-experience-design.md`. **Branch:** `feat/platform-rules-experience`.

---

## Conventions

- From `frontend/`: `bun check-types`, `bun run build`. TanStack regenerates `routeTree.gen.ts` on `bun dev`/build — commit it when it changes.
- Read `frontend/src/components/platforms/RuleEditorDialog.tsx` and `RuleCard.tsx` before Tasks 2 and 4 — this plan reuses their form/field logic verbatim and only re-houses it.

## File Structure

| File | Responsibility |
|------|----------------|
| `frontend/src/components/ui/sheet.tsx` | NEW — right slide-over primitive (Task 1) |
| `frontend/src/components/platforms/SortableRuleList.tsx` | NEW — dnd-sortable rule rows (Task 2) |
| `frontend/src/routes/rules.tsx` | NEW — the `/rules` page (Task 3 + 4) |
| `frontend/src/components/rail.tsx`, `mobile-tabbar.tsx`, `routes/settings.tsx`, `routes/settings/platforms.tsx` | nav wiring + remove old tab (Task 3) |
| `frontend/src/components/platforms/RuleEditorDialog.tsx` | re-house form in a Sheet (Task 4) |

---

## Task 1: `Sheet` primitive (right slide-over)

**Files:**
- Create: `frontend/src/components/ui/sheet.tsx`

- [ ] **Step 1: Implement**

Create `frontend/src/components/ui/sheet.tsx` (mirrors `ui/dialog.tsx` but anchored right, full-height, slides from the right; reuses Base UI Dialog):

```tsx
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";

const Sheet = DialogPrimitive.Root;
const SheetTrigger = DialogPrimitive.Trigger;
const SheetClose = DialogPrimitive.Close;

function SheetContent({
	className,
	children,
	...props
}: DialogPrimitive.Popup.Props) {
	return (
		<DialogPrimitive.Portal>
			<DialogPrimitive.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
			<DialogPrimitive.Popup
				className={cn(
					"fixed top-0 right-0 z-50 flex h-screen w-full flex-col overflow-y-auto bg-popover p-5 text-popover-foreground outline-none",
					"sm:max-w-lg sm:border-border sm:border-l sm:shadow-[var(--shadow-dialog)]",
					"transition-transform duration-200 data-[ending-style]:translate-x-full data-[starting-style]:translate-x-full",
					className,
				)}
				{...props}
			>
				{children}
				<DialogPrimitive.Close
					aria-label="Close"
					className="absolute top-4 right-4 rounded-md p-1 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					<XIcon className="size-4" />
				</DialogPrimitive.Close>
			</DialogPrimitive.Popup>
		</DialogPrimitive.Portal>
	);
}

function SheetTitle({ className, ...props }: DialogPrimitive.Title.Props) {
	return (
		<DialogPrimitive.Title
			className={cn("font-semibold text-[15px] text-foreground", className)}
			{...props}
		/>
	);
}

function SheetDescription({
	className,
	...props
}: DialogPrimitive.Description.Props) {
	return (
		<DialogPrimitive.Description
			className={cn("mt-0.5 text-muted-foreground text-xs", className)}
			{...props}
		/>
	);
}

function SheetFooter({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			className={cn("mt-auto flex justify-end gap-2 pt-5", className)}
			{...props}
		/>
	);
}

export {
	Sheet,
	SheetClose,
	SheetContent,
	SheetDescription,
	SheetFooter,
	SheetTitle,
	SheetTrigger,
};
```

- [ ] **Step 2: Verify**

Run (from `frontend/`): `bun check-types`
Expected: PASS (additive; `@base-ui/react/dialog` is already used by `ui/dialog.tsx`).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/sheet.tsx
git commit -m "feat(frontend): Sheet primitive (right slide-over)"
```

---

## Task 2: `@dnd-kit` + sortable rule list

**Files:**
- Modify: `frontend/package.json` (add deps)
- Create: `frontend/src/components/platforms/SortableRuleList.tsx`

- [ ] **Step 1: Add deps**

Run (from `frontend/`): `bun add @dnd-kit/core @dnd-kit/sortable @dnd-kit/utilities`
Expected: added to `package.json` + `bun.lock`.

- [ ] **Step 2: Implement the sortable list**

First read `frontend/src/components/platforms/RuleCard.tsx` to reuse its row visuals (icon via the new `RuleIcon`, name, key badge, type badge, enable toggle, edit/delete). Create `frontend/src/components/platforms/SortableRuleList.tsx`:

```tsx
import {
	DndContext,
	type DragEndEvent,
	PointerSensor,
	useSensor,
	useSensors,
} from "@dnd-kit/core";
import {
	SortableContext,
	useSortable,
	verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { RuleIcon } from "@/components/rule-icon";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import type { checker } from "@/lib/client.gen";
import { cn } from "@/lib/utils";
import { isApiError } from "@/lib/client";
import { useDeleteRule, useUpdateRule } from "@/queries";

type PlatformRule = checker.PlatformRule;

function Row({
	rule,
	onEdit,
}: {
	rule: PlatformRule;
	onEdit: (r: PlatformRule) => void;
}) {
	const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
		useSortable({ id: rule.id });
	const updateMut = useUpdateRule();
	const deleteMut = useDeleteRule();

	const patch = (p: Partial<checker.UpdateRuleParams>) =>
		updateMut.mutate({
			id: rule.id,
			params: {
				name: rule.name,
				icon: rule.icon,
				enabled: rule.enabled,
				rule_type: rule.rule_type,
				definition: rule.definition,
				sort_order: rule.sort_order,
				...p,
			},
		});

	return (
		<div
			ref={setNodeRef}
			style={{ transform: CSS.Transform.toString(transform), transition }}
			className={cn(
				"flex items-center gap-2 rounded-lg border border-border bg-card px-2 py-2",
				isDragging && "opacity-60",
			)}
		>
			<button
				type="button"
				className="cursor-grab text-muted-foreground/60 hover:text-foreground"
				aria-label="Drag to reorder"
				{...attributes}
				{...listeners}
			>
				<GripVertical size={15} />
			</button>
			<RuleIcon icon={rule.icon} label={rule.name} size={18} />
			<button
				type="button"
				onClick={() => onEdit(rule)}
				className="min-w-0 flex-1 truncate text-left font-medium text-foreground text-sm hover:text-primary"
			>
				{rule.name}
			</button>
			<code className="hidden rounded bg-secondary px-1 font-mono text-[10px] text-muted-foreground sm:inline">
				{rule.key}
			</code>
			<Badge tone="neutral">{rule.rule_type}</Badge>
			{rule.is_default ? <Badge tone="info">default</Badge> : null}
			<Switch
				checked={rule.enabled}
				onCheckedChange={(v) => patch({ enabled: v === true })}
			/>
			<button
				type="button"
				onClick={() => onEdit(rule)}
				className="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-foreground"
				aria-label="Edit"
			>
				<Pencil size={14} />
			</button>
			<button
				type="button"
				onClick={() =>
					deleteMut.mutate(rule.id, {
						onError: (e) =>
							toast.error(isApiError(e) ? e.message : "Failed to delete"),
					})
				}
				className="rounded p-1 text-muted-foreground hover:bg-secondary hover:text-danger"
				aria-label="Delete"
			>
				<Trash2 size={14} />
			</button>
		</div>
	);
}

export function SortableRuleList({
	rules,
	onEdit,
}: {
	rules: PlatformRule[];
	onEdit: (r: PlatformRule) => void;
}) {
	const sensors = useSensors(
		useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
	);
	const updateMut = useUpdateRule();

	const onDragEnd = (e: DragEndEvent) => {
		const { active, over } = e;
		if (!over || active.id === over.id) return;
		const oldIndex = rules.findIndex((r) => r.id === active.id);
		const newIndex = rules.findIndex((r) => r.id === over.id);
		if (oldIndex < 0 || newIndex < 0) return;
		// Persist the new sort_order for every rule whose position changed.
		const reordered = [...rules];
		const [moved] = reordered.splice(oldIndex, 1);
		reordered.splice(newIndex, 0, moved);
		reordered.forEach((r, i) => {
			if (r.sort_order !== i) {
				updateMut.mutate({
					id: r.id,
					params: {
						name: r.name,
						icon: r.icon,
						enabled: r.enabled,
						rule_type: r.rule_type,
						definition: r.definition,
						sort_order: i,
					},
				});
			}
		});
	};

	return (
		<DndContext sensors={sensors} onDragEnd={onDragEnd}>
			<SortableContext
				items={rules.map((r) => r.id)}
				strategy={verticalListSortingStrategy}
			>
				<div className="space-y-1.5">
					{rules.map((rule) => (
						<Row key={rule.id} rule={rule} onEdit={onEdit} />
					))}
				</div>
			</SortableContext>
		</DndContext>
	);
}
```

> Confirm `Badge`'s `tone` values (`neutral`/`info`/`danger`…) against `ui/badge.tsx` and adjust if the prop differs. Confirm `isApiError` is exported from `@/lib/client` (it is — used by export-tags/notify-dialog).

- [ ] **Step 3: Verify**

Run (from `frontend/`): `bun check-types`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/platforms/SortableRuleList.tsx frontend/package.json frontend/bun.lock
git commit -m "feat(frontend): @dnd-kit sortable rule list (persists sort_order)"
```

---

## Task 3: Move to `/rules` + sidebar nav

**Files:**
- Create: `frontend/src/routes/rules.tsx`
- Delete: `frontend/src/routes/settings/platforms.tsx`
- Modify: `frontend/src/routes/settings.tsx` (drop the tab), `rail.tsx`, `mobile-tabbar.tsx`

- [ ] **Step 1: Create the `/rules` route (logic moved from the old page)**

Create `frontend/src/routes/rules.tsx` — same logic as the old `settings/platforms.tsx` for now (redesign is Task 4), but at the new path and rendered full-width:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { Plus, Tv2 } from "lucide-react";
import { useState } from "react";
import { useMonacoSetup } from "@/components/platforms/engine";
import { RuleEditorDialog } from "@/components/platforms/RuleEditorDialog";
import { SortableRuleList } from "@/components/platforms/SortableRuleList";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import type { checker } from "@/lib/client.gen";
import { useRules } from "@/queries";

type PlatformRule = checker.PlatformRule;

export const Route = createFileRoute("/rules")({
	component: RulesPage,
});

function RulesPage() {
	useMonacoSetup();
	const [editingRule, setEditingRule] = useState<PlatformRule | null>(null);
	const [addOpen, setAddOpen] = useState(false);
	const [query, setQuery] = useState("");

	const { data, isLoading } = useRules();
	const rules = data?.rules ?? [];
	const filtered = query.trim()
		? rules.filter(
				(r) =>
					r.name.toLowerCase().includes(query.toLowerCase()) ||
					r.key.toLowerCase().includes(query.toLowerCase()),
			)
		: rules;

	return (
		<div className="mx-auto max-w-3xl px-4 py-6">
			<div className="mb-4 flex items-center gap-3">
				<div className="min-w-0 flex-1">
					<h1 className="font-semibold text-foreground text-lg">Platform Rules</h1>
					<p className="text-muted-foreground text-xs">
						Rules run during each proxy check and the server network-unlock probe.
						Enable a rule to detect it; drag to reorder.
					</p>
				</div>
				<Button variant="success" size="sm" onClick={() => setAddOpen(true)}>
					<Plus size={13} /> Add Rule
				</Button>
			</div>

			<input
				value={query}
				onChange={(e) => setQuery(e.target.value)}
				placeholder="Search rules…"
				className="mb-3 h-8 w-full rounded-md border border-border bg-background px-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
			/>

			{isLoading ? (
				<div className="space-y-2">
					<Skeleton className="h-12 w-full" />
					<Skeleton className="h-12 w-full" />
				</div>
			) : filtered.length === 0 ? (
				<div className="rounded-lg border border-border">
					<EmptyState
						icon={Tv2}
						title={query ? "No matching rules" : "No rules yet"}
						description="Add a detection rule to test custom platforms during checks."
					/>
				</div>
			) : (
				<SortableRuleList rules={filtered} onEdit={setEditingRule} />
			)}

			{addOpen && <RuleEditorDialog onClose={() => setAddOpen(false)} />}
			{editingRule && (
				<RuleEditorDialog
					rule={editingRule}
					onClose={() => setEditingRule(null)}
				/>
			)}
		</div>
	);
}
```

> Drag-reorder is disabled while a search filter is active in a follow-up if needed; for v1 reorder on the filtered list still persists `sort_order` by the displayed order — acceptable since reorder is rarely combined with search.

- [ ] **Step 2: Delete the old route + Settings tab**

- Delete `frontend/src/routes/settings/platforms.tsx`.
- In `frontend/src/routes/settings.tsx`, remove the `{ to: "/settings/platforms", label: "Platform Rules" }` entry from the `TABS` array.

- [ ] **Step 3: Add the sidebar entry**

In `frontend/src/components/rail.tsx`, add to `NAV_ITEMS` (after the Scheduler entry, before Settings) and import `Radar`:

```tsx
import { Clock, List, LogOut, Moon, Radar, Settings, Sun } from "lucide-react";
```

```tsx
	{
		to: "/rules",
		label: "Platform Rules",
		icon: Radar,
		exact: false,
		matchPrefix: "/rules",
	},
```

In `frontend/src/components/mobile-tabbar.tsx`, add the same `/rules` entry (label "Rules", icon `Radar`) to its nav array, matching the existing item shape in that file (read it first; mirror the Subscriptions/Scheduler/Settings entries).

- [ ] **Step 4: Verify**

Run (from `frontend/`): `bun check-types` then `bun run build`.
Expected: PASS; `routeTree.gen.ts` regenerates with `/rules` and without `/settings/platforms` (commit it).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/rules.tsx frontend/src/routes/settings.tsx frontend/src/components/rail.tsx frontend/src/components/mobile-tabbar.tsx frontend/src/routeTree.gen.ts
git rm frontend/src/routes/settings/platforms.tsx
git commit -m "feat(frontend): Platform Rules is a top-level /rules sidebar entry"
```

---

## Task 4: Re-house the editor in the Sheet + inline test

**Files:**
- Modify: `frontend/src/components/platforms/RuleEditorDialog.tsx`

- [ ] **Step 1: Convert the editor container to `Sheet`**

Read `RuleEditorDialog.tsx` fully. Keep ALL of its form state, fields (`name`, `key`, the `IconPickerInput`, `rule_type` select, the type-specific definition editor, `sort_order`, `enabled`), the `useCreateRule`/`useUpdateRule` save logic, and the existing inline **Test** panel + node picker + `ConsolePanel` if present — change ONLY the container:

- Replace the `Dialog`/`DialogContent`/`DialogTitle`/`DialogDescription`/`DialogFooter` imports from `@/components/ui/dialog` with the `Sheet`/`SheetContent`/`SheetTitle`/`SheetDescription`/`SheetFooter` equivalents from `@/components/ui/sheet`.
- Swap the JSX wrapper tags accordingly (`<Dialog open onOpenChange>` → `<Sheet open onOpenChange>`, `<DialogContent>` → `<SheetContent>`, etc.). Base UI's Dialog API is identical, so only the component names change.
- Widen the content for a comfortable editor: add `className="sm:max-w-xl"` (or `sm:max-w-2xl` if the test panel is side-by-side) to `<SheetContent>`.

If `RuleEditorDialog` does not already include an inline test panel, add one using `useTestRule()` + `useTestNodes()`: a node `<Select>`, a "Test" button that calls `testMut.mutate({ rule_type, definition, node_id })`, and render the result (`ok`/`error` + status/body) — but only if it's not already there; do not duplicate.

- [ ] **Step 2: Verify**

Run (from `frontend/`): `bun check-types` then `bun run build`.
Expected: PASS. The editor now opens as a right-side sheet.

- [ ] **Step 3: Browser check**

`bun dev` + `encore run`: navigate to `/rules` (sidebar Radar icon). Confirm: the action bar + search, drag-to-reorder persists across refresh, the right-side Sheet editor opens with the icon picker (search/quick/upload from Plan 1), and the inline test runs. Disable a rule → it drops out of the server network-unlock strip.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/platforms/RuleEditorDialog.tsx
git commit -m "feat(frontend): rule editor opens as a right-side Sheet"
```

---

## Self-Review Notes

- **Spec coverage (Section 4):** Sheet primitive → Task 1. dnd sortable list → Task 2. `/rules` route + rail + mobile tabbar + remove Settings tab → Task 3. Right-side sheet editor + inline test → Task 4. Icon picker (Plan 1) is reused by the editor.
- **Placeholder scan:** none. Two "read X first / confirm prop" notes (RuleEditorDialog, mobile-tabbar shape, Badge tones) are verification steps, not placeholders — the surrounding code is exact.
- **Type consistency:** `SortableRuleList({rules, onEdit})`, `RuleEditorDialog({rule?, onClose})` (unchanged signature), `useUpdateRule().mutate({id, params})` matches `queries/rules.ts`. The `Sheet*` exports match the names used in Task 4.
- **Risk:** moving the route changes the generated `routeTree.gen.ts` — must be committed. Reorder-while-filtered is a known minor edge (documented).
