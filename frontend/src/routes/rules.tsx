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
