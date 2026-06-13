import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Inbox } from "lucide-react";
import { useState } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { SubList } from "@/components/workbench/sub-list";
import { SubscriptionDialog } from "@/components/workbench/subscription-dialog";
import { cn } from "@/lib/utils";
import { useLatestJobs, useSubscriptions } from "@/queries";

const searchSchema = z.object({
	sub: z.string().optional(),
});

export const Route = createFileRoute("/")({
	validateSearch: searchSchema,
	component: WorkbenchPage,
});

function WorkbenchPage() {
	const navigate = useNavigate({ from: "/" });
	const { sub: selectedFromUrl } = Route.useSearch();
	const [addOpen, setAddOpen] = useState(false);

	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};

	// Invalid/deleted ?sub falls back to no selection (spec: silent).
	const selected = subs.find((s) => s.id === selectedFromUrl);
	const selectedId = selected?.id ?? null;

	const select = (id: string | null) =>
		navigate({ search: id ? { sub: id } : {}, replace: false });

	return (
		<div className="flex h-full min-h-0">
			{/* List column: full-width on mobile (hidden when a sub is open) */}
			<div
				className={cn(
					"h-full w-full min-w-0 border-border md:w-[280px] md:shrink-0 md:border-r lg:w-[300px]",
					selectedId ? "hidden md:block" : "block",
				)}
			>
				<SubList
					subs={subs}
					latestJobs={latestJobs}
					loading={subsQuery.isLoading}
					selectedId={selectedId}
					liveProgressPct={null /* wired in Task 15 */}
					onSelect={(id) => select(id)}
					onAdd={() => setAddOpen(true)}
				/>
			</div>

			{/* Detail pane */}
			<div
				className={cn(
					"h-full min-w-0 flex-1",
					selectedId ? "block" : "hidden md:block",
				)}
			>
				{selected ? (
					<DetailPane
						key={selected.id}
						sub={selected}
						onBack={() => select(null)}
					/>
				) : subs.length === 0 && !subsQuery.isLoading ? (
					<EmptyState
						icon={Inbox}
						title="No subscriptions yet"
						description="Add your first subscription URL to start checking nodes."
						action={
							<Button variant="success" onClick={() => setAddOpen(true)}>
								Add subscription
							</Button>
						}
					/>
				) : (
					<EmptyState
						icon={Inbox}
						title="Select a subscription"
						description="Pick a subscription on the left to see its nodes and run checks."
					/>
				)}
			</div>

			{/* Dialogs (Task 12) */}
			<SubscriptionDialog open={addOpen} onOpenChange={setAddOpen} />
		</div>
	);
}

// Placeholder until Task 13–15 build the real detail pane.
function DetailPane({
	sub,
	onBack,
}: {
	sub: { id: string; name: string; url: string };
	onBack: () => void;
}) {
	return (
		<div className="p-6">
			<button
				type="button"
				onClick={onBack}
				className="mb-2 text-muted-foreground text-xs md:hidden"
			>
				← Back
			</button>
			<p className="font-semibold text-foreground">{sub.name || sub.url}</p>
			<p className="text-muted-foreground text-xs">
				Detail pane lands in Task 13.
			</p>
		</div>
	);
}
