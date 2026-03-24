import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import {
	Check,
	ChevronDown,
	ChevronRight,
	Copy,
	Download,
	RefreshCw,
	Square,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { NodeTable } from "@/components/node-table";
import { client, isApiError } from "@/lib/client";
import { formatBytes } from "@/lib/format";

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
	if (ms < 50) return "var(--color-success)";
	if (ms <= 200) return "var(--color-warning)";
	return "var(--destructive)";
}

type JobStatus = "queued" | "running" | "completed" | "failed";

function JobStatusBadge({ status }: { status: string }) {
	const map: Record<JobStatus, { bg: string; color: string }> = {
		queued: {
			bg: "var(--color-badge-info-bg)",
			color: "var(--color-badge-info)",
		},
		running: {
			bg: "var(--color-badge-info-bg)",
			color: "var(--color-badge-info)",
		},
		completed: {
			bg: "var(--color-badge-success-bg)",
			color: "var(--color-badge-success)",
		},
		failed: {
			bg: "var(--color-badge-danger-bg)",
			color: "var(--color-badge-danger)",
		},
	};
	const s = map[status as JobStatus] ?? {
		bg: "var(--color-badge-info-bg)",
		color: "var(--muted-foreground)",
	};
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
		queryFn: () => client.checker.ListJobs(id, { Limit: 10, Offset: 0 }),
		staleTime: 5_000,
	});

	const resultsQuery = useQuery({
		queryKey: ["results", id, selectedJobId],
		queryFn: () =>
			client.checker.GetResults(id, { JobID: selectedJobId ?? "" }),
		retry: false,
		staleTime: 0,
	});

	// Resolve subscription name from cache
	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => client.subscription.List(),
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

	const cancelMut = useMutation({
		mutationFn: (jid: string) => client.checker.CancelCheck(jid),
		onSuccess: () => {
			setJobId(null);
			setProgress(null);
			qc.invalidateQueries({ queryKey: ["jobs", id] });
			resultsQuery.refetch();
			toast.success("Check cancelled");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Cancel failed"),
	});

	// Clear log when a new job starts
	// biome-ignore lint/correctness/useExhaustiveDependencies: clear log only when jobId changes
	useEffect(() => {
		setLogEntries([]);
		setProgress(null);
	}, [jobId]);

	useEffect(() => {
		if (!jobId) return;
		const es = new EventSource(
			`${window.location.origin}/api/check/${jobId}/progress`,
		);
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
				<h1 className="font-semibold text-foreground text-lg">
					{sub?.name || sub?.url || "Subscription Detail"}
				</h1>
				<p
					className="mt-0.5 font-mono text-xs"
					style={{ color: "var(--color-dimmed)" }}
				>
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
									borderColor: active ? "var(--primary)" : "var(--border)",
									color: active ? "var(--primary)" : "var(--color-dimmed)",
									background: active
										? "var(--color-badge-info-bg)"
										: "transparent",
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
								{j.total_traffic_bytes > 0
									? ` · ${formatBytes(j.total_traffic_bytes)}`
									: ""}
							</button>
						);
					})}
				</div>
			)}

			{/* Progress + live log */}
			{progress && !progress.done && (
				<div className="space-y-2 rounded-lg border border-border bg-background p-3">
					{/* Header row */}
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-1.5 text-sm text-foreground">
							<RefreshCw
								size={13}
								strokeWidth={1.5}
								className="animate-spin"
								style={{ color: "var(--primary)" }}
							/>
							Checking nodes…
						</div>
						<div className="flex items-center gap-2">
							<span
								className="font-mono text-xs text-muted-foreground"
							>
								{progress.progress ?? 0} / {progress.total ?? "?"}
							</span>
							{jobId && (
								<button
									type="button"
									onClick={() => cancelMut.mutate(jobId)}
									disabled={cancelMut.isPending}
									className="flex items-center gap-1 rounded border border-border px-2 py-0.5 text-[11px] transition-colors hover:border-[#f85149]/60 hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
									style={{ color: "var(--color-dimmed)" }}
								>
									<Square size={9} strokeWidth={2} />
									Stop
								</button>
							)}
						</div>
					</div>

					{/* Progress bar */}
					<div
						className="h-[3px] w-full overflow-hidden rounded-full bg-secondary"
					>
						<div
							className="h-full rounded-full transition-[width] duration-300 ease-out"
							style={{
								width: `${progressPct}%`,
								background: "var(--color-progress)",
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
										style={{
											color: entry.alive
												? "var(--color-success)"
												: "var(--destructive)",
										}}
									>
										{entry.alive ? "✓" : "✗"}
									</span>
									<span
										className="min-w-0 flex-1 truncate"
										style={{ color: "var(--color-code)" }}
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
											style={{ color: "var(--primary)" }}
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
					<span className="text-muted-foreground">
						{results.filter((r) => r.alive).length} / {job.total} alive
					</span>
					{job.total_traffic_bytes > 0 && (
						<span style={{ color: "var(--color-dimmed)" }}>
							· {formatBytes(job.total_traffic_bytes)}
						</span>
					)}
					{jobId && progress && !progress.done && (
						<span
							className="text-[11px]"
							style={{ color: "var(--color-dimmed)" }}
						>
							· previous results
						</span>
					)}
				</div>
			)}

			{resultsQuery.isLoading && (
				<p className="text-sm text-muted-foreground">Loading results…</p>
			)}
			{resultsQuery.isError && !resultsQuery.isLoading && (
				<div className="py-12 text-center">
					<p className="text-sm text-muted-foreground">No checks run yet.</p>
					<p className="mt-1 text-xs" style={{ color: "var(--color-dimmed)" }}>
						Click Check on the subscriptions page to start a check.
					</p>
				</div>
			)}

			<NodeTable results={results} />

			<ExportLinksSection subscriptionId={id} />

			<ExportLogsSection subscriptionId={id} />
		</div>
	);
}

function CopyButton({ text }: { text: string }) {
	const [copied, setCopied] = useState(false);
	return (
		<button
			type="button"
			onClick={() => {
				navigator.clipboard.writeText(text);
				setCopied(true);
				setTimeout(() => setCopied(false), 1500);
			}}
			className="flex-shrink-0 rounded p-1 transition-colors hover:bg-secondary"
			style={{ color: copied ? "var(--color-success)" : "var(--color-dimmed)" }}
		>
			{copied ? <Check size={12} /> : <Copy size={12} />}
		</button>
	);
}

function ExportLinksSection({ subscriptionId }: { subscriptionId: string }) {
	const [open, setOpen] = useState(false);

	const apiKeyQuery = useQuery({
		queryKey: ["api-key"],
		queryFn: () => client.settings.GetAPIKey(),
		enabled: open,
		staleTime: Number.POSITIVE_INFINITY,
	});

	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const base = `${window.location.origin}/api/export/${subscriptionId}`;

	const links = [
		{ label: "Clash", url: `${base}?token=${apiKey}&target=clash` },
		{ label: "Base64", url: `${base}?token=${apiKey}&target=base64` },
		{ label: "RouterOS (.rsc)", url: `${base}?token=${apiKey}&target=routeros` },
	];

	return (
		<div className="rounded-lg border border-border bg-card">
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="flex w-full items-center justify-between px-4 py-3"
			>
				<div className="flex items-center gap-2 text-sm text-muted-foreground">
					<Download size={13} strokeWidth={1.5} />
					Export links
				</div>
				{open ? (
					<ChevronDown size={13} strokeWidth={1.5} style={{ color: "var(--color-dimmed)" }} />
				) : (
					<ChevronRight size={13} strokeWidth={1.5} style={{ color: "var(--color-dimmed)" }} />
				)}
			</button>

			{open && (
				<div className="space-y-2 border-t border-border px-4 py-3">
					{apiKeyQuery.isLoading && (
						<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>Loading…</p>
					)}
					{!apiKeyQuery.isLoading && links.map(({ label, url }) => (
						<div key={label}>
							<p className="mb-1 text-[11px] font-medium text-muted-foreground">{label}</p>
							<div className="flex items-center gap-1.5 rounded-md border border-border bg-background px-2.5 py-1.5">
								<span
									className="min-w-0 flex-1 truncate font-mono text-[11px]"
									style={{ color: "var(--color-code)" }}
								>
									{url}
								</span>
								<CopyButton text={url} />
							</div>
						</div>
					))}
				</div>
			)}
		</div>
	);
}

function ExportLogsSection({ subscriptionId }: { subscriptionId: string }) {
	const [open, setOpen] = useState(false);

	const logsQuery = useQuery({
		queryKey: ["export-logs", subscriptionId],
		queryFn: () => client.checker.GetExportLogs(subscriptionId),
		enabled: open,
		staleTime: 30_000,
	});

	const logs = logsQuery.data?.logs ?? [];

	return (
		<div className="rounded-lg border border-border bg-card">
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="flex w-full items-center justify-between px-4 py-3"
			>
				<div className="flex items-center gap-2 text-sm text-muted-foreground">
					<Download size={13} strokeWidth={1.5} />
					Export requests
				</div>
				{open ? (
					<ChevronDown
						size={13}
						strokeWidth={1.5}
						style={{ color: "var(--color-dimmed)" }}
					/>
				) : (
					<ChevronRight
						size={13}
						strokeWidth={1.5}
						style={{ color: "var(--color-dimmed)" }}
					/>
				)}
			</button>

			{open && (
				<div className="border-t border-border px-4 py-3">
					{logsQuery.isLoading && (
						<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
							Loading…
						</p>
					)}
					{!logsQuery.isLoading && logs.length === 0 && (
						<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
							No export requests yet.
						</p>
					)}
					<div className="space-y-1">
						{logs.map((log) => (
							<div
								key={log.id}
								className="flex items-center justify-between py-1"
							>
								<span
									className="font-mono text-[11px]"
									style={{ color: "var(--color-code)" }}
								>
									{new Date(log.requested_at).toLocaleString(undefined, {
										month: "short",
										day: "numeric",
										hour: "2-digit",
										minute: "2-digit",
										second: "2-digit",
									})}
								</span>
								<span className="font-mono text-[11px] text-muted-foreground">
									{log.ip || "—"}
								</span>
							</div>
						))}
					</div>
				</div>
			)}
		</div>
	);
}
