import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, Play } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { CheckOptionsFields } from "@/components/check-options-fields";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import {
	type CheckFormOptions,
	hasStoredCheckOptions,
	loadCheckOptions,
	reconcileMediaApps,
	saveCheckOptions,
} from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import { queryKeys, useRules, useTriggerCheck } from "@/queries";

// Split button: primary click re-runs with the last-used options
// (localStorage per subscription); the chevron opens the options popover.
export function RunCheckButton({
	subscriptionId,
	disabled,
	onStarted,
}: {
	subscriptionId: string;
	disabled?: boolean;
	onStarted: (jobId: string) => void;
}) {
	const [open, setOpen] = useState(false);
	const [opts, setOpts] = useState<CheckFormOptions>(() =>
		loadCheckOptions(subscriptionId),
	);
	const { data: rulesData } = useRules();
	const availablePlatforms = useMemo(
		() =>
			(rulesData?.rules ?? []).filter((r) => r.enabled).map((r) => r.key),
		[rulesData],
	);
	const reconciled = useRef(false);
	useEffect(() => {
		if (!rulesData || reconciled.current) return;
		reconciled.current = true;
		setOpts((prev) => ({
			...prev,
			media_apps: hasStoredCheckOptions(subscriptionId)
				? reconcileMediaApps(prev.media_apps, availablePlatforms)
				: availablePlatforms,
		}));
	}, [rulesData, availablePlatforms, subscriptionId]);
	const triggerMut = useTriggerCheck();
	const qc = useQueryClient();

	function start(withOpts: CheckFormOptions) {
		saveCheckOptions(subscriptionId, withOpts);
		triggerMut.mutate(
			{ subscriptionId, params: withOpts },
			{
				onSuccess: (resp) => {
					setOpen(false);
					toast.success("Check started");
					onStarted(resp.job_id);
				},
				onError: (e) => {
					// 409/412 = a check is already running (manual or scheduled).
					// Refresh latest-jobs so the workbench effect attaches to it.
					if (isApiError(e) && (e.status === 409 || e.status === 412)) {
						toast.error("A check is already running for this subscription");
						qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
						return;
					}
					toast.error(isApiError(e) ? e.message : "Failed to start check");
				},
			},
		);
	}

	return (
		<div className="flex">
			<Button
				variant="success"
				className="rounded-r-none"
				loading={triggerMut.isPending}
				disabled={disabled}
				onClick={() => start(opts)}
			>
				<Play size={13} /> Run Check
			</Button>
			<Popover open={open} onOpenChange={setOpen}>
				<PopoverTrigger
					render={
						<Button
							variant="success"
							size="icon"
							aria-label="Check options"
							className="rounded-l-none border-white/20 border-l"
							disabled={disabled || triggerMut.isPending}
						/>
					}
				>
					<ChevronDown size={14} />
				</PopoverTrigger>
				<PopoverContent className="w-80">
					<CheckOptionsFields
						value={opts}
						onChange={setOpts}
						availablePlatforms={availablePlatforms}
						showDebug
					/>
					<Button
						variant="success"
						className="mt-3 w-full"
						loading={triggerMut.isPending}
						onClick={() => start(opts)}
					>
						Start check
					</Button>
				</PopoverContent>
			</Popover>
		</div>
	);
}
