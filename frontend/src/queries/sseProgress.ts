import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import type { NodeDebug } from "@/components/debug-panel";
import type { checker } from "@/lib/client.gen";
import { queryKeys } from "./queryKeys";

// Per-node events mirror checker.NodeResult so the live table can render the
// same columns as GetResults; unmeasured dimensions arrive already inherited.
export interface SSEProgress {
	progress?: number;
	total?: number;
	node_id?: string;
	node_name?: string;
	node_type?: string;
	enabled?: boolean;
	alive?: boolean;
	latency_ms?: number;
	speed_kbps?: number;
	upload_speed_kbps?: number;
	country?: string;
	ip?: string;
	server?: string;
	port?: number;
	config?: string;
	platforms?: Record<string, checker.PlatformOutcome>;
	traffic_bytes?: number;
	phase?: string;
	done?: boolean;
	status?: string;
	debug?: NodeDebug;
}

export interface InflightNode {
	node_id: string;
	node_name: string;
	phase: string;
}

// A phase event announces the test a node is starting (in-flight); it carries a
// `phase` and never advances the completed counter.
export function isPhaseEvent(d: SSEProgress): boolean {
	return !!d.phase && !!d.node_id;
}

// A result event is a finished node (no phase, has a name) — the existing
// per-node payload that feeds the completed log and the live table.
export function isResultEvent(d: SSEProgress): boolean {
	return !d.phase && !!d.node_name;
}

export type SSEConnection = "idle" | "open" | "reconnecting" | "done";

interface UseSSEProgressOptions {
	jobId: string | null;
	subscriptionId: string;
	onDone?: () => void;
}

interface UseSSEProgressResult {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	debugData: NodeDebug[];
	connection: SSEConnection;
	inflight: InflightNode[];
}

const MAX_LOG_ENTRIES = 500;
const FLUSH_INTERVAL_MS = 800;

// useSSEProgress subscribes to /api/check/:jobId/progress.
// - Per-node events are buffered and flushed every FLUSH_INTERVAL_MS so a
//   200-node burst doesn't render 200 times (spec: throttled live inserts).
// - EventSource reconnects automatically; we surface that as `connection:
//   "reconnecting"` instead of silently closing like the old version did.
// - On done: closes, invalidates jobs + latest-jobs + results so every list
//   refreshes, then fires onDone.
export function useSSEProgress({
	jobId,
	subscriptionId,
	onDone,
}: UseSSEProgressOptions): UseSSEProgressResult {
	const [progress, setProgress] = useState<SSEProgress | null>(null);
	const [logEntries, setLogEntries] = useState<SSEProgress[]>([]);
	const [debugData, setDebugData] = useState<NodeDebug[]>([]);
	const [connection, setConnection] = useState<SSEConnection>("idle");
	const [inflight, setInflight] = useState<InflightNode[]>([]);
	const qc = useQueryClient();
	const onDoneRef = useRef(onDone);
	onDoneRef.current = onDone;

	useEffect(() => {
		setLogEntries([]);
		setProgress(null);
		setDebugData([]);
		setInflight([]);
		setConnection(jobId ? "reconnecting" : "idle");
	}, [jobId]);

	useEffect(() => {
		if (!jobId) return;

		const buffer: SSEProgress[] = [];
		const debugBuffer: NodeDebug[] = [];
		const inflightMap = new Map<string, InflightNode>();
		let inflightDirty = false;
		const flush = () => {
			if (buffer.length > 0) {
				const batch = buffer.splice(0, buffer.length);
				setLogEntries((prev) => [...prev, ...batch].slice(-MAX_LOG_ENTRIES));
			}
			if (debugBuffer.length > 0) {
				const batch = debugBuffer.splice(0, debugBuffer.length);
				setDebugData((prev) => [...prev, ...batch]);
			}
			if (inflightDirty) {
				inflightDirty = false;
				setInflight([...inflightMap.values()]);
			}
		};
		const timer = setInterval(flush, FLUSH_INTERVAL_MS);

		const es = new EventSource(
			`${window.location.origin}/api/check/${jobId}/progress`,
		);
		es.onopen = () => setConnection("open");
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			if (data.debug) debugBuffer.push(data.debug);

			if (isPhaseEvent(data)) {
				// In-flight: record/update this node's current phase. Do not touch
				// progress counters or the completed log.
				inflightMap.set(data.node_id as string, {
					node_id: data.node_id as string,
					node_name: data.node_name ?? "",
					phase: data.phase as string,
				});
				inflightDirty = true;
				return;
			}

			if (isResultEvent(data)) {
				// Finished: drop from in-flight, append to the completed log.
				if (data.node_id && inflightMap.delete(data.node_id)) {
					inflightDirty = true;
				}
				buffer.push(data);
			}

			// Counters advance only from non-phase events.
			setProgress(data);

			if (data.done) {
				inflightMap.clear();
				setInflight([]);
				flush();
				setConnection("done");
				es.close();
				qc.invalidateQueries({ queryKey: queryKeys.jobs(subscriptionId) });
				qc.invalidateQueries({ queryKey: queryKeys.latestJobs() });
				qc.invalidateQueries({
					queryKey: queryKeys.results(subscriptionId),
				});
				onDoneRef.current?.();
			}
		};
		// Do NOT close on error: EventSource retries on its own. If the job
		// finished while we were away, the next message is a done event
		// (checker re-sends terminal state to late subscribers).
		es.onerror = () => setConnection("reconnecting");

		return () => {
			clearInterval(timer);
			es.close();
		};
	}, [jobId, qc, subscriptionId]);

	return { progress, logEntries, debugData, connection, inflight };
}
