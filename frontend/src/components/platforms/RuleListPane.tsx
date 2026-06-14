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
			// biome-ignore lint/style/useNamingConvention: API field name
			rule_type: r.rule_type,
			definition: r.definition,
			// biome-ignore lint/style/useNamingConvention: API field name
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
	const {
		attributes,
		listeners,
		setNodeRef,
		transform,
		transition,
		isDragging,
	} = useSortable({ id: rule.id });
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
				<Group
					label="Built-in"
					rules={builtin}
					selectedId={selectedId}
					onSelect={onSelect}
				/>
				<Group
					label="Custom"
					rules={custom}
					selectedId={selectedId}
					onSelect={onSelect}
				/>
				{filtered.length === 0 && (
					<p className="px-2 py-6 text-center text-muted-foreground text-xs">
						No matching rules.
					</p>
				)}
			</div>
		</div>
	);
}
