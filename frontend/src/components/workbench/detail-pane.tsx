import { ChevronDown, ChevronRight, Download, PlayCircle } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DebugPanel, type NodeDebug } from "@/components/debug-panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { DetailHeader } from "@/components/workbench/detail-header";
import { ProgressPanel } from "@/components/workbench/progress-panel";
import { ResultsSection } from "@/components/workbench/results-section";
import { isApiError } from "@/lib/client";
import type { checker, subscription } from "@/lib/client.gen";
import type { SSEConnection, SSEProgress } from "@/queries";
import {
	useCancelCheck,
	useExportLogs,
	useJobs,
	useResults,
	useRules,
	useSetNodeEnabled,
} from "@/queries";

type Subscription = subscription.Subscription;

export function DetailPane({
	sub,
	activeJobId,
	progress,
	logEntries,
	debugData,
	connection,
	selectedJobId,
	onSelectJob,
	onRunStarted,
	onEdit,
	onToggleEnabled,
	onDelete,
	onBack,
}: {
	sub: Subscription;
	activeJobId: string | null;
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	debugData: NodeDebug[];
	connection: SSEConnection;
	selectedJobId: string | null;
	onSelectJob: (jobId: string | null) => void;
	onRunStarted: (jobId: string) => void;
	onEdit: () => void;
	onToggleEnabled: () => void;
	onDelete: () => void;
	onBack: () => void;
}) {
	const jobsQuery = useJobs(sub.id, { Limit: 20 });
	const rulesQuery = useRules();
	const resultsQuery = useResults(sub.id, selectedJobId);
	const cancelMut = useCancelCheck(sub.id);
	const toggleNodeMut = useSetNodeEnabled(sub.id);

	const running = !!activeJobId && !progress?.done;
	const job = resultsQuery.data?.job;
	const results = resultsQuery.data?.results ?? [];
	const noChecksYet =
		resultsQuery.isError &&
		isApiError(resultsQuery.error) &&
		resultsQuery.error.status === 404;

	const handleCancel = () => {
		if (!activeJobId) return;
		cancelMut.mutate(activeJobId, {
			onSuccess: () => toast.success("Check cancelled"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Cancel failed"),
		});
	};

	const handleToggleNode = (nodeId: string, enabled: boolean) => {
		toggleNodeMut.mutate(
			{ nodeId, enabled },
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update node"),
			},
		);
	};

	const streamedResults: checker.NodeResult[] = useMemo(
		() =>
			logEntries
				.filter((e) => e.alive)
				.map((e, i) => ({
					node_id: `live-${i}`,
					node_name: e.node_name ?? "",
					node_type: "",
					enabled: true,
					alive: true,
					latency_ms: e.latency_ms ?? 0,
					speed_kbps: e.speed_kbps ?? 0,
					upload_speed_kbps: e.upload_speed_kbps ?? 0,
					country: "",
					ip: "",
					server: "",
					port: 0,
					config: "",
					platforms: {},
					traffic_bytes: 0,
				})),
		[logEntries],
	);

	return (
		<div className="flex h-full min-h-0 flex-col">
			<DetailHeader
				sub={sub}
				jobs={jobsQuery.data?.jobs ?? []}
				selectedJobId={selectedJobId}
				activeJobId={activeJobId}
				onSelectJob={onSelectJob}
				onRunStarted={onRunStarted}
				onEdit={onEdit}
				onToggleEnabled={onToggleEnabled}
				onDelete={onDelete}
				onBack={onBack}
			/>

			<div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 md:p-5">
				{running ? (
					<ProgressPanel
						progress={progress}
						logEntries={logEntries}
						connection={connection}
						cancelPending={cancelMut.isPending}
						onCancel={handleCancel}
					/>
				) : null}

				{debugData.length > 0 ? <DebugPanel data={debugData} /> : null}

				{!running && job ? (
					<div className="flex items-center gap-2 text-xs">
						<Badge
							tone={
								job.status === "completed"
									? "success"
									: job.status === "failed"
										? "danger"
										: "info"
							}
						>
							{job.status}
						</Badge>
						<span className="text-muted-foreground tabular-nums">
							{new Date(job.created_at).toLocaleString()}
						</span>
					</div>
				) : null}

				{running ? (
					// Spec §Running state: alive nodes stream into the table as
					// found. Partial rows (no unlocks/traffic/country yet, no
					// node_id) — the final refetch replaces them with full data.
					streamedResults.length > 0 ? (
						<ResultsSection results={streamedResults} rules={[]} />
					) : null
				) : resultsQuery.isLoading && !noChecksYet ? (
					<div className="space-y-2">
						<Skeleton className="h-8 w-2/3" />
						<Skeleton className="h-40 w-full" />
					</div>
				) : noChecksYet ? (
					<EmptyState
						icon={PlayCircle}
						title="No checks yet"
						description="Run your first check to see node availability, latency and unlocks."
					/>
				) : (
					<ResultsSection
						results={results}
						rules={rulesQuery.data?.rules}
						onToggleEnabled={handleToggleNode}
					/>
				)}

				{!running ? <ExportLogsDisclosure subscriptionId={sub.id} /> : null}
			</div>
		</div>
	);
}

// Compact disclosure for the per-subscription export request log (existing
// feature carried over from the old detail page).
function ExportLogsDisclosure({ subscriptionId }: { subscriptionId: string }) {
	const [open, setOpen] = useState(false);
	const logsQuery = useExportLogs(subscriptionId, { enabled: open });
	const logs = logsQuery.data?.logs ?? [];

	return (
		<div className="rounded-lg border border-border bg-card">
			<button
				type="button"
				onClick={() => setOpen((v) => !v)}
				className="flex w-full items-center justify-between px-4 py-2.5 text-muted-foreground text-xs transition-colors hover:text-foreground"
			>
				<span className="inline-flex items-center gap-2">
					<Download size={13} strokeWidth={1.5} /> Export requests
				</span>
				{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
			</button>
			{open ? (
				<div className="border-border border-t px-4 py-3">
					{logsQuery.isLoading ? (
						<Skeleton className="h-4 w-40" />
					) : logs.length === 0 ? (
						<p className="text-muted-foreground text-xs">
							No export requests yet.
						</p>
					) : (
						<div className="space-y-1">
							{logs.map((log) => (
								<div
									key={log.id}
									className="flex items-center justify-between py-0.5 font-mono text-[11px] tabular-nums"
								>
									<span className="text-foreground">
										{new Date(log.requested_at).toLocaleString()}
									</span>
									<span className="text-muted-foreground">{log.ip || "—"}</span>
								</div>
							))}
						</div>
					)}
				</div>
			) : null}
		</div>
	);
}
