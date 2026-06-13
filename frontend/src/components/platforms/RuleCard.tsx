import { Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Switch } from "@/components/ui/switch";
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
	const [confirmOpen, setConfirmOpen] = useState(false);

	const deleteMut = useDeleteRule();
	const toggleMut = useUpdateRule();

	const handleDelete = () =>
		deleteMut.mutate(rule.id, {
			onSuccess: () => {
				toast.success("Rule deleted");
				setConfirmOpen(false);
			},
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
			<Switch
				checked={rule.enabled}
				onCheckedChange={(v) => handleToggle(v === true)}
				disabled={toggleMut.isPending}
			/>

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
				<Button variant="ghost" size="sm" onClick={onEdit}>
					Edit
				</Button>
				<Button
					variant="ghost"
					size="icon-sm"
					aria-label="Delete rule"
					className="text-muted-foreground hover:text-danger"
					onClick={() => setConfirmOpen(true)}
				>
					<Trash2 size={13} />
				</Button>
			</div>

			<ConfirmDialog
				open={confirmOpen}
				onOpenChange={setConfirmOpen}
				title={`Delete rule "${rule.name}"?`}
				description="Nodes stop being tested against this platform on future checks."
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
		</div>
	);
}
