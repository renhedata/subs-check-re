import { ArrowDown, ArrowUp } from "lucide-react";
import { useState } from "react";
import { PlatformIcon, PlatformIconAny } from "@/components/platform-icons";
import { Badge } from "@/components/ui/badge";
import { Tooltip } from "@/components/ui/tooltip";
import { NodeDetailDialog } from "@/components/workbench/node-detail-dialog";
import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";
import { latencyTone, type SortDir, type SortKey } from "@/lib/nodeFilters";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

function formatSpeed(kbps: number): string {
	return kbps >= 1024 ? `${(kbps / 1024).toFixed(1)} MB/s` : `${kbps} KB/s`;
}

const toneText: Record<string, string> = {
	success: "text-success",
	warning: "text-warning",
	danger: "text-danger",
};

function Latency({ r }: { r: NodeResult }) {
	if (!r.alive) return <span className="text-muted-foreground/60">—</span>;
	return (
		<span className={cn("font-medium", toneText[latencyTone(r.latency_ms)])}>
			{r.latency_ms}ms
		</span>
	);
}

// PlatformIcon already has title={meta.label} on the wrapping <span>, so
// hover tooltips work natively. For PlatformIconAny (custom rules), the
// component also renders title={displayLabel} on its wrapper. No additional
// Tooltip wrapping is needed.
function UnlockIcons({
	r,
	ruleByKey,
}: {
	r: NodeResult;
	ruleByKey: Record<string, PlatformRule>;
}) {
	return (
		<div className="flex flex-wrap items-center gap-1.5">
			{r.netflix && <PlatformIcon platform="netflix" />}
			{r.youtube && !r.youtube_premium && <PlatformIcon platform="youtube" />}
			{r.youtube_premium && <PlatformIcon platform="youtube_premium" />}
			{r.openai && <PlatformIcon platform="openai" />}
			{r.claude && <PlatformIcon platform="claude" />}
			{r.gemini && <PlatformIcon platform="gemini" />}
			{r.grok && <PlatformIcon platform="grok" />}
			{r.disney && <PlatformIcon platform="disney" />}
			{r.tiktok && <PlatformIcon platform="tiktok" />}
			{r.extra_platforms &&
				Object.entries(r.extra_platforms)
					.filter(([, v]) => v)
					.map(([key]) => {
						const rule = ruleByKey[key];
						return (
							<PlatformIconAny
								key={key}
								platformKey={key}
								icon={rule?.icon}
								label={rule?.name ?? key}
							/>
						);
					})}
		</div>
	);
}

function SortHeader({
	label,
	myKey,
	sortKey,
	sortDir,
	onSort,
	className,
}: {
	label: string;
	myKey: SortKey;
	sortKey: SortKey;
	sortDir: SortDir;
	onSort: (key: SortKey) => void;
	className?: string;
}) {
	const active = sortKey === myKey;
	return (
		<th className={cn("px-3 py-2 text-left", className)}>
			<button
				type="button"
				onClick={() => onSort(myKey)}
				className={cn(
					"inline-flex items-center gap-1 font-medium text-[11px] uppercase tracking-[0.4px] transition-colors",
					active
						? "text-primary"
						: "text-muted-foreground hover:text-foreground",
				)}
			>
				{label}
				{active ? (
					sortDir === "asc" ? (
						<ArrowUp size={11} />
					) : (
						<ArrowDown size={11} />
					)
				) : null}
			</button>
		</th>
	);
}

function EnableToggle({
	r,
	onToggleEnabled,
}: {
	r: NodeResult;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}) {
	if (!onToggleEnabled) return null;
	return (
		<Tooltip
			content={r.enabled ? "Exclude from exports" : "Include in exports"}
		>
			<button
				type="button"
				aria-pressed={r.enabled}
				onClick={() => onToggleEnabled(r.node_id, !r.enabled)}
				className={cn(
					"rounded-full p-1 text-[13px] leading-none transition-colors hover:bg-secondary",
					r.enabled ? "text-success" : "text-muted-foreground/50",
				)}
			>
				{r.enabled ? "●" : "○"}
			</button>
		</Tooltip>
	);
}

export interface NodeTableProps {
	results: NodeResult[];
	rules?: PlatformRule[];
	sortKey: SortKey;
	sortDir: SortDir;
	onSort: (key: SortKey) => void;
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}

export function NodeTable({
	results,
	rules = [],
	sortKey,
	sortDir,
	onSort,
	onToggleEnabled,
}: NodeTableProps) {
	const ruleByKey = Object.fromEntries(rules.map((r) => [r.key, r]));
	const [detail, setDetail] = useState<NodeResult | null>(null);

	return (
		<>
			{/* Mobile: cards */}
			<div className="space-y-2 md:hidden">
				{results.map((r) => (
					<div
						key={r.node_id}
						className={cn(
							"rounded-lg border border-border bg-card p-3",
							!r.enabled && "opacity-50",
						)}
					>
						<div className="flex items-center gap-2">
							<button
								type="button"
								onClick={() => setDetail(r)}
								className="min-w-0 flex-1 truncate text-left font-mono text-foreground text-xs hover:text-primary"
							>
								{r.node_name}
							</button>
							<Badge tone={r.alive ? "success" : "danger"}>
								{r.alive ? "alive" : "dead"}
							</Badge>
							<EnableToggle r={r} onToggleEnabled={onToggleEnabled} />
						</div>
						<div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs tabular-nums">
							<span>
								<Latency r={r} />
							</span>
							{r.alive && r.speed_kbps ? (
								<span className="text-foreground">
									↓ {formatSpeed(r.speed_kbps)}
								</span>
							) : null}
							{r.alive && r.upload_speed_kbps ? (
								<span className="text-muted-foreground">
									↑ {formatSpeed(r.upload_speed_kbps)}
								</span>
							) : null}
							{r.traffic_bytes > 0 ? (
								<span className="text-muted-foreground">
									{formatBytes(r.traffic_bytes)}
								</span>
							) : null}
							{r.country ? (
								<span className="text-muted-foreground">{r.country}</span>
							) : null}
						</div>
						<div className="mt-2">
							<UnlockIcons r={r} ruleByKey={ruleByKey} />
						</div>
					</div>
				))}
			</div>

			{/* Desktop: table */}
			<div className="hidden overflow-hidden rounded-lg border border-border md:block">
				<table className="w-full border-collapse text-[12.5px]">
					<thead>
						<tr className="border-border border-b bg-card">
							<th className="w-8 px-2 py-2" aria-label="Enabled" />
							<SortHeader
								label="Node"
								myKey="name"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<SortHeader
								label="Latency"
								myKey="latency"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<SortHeader
								label="↓ Speed"
								myKey="speed"
								sortKey={sortKey}
								sortDir={sortDir}
								onSort={onSort}
							/>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] lg:table-cell">
								↑ Upload
							</th>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] lg:table-cell">
								Traffic
							</th>
							<th className="hidden px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px] xl:table-cell">
								Country
							</th>
							<th className="px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
								Unlocks
							</th>
						</tr>
					</thead>
					<tbody className="tabular-nums">
						{results.map((r) => (
							<tr
								key={r.node_id}
								className={cn(
									"border-secondary border-b transition-colors last:border-0 hover:bg-secondary/40",
									!r.enabled && "opacity-50",
								)}
							>
								<td className="px-2 py-1.5">
									<EnableToggle r={r} onToggleEnabled={onToggleEnabled} />
								</td>
								<td
									className={cn(
										"max-w-52 truncate px-3 py-1.5 font-mono text-[11px]",
										r.alive ? "text-foreground" : "text-muted-foreground/70",
									)}
								>
									<button
										type="button"
										onClick={() => setDetail(r)}
										className="truncate text-left hover:text-primary hover:underline"
									>
										{r.node_name}
									</button>
								</td>
								<td className="px-3 py-1.5">
									<Latency r={r} />
								</td>
								<td className="px-3 py-1.5">
									{r.alive && r.speed_kbps ? (
										formatSpeed(r.speed_kbps)
									) : (
										<span className="text-muted-foreground/60">—</span>
									)}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground lg:table-cell">
									{r.alive && r.upload_speed_kbps
										? formatSpeed(r.upload_speed_kbps)
										: "—"}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground lg:table-cell">
									{formatBytes(r.traffic_bytes)}
								</td>
								<td className="hidden px-3 py-1.5 text-muted-foreground xl:table-cell">
									{r.country || "—"}
								</td>
								<td className="px-3 py-1.5">
									<UnlockIcons r={r} ruleByKey={ruleByKey} />
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>

			<NodeDetailDialog
				result={detail}
				rules={rules}
				open={!!detail}
				onOpenChange={(o) => !o && setDetail(null)}
			/>
		</>
	);
}
