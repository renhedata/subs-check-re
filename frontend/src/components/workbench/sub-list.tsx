import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { type DotTone, StatusDot } from "@/components/ui/status-dot";
import { UnlockStrip } from "@/components/workbench/unlock-strip";
import type { checker, subscription } from "@/lib/client.gen";
import { cn } from "@/lib/utils";

type Subscription = subscription.Subscription;
type LatestJobSummary = checker.LatestJobSummary;

function relativeTime(iso: string): string {
	const ms = Date.now() - new Date(iso).getTime();
	const minutes = Math.floor(ms / 60_000);
	if (minutes < 1) return "just now";
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	return `${Math.floor(hours / 24)}d ago`;
}

function dotTone(sub: Subscription, latest?: LatestJobSummary): DotTone {
	if (!sub.enabled) return "neutral";
	if (!latest) return "neutral";
	if (latest.status === "running" || latest.status === "queued") return "info";
	if (latest.status === "failed") return "danger";
	return "success";
}

function SubListItem({
	sub,
	latest,
	selected,
	liveProgressPct,
	onSelect,
}: {
	sub: Subscription;
	latest?: LatestJobSummary;
	selected: boolean;
	// Set only for the selected subscription while its check runs (mirrors
	// the detail pane's SSE data — non-selected rows don't open streams).
	liveProgressPct: number | null;
	onSelect: () => void;
}) {
	const running = latest?.status === "running" || latest?.status === "queued";
	return (
		<button
			type="button"
			onClick={onSelect}
			className={cn(
				"w-full rounded-lg border border-transparent px-3 py-2.5 text-left transition-colors",
				selected ? "" : "hover:bg-secondary/50",
				!sub.enabled && "opacity-45",
			)}
			style={
				selected
					? {
							background: "var(--color-active-bg)",
							borderColor: "var(--color-active-border)",
						}
					: undefined
			}
		>
			<div className="flex items-center gap-2">
				<StatusDot tone={dotTone(sub, latest)} pulse={running} />
				<span className="min-w-0 flex-1 truncate font-medium text-foreground text-sm">
					{sub.name || sub.url}
				</span>
				<span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
					{running
						? liveProgressPct !== null
							? `${Math.round(liveProgressPct)}%`
							: "Running…"
						: latest?.finished_at
							? relativeTime(latest.finished_at)
							: ""}
				</span>
			</div>
			{running && liveProgressPct !== null ? (
				<div className="mt-1.5 pl-4">
					<Progress value={liveProgressPct} className="h-[3px]" />
				</div>
			) : latest && !running ? (
				<p className="mt-0.5 truncate pl-4 text-muted-foreground text-xs tabular-nums">
					{latest.status === "failed"
						? "last check failed"
						: `${latest.available}/${latest.total} alive`}
					{latest.status === "completed" && latest.avg_latency_ms > 0
						? ` · ⚡${latest.avg_latency_ms}ms avg`
						: ""}
				</p>
			) : null}
		</button>
	);
}

export function SubList({
	subs,
	latestJobs,
	loading,
	selectedId,
	liveProgressPct,
	onSelect,
	onAdd,
}: {
	subs: Subscription[];
	latestJobs: Record<string, LatestJobSummary>;
	loading: boolean;
	selectedId: string | null;
	liveProgressPct: number | null;
	onSelect: (id: string) => void;
	onAdd: () => void;
}) {
	return (
		<div className="flex h-full min-h-0 flex-col">
			<div className="flex shrink-0 items-center justify-between border-border border-b px-4 py-3">
				<h2 className="font-semibold text-foreground text-sm">Subscriptions</h2>
				<Button variant="outline" size="sm" onClick={onAdd}>
					<Plus size={13} /> Add
				</Button>
			</div>

			<div className="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
				{loading
					? (["sk-0", "sk-1", "sk-2"] as const).map((k) => (
							<div key={k} className="space-y-2 px-3 py-2.5">
								<Skeleton className="h-4 w-36" />
								<Skeleton className="h-3 w-24" />
							</div>
						))
					: subs.map((sub) => (
							<SubListItem
								key={sub.id}
								sub={sub}
								latest={latestJobs[sub.id]}
								selected={sub.id === selectedId}
								liveProgressPct={sub.id === selectedId ? liveProgressPct : null}
								onSelect={() => onSelect(sub.id)}
							/>
						))}
			</div>

			<UnlockStrip />
		</div>
	);
}
