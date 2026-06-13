import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Inbox } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { DetailPane } from "@/components/workbench/detail-pane";
import { SubList } from "@/components/workbench/sub-list";
import { SubscriptionDialog } from "@/components/workbench/subscription-dialog";
import { isApiError } from "@/lib/client";
import { cn } from "@/lib/utils";
import {
	useDeleteSubscription,
	useLatestJobs,
	useSSEProgress,
	useSubscriptions,
	useUpdateSubscription,
} from "@/queries";

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
	const [editOpen, setEditOpen] = useState(false);
	const [deleteOpen, setDeleteOpen] = useState(false);
	const [activeJobId, setActiveJobId] = useState<string | null>(null);
	const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};

	// Invalid/deleted ?sub falls back to no selection (spec: silent).
	const selected = subs.find((s) => s.id === selectedFromUrl);
	const selectedId = selected?.id ?? null;

	// Reset per-subscription view state when switching subscriptions.
	// biome-ignore lint/correctness/useExhaustiveDependencies: reset on id change only
	useEffect(() => {
		setActiveJobId(null);
		setSelectedJobId(null);
	}, [selectedId]);

	// If the selected subscription already has a running/queued job (page
	// reload, scheduled run, 409 on trigger), attach to it.
	const latestForSelected = selectedId ? latestJobs[selectedId] : undefined;
	useEffect(() => {
		if (
			latestForSelected &&
			(latestForSelected.status === "running" ||
				latestForSelected.status === "queued")
		) {
			setActiveJobId((cur) => cur ?? latestForSelected.id);
		}
	}, [latestForSelected]);

	const { progress, logEntries, debugData, connection } = useSSEProgress({
		jobId: activeJobId,
		subscriptionId: selectedId ?? "",
		onDone: () => {
			setActiveJobId(null);
			setSelectedJobId(null); // jump to the fresh latest result
		},
	});

	const liveProgressPct =
		activeJobId && progress?.total
			? ((progress.progress ?? 0) / progress.total) * 100
			: null;

	const updateMut = useUpdateSubscription();
	const deleteMut = useDeleteSubscription();

	const select = (id: string | null) =>
		navigate({ search: id ? { sub: id } : {}, replace: false });

	const handleToggleEnabled = () => {
		if (!selected) return;
		updateMut.mutate(
			{
				id: selected.id,
				params: {
					name: selected.name,
					url: selected.url,
					enabled: !selected.enabled,
					cron_expr: selected.cron_expr ?? "",
					clear_cron_expr: false,
					export_include_dead: selected.export_include_dead ?? false,
					export_sort: selected.export_sort ?? "speed_desc",
				},
			},
			{
				onSuccess: () =>
					toast.success(
						selected.enabled ? "Subscription disabled" : "Subscription enabled",
					),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);
	};

	const handleDelete = () => {
		if (!selected) return;
		deleteMut.mutate(selected.id, {
			onSuccess: () => {
				toast.success("Subscription deleted");
				setDeleteOpen(false);
				select(null);
			},
			onError: (e) => toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

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
					liveProgressPct={liveProgressPct}
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
						activeJobId={activeJobId}
						progress={progress}
						logEntries={logEntries}
						debugData={debugData}
						connection={connection}
						selectedJobId={selectedJobId}
						onSelectJob={setSelectedJobId}
						onRunStarted={(jobId) => setActiveJobId(jobId)}
						onEdit={() => setEditOpen(true)}
						onToggleEnabled={handleToggleEnabled}
						onDelete={() => setDeleteOpen(true)}
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

			{/* Dialogs */}
			<SubscriptionDialog open={addOpen} onOpenChange={setAddOpen} />
			<SubscriptionDialog
				open={editOpen}
				onOpenChange={setEditOpen}
				sub={selected ?? null}
			/>
			<ConfirmDialog
				open={deleteOpen}
				onOpenChange={setDeleteOpen}
				title={`Delete "${selected?.name || selected?.url || ""}"?`}
				description="This removes the subscription, all of its nodes and the entire check history. This cannot be undone."
				confirmLabel="Delete"
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
		</div>
	);
}
