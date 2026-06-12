import { Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import type { checker } from "@/lib/client.gen";
import { useDeleteRule, useUpdateRule } from "@/queries";
import { RULE_TYPE_LABELS, type RuleType, TYPE_COLORS } from "./engine";
import { IconDisplay } from "./IconPicker";

type PlatformRule = checker.PlatformRule;

export function RuleCard({
	rule,
	onEdit,
}: {
	rule: PlatformRule;
	onEdit: () => void;
}) {
	const ruleType = rule.rule_type as RuleType;

	const deleteMut = useDeleteRule();
	const toggleMut = useUpdateRule();

	const handleDelete = () =>
		deleteMut.mutate(rule.id, {
			onSuccess: () => toast.success("Rule deleted"),
			onError: () => toast.error("Failed to delete"),
		});

	const handleToggle = (enabled: boolean) =>
		toggleMut.mutate(
			{
				id: rule.id,
				params: {
					name: rule.name,
					icon: rule.icon,
					enabled,
					rule_type: rule.rule_type,
					definition: rule.definition,
					sort_order: rule.sort_order,
				},
			},
			{
				onError: () => toast.error("Failed to update"),
			},
		);

	return (
		<div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
			<button
				type="button"
				onClick={() => handleToggle(!rule.enabled)}
				disabled={toggleMut.isPending}
				className={[
					"relative h-5 w-9 flex-shrink-0 rounded-full transition-colors",
					rule.enabled ? "bg-green-500" : "bg-muted",
				].join(" ")}
				aria-label="Toggle"
			>
				<span
					className={[
						"absolute top-0.5 left-0 h-4 w-4 rounded-full bg-white shadow transition-transform",
						rule.enabled ? "translate-x-[18px]" : "translate-x-0.5",
					].join(" ")}
				/>
			</button>

			<IconDisplay icon={rule.icon} name={rule.name} />

			<div className="min-w-0 flex-1">
				<div className="flex flex-wrap items-center gap-2">
					<span className="font-medium text-foreground text-sm">
						{rule.name}
					</span>
					<span className="rounded bg-secondary px-1.5 py-0.5 font-mono text-muted-foreground text-xs">
						{rule.key}
					</span>
					<span
						className={[
							"rounded border px-1.5 py-0.5 text-xs",
							TYPE_COLORS[ruleType] ??
								"border-border bg-secondary text-muted-foreground",
						].join(" ")}
					>
						{RULE_TYPE_LABELS[ruleType] ?? rule.rule_type}
					</span>
					{rule.is_default && (
						<span className="text-muted-foreground text-xs opacity-50">
							default
						</span>
					)}
				</div>
			</div>

			<div className="flex items-center gap-1">
				<button
					type="button"
					onClick={onEdit}
					className="rounded px-2 py-1 text-muted-foreground text-xs hover:bg-secondary hover:text-foreground"
				>
					Edit
				</button>
				<button
					type="button"
					onClick={handleDelete}
					disabled={deleteMut.isPending}
					className="rounded p-1 text-muted-foreground hover:text-red-500 disabled:opacity-40"
				>
					{deleteMut.isPending ? (
						<Loader2 size={12} className="animate-spin" />
					) : (
						<Trash2 size={12} />
					)}
				</button>
			</div>
		</div>
	);
}
