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
		<div className="flex h-full min-h-0 flex-col">
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
