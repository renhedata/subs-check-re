import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";

type NodeResult = checker.NodeResult;

interface Props {
	results: NodeResult[];
}

function latencyColor(ms: number): string {
	if (ms < 50) return "var(--color-success)";
	if (ms <= 200) return "var(--color-warning)";
	return "var(--destructive)";
}

function UnlockBadge({
	label,
	style,
}: {
	label: string;
	style: "media" | "ai" | "other";
}) {
	const styles = {
		media: {
			background: "var(--color-badge-danger-bg)",
			color: "var(--color-badge-danger)",
		},
		ai: {
			background: "var(--color-badge-ai-bg)",
			color: "var(--color-badge-ai)",
		},
		other: {
			background: "var(--color-badge-info-bg)",
			color: "var(--color-badge-info)",
		},
	};
	return (
		<span
			className="rounded px-1.5 py-0.5 font-semibold text-[10px]"
			style={styles[style]}
		>
			{label}
		</span>
	);
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

export function NodeTable({ results }: Props) {
	const alive = results.filter((r) => r.alive);
	const dead = results.filter((r) => !r.alive);
	const sorted = [...alive, ...dead];

	if (sorted.length === 0) {
		return (
			<p className="text-sm text-muted-foreground">No results yet.</p>
		);
	}

	return (
		<div className="overflow-x-auto rounded-lg border border-border">
			<table className="w-full border-collapse">
				<thead>
					<tr style={{ borderBottom: "1px solid var(--border)" }}>
						{[
							"Node",
							"Status",
							"Latency",
							"Speed",
							"Traffic",
							"Country",
							"Unlocks",
						].map((h) => (
							<th
								key={h}
								className="px-3 py-2 text-left font-medium text-[11px] uppercase tracking-[0.4px] text-muted-foreground"
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
							<td
								className="max-w-[180px] truncate px-3 py-2 font-mono text-[11px]"
								style={{
									color: r.alive ? "var(--foreground)" : "var(--color-dimmed)",
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
							<td className="px-3 py-2 text-xs text-muted-foreground">
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
								<div className="flex flex-wrap gap-1">
									{r.netflix && <UnlockBadge label="NF" style="media" />}
									{r.youtube && !r.youtube_premium && (
										<UnlockBadge label="YT" style="media" />
									)}
									{r.youtube_premium && (
										<UnlockBadge label="YT+" style="media" />
									)}
									{r.openai && <UnlockBadge label="GPT" style="ai" />}
									{r.claude && <UnlockBadge label="CL" style="ai" />}
									{r.gemini && <UnlockBadge label="GM" style="ai" />}
									{r.grok && <UnlockBadge label="GK" style="ai" />}
									{r.disney && <UnlockBadge label="D+" style="other" />}
									{r.tiktok && <UnlockBadge label="TK" style="other" />}
								</div>
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}
