import { createFileRoute } from "@tanstack/react-router";
import { Loader2, Plus } from "lucide-react";
import { useState } from "react";
import { useMonacoSetup } from "@/components/platforms/engine";
import { RuleCard } from "@/components/platforms/RuleCard";
import { RuleEditorDialog } from "@/components/platforms/RuleEditorDialog";
import type { checker } from "@/lib/client.gen";
import { useRules } from "@/queries";

type PlatformRule = checker.PlatformRule;

export const Route = createFileRoute("/settings/platforms")({
	component: PlatformsPage,
});

function PlatformsPage() {
	useMonacoSetup();

	const [editingRule, setEditingRule] = useState<PlatformRule | null>(null);
	const [addOpen, setAddOpen] = useState(false);

	const { data, isLoading } = useRules();
	const rules = data?.rules ?? [];

	return (
		<div className="max-w-2xl space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-foreground text-lg">
					Platform Detection Rules
				</h1>
				<button
					type="button"
					onClick={() => setAddOpen(true)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white"
					style={{ background: "var(--color-btn-success)" }}
				>
					<Plus size={13} /> Add Rule
				</button>
			</div>

			<p className="text-muted-foreground text-xs">
				Rules run during each proxy check. Built-in rules are seeded on first
				visit. Custom keys store results in{" "}
				<code className="rounded bg-secondary px-1 font-mono">
					extra_platforms
				</code>
				.
			</p>

			{isLoading ? (
				<div className="flex justify-center py-8">
					<Loader2 size={18} className="animate-spin text-muted-foreground" />
				</div>
			) : (
				<div className="space-y-2">
					{rules.map((rule) => (
						<RuleCard
							key={rule.id}
							rule={rule}
							onEdit={() => setEditingRule(rule)}
						/>
					))}
					{rules.length === 0 && (
						<p className="py-6 text-center text-muted-foreground text-sm">
							No rules yet.
						</p>
					)}
				</div>
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
