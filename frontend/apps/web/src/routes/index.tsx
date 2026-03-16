// frontend/apps/web/src/routes/index.tsx

import { Skeleton } from "@frontend/ui/components/skeleton";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { CheckCircle, Clock, FileText } from "lucide-react";
import { toast } from "sonner";

import { api, type Subscription } from "@/lib/api";
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
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
	});

	const apiKeyQuery = useQuery({
		queryKey: ["api-key"],
		queryFn: () => api.get<{ api_key: string }>("/settings/api-key"),
		staleTime: Number.POSITIVE_INFINITY,
	});

	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const origin = typeof window !== "undefined" ? window.location.origin : "";

	const regenerateMut = useMutation({
		mutationFn: () =>
			api.post<{ api_key: string }>("/settings/api-key/regenerate"),
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
