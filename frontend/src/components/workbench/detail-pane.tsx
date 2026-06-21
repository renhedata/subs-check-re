import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, Download, PlayCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { DebugPanel, type NodeDebug } from "@/components/debug-panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { DetailHeader } from "@/components/workbench/detail-header";
import { ProgressPanel } from "@/components/workbench/progress-panel";
import { ResultsSection } from "@/components/workbench/results-section";
import { loadCheckOptions } from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import type { checker, subscription } from "@/lib/client.gen";
import type { InflightNode, SSEConnection, SSEProgress } from "@/queries";
import {
	queryKeys,
	useCancelCheck,
	useExportLogs,
	useJobs,
	useNodes,
	useResults,
	useRules,
	useSetNodeEnabled,
	useTriggerCheck,
} from "@/queries";

type Subscription = subscription.Subscription;

export function DetailPane({
	sub,
	activeJobId,
	progress,
	logEntries,
	debugData,
	connection,
	inflight,
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
	inflight: InflightNode[];
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
	const qc = useQueryClient();
	const nodesQuery = useNodes(sub.id);
	const triggerMut = useTriggerCheck();

	// On the default "Latest result" view, the node list is the source of truth.
	const showNodeList = selectedJobId === null;

	// Refetch the node list once a run finishes so live overlay reconciles with
	// the persisted latest-per-node state.
	useEffect(() => {
		if (progress?.done) {
			qc.invalidateQueries({ queryKey: queryKeys.nodes(sub.id) });
		}
	}, [progress?.done, qc, sub.id]);

	// Map a Node (superset) to the NodeResult shape the table renders.
	const nodeResults: checker.NodeResult[] = useMemo(
		() =>
			(nodesQuery.data?.nodes ?? []).map((n) => ({
				node_id: n.node_id,
				node_name: n.node_name,
				node_type: n.node_type,
				enabled: n.enabled,
				alive: n.alive,
				latency_ms: n.latency_ms,
				speed_kbps: n.speed_kbps,
				upload_speed_kbps: n.upload_speed_kbps,
				country: n.country,
				ip: n.ip,
				server: n.server,
				port: n.port,
				config: n.config,
				platforms: n.platforms,
				traffic_bytes: n.traffic_bytes,
			})),
		[nodesQuery.data],
	);

	// Trigger a check over the given node ids ([] = all), reusing the saved
	// per-subscription check options.
	const handleCheckNodes = (nodeIds: string[]) => {
		const opts = loadCheckOptions(sub.id);
		triggerMut.mutate(
			{ subscriptionId: sub.id, params: { ...opts, node_ids: nodeIds } },
			{
				onSuccess: (resp) => {
					toast.success(
						nodeIds.length === 0
							? "Check started"
							: `Checking ${nodeIds.length} node${nodeIds.length > 1 ? "s" : ""}`,
					);
					onRunStarted(resp.job_id);
				},
				onError: (e) => {
					if (isApiError(e) && (e.status === 409 || e.status === 412)) {
						toast.error("A check is already running for this subscription");
						return;
					}
					toast.error(isApiError(e) ? e.message : "Failed to start check");
				},
			},
		);
	};

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

	// Each SSE event already carries a full, inheritance-applied NodeResult
	// (matching GetResults), so live rows render complete — including unlocks
	// and country inherited from prior runs — with no placeholder gaps.
	const streamedResults: checker.NodeResult[] = useMemo(
		() =>
			logEntries
				.filter((e) => e.alive)
				.map((e, i) => ({
					node_id: e.node_id ?? `live-${i}`,
					node_name: e.node_name ?? "",
					node_type: e.node_type ?? "",
					enabled: e.enabled ?? true,
					alive: true,
					latency_ms: e.latency_ms ?? 0,
					speed_kbps: e.speed_kbps ?? 0,
					upload_speed_kbps: e.upload_speed_kbps ?? 0,
					country: e.country ?? "",
					ip: e.ip ?? "",
					server: e.server ?? "",
					port: e.port ?? 0,
					config: e.config ?? "",
					platforms: e.platforms ?? {},
					traffic_bytes: e.traffic_bytes ?? 0,
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
						inflight={inflight}
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
					// found, carrying full inheritance-applied data. The final
					// refetch only reconciles ordering and any disabled flags.
					streamedResults.length > 0 ? (
						<ResultsSection
							results={streamedResults}
							rules={rulesQuery.data?.rules}
						/>
					) : null
				) : showNodeList ? (
					// Default view: the persisted node list is the source of truth,
					// shown right after fetch/import — before any check has run. It
					// must take priority over the results-404 "No checks yet" guard.
					nodesQuery.isLoading ? (
						<div className="space-y-2">
							<Skeleton className="h-8 w-2/3" />
							<Skeleton className="h-40 w-full" />
						</div>
					) : nodeResults.length === 0 ? (
						<EmptyState
							icon={PlayCircle}
							title="No nodes yet"
							description="Refresh from the URL or import nodes to populate this list, then run a check."
						/>
					) : (
						<ResultsSection
							results={nodeResults}
							rules={rulesQuery.data?.rules}
							onToggleEnabled={handleToggleNode}
							selectable
							onCheck={handleCheckNodes}
							checkDisabled={!!activeJobId}
						/>
					)
				) : resultsQuery.isLoading ? (
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
