# Platform Rules — Master–Detail Inspector Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `/rules` page (centered list + slide-over Sheet editor) with a professional master–detail inspector: left rule list, right inline inspector/editor, no dialog/sheet.

**Architecture:** A redesign that is frontend-only except for **Task 0** (customizable built-in rules + a Reset endpoint). It **restructures existing, working pieces** (`ConditionEditor`, `ScriptEditorArea`, `ConsolePanel`, `DocsPanel`, `IconPicker`, `engine` helpers, the save/test logic from `RuleEditorDialog`) into new focused components: `RuleListPane`, `RuleInspector`, `RuleDefinitionEditor`, `RuleTestPanel`, `IconPickerPopover`. Build new alongside old (build stays green), swap the page, then delete the old (`RuleEditorDialog`, `SortableRuleList`, `ui/sheet`, `RuleCard`).

**Tech Stack:** React + TanStack Router, `@dnd-kit` (already a dep), Base UI Popover, Monaco (existing `engine`), Biome (tabs). All `bun` commands from `frontend/`.

**Spec:** `docs/superpowers/specs/2026-06-14-platform-rules-inspector-redesign-design.md`. **Branch:** `feat/platform-rules-experience`. Match the approved mockups in `.superpowers/brainstorm/69568-1781411761/content/` (`b-refined.html`, `states.html`) for visual fidelity, but use the app's theme tokens (`bg-card`, `border-border`, `text-success`, `text-muted-foreground`, `--color-active-bg`/`--color-active-border`, the `Button`/`Switch`/`Badge` primitives) — NOT the mockup hex.

---

## Conventions

- Build stays green after every task: Tasks 1–3 add new files (unimported) → Task 4 swaps `rules.tsx` → Task 5 deletes old.
- Reused sub-component interfaces (from reading `RuleEditorDialog.tsx`):
  - `ConditionEditor({ def: Record<string,unknown>, onChange })`
  - `ScriptEditorArea({ def, onChange, lang: RuleType, monacoTheme, activeTab, onTabChange })`
  - `ConsolePanel({ result: TestRuleResult|null, loading: boolean, nodeLabel: string })`
  - `DocsPanel({ ruleType })`
  - `engine`: `defaultDef(type)`, `RULE_TYPES`, `RULE_TYPE_LABELS`, `type RuleType`, `useMonacoSetup()`
  - Hooks: `useRules`, `useCreateRule`, `useUpdateRule`, `useDeleteRule`, `useTestNodes` (+ `client.checker.TestRule`)
- Verify after each task: `bun check-types`.

## File Structure

| File | Action |
|------|--------|
| `frontend/src/components/platforms/IconPickerPopover.tsx` | **New** — clickable icon tile → popover (search/quick/emoji/upload + preview) |
| `frontend/src/components/platforms/RuleListPane.tsx` | **New** — left pane: search, `+`, Built-in/Custom grouped sortable list, selection |
| `frontend/src/components/platforms/RuleInspector.tsx` | **New** — right pane: identity header + definition editor + test + footer (edit + draft) |
| `frontend/src/routes/rules.tsx` | **Rewrite** — master–detail shell, responsive, empty states |
| `frontend/src/components/platforms/RuleEditorDialog.tsx`, `SortableRuleList.tsx`, `RuleCard.tsx`, `components/ui/sheet.tsx` | **Delete** (Task 5) |

> `RuleDefinitionEditor` and `RuleTestPanel` from the spec are realized **inline inside `RuleInspector`** to keep file count sane (the inspector is the one owner of edit state). If `RuleInspector.tsx` exceeds ~250 lines, extract them then.

---

## Task 0: Backend — customizable built-in rules + Reset

**Files:**
- Create: `services/checker/migrations/23_rule_customized.up.sql` + `.down.sql`
- Modify: `services/checker/rules.go` (struct field, loaders, `UpdateRule`, new `ResetRule`, helper)
- Modify: `services/checker/rules_defaults.go` (`syncDefaultRules` respects `customized`, stops resetting `sort_order`)
- Test: `services/checker/rules_reset_test.go`
- Regenerate the typed client at the end.

Makes built-in rule edits persist (instead of being overwritten by `syncDefaultRules`) and adds a reset-to-default endpoint.

- [ ] **Step 1: Write the failing test**

`services/checker/rules_reset_test.go`:

```go
package checker

import (
	"context"
	"encoding/json"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func TestCustomizeAndResetBuiltinRule(t *testing.T) {
	userID := "reset-user-" + uuid.New().String()
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	ctx := context.Background()

	if err := syncDefaultRules(ctx, userID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// find the seeded netflix rule
	var id string
	if err := db.QueryRow(ctx,
		`SELECT id FROM platform_rules WHERE user_id=$1 AND key='netflix'`, userID).Scan(&id); err != nil {
		t.Fatalf("find netflix: %v", err)
	}

	// edit its content -> should mark customized
	edited := json.RawMessage(`{"code":"return {unlocked:true,status:\"Yes\",region:\"ZZ\"};"}`)
	if _, err := UpdateRule(ctx, id, &UpdateRuleParams{
		Name: "Netflix", Icon: "simple-icons:netflix", Enabled: true,
		RuleType: "js", Definition: edited, SortOrder: 0,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	var customized bool
	db.QueryRow(ctx, `SELECT customized FROM platform_rules WHERE id=$1`, id).Scan(&customized)
	if !customized {
		t.Fatal("editing a built-in rule's content must set customized=true")
	}

	// sync again -> must NOT overwrite the customized rule
	if err := syncDefaultRules(ctx, userID); err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	var defAfter string
	db.QueryRow(ctx, `SELECT definition::text FROM platform_rules WHERE id=$1`, id).Scan(&defAfter)
	if defAfter != string(edited) {
		t.Fatalf("sync overwrote a customized rule: %s", defAfter)
	}

	// reset -> back to seed + customized cleared
	if _, err := ResetRule(ctx, id); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var cust2 bool
	var def2 string
	db.QueryRow(ctx, `SELECT customized, definition::text FROM platform_rules WHERE id=$1`, id).Scan(&cust2, &def2)
	if cust2 {
		t.Fatal("reset must clear customized")
	}
	if def2 == string(edited) {
		t.Fatal("reset must restore the seed definition")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestCustomizeAndResetBuiltinRule`
Expected: FAIL — `customized` column / `ResetRule` don't exist yet (compile error).

- [ ] **Step 3: Migration**

`services/checker/migrations/23_rule_customized.up.sql`:

```sql
ALTER TABLE platform_rules ADD COLUMN customized boolean NOT NULL DEFAULT false;
```

`services/checker/migrations/23_rule_customized.down.sql`:

```sql
ALTER TABLE platform_rules DROP COLUMN customized;
```

- [ ] **Step 4: `rules.go` — field, loaders, UpdateRule, ResetRule, helper**

1. Add `"reflect"` to imports. Add the field to `PlatformRule` (after `IsDefault`):

```go
	IsDefault  bool            `json:"is_default"`
	Customized bool            `json:"customized"`
```

2. `loadUserRules` — add `customized` to the SELECT (after `is_default`) and to the Scan (after `&r.IsDefault`):

```go
		`SELECT id, user_id, name, key, icon, enabled, rule_type, definition, is_default, customized, sort_order, created_at, updated_at
		 FROM platform_rules WHERE user_id=$1 ORDER BY sort_order, created_at`,
```
```go
			if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Key, &r.Icon, &r.Enabled, &r.RuleType,
				&rawDef, &r.IsDefault, &r.Customized, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
```

3. Add the seed-comparison helper (near `defaultRuleByKey`, but it lives in rules.go is fine):

```go
// ruleContentMatchesSeed reports whether a rule's editable content equals the seed.
func ruleContentMatchesSeed(name, icon, ruleType string, def json.RawMessage, seed defaultRule) bool {
	if name != seed.name || icon != seed.icon || ruleType != seed.ruleType {
		return false
	}
	seedJSON, _ := json.Marshal(seed.def)
	var a, b any
	if json.Unmarshal(seedJSON, &a) != nil || json.Unmarshal(def, &b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}
```

4. `UpdateRule` — compute `customized` and write it. Replace the body between the `validRuleTypes` check and the final read-back SELECT:

```go
	now := time.Now()
	defJSON, _ := json.Marshal(p.Definition)

	// Determine customized: a built-in becomes customized when its content
	// diverges from the seed (enable-toggle / reorder alone don't count).
	var isDefault bool
	var ruleKey string
	if err := db.QueryRow(ctx,
		`SELECT is_default, key FROM platform_rules WHERE id=$1 AND user_id=$2`,
		ruleId, claims.UserID).Scan(&isDefault, &ruleKey); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("rule not found").Err()
	}
	customized := false
	if isDefault {
		if seed, ok := defaultRuleByKey(ruleKey); ok {
			customized = !ruleContentMatchesSeed(p.Name, p.Icon, p.RuleType, defJSON, seed)
		} else {
			customized = true
		}
	}

	result, err := db.Exec(ctx, `
		UPDATE platform_rules
		SET name=$3, icon=$4, enabled=$5, rule_type=$6, definition=$7, sort_order=$8, customized=$9, updated_at=$10
		WHERE id=$1 AND user_id=$2
	`, ruleId, claims.UserID, p.Name, p.Icon, p.Enabled, p.RuleType, defJSON, p.SortOrder, customized, now)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to update rule").Err()
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("rule not found").Err()
	}
```

Then update the read-back SELECT/Scan at the end of `UpdateRule` to include `customized` (after `is_default`):

```go
	if err := db.QueryRow(ctx,
		`SELECT id, user_id, name, key, icon, enabled, rule_type, definition, is_default, customized, sort_order, created_at, updated_at
		 FROM platform_rules WHERE id=$1`,
		ruleId,
	).Scan(&rule.ID, &rule.UserID, &rule.Name, &rule.Key, &rule.Icon, &rule.Enabled, &rule.RuleType,
		&rawDef, &rule.IsDefault, &rule.Customized, &rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
```

5. Add `ResetRule` (after `DeleteRule`):

```go
// ResetRule restores a built-in rule to its seeded default and clears the
// customized flag. The enabled state is preserved.
//
//encore:api auth method=POST path=/platform-rules/:ruleId/reset
func ResetRule(ctx context.Context, ruleId string) (*PlatformRule, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var isDefault bool
	var ruleKey string
	if err := db.QueryRow(ctx,
		`SELECT is_default, key FROM platform_rules WHERE id=$1 AND user_id=$2`,
		ruleId, claims.UserID).Scan(&isDefault, &ruleKey); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("rule not found").Err()
	}
	seed, ok := defaultRuleByKey(ruleKey)
	if !isDefault || !ok {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("not a built-in rule").Err()
	}
	defJSON, _ := json.Marshal(seed.def)
	if _, err := db.Exec(ctx, `
		UPDATE platform_rules
		SET name=$3, icon=$4, rule_type=$5, definition=$6, sort_order=$7, customized=false, updated_at=$8
		WHERE id=$1 AND user_id=$2
	`, ruleId, claims.UserID, seed.name, seed.icon, seed.ruleType, defJSON, seed.sortOrder, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to reset rule").Err()
	}

	var rule PlatformRule
	var rawDef []byte
	if err := db.QueryRow(ctx,
		`SELECT id, user_id, name, key, icon, enabled, rule_type, definition, is_default, customized, sort_order, created_at, updated_at
		 FROM platform_rules WHERE id=$1`,
		ruleId,
	).Scan(&rule.ID, &rule.UserID, &rule.Name, &rule.Key, &rule.Icon, &rule.Enabled, &rule.RuleType,
		&rawDef, &rule.IsDefault, &rule.Customized, &rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to read reset rule").Err()
	}
	rule.Definition = rawDef
	return &rule, nil
}
```

6. `CreateRule`'s returned struct literal — add `Customized: false,` (it already sets `IsDefault: false`).

- [ ] **Step 5: `rules_defaults.go` — sync respects customized, keeps user order**

In `syncDefaultRules`, change the `ON CONFLICT … DO UPDATE` so it (a) no longer resets `sort_order`, and (b) skips customized rows:

```go
			ON CONFLICT (user_id, key) DO UPDATE SET
			  name=EXCLUDED.name, icon=EXCLUDED.icon, rule_type=EXCLUDED.rule_type,
			  definition=EXCLUDED.definition, updated_at=EXCLUDED.updated_at
			WHERE platform_rules.is_default = true AND platform_rules.customized = false
```

(The `INSERT … VALUES` still sets `sort_order` for brand-new rows — only the conflict-update drops it.)

- [ ] **Step 6: Run test + full suite**

Run: `encore test ./services/checker/ -run TestCustomizeAndResetBuiltinRule` → PASS.
Run: `encore test ./services/checker/` → full suite green.

- [ ] **Step 7: Regenerate the typed client**

Run: `encore gen client subs-check-uqti --lang=typescript --output=./frontend/src/lib/client.gen.ts`
Expected: `PlatformRule` gains `customized: boolean`; a `ResetRule` method appears under `checker`.

- [ ] **Step 8: Commit**

```bash
git add services/checker/migrations/23_rule_customized.up.sql services/checker/migrations/23_rule_customized.down.sql services/checker/rules.go services/checker/rules_defaults.go services/checker/rules_reset_test.go frontend/src/lib/client.gen.ts
git commit -m "feat(checker): customizable built-in rules + ResetRule (sync respects customized)"
```

---

## Task 1: `IconPickerPopover` — icon tile + popover

**Files:**
- Create: `frontend/src/components/platforms/IconPickerPopover.tsx`

The existing `IconPicker.tsx` already has Iconify search + quick-pick + upload logic (in `IconPickerInput`). This task wraps that capability as a **clickable `RuleIcon` tile trigger** + a Base UI `Popover`, suited to the inspector's identity header.

- [ ] **Step 1: Implement**

Read `frontend/src/components/ui/popover.tsx` for the exact exports (`Popover`, `PopoverTrigger`, `PopoverContent`). Read the current `frontend/src/components/platforms/IconPicker.tsx` to reuse its search effect, `QUICK_SETS`, and the `onUpload` handler (with `validateIconFile`/`readIconAsDataUrl` from `@/lib/iconUpload`). Create `IconPickerPopover.tsx`:

```tsx
import { Icon as IconifyIcon } from "@iconify/react";
import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { RuleIcon } from "@/components/rule-icon";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { readIconAsDataUrl, validateIconFile } from "@/lib/iconUpload";
import { cn } from "@/lib/utils";

const QUICK = [
	{ label: "Brands", q: "simple-icons:" },
	{ label: "Logos", q: "logos:" },
	{ label: "Generic", q: "lucide:" },
];

export function IconPickerPopover({
	value,
	onChange,
	name,
	size = 40,
}: {
	value: string;
	onChange: (v: string) => void;
	name: string;
	size?: number;
}) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [results, setResults] = useState<string[]>([]);
	const [searching, setSearching] = useState(false);
	const fileRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (!query.trim()) {
			setResults([]);
			return;
		}
		const id = setTimeout(async () => {
			setSearching(true);
			try {
				const res = await fetch(
					`https://api.iconify.design/search?query=${encodeURIComponent(query)}&limit=48`,
				);
				const data = (await res.json()) as { icons?: string[] };
				setResults(data.icons ?? []);
			} catch {
				setResults([]);
			} finally {
				setSearching(false);
			}
		}, 350);
		return () => clearTimeout(id);
	}, [query]);

	const pick = (v: string) => {
		onChange(v);
		setOpen(false);
	};

	const onUpload = async (file: File | undefined) => {
		if (!file) return;
		const err = validateIconFile(file);
		if (err) {
			toast.error(err);
			return;
		}
		try {
			onChange(await readIconAsDataUrl(file));
			setOpen(false);
		} catch {
			toast.error("Could not read file");
		}
	};

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger
				className="group relative flex items-center justify-center rounded-[10px] border border-border bg-card transition-colors hover:border-primary/60"
				style={{ width: size, height: size }}
				aria-label="Change icon"
			>
				<RuleIcon icon={value} label={name || "?"} size={Math.round(size * 0.55)} />
				<span className="absolute -right-1 -bottom-1 flex size-4 items-center justify-center rounded-full border border-border bg-popover text-[9px] text-muted-foreground">
					✎
				</span>
			</PopoverTrigger>
			<PopoverContent align="start" className="w-80 p-3">
				<input
					value={query}
					onChange={(e) => setQuery(e.target.value)}
					placeholder="Search 200k+ icons…"
					className="mb-2 h-8 w-full rounded-md border border-border bg-background px-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
				/>
				<div className="mb-2 flex flex-wrap gap-1.5">
					{QUICK.map((s) => (
						<button
							key={s.q}
							type="button"
							onClick={() => setQuery(s.q)}
							className="rounded-md border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
						>
							{s.label}
						</button>
					))}
					<button
						type="button"
						onClick={() => fileRef.current?.click()}
						className="rounded-md border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
					>
						⬆ Upload
					</button>
					<input
						ref={fileRef}
						type="file"
						accept="image/svg+xml,image/png,image/jpeg,image/webp"
						className="hidden"
						onChange={(e) => onUpload(e.target.files?.[0])}
					/>
				</div>
				{searching ? (
					<div className="flex justify-center py-4">
						<Loader2 size={16} className="animate-spin text-muted-foreground" />
					</div>
				) : results.length > 0 ? (
					<div className="grid max-h-52 grid-cols-8 gap-1 overflow-y-auto">
						{results.map((id) => (
							<button
								key={id}
								type="button"
								title={id}
								onClick={() => pick(id)}
								className="flex aspect-square items-center justify-center rounded-md hover:bg-secondary"
							>
								<IconifyIcon icon={id} width={18} height={18} />
							</button>
						))}
					</div>
				) : (
					<p className="px-1 py-2 text-muted-foreground text-xs leading-relaxed">
						Search Iconify, pick a quick set, paste an emoji below, or upload an
						SVG/PNG (≤32 KB).
					</p>
				)}
				<input
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder="emoji · simple-icons:netflix · https://… · data:…"
					className={cn(
						"mt-2 h-8 w-full rounded-md border border-border bg-background px-2.5 font-mono text-xs",
						"focus:outline-none focus:ring-1 focus:ring-ring",
					)}
				/>
			</PopoverContent>
		</Popover>
	);
}
```

> If `PopoverContent` does not accept an `align` prop or its API differs, adapt to the real `ui/popover.tsx` signature (read it). The `RuleIcon` + `iconUpload` imports already exist on this branch.

- [ ] **Step 2: Verify**

Run: `bun check-types` → PASS (additive; unimported).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/platforms/IconPickerPopover.tsx
git commit -m "feat(frontend): IconPickerPopover — icon tile + search/quick/upload popover"
```

---

## Task 2: `RuleListPane` — left list

**Files:**
- Create: `frontend/src/components/platforms/RuleListPane.tsx`

Left pane: search, `+` new, **Built-in / Custom** grouped, drag-sortable within each group (reuses `@dnd-kit`, persists `sort_order` via `useUpdateRule`), selection highlight. Adapts the dnd logic from `SortableRuleList.tsx` (read it).

- [ ] **Step 1: Implement**

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
import { GripVertical, Plus, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { RuleIcon } from "@/components/rule-icon";
import type { checker } from "@/lib/client.gen";
import { cn } from "@/lib/utils";
import { useUpdateRule } from "@/queries";

type PlatformRule = checker.PlatformRule;

function updateParams(r: PlatformRule, sortOrder: number) {
	return {
		id: r.id,
		params: {
			name: r.name,
			icon: r.icon,
			enabled: r.enabled,
			rule_type: r.rule_type,
			definition: r.definition,
			sort_order: sortOrder,
		},
	};
}

function RuleRow({
	rule,
	selected,
	onSelect,
}: {
	rule: PlatformRule;
	selected: boolean;
	onSelect: (id: string) => void;
}) {
	const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
		useSortable({ id: rule.id });
	return (
		<button
			type="button"
			ref={setNodeRef}
			onClick={() => onSelect(rule.id)}
			style={{ transform: CSS.Transform.toString(transform), transition }}
			className={cn(
				"group flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left",
				selected
					? "bg-[var(--color-active-bg)] ring-1 ring-[var(--color-active-border)]"
					: "hover:bg-secondary/60",
				isDragging && "opacity-60",
			)}
		>
			<span
				className="cursor-grab text-muted-foreground/40 opacity-0 group-hover:opacity-100"
				{...attributes}
				{...listeners}
				onClick={(e) => e.stopPropagation()}
			>
				<GripVertical size={13} />
			</span>
			<RuleIcon icon={rule.icon} label={rule.name} size={20} />
			<span
				className={cn(
					"min-w-0 flex-1 truncate text-[13px]",
					selected ? "font-medium text-foreground" : "text-muted-foreground",
				)}
			>
				{rule.name}
			</span>
			<span
				className={cn(
					"size-1.5 shrink-0 rounded-full",
					rule.enabled ? "bg-success" : "bg-muted",
				)}
			/>
		</button>
	);
}

function Group({
	label,
	rules,
	selectedId,
	onSelect,
}: {
	label: string;
	rules: PlatformRule[];
	selectedId: string | null;
	onSelect: (id: string) => void;
}) {
	const updateMut = useUpdateRule();
	const sensors = useSensors(
		useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
	);
	const onDragEnd = (e: DragEndEvent) => {
		const { active, over } = e;
		if (!over || active.id === over.id) return;
		const oldI = rules.findIndex((r) => r.id === active.id);
		const newI = rules.findIndex((r) => r.id === over.id);
		if (oldI < 0 || newI < 0) return;
		const next = [...rules];
		const [m] = next.splice(oldI, 1);
		next.splice(newI, 0, m);
		next.forEach((r, i) => {
			if (r.sort_order !== i) updateMut.mutate(updateParams(r, i));
		});
	};
	if (rules.length === 0) return null;
	return (
		<>
			<div className="px-2 pt-3 pb-1 font-medium text-[9.5px] text-muted-foreground/70 uppercase tracking-[0.6px]">
				{label} · {rules.length}
			</div>
			<DndContext sensors={sensors} onDragEnd={onDragEnd}>
				<SortableContext
					items={rules.map((r) => r.id)}
					strategy={verticalListSortingStrategy}
				>
					{rules.map((r) => (
						<RuleRow
							key={r.id}
							rule={r}
							selected={r.id === selectedId}
							onSelect={onSelect}
						/>
					))}
				</SortableContext>
			</DndContext>
		</>
	);
}

export function RuleListPane({
	rules,
	selectedId,
	onSelect,
	onNew,
}: {
	rules: PlatformRule[];
	selectedId: string | null;
	onSelect: (id: string) => void;
	onNew: () => void;
}) {
	const [q, setQ] = useState("");
	const filtered = useMemo(() => {
		const t = q.trim().toLowerCase();
		return t
			? rules.filter(
					(r) =>
						r.name.toLowerCase().includes(t) || r.key.toLowerCase().includes(t),
				)
			: rules;
	}, [rules, q]);
	const builtin = filtered.filter((r) => r.is_default);
	const custom = filtered.filter((r) => !r.is_default);

	return (
		<div className="flex h-full flex-col">
			<div className="flex items-center gap-2 border-border border-b p-2.5">
				<div className="flex flex-1 items-center gap-1.5 rounded-lg border border-border bg-background px-2.5">
					<Search size={13} className="text-muted-foreground" />
					<input
						value={q}
						onChange={(e) => setQ(e.target.value)}
						placeholder="Search rules…"
						className="h-7 w-full bg-transparent text-sm focus:outline-none"
					/>
				</div>
				<button
					type="button"
					onClick={onNew}
					aria-label="New rule"
					className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground hover:opacity-90"
				>
					<Plus size={16} />
				</button>
			</div>
			<div className="flex-1 overflow-y-auto p-1.5">
				<Group label="Built-in" rules={builtin} selectedId={selectedId} onSelect={onSelect} />
				<Group label="Custom" rules={custom} selectedId={selectedId} onSelect={onSelect} />
				{filtered.length === 0 && (
					<p className="px-2 py-6 text-center text-muted-foreground text-xs">
						No matching rules.
					</p>
				)}
			</div>
		</div>
	);
}
```

For the selected-row background, add this once to the file's rows via a wrapping style — simplest: give the selected `<button>` the active tokens inline. Replace the `data-active` approach by adding to `RuleRow`'s `className` when `selected`: append `"bg-[var(--color-active-bg)] ring-1 ring-[var(--color-active-border)]"`. (Update the `cn(...)` in `RuleRow` accordingly.)

- [ ] **Step 2: Verify**

Run: `bun check-types` → PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/platforms/RuleListPane.tsx
git commit -m "feat(frontend): RuleListPane — grouped, sortable rule list with selection"
```

---

## Task 3: `RuleInspector` — right inspector (edit + draft)

**Files:**
- Create: `frontend/src/components/platforms/RuleInspector.tsx`

Restructures `RuleEditorDialog`'s logic (save/test/type-switch, `ConditionEditor`/`ScriptEditorArea`/`ConsolePanel`/`DocsPanel`) into the inspector layout: identity header (icon tile→`IconPickerPopover`, name, key chip, **type segmented control**, enabled `Switch`, delete), definition editor, docked test panel, Save footer. Handles both an existing `rule` and a new `draft`.

- [ ] **Step 1: Implement**

First add the reset hook to `frontend/src/queries/rules.ts` (it needs Task 0's regenerated client with `client.checker.ResetRule`):

```ts
export function useResetRule() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (id: string) => client.checker.ResetRule(id),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: queryKeys.platformRules() }),
	});
}
```

(Re-export it from `@/queries` if that barrel explicitly lists rule hooks — check `frontend/src/queries/index.ts`.) Then read `frontend/src/components/platforms/engine.ts(x)` (for `defaultDef`, `RULE_TYPES`, `RULE_TYPE_LABELS`, `RuleType`, `useMonacoSetup`) before writing. The `warning`/`warning-muted` tokens used by the Modified badge already exist in the theme (used elsewhere, e.g. latency tones). Create `RuleInspector.tsx`:

```tsx
import { ChevronLeft, Loader2, Play, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";
import { IconPickerPopover } from "@/components/platforms/IconPickerPopover";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";
import {
	useCreateRule,
	useDeleteRule,
	useResetRule,
	useTestNodes,
	useUpdateRule,
} from "@/queries";
import { cn } from "@/lib/utils";
import { ConditionEditor } from "./ConditionEditor";
import { ConsolePanel } from "./ConsolePanel";
import {
	defaultDef,
	RULE_TYPE_LABELS,
	RULE_TYPES,
	type RuleType,
} from "./engine";
import { ScriptEditorArea } from "./ScriptEditorArea";

type PlatformRule = checker.PlatformRule;
type TestRuleResult = checker.TestRuleResult;

export function RuleInspector({
	rule,
	onClose,
	onMobileBack,
}: {
	// rule === undefined => draft (create); else edit
	rule?: PlatformRule;
	onClose: () => void;
	onMobileBack?: () => void;
}) {
	const isEdit = !!rule;
	const { theme } = useTheme();
	const monacoTheme = theme === "dark" ? "vs-dark" : "vs";

	const [name, setName] = useState(rule?.name ?? "");
	const [ruleKey, setRuleKey] = useState(rule?.key ?? "");
	const [icon, setIcon] = useState(rule?.icon ?? "");
	const [enabled, setEnabled] = useState(rule?.enabled ?? true);
	const [ruleType, setRuleType] = useState<RuleType>(
		(rule?.rule_type as RuleType) ?? "js",
	);
	const [def, setDef] = useState<Record<string, unknown>>(
		(rule?.definition as Record<string, unknown>) ?? defaultDef("js"),
	);
	const [activeTab, setActiveTab] = useState<"prelude" | "code">("code");
	const [testResult, setTestResult] = useState<TestRuleResult | null>(null);
	const [testing, setTesting] = useState(false);
	const [testNodeId, setTestNodeId] = useState("");
	const consoleRef = useRef<HTMLDivElement>(null);

	const testNodes = useTestNodes().data?.nodes ?? [];
	const createMut = useCreateRule();
	const updateMut = useUpdateRule();
	const deleteMut = useDeleteRule();
	const resetMut = useResetRule();
	const saving = createMut.isPending || updateMut.isPending;

	function changeType(t: RuleType) {
		setRuleType(t);
		setDef(defaultDef(t));
		setTestResult(null);
	}

	async function runTest() {
		setTesting(true);
		setTestResult(null);
		try {
			const res = await client.checker.TestRule({
				rule_type: ruleType,
				definition: def as never,
				node_id: testNodeId || "",
			});
			setTestResult(res);
			setTimeout(
				() => consoleRef.current?.scrollIntoView({ behavior: "smooth" }),
				80,
			);
		} catch {
			setTestResult({
				ok: false,
				error: "Request failed",
				duration_ms: 0,
				status_code: 0,
				final_url: "",
				body: "",
				response_headers: {},
				node_name: "",
				trace: { platform: "", result: false, steps: [] },
			});
		} finally {
			setTesting(false);
		}
	}

	function save() {
		const onSuccess = () => {
			toast.success(isEdit ? "Rule saved" : "Rule created");
			onClose();
		};
		const onError = () => toast.error(isEdit ? "Failed to save" : "Failed to create");
		if (isEdit && rule) {
			updateMut.mutate(
				{
					id: rule.id,
					params: {
						name,
						icon,
						enabled,
						rule_type: ruleType,
						definition: def as never,
						sort_order: rule.sort_order,
					},
				},
				{ onSuccess, onError },
			);
		} else {
			createMut.mutate(
				{
					name,
					key: ruleKey,
					icon,
					enabled,
					rule_type: ruleType,
					definition: def as never,
					sort_order: 1000,
				},
				{ onSuccess, onError },
			);
		}
	}

	function remove() {
		if (!rule) return;
		if (!confirm(`Delete rule "${rule.name}"?`)) return;
		deleteMut.mutate(rule.id, {
			onSuccess: () => {
				toast.success("Rule deleted");
				onClose();
			},
			onError: () => toast.error("Failed to delete"),
		});
	}

	const canSave = name.trim() && (isEdit || ruleKey.trim());

	return (
		<div className="flex h-full min-w-0 flex-col">
			{/* identity header */}
			<div className="flex items-center gap-3 border-border border-b px-4 py-3">
				{onMobileBack && (
					<button
						type="button"
						onClick={onMobileBack}
						className="-ml-1 rounded p-1 text-muted-foreground hover:text-foreground lg:hidden"
						aria-label="Back to list"
					>
						<ChevronLeft size={18} />
					</button>
				)}
				<IconPickerPopover value={icon} onChange={setIcon} name={name} size={40} />
				<div className="min-w-0">
					<input
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="Rule name"
						className="w-full bg-transparent font-semibold text-base focus:outline-none"
					/>
					<div className="mt-0.5">
						{isEdit ? (
							<span className="rounded bg-secondary px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
								{rule?.key}
							</span>
						) : (
							<input
								value={ruleKey}
								onChange={(e) =>
									setRuleKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))
								}
								placeholder="key"
								className="h-6 w-32 rounded border border-border bg-background px-1.5 font-mono text-[11px] focus:outline-none focus:ring-1 focus:ring-ring"
							/>
						)}
					</div>
				</div>
				<div className="ml-auto flex items-center gap-2.5">
					<div className="flex rounded-lg border border-border bg-background p-0.5">
						{RULE_TYPES.map((t) => (
							<button
								key={t}
								type="button"
								onClick={() => changeType(t)}
								className={cn(
									"rounded-md px-2 py-1 text-[11px] transition-colors",
									ruleType === t
										? "bg-secondary text-foreground"
										: "text-muted-foreground hover:text-foreground",
								)}
							>
								{RULE_TYPE_LABELS[t]}
							</button>
						))}
					</div>
					<Switch checked={enabled} onCheckedChange={(v) => setEnabled(v === true)} />
					{isEdit && (
						<button
							type="button"
							onClick={remove}
							className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-danger"
							aria-label="Delete rule"
						>
							<Trash2 size={15} />
						</button>
					)}
				</div>
			</div>

			{/* definition */}
			<div className="flex min-h-0 flex-1 flex-col overflow-hidden">
				{ruleType === "condition" ? (
					<div className="flex-1 overflow-y-auto p-4">
						<ConditionEditor def={def} onChange={setDef} />
					</div>
				) : (
					<ScriptEditorArea
						def={def}
						onChange={setDef}
						lang={ruleType}
						monacoTheme={monacoTheme}
						activeTab={activeTab}
						onTabChange={setActiveTab}
					/>
				)}
			</div>

			{/* test */}
			<div className="border-border border-t bg-card/40">
				<div className="flex items-center gap-2 px-4 py-2.5">
					<select
						value={testNodeId}
						onChange={(e) => setTestNodeId(e.target.value)}
						className="h-7 max-w-[170px] rounded-lg border border-border bg-background px-2 text-muted-foreground text-xs focus:outline-none"
						title="Node to test through"
					>
						<option value="">Direct (no proxy)</option>
						{testNodes.map((n) => (
							<option key={n.id} value={n.id}>
								{n.name}
							</option>
						))}
					</select>
					<button
						type="button"
						onClick={runTest}
						disabled={testing}
						className="flex h-7 items-center gap-1.5 rounded-lg bg-primary px-3 font-medium text-primary-foreground text-xs hover:opacity-90 disabled:opacity-50"
					>
						{testing ? <Loader2 size={11} className="animate-spin" /> : <Play size={11} />}
						{testing ? "Running…" : "Run test"}
					</button>
				</div>
				<div ref={consoleRef}>
					{(testResult || testing) && (
						<ConsolePanel
							result={testResult}
							loading={testing}
							nodeLabel={
								testResult?.node_name ??
								(testNodeId
									? (testNodes.find((n) => n.id === testNodeId)?.name ?? "")
									: "")
							}
						/>
					)}
				</div>
			</div>

			{/* footer */}
			<div className="flex items-center gap-2 border-border border-t px-4 py-3">
				{isEdit && rule?.is_default && rule.customized && (
					<>
						<span className="rounded bg-warning-muted px-1.5 py-0.5 text-[11px] text-warning">
							Modified
						</span>
						<button
							type="button"
							onClick={() =>
								resetMut.mutate(rule.id, {
									onSuccess: () => {
										toast.success("Reset to default");
										onClose();
									},
									onError: () => toast.error("Failed to reset"),
								})
							}
							disabled={resetMut.isPending}
							className="rounded-md border border-border px-2.5 py-1 text-muted-foreground text-xs hover:bg-secondary disabled:opacity-50"
						>
							Reset to default
						</button>
					</>
				)}
				<Button
					variant="success"
					size="sm"
					className="ml-auto"
					onClick={save}
					disabled={saving || !canSave}
				>
					{saving && <Loader2 size={11} className="animate-spin" />}
					{isEdit ? "Save changes" : "Create rule"}
				</Button>
			</div>
		</div>
	);
}
```

> `confirm(...)` is used for delete simplicity; if the repo's `confirm-dialog` primitive is preferred, swap it in (read `ui/confirm-dialog.tsx`). `DocsPanel` is intentionally dropped from the inspector to keep it focused — if you want it, add a toggle that slides it in from the right like the old editor; out of scope for the core redesign.

- [ ] **Step 2: Verify**

Run: `bun check-types` → PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/platforms/RuleInspector.tsx
git commit -m "feat(frontend): RuleInspector — inline identity + editor + test (edit & draft)"
```

---

## Task 4: Rewrite `routes/rules.tsx` as the master–detail shell

**Files:**
- Modify (rewrite): `frontend/src/routes/rules.tsx`

- [ ] **Step 1: Implement**

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { Radar } from "lucide-react";
import { useEffect, useState } from "react";
import { useMonacoSetup } from "@/components/platforms/engine";
import { RuleInspector } from "@/components/platforms/RuleInspector";
import { RuleListPane } from "@/components/platforms/RuleListPane";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useRules } from "@/queries";

export const Route = createFileRoute("/rules")({
	component: RulesPage,
});

function RulesPage() {
	useMonacoSetup();
	const { data, isLoading } = useRules();
	const rules = data?.rules ?? [];

	const [selectedId, setSelectedId] = useState<string | null>(null);
	const [draft, setDraft] = useState(false);

	// Keep selection valid as rules change (after create/delete).
	useEffect(() => {
		if (selectedId && !rules.some((r) => r.id === selectedId)) {
			setSelectedId(null);
		}
	}, [rules, selectedId]);

	const selected = rules.find((r) => r.id === selectedId) ?? null;
	const showInspector = draft || !!selected;

	const startNew = () => {
		setDraft(true);
		setSelectedId(null);
	};
	const select = (id: string) => {
		setSelectedId(id);
		setDraft(false);
	};
	const close = () => {
		setDraft(false);
		setSelectedId(null);
	};

	if (isLoading) {
		return (
			<div className="p-4">
				<Skeleton className="h-[70vh] w-full" />
			</div>
		);
	}

	return (
		<div className="flex h-[calc(100vh-0px)] flex-col lg:h-screen">
			<div className="flex h-full min-h-0">
				{/* LIST — full width on mobile when no inspector, fixed col on lg */}
				<div
					className={[
						"min-h-0 w-full border-border lg:w-[256px] lg:flex-shrink-0 lg:border-r",
						showInspector ? "hidden lg:flex" : "flex",
					].join(" ")}
				>
					<RuleListPane
						rules={rules}
						selectedId={selectedId}
						onSelect={select}
						onNew={startNew}
					/>
				</div>

				{/* INSPECTOR / EMPTY */}
				<div
					className={[
						"min-h-0 min-w-0 flex-1",
						showInspector ? "flex" : "hidden lg:flex",
					].join(" ")}
				>
					{draft ? (
						<RuleInspector onClose={close} onMobileBack={close} />
					) : selected ? (
						<RuleInspector
							key={selected.id}
							rule={selected}
							onClose={close}
							onMobileBack={close}
						/>
					) : (
						<div className="flex flex-1 items-center justify-center">
							<EmptyState
								icon={Radar}
								title={rules.length === 0 ? "No rules yet" : "Select a rule"}
								description={
									rules.length === 0
										? "Built-ins seed automatically. Create one to detect a custom platform."
										: "Pick a rule on the left to inspect, edit, and test it."
								}
							/>
						</div>
					)}
				</div>
			</div>
		</div>
	);
}
```

> The `key={selected.id}` on the edit inspector forces a fresh form state when switching rules. Confirm `EmptyState` accepts `icon`/`title`/`description` (the old page used it the same way). Adjust the outer height wrapper if the app's main content area already constrains height (read `__root.tsx`'s `<main>`); the goal is the two panes fill the viewport below the header.

- [ ] **Step 2: Verify**

Run: `bun check-types` → PASS. Then `bun run build` → PASS. (`RuleEditorDialog`/`SortableRuleList` are now unimported by the page but still exist — they still compile.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/rules.tsx
git commit -m "feat(frontend): /rules master-detail shell (list + inspector, responsive)"
```

---

## Task 5: Delete the superseded components + verify

**Files:**
- Delete: `frontend/src/components/platforms/RuleEditorDialog.tsx`, `frontend/src/components/platforms/SortableRuleList.tsx`, `frontend/src/components/ui/sheet.tsx`, `frontend/src/components/platforms/RuleCard.tsx`

- [ ] **Step 1: Confirm no importers, then delete**

Run (from `frontend/`):
```bash
grep -rnE 'RuleEditorDialog|SortableRuleList|ui/sheet|RuleCard' src | grep -v 'routes/rules.tsx'
```
Expected: no hits outside any file you're about to delete. (If `RuleCard` is still imported anywhere, leave it; otherwise remove.) Then:

```bash
git rm frontend/src/components/platforms/RuleEditorDialog.tsx frontend/src/components/platforms/SortableRuleList.tsx frontend/src/components/ui/sheet.tsx frontend/src/components/platforms/RuleCard.tsx
```

- [ ] **Step 2: Verify**

Run (from `frontend/`): `bun check-types` (PASS) then `bun run build` (PASS). Fix any dangling import the deletes surfaced.

- [ ] **Step 3: Browser walkthrough**

`bun dev` + `encore run` (already on :4000). At `/rules` confirm against the mockups: master–detail layout; left search + Built-in/Custom groups + drag-reorder (persists on refresh) + selection drives the inspector; identity header (icon tile opens picker with search/quick/emoji/upload + live preview; changing icon updates the row); type segmented control swaps condition-form ↔ code editor; enabled switch; inline Run test shows verdict + console; New (`+`) creates a draft → Create; delete confirms + removes; disabling a rule drops it from the network-unlock strip. Resize narrow → single-pane with back chevron.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(frontend): remove Sheet editor / sortable-list / card superseded by inspector"
```

---

## Self-Review Notes

- **Spec coverage:** master–detail layout → Task 4; left list (search/group/sort/select) → Task 2; inspector (identity/type/editor/test/footer + draft) → Task 3; icon picker popover → Task 1; deletes (`RuleEditorDialog`/`SortableRuleList`/`sheet`/`RuleCard`) → Task 5; responsive single-pane → Task 4; empty states → Task 4; theme tokens → throughout. Backend unchanged (spec: out of scope).
- **Build-green ordering:** Tasks 1-3 additive → Task 4 swaps the page (old files unimported but present) → Task 5 deletes them. No broken window.
- **Type consistency:** `IconPickerPopover({value,onChange,name,size?})`, `RuleListPane({rules,selectedId,onSelect,onNew})`, `RuleInspector({rule?,onClose,onMobileBack?})` used identically in Task 4. Reused interfaces (`ConditionEditor`/`ScriptEditorArea`/`ConsolePanel`/`engine`) match `RuleEditorDialog`'s usage exactly.
- **Pre-flight to verify during execution:** `ui/popover.tsx` API (`align` prop), `engine` export names, `EmptyState` props, `__root.tsx` main height constraint, `Switch`/`Button` variants. All are read-then-adapt notes, not placeholders.
- **Deliberate simplifications (YAGNI):** `DocsPanel` dropped from the inspector; native `confirm()` for delete (swap to `confirm-dialog` if preferred); `RuleDefinitionEditor`/`RuleTestPanel` realized inline in `RuleInspector` (extract only if it grows > ~250 lines).
