import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";

import { PlatformIcon, PlatformIconAny } from "./platform-icons";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

interface Props {
	results: NodeResult[];
	rules?: PlatformRule[];
	onToggleEnabled?: (nodeId: string, enabled: boolean) => void;
}

function latencyColor(ms: number): string {
	if (ms < 50) return "var(--color-success)";
	if (ms <= 200) return "var(--color-warning)";
	return "var(--destructive)";
}

function StatusBadge({ alive }: { alive: boolean }) {
	return alive ? (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium text-[10px]"
			style={{
				background: "var(--color-badge-success-bg)",
				color: "var(--color-badge-success)",
			}}
		>
			<span
				className="h-1.5 w-1.5 rounded-full"
				style={{ background: "var(--color-success)" }}
			/>
			alive
		</span>
	) : (
		<span
			className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium text-[10px]"
			style={{
				background: "var(--color-badge-danger-bg)",
				color: "var(--color-badge-danger)",
			}}
		>
			<span
				className="h-1.5 w-1.5 rounded-full"
				style={{ background: "var(--destructive)" }}
			/>
			dead
		</span>
	);
}

export function NodeTable({ results, rules = [], onToggleEnabled }: Props) {
	const ruleByKey = Object.fromEntries(rules.map((r) => [r.key, r]));
	const alive = results.filter((r) => r.alive);
	const dead = results.filter((r) => !r.alive);
	const sorted = [...alive, ...dead];

	if (sorted.length === 0) {
		return <p className="text-muted-foreground text-sm">No results yet.</p>;
	}

	return (
		<div className="overflow-x-auto rounded-lg border border-border">
			<table className="w-full border-collapse">
				<thead>
					<tr style={{ borderBottom: "1px solid var(--border)" }}>
						{[
							"",
							"Node",
							"Status",
							"Latency",
							"↓ Speed",
							"↑ Upload",
							"Traffic",
							"Country",
							"Unlocks",
						].map((h) => (
							<th
								key={h}
								className="px-3 py-2 text-left font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]"
							>
								{h}
							</th>
						))}
					</tr>
				</thead>
				<tbody>
					{sorted.map((r) => (
						<tr
							key={r.node_id}
							className="transition-colors hover:bg-white/[0.02]"
							style={{ borderBottom: "1px solid var(--secondary)" }}
						>
							<td className="px-2 py-2">
								{onToggleEnabled && (
									<button
										type="button"
										onClick={() => onToggleEnabled(r.node_id, !r.enabled)}
										title={r.enabled ? "Disable node" : "Enable node"}
										className="rounded px-1.5 py-0.5 text-[10px] transition-colors"
										style={{
											background: r.enabled
												? "var(--color-badge-success-bg)"
												: "var(--color-badge-danger-bg)",
											color: r.enabled
												? "var(--color-badge-success)"
												: "var(--color-badge-danger)",
										}}
									>
										{r.enabled ? "on" : "off"}
									</button>
								)}
							</td>
							<td
								className="max-w-[180px] truncate px-3 py-2 font-mono text-[11px]"
								style={{
									color: r.enabled
										? r.alive
											? "var(--foreground)"
											: "var(--color-dimmed)"
										: "var(--color-dimmed)",
									opacity: r.enabled ? 1 : 0.5,
								}}
							>
								{r.node_name}
							</td>
							<td className="px-3 py-2">
								<StatusBadge alive={r.alive} />
							</td>
							<td className="px-3 py-2 font-medium text-xs">
								{r.alive ? (
									<span style={{ color: latencyColor(r.latency_ms) }}>
										{r.latency_ms}ms
									</span>
								) : (
									<span style={{ color: "var(--color-dimmed)" }}>—</span>
								)}
							</td>
							<td className="px-3 py-2 text-xs">
								{r.alive && r.speed_kbps ? (
									<span style={{ color: "var(--primary)" }}>
										{r.speed_kbps >= 1024
											? `${(r.speed_kbps / 1024).toFixed(1)} MB/s`
											: `${r.speed_kbps} KB/s`}
									</span>
								) : (
									<span style={{ color: "var(--color-dimmed)" }}>—</span>
								)}
							</td>
							<td className="px-3 py-2 text-xs">
								{r.alive && r.upload_speed_kbps ? (
									<span style={{ color: "var(--color-warning)" }}>
										{r.upload_speed_kbps >= 1024
											? `${(r.upload_speed_kbps / 1024).toFixed(1)} MB/s`
											: `${r.upload_speed_kbps} KB/s`}
									</span>
								) : (
									<span style={{ color: "var(--color-dimmed)" }}>—</span>
								)}
							</td>
							<td className="px-3 py-2 text-muted-foreground text-xs">
								{formatBytes(r.traffic_bytes)}
							</td>
							<td
								className="px-3 py-2 text-xs"
								style={{
									color: r.alive ? "var(--foreground)" : "var(--color-dimmed)",
								}}
							>
								{r.country || "—"}
							</td>
							<td className="px-3 py-2">
								<div className="flex flex-wrap gap-1.5">
									{r.netflix && <PlatformIcon platform="netflix" />}
									{r.youtube && !r.youtube_premium && (
										<PlatformIcon platform="youtube" />
									)}
									{r.youtube_premium && (
										<PlatformIcon platform="youtube_premium" />
									)}
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
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}
