import { ChevronDown, ChevronRight, WifiOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import type { InflightNode, SSEConnection, SSEProgress } from "@/queries";

const PHASE_LABELS: Record<string, string> = {
	latency: "Latency",
	speed: "Speed test",
	upload: "Upload",
	region: "Region",
	streaming: "Streaming",
};

function formatElapsed(startedAt: number): string {
	const s = Math.floor((Date.now() - startedAt) / 1000);
	return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
}

export function ProgressPanel({
	progress,
	logEntries,
	connection,
	inflight,
	cancelPending,
	onCancel,
}: {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	connection: SSEConnection;
	inflight: InflightNode[];
	cancelPending: boolean;
	onCancel: () => void;
}) {
	const [logOpen, setLogOpen] = useState(true);
	const startedAtRef = useRef(Date.now());
	const logRef = useRef<HTMLDivElement | null>(null);
	const [, forceTick] = useState(0);

	// Re-render every second for the elapsed clock.
	useEffect(() => {
		const t = setInterval(() => forceTick((n) => n + 1), 1000);
		return () => clearInterval(t);
	}, []);

	// Auto-scroll the log container only (not the page).
	// biome-ignore lint/correctness/useExhaustiveDependencies: scroll on new entries
	useEffect(() => {
		const el = logRef.current;
		if (el) el.scrollTop = el.scrollHeight;
	}, [logEntries.length]);

	const done = progress?.total ?? 0;
	const current = progress?.progress ?? 0;
	const pct = done > 0 ? (current / done) * 100 : 0;
	const aliveSoFar = logEntries.filter((e) => e.alive).length;
	const reconnecting = connection === "reconnecting";

	// ETA from the average pace so far (spec: "elapsed + ETA").
	const elapsedSec = (Date.now() - startedAtRef.current) / 1000;
	const eta =
		current > 0 && done > current
			? Math.round(((done - current) * elapsedSec) / current)
			: null;
	const etaLabel =
		eta !== null
			? ` · ~${String(Math.floor(eta / 60)).padStart(2, "0")}:${String(eta % 60).padStart(2, "0")} left`
			: "";

	return (
		<div className="space-y-2.5 rounded-lg border border-info-line bg-info-muted/30 p-4">
			<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
				{reconnecting ? (
					<WifiOff size={14} className="text-warning" />
				) : (
					<Spinner className="size-3.5 text-info" />
				)}
				<span className="font-medium text-foreground text-sm">
					{reconnecting ? "Reconnecting…" : "Checking nodes…"}
				</span>
				<span className="text-muted-foreground text-xs tabular-nums">
					{current} / {done || "?"}
				</span>
				<span className="ml-auto text-xs tabular-nums">
					<b className="text-success">{aliveSoFar}</b>{" "}
					<span className="text-muted-foreground">alive so far</span>
				</span>
				<Button
					variant="outline"
					size="sm"
					loading={cancelPending}
					onClick={onCancel}
					className="text-danger"
				>
					Cancel
				</Button>
			</div>

			<Progress value={pct} />

			<div className="flex items-center justify-between text-[11px] text-muted-foreground">
				<button
					type="button"
					onClick={() => setLogOpen((v) => !v)}
					className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
				>
					{logOpen ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
					Live log ({inflight.length + logEntries.length})
				</button>
				<span className="tabular-nums">
					elapsed {formatElapsed(startedAtRef.current)}
					{etaLabel}
				</span>
			</div>

			{logOpen ? (
				<div
					ref={logRef}
					className="max-h-52 overflow-y-auto rounded-md bg-background/60 p-2"
				>
					{inflight.map((n: InflightNode) => (
						<div
							key={`live-${n.node_id}`}
							className="flex items-baseline gap-2 py-0.5 font-mono text-[11px] tabular-nums"
						>
							<Spinner className="size-3 text-info" />
							<span className="min-w-0 flex-1 truncate text-foreground">
								{n.node_name}
							</span>
							<span className="text-muted-foreground">
								{PHASE_LABELS[n.phase] ?? n.phase}
							</span>
						</div>
					))}
					{logEntries.map((e, i) => (
						<div
							key={`${i}-${e.node_name ?? ""}`}
							className="flex items-baseline gap-2 py-0.5 font-mono text-[11px] tabular-nums"
						>
							<span className={cn(e.alive ? "text-success" : "text-danger")}>
								{e.alive ? "✓" : "✗"}
							</span>
							<span className="min-w-0 flex-1 truncate text-foreground">
								{e.node_name}
							</span>
							{e.alive && e.latency_ms ? <span>{e.latency_ms}ms</span> : null}
							{e.alive && e.speed_kbps ? (
								<span className="text-muted-foreground">
									{e.speed_kbps >= 1024
										? `${(e.speed_kbps / 1024).toFixed(1)}MB/s`
										: `${e.speed_kbps}KB/s`}
								</span>
							) : null}
						</div>
					))}
				</div>
			) : null}
		</div>
	);
}
