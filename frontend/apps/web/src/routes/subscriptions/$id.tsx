import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";
import { NodeTable } from "@/components/node-table";
import {
	api,
	type CheckJob,
	type NodeResult,
	type Subscription,
} from "@/lib/api";

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
			className="rounded-full px-2 py-0.5 font-medium text-[10px]"
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
	const [logEntries, setLogEntries] = useState<SSEProgress[]>([]);
	const logEndRef = useRef<HTMLDivElement | null>(null);
	const esRef = useRef<EventSource | null>(null);
	const qc = useQueryClient();
	const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

	const jobsQuery = useQuery({
		queryKey: ["jobs", id],
		queryFn: () =>
			api.get<{ jobs: CheckJob[]; total: number }>(
				`/check/${id}/jobs?limit=10`,
			),
		staleTime: 5_000,
	});

	const resultsQuery = useQuery({
		queryKey: ["results", id, selectedJobId],
		queryFn: () => {
			const qs = selectedJobId ? `?job_id=${selectedJobId}` : "";
			return api.get<{ job: CheckJob; results: NodeResult[] }>(
				`/check/${id}/results${qs}`,
			);
		},
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
		if (
			job &&
			(job.status === "running" || job.status === "queued") &&
			!jobId
		) {
			setJobId(job.id);
		}
	}, [resultsQuery.data?.job?.id, resultsQuery.data?.job?.status, jobId]);

	// Clear log when a new job starts
	// biome-ignore lint/correctness/useExhaustiveDependencies: clear log only when jobId changes
	useEffect(() => {
		setLogEntries([]);
		setProgress(null);
	}, [jobId]);

	useEffect(() => {
		if (!jobId) return;
		const es = new EventSource(`/api/check/${jobId}/progress`);
		esRef.current = es;
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			setProgress(data);
			if (data.node_name) {
				setLogEntries((prev) => [...prev, data]);
			}
			if (data.done) {
				es.close();
				setSelectedJobId(null);
				resultsQuery.refetch();
				qc.invalidateQueries({ queryKey: ["jobs", id] });
			}
		};
		es.onerror = () => es.close();
		return () => es.close();
	}, [jobId, resultsQuery.refetch, qc, id]);

	// Auto-scroll log to bottom
	// biome-ignore lint/correctness/useExhaustiveDependencies: scroll on new entries
	useEffect(() => {
		logEndRef.current?.scrollIntoView({ behavior: "smooth" });
	}, [logEntries.length]);

	const job = resultsQuery.data?.job;
	const results = resultsQuery.data?.results ?? [];
	const progressPct = progress?.total
		? ((progress.progress ?? 0) / progress.total) * 100
		: 0;

	return (
		<div className="space-y-5">
			{/* Header */}
			<div>
				<h1 className="font-semibold text-[#f0f6fc] text-lg">
					{sub?.name || sub?.url || "Subscription Detail"}
				</h1>
				<p className="mt-0.5 font-mono text-xs" style={{ color: "#6e7681" }}>
					{id.slice(0, 8)}…
				</p>
			</div>

			{/* Job history pills */}
			{(jobsQuery.data?.jobs.length ?? 0) > 0 && (
				<div className="flex flex-wrap gap-1.5">
					{jobsQuery.data?.jobs.map((j) => {
						const active =
							selectedJobId === j.id ||
							(!selectedJobId && j.id === resultsQuery.data?.job.id);
						return (
							<button
								key={j.id}
								type="button"
								onClick={() => setSelectedJobId(j.id)}
								className="rounded-full border px-2.5 py-0.5 font-mono text-[11px] transition-colors"
								style={{
									borderColor: active ? "#58a6ff" : "#30363d",
									color: active ? "#58a6ff" : "#6e7681",
									background: active ? "#1a2a3a" : "transparent",
								}}
							>
								{new Date(j.created_at).toLocaleString(undefined, {
									month: "short",
									day: "numeric",
									hour: "2-digit",
									minute: "2-digit",
								})}
								{" · "}
								{j.available}/{j.total}
							</button>
						);
					})}
				</div>
			)}

			{/* Progress + live log */}
			{progress && !progress.done && (
				<div
					className="space-y-2 rounded-lg border p-3"
					style={{ background: "#0d1117", borderColor: "#30363d" }}
				>
					{/* Header row */}
					<div className="flex items-center justify-between">
						<div
							className="flex items-center gap-1.5 text-sm"
							style={{ color: "#f0f6fc" }}
						>
							<RefreshCw
								size={13}
								strokeWidth={1.5}
								className="animate-spin"
								style={{ color: "#58a6ff" }}
							/>
							Checking nodes…
						</div>
						<span className="font-mono text-xs" style={{ color: "#8b949e" }}>
							{progress.progress ?? 0} / {progress.total ?? "?"}
						</span>
					</div>

					{/* Progress bar */}
					<div
						className="h-[3px] w-full overflow-hidden rounded-full"
						style={{ background: "#21262d" }}
					>
						<div
							className="h-full rounded-full transition-[width] duration-300 ease-out"
							style={{
								width: `${progressPct}%`,
								background: "linear-gradient(90deg, #1f6feb, #58a6ff)",
							}}
						/>
					</div>

					{/* Scrollable log */}
					{logEntries.length > 0 && (
						<div
							className="max-h-48 overflow-y-auto"
							style={{ scrollbarWidth: "thin" }}
						>
							{logEntries.map((entry, i) => (
								<div
									key={i}
									className="flex items-baseline gap-2 py-0.5 font-mono text-[11px]"
								>
									<span
										className="flex-shrink-0"
										style={{ color: entry.alive ? "#3fb950" : "#f85149" }}
									>
										{entry.alive ? "✓" : "✗"}
									</span>
									<span
										className="min-w-0 flex-1 truncate"
										style={{ color: "#c9d1d9" }}
									>
										{entry.node_name}
									</span>
									{entry.alive && entry.latency_ms ? (
										<span
											className="flex-shrink-0"
											style={{ color: latencyColor(entry.latency_ms) }}
										>
											{entry.latency_ms}ms
										</span>
									) : null}
									{entry.alive && entry.speed_kbps ? (
										<span
											className="flex-shrink-0"
											style={{ color: "#58a6ff" }}
										>
											{entry.speed_kbps >= 1024
												? `${(entry.speed_kbps / 1024).toFixed(1)}MB/s`
												: `${entry.speed_kbps}KB/s`}
										</span>
									) : null}
								</div>
							))}
							<div ref={logEndRef} />
						</div>
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
					{jobId && progress && !progress.done && (
						<span className="text-[11px]" style={{ color: "#6e7681" }}>
							· previous results
						</span>
					)}
				</div>
			)}

			{resultsQuery.isLoading && (
				<p className="text-sm" style={{ color: "#8b949e" }}>
					Loading results…
				</p>
			)}
			{resultsQuery.isError && !resultsQuery.isLoading && (
				<div className="py-12 text-center">
					<p className="text-sm" style={{ color: "#8b949e" }}>
						No checks run yet.
					</p>
					<p className="mt-1 text-xs" style={{ color: "#6e7681" }}>
						Click Check on the subscriptions page to start a check.
					</p>
				</div>
			)}

			<NodeTable results={results} />
		</div>
	);
}
