import { CopyButton } from "@/components/copy-button";
import { RuleIcon } from "@/components/rule-icon";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog";
import type { checker } from "@/lib/client.gen";
import { countryToFlag } from "@/lib/countryToFlag";
import { formatBytes, formatSpeed } from "@/lib/format";
import { BUILTIN_PLATFORMS, latencyTone } from "@/lib/nodeFilters";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

const toneText: Record<string, string> = {
	success: "text-success",
	warning: "text-warning",
	danger: "text-danger",
};

function Row({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div className="flex items-baseline justify-between gap-3 py-1">
			<span className="shrink-0 text-muted-foreground text-xs">{label}</span>
			<span className="min-w-0 truncate text-right text-foreground text-sm tabular-nums">
				{children}
			</span>
		</div>
	);
}

function platformRows(
	r: NodeResult,
	rules: PlatformRule[],
): Array<{
	key: string;
	label: string;
	icon: string;
	unlocked: boolean;
	status: string;
	region: string;
}> {
	const ruleByKey = Object.fromEntries(rules.map((x) => [x.key, x]));
	const platforms = r.platforms ?? {};
	const seen = new Set<string>();
	const rows = BUILTIN_PLATFORMS.map((key) => {
		seen.add(key);
		const o = platforms[key];
		return {
			key,
			label: ruleByKey[key]?.name ?? key,
			icon: ruleByKey[key]?.icon ?? "",
			unlocked: o?.unlocked === true,
			status: o?.status ?? "",
			region: o?.region ?? "",
		};
	});
	const extra = Object.entries(platforms)
		.filter(([key]) => !seen.has(key))
		.map(([key, o]) => ({
			key,
			label: ruleByKey[key]?.name ?? key,
			icon: ruleByKey[key]?.icon ?? "",
			unlocked: o?.unlocked === true,
			status: o?.status ?? "",
			region: o?.region ?? "",
		}));
	return [...rows, ...extra];
}

function prettyConfig(raw: string): string {
	try {
		return JSON.stringify(JSON.parse(raw), null, 2);
	} catch {
		return raw;
	}
}

export function NodeDetailDialog({
	result,
	rules = [],
	open,
	onOpenChange,
}: {
	result: NodeResult | null;
	rules?: PlatformRule[];
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-3xl">
				{result ? (
					<>
						<DialogTitle className="truncate pr-6 font-mono">
							{result.node_name}
						</DialogTitle>
						<DialogDescription>
							{result.alive ? "Alive" : "Dead"} · {result.node_type || "—"}
						</DialogDescription>

						<div className="mt-4 space-y-4">
							<div className="grid gap-x-8 gap-y-4 sm:grid-cols-2">
								<section>
									<p className="mb-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
										Identity
									</p>
									<Row label="Protocol">{result.node_type || "—"}</Row>
									<Row label="Server">
										<span className="font-mono">
											{result.server ? `${result.server}:${result.port}` : "—"}
										</span>
									</Row>
									<Row label="Exit IP">
										<span className="font-mono">{result.ip || "—"}</span>
									</Row>
									<Row label="Country">{result.country || "—"}</Row>
								</section>

								<section>
									<p className="mb-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
										Performance
									</p>
									<Row label="Latency">
										{result.alive ? (
											<span
												className={toneText[latencyTone(result.latency_ms)]}
											>
												{result.latency_ms}ms
											</span>
										) : (
											"—"
										)}
									</Row>
									<Row label="Download">
										{result.alive && result.speed_kbps
											? formatSpeed(result.speed_kbps)
											: "—"}
									</Row>
									<Row label="Upload">
										{result.alive && result.upload_speed_kbps
											? formatSpeed(result.upload_speed_kbps)
											: "—"}
									</Row>
									<Row label="Traffic">{formatBytes(result.traffic_bytes)}</Row>
								</section>
							</div>

							<section>
								<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									Platforms
								</p>
								<div className="grid grid-cols-1 gap-x-8 gap-y-1 sm:grid-cols-2">
									{platformRows(result, rules).map((p) => (
										<div
											key={p.key}
											className="flex items-center justify-between gap-2 text-xs"
										>
											<span className="flex min-w-0 items-center gap-1.5 text-foreground">
												<RuleIcon icon={p.icon} label={p.label} size={14} />
												<span className="truncate">{p.label}</span>
											</span>
											<span className="flex shrink-0 items-center gap-1.5">
												{p.region ? (
													<span className="text-muted-foreground">
														{countryToFlag(p.region)} {p.region}
													</span>
												) : null}
												<span
													className={cn(
														"inline-flex items-center rounded-full border px-2 py-0.5",
														p.unlocked
															? "border-success-line bg-success-muted text-success"
															: "border-border text-muted-foreground",
													)}
												>
													{p.status || (p.unlocked ? "Yes" : "No")}
												</span>
											</span>
										</div>
									))}
								</div>
							</section>

							<section>
								<details>
									<summary className="cursor-pointer font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
										Raw config
									</summary>
									<div className="mt-2 flex items-start gap-2">
										<pre className="min-w-0 flex-1 overflow-x-auto rounded-md bg-secondary p-2 font-mono text-[11px] text-foreground">
											{prettyConfig(result.config || "")}
										</pre>
										<CopyButton text={result.config || ""} />
									</div>
								</details>
							</section>
						</div>
					</>
				) : null}
			</DialogContent>
		</Dialog>
	);
}
