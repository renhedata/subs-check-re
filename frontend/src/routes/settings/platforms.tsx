import { createFileRoute } from "@tanstack/react-router";
import { Plus, Tv2 } from "lucide-react";
import { useState } from "react";
import { useMonacoSetup } from "@/components/platforms/engine";
import { RuleCard } from "@/components/platforms/RuleCard";
import { RuleEditorDialog } from "@/components/platforms/RuleEditorDialog";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
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
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<p className="text-muted-foreground text-xs">
					Rules run during each proxy check. Built-in rules are seeded on first
					visit. Custom keys store results in{" "}
					<code className="rounded bg-secondary px-1 font-mono">
						extra_platforms
					</code>
					.
				</p>
				<Button
					variant="success"
					size="sm"
					onClick={() => setAddOpen(true)}
					className="shrink-0"
				>
					<Plus size={13} /> Add Rule
				</Button>
			</div>

			{isLoading ? (
				<div className="space-y-2">
					<Skeleton className="h-12 w-full" />
					<Skeleton className="h-12 w-full" />
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
						<div className="rounded-lg border border-border">
							<EmptyState
								icon={Tv2}
								title="No rules yet"
								description="Add a detection rule to test custom platforms during checks."
							/>
						</div>
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
