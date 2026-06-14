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
