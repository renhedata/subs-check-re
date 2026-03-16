// frontend/apps/web/src/routes/index.tsx

import { Skeleton } from "@frontend/ui/components/skeleton";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { CheckCircle, Clock, FileText } from "lucide-react";

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
	const { data, isLoading } = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
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
		</div>
	);
}
