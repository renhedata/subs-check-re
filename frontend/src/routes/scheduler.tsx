import { createFileRoute } from "@tanstack/react-router";
import { CalendarClock, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ScheduleDialog } from "@/components/schedule-dialog";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { scheduler } from "@/lib/client.gen";
import { describeCron, formatUntil, nextRun } from "@/lib/cron";
import {
	useDeleteScheduledJob,
	useLatestJobs,
	useScheduledJobs,
	useSetScheduleEnabled,
	useSubscriptions,
} from "@/queries";

type ScheduledJob = scheduler.ScheduledJob;

export const Route = createFileRoute("/scheduler")({
	component: SchedulerPage,
});

function SchedulerPage() {
	const [dialogOpen, setDialogOpen] = useState(false);
	const [editing, setEditing] = useState<ScheduledJob | null>(null);
	const [deleting, setDeleting] = useState<ScheduledJob | null>(null);

	const jobsQuery = useScheduledJobs();
	const subsQuery = useSubscriptions();
	const latestQuery = useLatestJobs();

	const jobs = jobsQuery.data?.jobs ?? [];
	const subs = subsQuery.data?.subscriptions ?? [];
	const latestJobs = latestQuery.data?.jobs ?? {};
	const subName = (id: string) => {
		const s = subs.find((x) => x.id === id);
		return s ? s.name || s.url : id.slice(0, 8);
	};

	const toggleMut = useSetScheduleEnabled();
	const deleteMut = useDeleteScheduledJob();

	const handleToggle = (job: ScheduledJob, enabled: boolean) =>
		toggleMut.mutate(
			{ id: job.id, enabled },
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);

	const handleDelete = () => {
		if (!deleting) return;
		deleteMut.mutate(deleting.id, {
			onSuccess: () => {
				toast.success("Schedule deleted");
				setDeleting(null);
			},
			onError: (e) => toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

	return (
		<div className="h-full overflow-y-auto">
			<div className="mx-auto max-w-4xl space-y-5 p-4 pb-8 md:p-6">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="font-semibold text-foreground text-lg">Scheduler</h1>
						<p className="mt-0.5 text-muted-foreground text-sm">
							Automatic checks on a cron schedule
						</p>
					</div>
					<Button
						variant="success"
						onClick={() => {
							setEditing(null);
							setDialogOpen(true);
						}}
					>
						<Plus size={14} /> New schedule
					</Button>
				</div>

				{jobsQuery.isLoading ? (
					<div className="space-y-2">
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
					</div>
				) : jobs.length === 0 ? (
					<div className="rounded-lg border border-border">
						<EmptyState
							icon={CalendarClock}
							title="No schedules yet"
							description="Create one to check a subscription automatically — every few hours or at a fixed time."
							action={
								<Button variant="success" onClick={() => setDialogOpen(true)}>
									New schedule
								</Button>
							}
						/>
					</div>
				) : (
					<div className="overflow-x-auto rounded-lg border border-border">
						<table className="w-full border-collapse text-[12.5px]">
							<thead>
								<tr className="border-border border-b bg-card text-left text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									<th className="px-3 py-2 font-medium">Subscription</th>
									<th className="px-3 py-2 font-medium">Schedule</th>
									<th className="px-3 py-2 font-medium">Next run</th>
									<th className="px-3 py-2 font-medium">Last check</th>
									<th className="px-3 py-2 text-right font-medium">Enabled</th>
									<th className="w-10 px-2 py-2" aria-label="Actions" />
								</tr>
							</thead>
							<tbody className="tabular-nums">
								{jobs.map((job) => {
									const next = job.enabled ? nextRun(job.cron_expr) : null;
									const latest = latestJobs[job.subscription_id];
									return (
										<tr
											key={job.id}
											className="border-secondary border-b last:border-0"
										>
											<td className="px-3 py-2.5 font-medium text-foreground">
												<button
													type="button"
													onClick={() => {
														setEditing(job);
														setDialogOpen(true);
													}}
													className="hover:underline"
												>
													{subName(job.subscription_id)}
												</button>
											</td>
											<td className="px-3 py-2.5">
												{describeCron(job.cron_expr)}{" "}
												<span className="font-mono text-[11px] text-muted-foreground">
													{job.cron_expr}
												</span>
											</td>
											<td className="px-3 py-2.5 text-info">
												{next ? formatUntil(next) : "—"}
											</td>
											<td className="px-3 py-2.5 text-muted-foreground">
												{latest ? (
													<>
														<span
															className={
																latest.status === "failed"
																	? "text-danger"
																	: latest.status === "completed"
																		? "text-success"
																		: "text-info"
															}
														>
															{latest.status === "completed"
																? "✓"
																: latest.status === "failed"
																	? "✕"
																	: "⟳"}
														</span>{" "}
														{new Date(
															latest.finished_at ?? latest.created_at,
														).toLocaleString(undefined, {
															month: "short",
															day: "numeric",
															hour: "2-digit",
															minute: "2-digit",
														})}
														{latest.status === "completed"
															? ` · ${latest.available}/${latest.total}`
															: ""}
													</>
												) : (
													"—"
												)}
											</td>
											<td className="px-3 py-2.5 text-right">
												<Switch
													checked={job.enabled}
													onCheckedChange={(v) => handleToggle(job, v)}
													disabled={toggleMut.isPending}
												/>
											</td>
											<td className="px-2 py-2.5 text-right">
												<button
													type="button"
													aria-label="Delete schedule"
													onClick={() => setDeleting(job)}
													className="rounded p-1.5 text-muted-foreground transition-colors hover:bg-danger-muted hover:text-danger"
												>
													<Trash2 size={13} />
												</button>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>
				)}

				<ScheduleDialog
					open={dialogOpen}
					onOpenChange={setDialogOpen}
					subs={subs}
					editing={editing}
				/>
				<ConfirmDialog
					open={!!deleting}
					onOpenChange={(o) => !o && setDeleting(null)}
					title={`Delete schedule for "${deleting ? subName(deleting.subscription_id) : ""}"?`}
					description="Automatic checks for this subscription will stop. The subscription itself is not affected."
					pending={deleteMut.isPending}
					onConfirm={handleDelete}
				/>
			</div>
		</div>
	);
}
