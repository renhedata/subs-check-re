import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import type { NodeDebug } from "@/components/debug-panel";
import { queryKeys } from "./queryKeys";

export interface SSEProgress {
	progress?: number;
	total?: number;
	node_name?: string;
	alive?: boolean;
	latency_ms?: number;
	speed_kbps?: number;
	upload_speed_kbps?: number;
	done?: boolean;
	status?: string;
	debug?: NodeDebug;
}

interface UseSSEProgressOptions {
	jobId: string | null;
	subscriptionId: string;
	onDone?: () => void;
}

interface UseSSEProgressResult {
	progress: SSEProgress | null;
	logEntries: SSEProgress[];
	debugData: NodeDebug[];
}

// useSSEProgress subscribes to /api/check/:jobId/progress, exposing the latest
// progress event plus accumulated log entries and per-node debug traces. It
// auto-resets when jobId changes and invalidates the jobs list once the stream
// reports done so the parent route can refresh.
export function useSSEProgress({
	jobId,
	subscriptionId,
	onDone,
}: UseSSEProgressOptions): UseSSEProgressResult {
	const [progress, setProgress] = useState<SSEProgress | null>(null);
	const [logEntries, setLogEntries] = useState<SSEProgress[]>([]);
	const [debugData, setDebugData] = useState<NodeDebug[]>([]);
	const qc = useQueryClient();
	const onDoneRef = useRef(onDone);
	onDoneRef.current = onDone;

	useEffect(() => {
		setLogEntries([]);
		setProgress(null);
		setDebugData([]);
	}, [jobId]);

	useEffect(() => {
		if (!jobId) return;
		const es = new EventSource(
			`${window.location.origin}/api/check/${jobId}/progress`,
		);
		es.onmessage = (e) => {
			const data: SSEProgress = JSON.parse(e.data);
			setProgress(data);
			if (data.debug) {
				setDebugData((prev) => [...prev, data.debug as NodeDebug]);
			}
			if (data.node_name) {
				setLogEntries((prev) => [...prev, data]);
			}
			if (data.done) {
				es.close();
				qc.invalidateQueries({ queryKey: queryKeys.jobs(subscriptionId) });
				onDoneRef.current?.();
			}
		};
		es.onerror = () => es.close();
		return () => es.close();
	}, [jobId, qc, subscriptionId]);

	return { progress, logEntries, debugData };
}
