// frontend/apps/web/src/routes/index.tsx

import { Skeleton } from "@frontend/ui/components/skeleton";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { CheckCircle, Clock, FileText, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";

type LocalUnlockResult = checker.LocalUnlockResult;

export const Route = createFileRoute("/")({
	component: DashboardPage,
});

function StatCard({
	label,
	value,
	icon: Icon,
	valueColor,
	sub,
	loading,
}: {
	label: string;
	value: number;
	icon: React.ElementType;
	valueColor?: string;
	sub?: string;
	loading: boolean;
}) {
	return (
		<div className="rounded-lg border border-border bg-card p-4">
			<div className="mb-2 flex items-center gap-1.5">
				<Icon size={13} strokeWidth={1.5} className="text-muted-foreground" />
				<span className="font-medium text-[11px] uppercase tracking-[0.4px] text-muted-foreground">
					{label}
				</span>
			</div>
			{loading ? (
				<Skeleton className="h-8 w-12" />
			) : (
				<p
					className="font-bold text-[28px] leading-none"
					style={{ color: valueColor ?? "var(--foreground)" }}
				>
					{value}
				</p>
			)}
			{sub && !loading && (
				<p className="mt-1 text-[11px] text-muted-foreground">{sub}</p>
			)}
		</div>
	);
}

function DashboardPage() {
	const qc = useQueryClient();

	const { data, isLoading } = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => client.subscription.List(),
	});

	const apiKeyQuery = useQuery({
		queryKey: ["api-key"],
		queryFn: () => client.settings.GetAPIKey(),
		staleTime: Number.POSITIVE_INFINITY,
	});

	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const origin = typeof window !== "undefined" ? window.location.origin : "";

	const regenerateMut = useMutation({
		mutationFn: () => client.settings.RegenerateAPIKey(),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["api-key"] });
			toast.success("API key regenerated");
		},
	});

	const subs = data?.subscriptions ?? [];
	const enabled = subs.filter((s) => s.enabled).length;
	const scheduled = subs.filter((s) => s.cron_expr).length;

	return (
		<div className="space-y-6">
			<div>
				<h1 className="font-semibold text-foreground text-lg">Dashboard</h1>
				<p className="mt-0.5 text-sm text-muted-foreground">
					Overview of your proxy subscriptions
				</p>
			</div>

			<div className="grid gap-3 sm:grid-cols-3">
				<StatCard
					label="Subscriptions"
					icon={FileText}
					value={subs.length}
					loading={isLoading}
				/>
				<StatCard
					label="Active"
					icon={CheckCircle}
					value={enabled}
					valueColor="var(--color-success)"
					sub={`of ${subs.length} total`}
					loading={isLoading}
				/>
				<StatCard
					label="Scheduled"
					icon={Clock}
					value={scheduled}
					sub="cron jobs"
					loading={isLoading}
				/>
			</div>

			<NetworkUnlockPanel />

			{/* Export API */}
			<div className="space-y-4">
				<div>
					<h2 className="font-semibold text-foreground text-sm">Export API</h2>
					<p className="mt-0.5 text-xs text-muted-foreground">
						Use these URLs as subscription links directly in your proxy client.
					</p>
				</div>

				{/* API Key */}
				<div className="space-y-2 rounded-lg border border-border bg-card p-4">
					<p className="font-medium text-xs text-muted-foreground">API Key</p>
					<div className="flex items-center gap-2">
						<code className="flex-1 truncate rounded bg-background px-3 py-1.5 font-mono text-foreground text-xs">
							{apiKey || "—"}
						</code>
						<button
							type="button"
							onClick={() => {
								navigator.clipboard.writeText(apiKey);
								toast.success("Copied");
							}}
							className="rounded-md border border-border px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-white/5"
						>
							Copy
						</button>
						<button
							type="button"
							onClick={() => regenerateMut.mutate()}
							disabled={regenerateMut.isPending}
							className="rounded-md border border-border px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-white/5 disabled:opacity-50"
						>
							Regenerate
						</button>
					</div>
				</div>

				{/* All Subscriptions combined export */}
				{subs.length > 0 && apiKey && (
					<div className="space-y-2 rounded-lg border border-border bg-card p-4">
						<p className="font-medium text-foreground text-sm">
							All Subscriptions
						</p>
						<div className="flex flex-col gap-1.5">
							{(["clash", "base64"] as const).map((t) => {
								const url = `${origin}/api/export/all?token=${apiKey}&target=${t}`;
								return (
									<div key={t} className="flex items-center gap-2">
										<code className="flex-1 truncate rounded bg-background px-2 py-1 font-mono text-muted-foreground text-[11px]">
											{url}
										</code>
										<button
											type="button"
											onClick={() => {
												navigator.clipboard.writeText(url);
												toast.success("Copied");
											}}
											className="flex-shrink-0 rounded border border-border px-2 py-1 text-[11px] text-muted-foreground hover:bg-white/5"
										>
											{t}
										</button>
									</div>
								);
							})}
						</div>
					</div>
				)}

				{/* Per-subscription URLs */}
				{subs.length > 0 && apiKey && (
					<div className="space-y-2">
						{subs.map((sub) => {
							const base = `${origin}/api/export/${sub.id}?token=${apiKey}`;
							return (
								<div
									key={sub.id}
									className="space-y-2 rounded-lg border border-border bg-card p-4"
								>
									<p className="font-medium text-foreground text-sm">
										{sub.name || sub.url}
									</p>
									<div className="flex flex-col gap-1.5">
										{(["clash", "base64"] as const).map((t) => (
											<div key={t} className="flex items-center gap-2">
												<code className="flex-1 truncate rounded bg-background px-2 py-1 font-mono text-muted-foreground text-[11px]">
													{base}&target={t}
												</code>
												<button
													type="button"
													onClick={() => {
														navigator.clipboard.writeText(
															`${base}&target=${t}`,
														);
														toast.success("Copied");
													}}
													className="flex-shrink-0 rounded border border-border px-2 py-1 text-[11px] text-muted-foreground hover:bg-white/5"
												>
													{t}
												</button>
											</div>
										))}
									</div>
								</div>
							);
						})}
					</div>
				)}

				{/* Parameter reference */}
				<div className="rounded-lg border border-border bg-card p-4">
					<p className="mb-2 font-medium text-xs text-muted-foreground">
						Parameters
					</p>
					<table className="w-full text-xs text-muted-foreground">
						<tbody>
							<tr>
								<td className="py-0.5 pr-4 font-mono text-primary">token</td>
								<td>Your API key (required)</td>
							</tr>
							<tr>
								<td className="py-0.5 pr-4 font-mono text-primary">target</td>
								<td>
									<code>clash</code> (default) — Clash YAML ·{" "}
									<code>base64</code> — base64-encoded URI list
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>
		</div>
	);
}

const PLATFORM_DEFS: {
	key: keyof LocalUnlockResult;
	label: string;
	style: "media" | "ai" | "other";
}[] = [
	{ key: "openai", label: "GPT", style: "ai" },
	{ key: "claude", label: "CL", style: "ai" },
	{ key: "gemini", label: "GM", style: "ai" },
	{ key: "grok", label: "GK", style: "ai" },
	{ key: "netflix", label: "NF", style: "media" },
	{ key: "youtube", label: "YT", style: "media" },
	{ key: "youtube_premium", label: "YT+", style: "media" },
	{ key: "disney", label: "D+", style: "other" },
	{ key: "tiktok", label: "TK", style: "other" },
];

const BADGE_STYLES = {
	media: {
		on: {
			background: "var(--color-badge-danger-bg)",
			color: "var(--color-badge-danger)",
		},
		off: { background: "var(--secondary)", color: "var(--muted-foreground)" },
	},
	ai: {
		on: {
			background: "var(--color-badge-ai-bg)",
			color: "var(--color-badge-ai)",
		},
		off: { background: "var(--secondary)", color: "var(--muted-foreground)" },
	},
	other: {
		on: {
			background: "var(--color-badge-info-bg)",
			color: "var(--color-badge-info)",
		},
		off: { background: "var(--secondary)", color: "var(--muted-foreground)" },
	},
};

function NetworkUnlockPanel() {
	const { data, isLoading, isFetching, refetch } = useQuery({
		queryKey: ["local-unlock"],
		queryFn: () => client.checker.GetLocalUnlock(),
		staleTime: 5 * 60 * 1000,
		retry: false,
	});

	return (
		<div>
			<div className="mb-3 flex items-center justify-between">
				<div>
					<h2 className="font-semibold text-foreground text-sm">
						Server Network Unlock
					</h2>
					<p className="mt-0.5 text-xs text-muted-foreground">
						Platforms accessible from this server's IP
					</p>
				</div>
				<button
					type="button"
					onClick={() => refetch()}
					disabled={isFetching}
					className="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-white/5 disabled:opacity-50"
				>
					<RefreshCw
						size={11}
						strokeWidth={1.5}
						className={isFetching ? "animate-spin" : ""}
					/>
					Refresh
				</button>
			</div>
			<div className="rounded-lg border border-border bg-card p-4">
				{isLoading ? (
					<div className="flex flex-wrap gap-2">
						{PLATFORM_DEFS.map((p) => (
							<div
								key={p.key}
								className="h-6 w-10 animate-pulse rounded bg-secondary"
							/>
						))}
					</div>
				) : data ? (
					<div className="space-y-3">
						<div className="flex flex-wrap gap-2">
							{PLATFORM_DEFS.map((p) => {
								const val = data[p.key];
								const available = typeof val === "boolean" ? val : val !== "";
								const s = available
									? BADGE_STYLES[p.style].on
									: BADGE_STYLES[p.style].off;
								return (
									<span
										key={p.key}
										className="rounded px-2 py-1 font-semibold text-[11px]"
										style={s}
									>
										{p.label}
									</span>
								);
							})}
						</div>
						{(data.ip || data.country) && (
							<p
								className="font-mono text-[11px]"
								style={{ color: "var(--color-dimmed)" }}
							>
								{data.country && <span className="mr-1">{data.country}</span>}
								{data.ip}
							</p>
						)}
					</div>
				) : (
					<p
						className="text-xs"
						style={{ color: "var(--color-dimmed)" }}
					>
						Click Refresh to check.
					</p>
				)}
			</div>
		</div>
	);
}
