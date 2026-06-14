import { CheckCircle, Globe, RefreshCw } from "lucide-react";
import { RulePlatformIcon } from "@/components/rule-icon";
import { Button } from "@/components/ui/button";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { useLocalUnlock } from "@/queries";

// Compact footer strip showing what the server's own IP can reach — the old
// Dashboard panel, demoted to a popover. Matters when reading node results:
// a platform blocked for the server itself shows as blocked on every node.
export function UnlockStrip() {
	const { data, isLoading, isFetching, refetch } = useLocalUnlock();

	const keys = data ? Object.keys(data.platforms ?? {}) : [];
	const unlockCount = keys.filter(
		(k) => data?.platforms?.[k]?.unlocked === true,
	).length;

	return (
		<Popover>
			<PopoverTrigger className="flex w-full items-center gap-2 border-border border-t px-4 py-2.5 text-left text-muted-foreground text-xs outline-none transition-colors hover:bg-secondary/50">
				<Globe size={13} strokeWidth={1.75} className="shrink-0" />
				{isLoading ? (
					<span>Checking server network…</span>
				) : data ? (
					<>
						<span className="truncate font-mono tabular-nums">
							{data.country ? `${data.country} ` : ""}
							{data.ip || "server"}
						</span>
						<span className="ml-auto shrink-0 font-medium text-success">
							{unlockCount} unlocks ›
						</span>
					</>
				) : (
					<span>Server network unlock ›</span>
				)}
			</PopoverTrigger>
			<PopoverContent align="start" className="w-80">
				<div className="mb-2 flex items-center justify-between">
					<p className="font-medium text-foreground text-xs">
						Server network unlock
					</p>
					<Button
						variant="ghost"
						size="xs"
						onClick={() => refetch()}
						disabled={isFetching}
					>
						<RefreshCw size={12} className={isFetching ? "animate-spin" : ""} />
						Refresh
					</Button>
				</div>
				<p className="mb-3 text-muted-foreground text-xs">
					Platforms reachable from this server's own IP.
				</p>
				<div className="flex flex-wrap gap-2">
					{keys.length === 0 ? (
						<span className="text-muted-foreground text-xs">
							No enabled rules.
						</span>
					) : (
						keys.map((k) => {
							const available = data?.platforms?.[k]?.unlocked === true;
							return (
								<span
									key={k}
									className={cn(
										"inline-flex items-center gap-1.5 rounded-md bg-secondary px-2 py-1",
										available ? "" : "opacity-35",
									)}
								>
									<RulePlatformIcon platformKey={k} size={14} showLabel />
									{available ? (
										<CheckCircle size={10} className="text-success" />
									) : null}
								</span>
							);
						})
					)}
				</div>
			</PopoverContent>
		</Popover>
	);
}
