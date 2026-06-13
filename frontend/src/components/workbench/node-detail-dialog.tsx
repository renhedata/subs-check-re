import { CopyButton } from "@/components/copy-button";
import { PLATFORM_META, type PlatformKey } from "@/components/platform-icons";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog";
import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";
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

function formatSpeed(kbps: number): string {
	return kbps >= 1024 ? `${(kbps / 1024).toFixed(1)} MB/s` : `${kbps} KB/s`;
}

function platformRows(
	r: NodeResult,
	rules: PlatformRule[],
): Array<{ key: string; label: string; unlocked: boolean }> {
	const ruleByKey = Object.fromEntries(rules.map((x) => [x.key, x]));
	const builtin = BUILTIN_PLATFORMS.map((key) => ({
		key,
		label: PLATFORM_META[key as PlatformKey]?.label ?? key,
		unlocked: (r as unknown as Record<string, boolean>)[key] === true,
	}));
	const extra = Object.entries(r.extra_platforms ?? {}).map(([key, v]) => ({
		key,
		label: ruleByKey[key]?.name ?? key,
		unlocked: v === true,
	}));
	return [...builtin, ...extra];
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
			<DialogContent className="sm:max-w-lg">
				{result ? (
					<>
						<DialogTitle className="truncate pr-6 font-mono">
							{result.node_name}
						</DialogTitle>
						<DialogDescription>
							{result.alive ? "Alive" : "Dead"} · {result.node_type || "—"}
						</DialogDescription>

						<div className="mt-4 max-h-[60vh] space-y-4 overflow-y-auto pr-1">
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
										<span className={toneText[latencyTone(result.latency_ms)]}>
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

							<section>
								<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									Platforms
								</p>
								<div className="flex flex-wrap gap-1.5">
									{platformRows(result, rules).map((p) => (
										<span
											key={p.key}
											className={cn(
												"inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs",
												p.unlocked
													? "border-success-line bg-success-muted text-success"
													: "border-border text-muted-foreground",
											)}
										>
											{p.unlocked ? "✓" : "✗"} {p.label}
										</span>
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
