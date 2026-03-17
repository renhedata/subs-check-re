// frontend/apps/web/src/routes/index.tsx

import { Skeleton } from "@frontend/ui/components/skeleton";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { CheckCircle, Clock, FileText, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";

type LocalUnlockResult = checker.LocalUnlockResult;

import { isAuthenticated } from "@/lib/auth";

export const Route = createFileRoute("/")({
	beforeLoad: () => {
		if (!isAuthenticated()) throw redirect({ to: "/login" });
	},
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
		<div
			className="rounded-lg border p-4"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			<div className="mb-2 flex items-center gap-1.5">
				<Icon size={13} strokeWidth={1.5} className="text-[#8b949e]" />
				<span
					className="font-medium text-[11px] uppercase tracking-[0.4px]"
					style={{ color: "#8b949e" }}
				>
					{label}
				</span>
			</div>
			{loading ? (
				<Skeleton className="h-8 w-12" />
			) : (
				<p
					className="font-bold text-[28px] leading-none"
					style={{ color: valueColor ?? "#f0f6fc" }}
				>
					{value}
				</p>
			)}
			{sub && !loading && (
				<p className="mt-1 text-[11px]" style={{ color: "#8b949e" }}>
					{sub}
				</p>
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
				<h1 className="font-semibold text-[#f0f6fc] text-lg">Dashboard</h1>
				<p className="mt-0.5 text-sm" style={{ color: "#8b949e" }}>
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
					valueColor="#3fb950"
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
					<h2 className="font-semibold text-[#f0f6fc] text-sm">Export API</h2>
					<p className="mt-0.5 text-xs" style={{ color: "#8b949e" }}>
						Use these URLs as subscription links directly in your proxy client.
					</p>
				</div>

				{/* API Key */}
				<div
					className="space-y-2 rounded-lg border p-4"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<p className="font-medium text-xs" style={{ color: "#8b949e" }}>
						API Key
					</p>
					<div className="flex items-center gap-2">
						<code className="flex-1 truncate rounded bg-[#0d1117] px-3 py-1.5 font-mono text-[#f0f6fc] text-xs">
							{apiKey || "—"}
						</code>
						<button
							type="button"
							onClick={() => {
								navigator.clipboard.writeText(apiKey);
								toast.success("Copied");
							}}
							className="rounded-md border px-2.5 py-1.5 text-xs transition-colors hover:bg-white/5"
							style={{ borderColor: "#30363d", color: "#8b949e" }}
						>
							Copy
						</button>
						<button
							type="button"
							onClick={() => regenerateMut.mutate()}
							disabled={regenerateMut.isPending}
							className="rounded-md border px-2.5 py-1.5 text-xs transition-colors hover:bg-white/5 disabled:opacity-50"
							style={{ borderColor: "#30363d", color: "#8b949e" }}
						>
							Regenerate
						</button>
					</div>
				</div>

				{/* All Subscriptions combined export */}
				{subs.length > 0 && apiKey && (
					<div
						className="space-y-2 rounded-lg border p-4"
						style={{ background: "#161b22", borderColor: "#30363d" }}
					>
						<p className="font-medium text-[#f0f6fc] text-sm">
							All Subscriptions
						</p>
						<div className="flex flex-col gap-1.5">
							{(["clash", "base64"] as const).map((t) => {
								const url = `${origin}/api/export/all?token=${apiKey}&target=${t}`;
								return (
									<div key={t} className="flex items-center gap-2">
										<code className="flex-1 truncate rounded bg-[#0d1117] px-2 py-1 font-mono text-[#8b949e] text-[11px]">
											{url}
										</code>
										<button
											type="button"
											onClick={() => {
												navigator.clipboard.writeText(url);
												toast.success("Copied");
											}}
											className="flex-shrink-0 rounded border px-2 py-1 text-[11px] hover:bg-white/5"
											style={{ borderColor: "#30363d", color: "#6e7681" }}
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
									className="space-y-2 rounded-lg border p-4"
									style={{ background: "#161b22", borderColor: "#30363d" }}
								>
									<p className="font-medium text-[#f0f6fc] text-sm">
										{sub.name || sub.url}
									</p>
									<div className="flex flex-col gap-1.5">
										{(["clash", "base64"] as const).map((t) => (
											<div key={t} className="flex items-center gap-2">
												<code className="flex-1 truncate rounded bg-[#0d1117] px-2 py-1 font-mono text-[#8b949e] text-[11px]">
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
													className="flex-shrink-0 rounded border px-2 py-1 text-[11px] hover:bg-white/5"
													style={{ borderColor: "#30363d", color: "#6e7681" }}
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
				<div
					className="rounded-lg border p-4"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<p className="mb-2 font-medium text-xs" style={{ color: "#8b949e" }}>
						Parameters
					</p>
					<table className="w-full text-xs" style={{ color: "#8b949e" }}>
						<tbody>
							<tr>
								<td className="py-0.5 pr-4 font-mono text-[#58a6ff]">token</td>
								<td>Your API key (required)</td>
							</tr>
							<tr>
								<td className="py-0.5 pr-4 font-mono text-[#58a6ff]">target</td>
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
	media: { background: "#3d1a1a", color: "#f85149" },
	ai: { background: "#1a3a1a", color: "#3fb950" },
	other: { background: "#1a2a3a", color: "#58a6ff" },
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
					<h2 className="font-semibold text-[#f0f6fc] text-sm">
						Server Network Unlock
					</h2>
					<p className="mt-0.5 text-xs" style={{ color: "#8b949e" }}>
						Platforms accessible from this server's IP
					</p>
				</div>
				<button
					type="button"
					onClick={() => refetch()}
					disabled={isFetching}
					className="flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs transition-colors hover:bg-white/5 disabled:opacity-50"
					style={{ borderColor: "#30363d", color: "#8b949e" }}
				>
					<RefreshCw
						size={11}
						strokeWidth={1.5}
						className={isFetching ? "animate-spin" : ""}
					/>
					Refresh
				</button>
			</div>
			<div
				className="rounded-lg border p-4"
				style={{ background: "#161b22", borderColor: "#30363d" }}
			>
				{isLoading ? (
					<div className="flex flex-wrap gap-2">
						{PLATFORM_DEFS.map((p) => (
							<div
								key={p.key}
								className="h-6 w-10 animate-pulse rounded"
								style={{ background: "#21262d" }}
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
									? BADGE_STYLES[p.style]
									: { background: "#21262d", color: "#484f58" };
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
							<p className="font-mono text-[11px]" style={{ color: "#6e7681" }}>
								{data.country && <span className="mr-1">{data.country}</span>}
								{data.ip}
							</p>
						)}
					</div>
				) : (
					<p className="text-xs" style={{ color: "#6e7681" }}>
						Click Refresh to check.
					</p>
				)}
			</div>
		</div>
	);
}
