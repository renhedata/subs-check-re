// frontend/apps/web/src/routes/subscriptions/$id.tsx
import { useEffect, useRef, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

import { api, type CheckJob, type NodeResult } from "@/lib/api";
import { NodeTable } from "@/components/node-table";

const searchSchema = z.object({
  job: z.string().optional(),
});

export const Route = createFileRoute("/subscriptions/$id")({
  validateSearch: searchSchema,
  component: SubscriptionDetailPage,
});

interface SSEProgress {
  progress?: number;
  total?: number;
  node_name?: string;
  alive?: boolean;
  latency_ms?: number;
  speed_kbps?: number;
  done?: boolean;
  status?: string;
}

function SubscriptionDetailPage() {
  const { id } = Route.useParams();
  const { job: jobIdFromSearch } = Route.useSearch();
  const [jobId, setJobId] = useState<string | null>(jobIdFromSearch ?? null);
  const [progress, setProgress] = useState<SSEProgress | null>(null);
  const esRef = useRef<EventSource | null>(null);

  const resultsQuery = useQuery({
    queryKey: ["results", id],
    queryFn: () =>
      api.get<{ job: CheckJob; results: NodeResult[] }>(`/check/${id}/results`),
    retry: false,
    staleTime: 0, // always refetch on navigate to pick up latest results
  });

  // If the latest job is still running (e.g., navigated back), attach SSE to it
  useEffect(() => {
    const job = resultsQuery.data?.job;
    if (job && (job.status === "running" || job.status === "queued") && !jobId) {
      setJobId(job.id);
    }
  }, [resultsQuery.data?.job?.id, resultsQuery.data?.job?.status]);

  // Start SSE when jobId is set
  // Note: GetProgress is public (job UUID acts as capability token)
  useEffect(() => {
    if (!jobId) return;

    const es = new EventSource(`/api/check/${jobId}/progress`);
    esRef.current = es;

    es.onmessage = (e) => {
      const data: SSEProgress = JSON.parse(e.data);
      setProgress(data);
      if (data.done) {
        es.close();
        resultsQuery.refetch();
      }
    };
    es.onerror = () => {
      es.close();
    };

    return () => {
      es.close();
    };
  }, [jobId]);

  const job = resultsQuery.data?.job;
  const results = resultsQuery.data?.results ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Subscription Detail</h1>
        <span className="text-sm text-muted-foreground font-mono">{id.slice(0, 8)}…</span>
      </div>

      {/* Progress bar */}
      {progress && !progress.done && (
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span>Checking nodes…</span>
            <span>
              {progress.progress ?? 0} / {progress.total ?? "?"}
            </span>
          </div>
          <div className="h-2 w-full rounded bg-muted overflow-hidden">
            <div
              className="h-full bg-primary transition-all"
              style={{
                width: progress.total
                  ? `${((progress.progress ?? 0) / progress.total) * 100}%`
                  : "0%",
              }}
            />
          </div>
          {progress.node_name && (
            <p className="text-xs text-muted-foreground truncate">
              ↳ {progress.node_name}
              {progress.alive ? (
                <>
                  {progress.latency_ms ? (
                    <span className="ml-2 text-green-600 font-medium">{progress.latency_ms}ms</span>
                  ) : null}
                  {progress.speed_kbps ? (
                    <span className="ml-1 text-blue-500 font-medium">
                      {progress.speed_kbps >= 1024
                        ? `${(progress.speed_kbps / 1024).toFixed(1)}MB/s`
                        : `${progress.speed_kbps}KB/s`}
                    </span>
                  ) : null}
                </>
              ) : progress.alive === false ? (
                <span className="ml-2 text-red-500">dead</span>
              ) : null}
            </p>
          )}
        </div>
      )}

      {/* Job status */}
      {job && (
        <div className="flex gap-4 text-sm">
          <span>
            Status: <strong>{job.status}</strong>
          </span>
          <span>Nodes: {job.total}</span>
          <span>Available: {results.filter((r) => r.alive).length}</span>
        </div>
      )}

      {resultsQuery.isLoading && <p className="text-muted-foreground">Loading results…</p>}
      {resultsQuery.isError && <p className="text-muted-foreground">No check results yet.</p>}

      <NodeTable results={results} />
    </div>
  );
}
