import { useEffect, useState } from "react";
import { toast } from "sonner";
import { CheckOptionsFields } from "@/components/check-options-fields";
import { CronPicker } from "@/components/cron-picker";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	type CheckFormOptions,
	DEFAULT_CHECK_OPTIONS,
} from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import type { scheduler, subscription } from "@/lib/client.gen";
import { describeCron, nextRun } from "@/lib/cron";
import { useCreateScheduledJob } from "@/queries";

type ScheduledJob = scheduler.ScheduledJob;
type Subscription = subscription.Subscription;

const PRESETS = [
	{ label: "Every 6h", expr: "0 */6 * * *" },
	{ label: "Every 12h", expr: "0 */12 * * *" },
	{ label: "Daily 4:00", expr: "0 4 * * *" },
] as const;

// Create + edit share this dialog. scheduler.Create upserts on
// subscription_id, so "edit" is just Create with the same subscription.
export function ScheduleDialog({
	open,
	onOpenChange,
	subs,
	editing,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	subs: Subscription[];
	editing?: ScheduledJob | null;
}) {
	const [subId, setSubId] = useState("");
	const [cron, setCron] = useState("0 */6 * * *");
	const [opts, setOpts] = useState<CheckFormOptions>(DEFAULT_CHECK_OPTIONS);

	useEffect(() => {
		if (open) {
			setSubId(editing?.subscription_id ?? "");
			setCron(editing?.cron_expr ?? "0 */6 * * *");
			setOpts({
				...DEFAULT_CHECK_OPTIONS,
				speed_test: editing?.speed_test ?? true,
				media_apps: editing?.media_apps ?? [
					...DEFAULT_CHECK_OPTIONS.media_apps,
				],
			});
		}
	}, [open, editing]);

	const createMut = useCreateScheduledJob();
	const next = nextRun(cron);

	function submit() {
		createMut.mutate(
			{
				subscription_id: subId,
				cron_expr: cron,
				options: {
					speed_test: opts.speed_test,
					upload_speed_test: opts.upload_speed_test,
					media_apps: opts.media_apps,
					debug: opts.debug,
				},
			},
			{
				onSuccess: () => {
					toast.success(editing ? "Schedule updated" : "Schedule created");
					onOpenChange(false);
				},
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to save schedule"),
			},
		);
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				<DialogTitle>{editing ? "Edit schedule" : "New schedule"}</DialogTitle>
				<DialogDescription>
					Run automatic checks on a cron schedule.
				</DialogDescription>

				<div className="mt-4 space-y-4">
					<div className="space-y-1.5">
						<Label className="text-xs">Subscription</Label>
						<Select
							value={subId}
							onValueChange={(v) => v && setSubId(v)}
							disabled={!!editing}
						>
							<SelectTrigger className="w-full">
								<SelectValue placeholder="Choose a subscription…" />
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
						<Label className="text-xs">Schedule</Label>
						<div className="flex flex-wrap gap-1.5">
							{PRESETS.map((p) => (
								<button
									key={p.expr}
									type="button"
									aria-pressed={cron === p.expr}
									onClick={() => setCron(p.expr)}
									className={
										cron === p.expr
											? "rounded-full border border-info-line bg-info-muted px-3 py-1 font-medium text-info text-xs"
											: "rounded-full border border-border px-3 py-1 text-muted-foreground text-xs hover:bg-secondary"
									}
								>
									{p.label}
								</button>
							))}
						</div>
						<CronPicker value={cron} onChange={setCron} />
						<p className="text-muted-foreground text-xs">
							{describeCron(cron)}
							{next ? (
								<>
									{" · next: "}
									<span className="tabular-nums">{next.toLocaleString()}</span>
								</>
							) : (
								<span className="text-danger"> · invalid expression</span>
							)}
						</p>
					</div>

					<CheckOptionsFields value={opts} onChange={setOpts} />
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={createMut.isPending}
						disabled={!subId || !next}
						onClick={submit}
					>
						{editing ? "Save" : "Create"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
