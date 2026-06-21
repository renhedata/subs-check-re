import { ArrowLeft, MoreHorizontal } from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { ExportPopover } from "@/components/workbench/export-popover";
import { NodeSourceMenu } from "@/components/workbench/node-source-menu";
import { RunCheckButton } from "@/components/workbench/run-check-button";
import type { checker, subscription } from "@/lib/client.gen";

type Subscription = subscription.Subscription;
type JobSummary = checker.JobSummary;

function jobLabel(j: JobSummary): string {
	const when = new Date(j.created_at).toLocaleString(undefined, {
		month: "short",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
	if (j.status === "running" || j.status === "queued") {
		return `${when} · running`;
	}
	if (j.status === "failed") return `${when} · failed`;
	return `${when} · ${j.available}/${j.total}`;
}

export function DetailHeader({
	sub,
	jobs,
	selectedJobId,
	activeJobId,
	onSelectJob,
	onRunStarted,
	onEdit,
	onToggleEnabled,
	onDelete,
	onBack,
}: {
	sub: Subscription;
	jobs: JobSummary[];
	selectedJobId: string | null; // null = latest completed
	activeJobId: string | null; // currently running job (SSE attached)
	onSelectJob: (jobId: string | null) => void;
	onRunStarted: (jobId: string) => void;
	onEdit: () => void;
	onToggleEnabled: () => void;
	onDelete: () => void;
	onBack: () => void;
}) {
	return (
		<div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-border border-b px-4 py-3 md:px-5">
			<button
				type="button"
				onClick={onBack}
				aria-label="Back to list"
				className="rounded-md p-1 text-muted-foreground hover:bg-secondary md:hidden"
			>
				<ArrowLeft size={16} />
			</button>

			<div className="min-w-0 flex-1 basis-48">
				<h1 className="truncate font-semibold text-[15px] text-foreground">
					{sub.name || sub.url}
				</h1>
				<div className="flex items-center gap-1">
					<p className="truncate font-mono text-[11px] text-muted-foreground">
						{sub.url}
					</p>
					<CopyButton text={sub.url} />
				</div>
			</div>

			<div className="flex items-center gap-2">
				{jobs.length > 0 ? (
					<Select
						value={selectedJobId ?? "latest"}
						onValueChange={(v) =>
							onSelectJob(v === "latest" ? null : (v ?? null))
						}
					>
						<SelectTrigger
							size="sm"
							aria-label="Check history"
							className="max-w-52 text-xs"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent
							align="start"
							alignItemWithTrigger={false}
							className="w-auto min-w-56"
						>
							<SelectItem value="latest">Latest result</SelectItem>
							{jobs.map((j) => (
								<SelectItem key={j.id} value={j.id}>
									{jobLabel(j)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				) : null}

				<NodeSourceMenu subscriptionId={sub.id} hasUrl={!!sub.url} />

				<ExportPopover subscriptionId={sub.id} />

				<DropdownMenu>
					<DropdownMenuTrigger
						render={
							<Button
								variant="outline"
								size="icon-sm"
								aria-label="Subscription actions"
							/>
						}
					>
						<MoreHorizontal size={14} />
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						<DropdownMenuItem onClick={onEdit}>Edit</DropdownMenuItem>
						<DropdownMenuItem onClick={onToggleEnabled}>
							{sub.enabled ? "Disable" : "Enable"}
						</DropdownMenuItem>
						<DropdownMenuItem
							onClick={onDelete}
							className="text-danger focus:text-danger"
						>
							Delete
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>

				<RunCheckButton
					subscriptionId={sub.id}
					disabled={!sub.enabled || !!activeJobId}
					onStarted={onRunStarted}
				/>
			</div>
		</div>
	);
}
