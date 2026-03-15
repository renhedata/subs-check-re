import { useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { z } from "zod";

import { api, type CheckJob, type NodeResult, type Subscription } from "@/lib/api";
import { NodeTable } from "@/components/node-table";

const searchSchema = z.object({
	job: z.string().optional(),
});

export const Route = createFileRoute("/subscriptions/$id")({
	validateSearch: searchSchema,
	component: SubscriptionDetailPage,
});

interface SSEProgress {
	progress?: number;
	total?: number;
	node_name?: string;
	alive?: boolean;
	latency_ms?: number;
	speed_kbps?: number;
	done?: boolean;
	status?: string;
}

function latencyColor(ms: number): string {
	if (ms < 50) return "#3fb950";
	if (ms <= 200) return "#d29922";
	return "#f85149";
}

function JobStatusBadge({ status }: { status: CheckJob["status"] }) {
	const map: Record<CheckJob["status"], { bg: string; color: string }> = {
		queued: { bg: "#1a2a3a", color: "#58a6ff" },
		running: { bg: "#1a2a3a", color: "#58a6ff" },
		completed: { bg: "#1a4731", color: "#3fb950" },
		failed: { bg: "#3d1a1a", color: "#f85149" },
	};
	const s = map[status];
	return (
		<span
			className="rounded-full px-2 py-0.5 text-[10px] font-medium"
			style={{ background: s.bg, color: s.color }}
		>
			{status}
		</span>
	);
}

function SubscriptionDetailPage() {
	const { id } = Route.useParams();
	const { job: jobIdFromSearch } = Route.useSearch();
	const [jobId, setJobId] = useState<string | null>(jobIdFromSearch ?? null);
	const [progress, setProgress] = useState<SSEProgress | null>(null);
	const esRef = useRef<EventSource | null>(null);

	const resultsQuery = useQuery({
		queryKey: ["results", id],
		queryFn: () =>
			api.get<{ job: CheckJob; results: NodeResult[] }>(`/check/${id}/results`),
		retry: false,
		staleTime: 0,
	});

	// Resolve subscription name from cache
	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
		staleTime: 30_000,
	});
	const sub = subsQuery.data?.subscriptions.find(
		(s) => s.id === (resultsQuery.data?.job.subscription_id ?? id),
	);

	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally narrow deps to avoid re-running on every render
	useEffect(() => {
		const job = resultsQuery.data?.job;
		if (job && (job.status === "running" || job.status === "queued") && !jobId) {
			setJobId(job.id);
		}
	}, [resultsQuery.data?.job?.id, resultsQuery.data?.job?.status, jobId]);

	useEffect(() => {
		if (!jobId) return;
		const es = new EventSource(`/api/check/${jobId}/progress`);
		esRef.current = es;
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			setProgress(data);
			if (data.done) {
				es.close();
				resultsQuery.refetch();
			}
		};
		es.onerror = () => es.close();
		return () => es.close();
	}, [jobId]);

	const job = resultsQuery.data?.job;
	const results = resultsQuery.data?.results ?? [];
	const progressPct =
		progress?.total ? ((progress.progress ?? 0) / progress.total) * 100 : 0;

	return (
		<div className="space-y-5">
			{/* Header */}
			<div>
				<h1 className="text-lg font-semibold text-[#f0f6fc]">
					{sub?.name || sub?.url || "Subscription Detail"}
				</h1>
				<p className="mt-0.5 font-mono text-xs" style={{ color: "#6e7681" }}>
					{id.slice(0, 8)}…
				</p>
			</div>

			{/* Progress bar */}
			{progress && !progress.done && (
				<div className="space-y-1.5">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-1.5 text-sm" style={{ color: "#f0f6fc" }}>
							<RefreshCw size={13} strokeWidth={1.5} className="animate-spin" style={{ color: "#58a6ff" }} />
							Checking nodes…
						</div>
						<span className="text-xs" style={{ color: "#8b949e" }}>
							{progress.progress ?? 0} / {progress.total ?? "?"}
						</span>
					</div>
					<div
						className="h-[3px] w-full overflow-hidden rounded-sm"
						style={{ background: "#21262d" }}
					>
						<div
							className="h-full rounded-sm transition-[width] duration-300 ease-out"
							style={{
								width: `${progressPct}%`,
								background: "linear-gradient(90deg, #1f6feb, #58a6ff)",
							}}
						/>
					</div>
					{progress.node_name && (
						<p className="font-mono text-[11px]" style={{ color: "#8b949e" }}>
							↳ {progress.node_name}
							{progress.alive && progress.latency_ms ? (
								<span
									className="ml-2 font-medium"
									style={{ color: latencyColor(progress.latency_ms) }}
								>
									{progress.latency_ms}ms
								</span>
							) : null}
							{progress.alive && progress.speed_kbps ? (
								<span className="ml-1.5 font-medium" style={{ color: "#58a6ff" }}>
									{progress.speed_kbps >= 1024
										? `${(progress.speed_kbps / 1024).toFixed(1)}MB/s`
										: `${progress.speed_kbps}KB/s`}
								</span>
							) : null}
							{progress.alive === false ? (
								<span className="ml-2" style={{ color: "#f85149" }}>dead</span>
							) : null}
						</p>
					)}
				</div>
			)}

			{/* Job summary */}
			{job && (
				<div className="flex items-center gap-3 text-sm">
					<JobStatusBadge status={job.status} />
					<span style={{ color: "#8b949e" }}>
						{results.filter((r) => r.alive).length} / {job.total} alive
					</span>
				</div>
			)}

			{resultsQuery.isLoading && (
				<p className="text-sm" style={{ color: "#8b949e" }}>Loading results…</p>
			)}
			{resultsQuery.isError && (
				<p className="text-sm" style={{ color: "#8b949e" }}>No check results yet.</p>
			)}

			<NodeTable results={results} />
		</div>
	);
}
