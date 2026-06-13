import { ChevronDown, SearchX } from "lucide-react";
import type { ReactNode } from "react";
import { useMemo, useState } from "react";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { NodeTable } from "@/components/workbench/node-table";
import type { checker } from "@/lib/client.gen";
import {
	BUILTIN_PLATFORMS,
	filterNodes,
	nodeHasPlatform,
	type SortDir,
	type SortKey,
	sortNodes,
} from "@/lib/nodeFilters";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

function Chip({ children }: { children: ReactNode }) {
	return (
		<span className="inline-flex items-center gap-1 rounded-full border border-border bg-card px-3 py-1 text-xs tabular-nums">
			{children}
		</span>
	);
}

export function ResultsSection({
	results,
	rules = [],
	onToggleEnabled,
}: {
	results: NodeResult[];
	rules?: PlatformRule[];
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}) {
	const [text, setText] = useState("");
	const [aliveOnly, setAliveOnly] = useState(false);
	const [platforms, setPlatforms] = useState<string[]>([]);
	const [sortKey, setSortKey] = useState<SortKey>("latency");
	const [sortDir, setSortDir] = useState<SortDir>("asc");

	const handleSort = (key: SortKey) => {
		if (key === sortKey) {
			setSortDir((d) => (d === "asc" ? "desc" : "asc"));
		} else {
			setSortKey(key);
			setSortDir(key === "speed" ? "desc" : "asc");
		}
	};

	const alive = results.filter((r) => r.alive);
	const avgLatency =
		alive.length > 0
			? Math.round(
					alive.reduce((sum, r) => sum + r.latency_ms, 0) / alive.length,
				)
			: 0;
	const peakSpeed = alive.reduce((max, r) => Math.max(max, r.speed_kbps), 0);

	// Unlock counts for chips: top platforms with at least one unlocked node.
	const unlockCounts = useMemo(() => {
		const keys: string[] = [...BUILTIN_PLATFORMS, ...rules.map((r) => r.key)];
		return keys
			.map((key) => ({
				key,
				count: alive.filter((r) => nodeHasPlatform(r, key)).length,
			}))
			.filter((e) => e.count > 0)
			.sort((a, b) => b.count - a.count)
			.slice(0, 4);
	}, [alive, rules]);

	const visible = useMemo(
		() =>
			sortNodes(
				filterNodes(results, { text, aliveOnly, platforms }),
				sortKey,
				sortDir,
			),
		[results, text, aliveOnly, platforms, sortKey, sortDir],
	);

	const togglePlatform = (key: string) =>
		setPlatforms((prev) =>
			prev.includes(key) ? prev.filter((p) => p !== key) : [...prev, key],
		);

	return (
		<div className="space-y-3">
			{/* Summary chips */}
			<div className="flex flex-wrap gap-2">
				<Chip>
					<b className="text-success">{alive.length}</b> alive
				</Chip>
				<Chip>
					<b className="text-danger">{results.length - alive.length}</b> dead
				</Chip>
				{avgLatency > 0 ? (
					<Chip>
						avg <b>{avgLatency}ms</b>
					</Chip>
				) : null}
				{peakSpeed > 0 ? (
					<Chip>
						⬇{" "}
						<b>
							{peakSpeed >= 1024
								? `${(peakSpeed / 1024).toFixed(1)} MB/s`
								: `${peakSpeed} KB/s`}
						</b>{" "}
						peak
					</Chip>
				) : null}
				{unlockCounts.map((e) => (
					<Chip key={e.key}>
						{e.key} <b className="text-success">{e.count}</b>
					</Chip>
				))}
			</div>

			{/* Filter bar */}
			<div className="flex flex-wrap items-center gap-2">
				<Input
					value={text}
					onChange={(e) => setText(e.target.value)}
					placeholder="Filter nodes…"
					className="h-7 w-44 text-xs"
				/>
				<button
					type="button"
					aria-pressed={aliveOnly}
					onClick={() => setAliveOnly((v) => !v)}
					className={cn(
						"rounded-full border px-3 py-1 font-medium text-xs transition-colors",
						aliveOnly
							? "border-info-line bg-info-muted text-info"
							: "border-border text-muted-foreground hover:bg-secondary",
					)}
				>
					Alive only
				</button>
				<Popover>
					<PopoverTrigger
						className={cn(
							"inline-flex items-center gap-1 rounded-full border px-3 py-1 font-medium text-xs outline-none transition-colors",
							platforms.length > 0
								? "border-info-line bg-info-muted text-info"
								: "border-border text-muted-foreground hover:bg-secondary",
						)}
					>
						Unlocks{platforms.length > 0 ? ` (${platforms.length})` : ""}
						<ChevronDown size={12} />
					</PopoverTrigger>
					<PopoverContent className="w-72">
						<p className="mb-2 font-medium text-foreground text-xs">
							Show nodes unlocking
						</p>
						<div className="flex flex-wrap gap-1.5">
							{[...BUILTIN_PLATFORMS, ...rules.map((r) => r.key)].map((key) => (
								<button
									key={key}
									type="button"
									aria-pressed={platforms.includes(key)}
									onClick={() => togglePlatform(key)}
									className={cn(
										"rounded-full border px-2.5 py-1 text-xs transition-colors",
										platforms.includes(key)
											? "border-info-line bg-info-muted text-info"
											: "border-border text-muted-foreground hover:bg-secondary",
									)}
								>
									{key}
								</button>
							))}
						</div>
					</PopoverContent>
				</Popover>
				<span className="ml-auto text-[11px] text-muted-foreground tabular-nums">
					{visible.length} of {results.length} shown
				</span>
			</div>

			{visible.length === 0 ? (
				<EmptyState
					icon={SearchX}
					title="No nodes match"
					description="Loosen the filters above."
				/>
			) : (
				<NodeTable
					results={visible}
					rules={rules}
					sortKey={sortKey}
					sortDir={sortDir}
					onSort={handleSort}
					onToggleEnabled={onToggleEnabled}
				/>
			)}
		</div>
	);
}
