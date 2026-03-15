import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Clock, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";

import { api, ApiError, type ScheduledJob, type Subscription } from "@/lib/api";

export const Route = createFileRoute("/scheduler")({
	component: SchedulerPage,
});

function SchedulerPage() {
	const qc = useQueryClient();
	const [adding, setAdding] = useState(false);
	const [subId, setSubId] = useState("");
	const [cronExpr, setCronExpr] = useState("");

	const jobsQuery = useQuery({
		queryKey: ["scheduler"],
		queryFn: () => api.get<{ jobs: ScheduledJob[] }>("/scheduler"),
	});

	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
	});

	const createMut = useMutation({
		mutationFn: () => api.post("/scheduler", { subscription_id: subId, cron_expr: cronExpr }),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			setAdding(false);
			setSubId("");
			setCronExpr("");
			toast.success("Schedule created");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => api.delete(`/scheduler/${id}`),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			toast.success("Removed");
		},
		onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
	});

	const jobs = jobsQuery.data?.jobs ?? [];
	const subs = subsQuery.data?.subscriptions ?? [];

	function subName(subId: string) {
		const s = subs.find((s) => s.id === subId);
		return s ? s.name || s.url : subId.slice(0, 8) + "…";
	}

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="text-lg font-semibold text-[#f0f6fc]">Scheduler</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Schedule
				</button>
			</div>

			{adding && (
				<div
					className="rounded-lg border p-4 space-y-3"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<div className="space-y-1.5">
						<Label className="text-xs text-[#8b949e]">Subscription</Label>
						<Select value={subId} onValueChange={(v) => setSubId(v ?? "")}>
							<SelectTrigger className="h-8 text-sm">
								<SelectValue placeholder="Select subscription…" />
							</SelectTrigger>
							<SelectContent>
								{subs.map((s) => (
									<SelectItem key={s.id} value={s.id}>
										{s.name || s.url}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="space-y-1.5">
						<Label className="text-xs text-[#8b949e]">Cron Expression</Label>
						<Input
							placeholder="0 */6 * * *  (every 6 hours)"
							value={cronExpr}
							onChange={(e) => setCronExpr(e.target.value)}
							className="h-8 text-sm font-mono"
						/>
					</div>
					<div className="flex gap-2">
						<Button
							size="sm"
							onClick={() => createMut.mutate()}
							disabled={!subId || !cronExpr || createMut.isPending}
							style={{ background: "#238636", color: "#fff" }}
							className="border-0"
						>
							{createMut.isPending ? <Loader2 size={13} className="animate-spin" /> : "Save"}
						</Button>
						<Button size="sm" variant="outline" onClick={() => setAdding(false)}>
							Cancel
						</Button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{jobs.map((job) => (
					<div
						key={job.id}
						className="flex items-center justify-between rounded-lg border px-4 py-3"
						style={{ background: "#161b22", borderColor: "#30363d" }}
					>
						<div className="flex items-center gap-3">
							<Clock size={13} strokeWidth={1.5} style={{ color: "#8b949e" }} />
							<div>
								<p className="font-mono text-sm text-[#f0f6fc]">{job.cron_expr}</p>
								<p className="text-xs mt-0.5" style={{ color: "#8b949e" }}>
									{subName(job.subscription_id)}
								</p>
							</div>
						</div>
						<button
							type="button"
							onClick={() => deleteMut.mutate(job.id)}
							disabled={deleteMut.isPending}
							className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
							style={{ color: "#6e7681" }}
						>
							<Trash2 size={13} strokeWidth={1.5} />
						</button>
					</div>
				))}
				{!jobsQuery.isLoading && jobs.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No scheduled jobs.
					</p>
				)}
			</div>
		</div>
	);
}
