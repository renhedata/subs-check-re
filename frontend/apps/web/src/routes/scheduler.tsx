// frontend/apps/web/src/routes/scheduler.tsx
import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Trash2, Plus } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Card, CardContent } from "@frontend/ui/components/card";

import { api, ApiError, type ScheduledJob, type Subscription } from "@/lib/api";

export const Route = createFileRoute("/scheduler")({
  component: SchedulerPage,
});

function SchedulerPage() {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [subId, setSubId] = useState("");
  const [cronExpr, setCronExpr] = useState("");

  const jobsQuery = useQuery({
    queryKey: ["scheduler"],
    queryFn: () => api.get<{ jobs: ScheduledJob[] }>("/scheduler"),
  });

  const subsQuery = useQuery({
    queryKey: ["subscriptions"],
    queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
  });

  const createMut = useMutation({
    mutationFn: () => api.post("/scheduler", { subscription_id: subId, cron_expr: cronExpr }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["scheduler"] });
      setAdding(false);
      setSubId("");
      setCronExpr("");
      toast.success("Schedule created");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.delete(`/scheduler/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["scheduler"] });
      toast.success("Removed");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
  });

  const jobs = jobsQuery.data?.jobs ?? [];
  const subs = subsQuery.data?.subscriptions ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Scheduler</h1>
        <Button size="sm" onClick={() => setAdding(!adding)}>
          <Plus className="mr-1 h-4 w-4" /> Add Schedule
        </Button>
      </div>

      {adding && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label>Subscription</Label>
              <select
                className="w-full rounded border px-3 py-2 text-sm bg-background"
                value={subId}
                onChange={(e) => setSubId(e.target.value)}
              >
                <option value="">Select subscription…</option>
                {subs.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name || s.url}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <Label>Cron Expression</Label>
              <Input
                placeholder="0 */6 * * *  (every 6 hours)"
                value={cronExpr}
                onChange={(e) => setCronExpr(e.target.value)}
              />
            </div>
            <div className="flex gap-2">
              <Button
                onClick={() => createMut.mutate()}
                disabled={!subId || !cronExpr || createMut.isPending}
              >
                {createMut.isPending ? "Saving..." : "Save"}
              </Button>
              <Button variant="outline" onClick={() => setAdding(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="space-y-3">
        {jobs.map((job) => (
          <Card key={job.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div>
                <p className="font-mono text-sm">{job.cron_expr}</p>
                <p className="text-xs text-muted-foreground">{job.subscription_id}</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => deleteMut.mutate(job.id)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </CardContent>
          </Card>
        ))}
        {!jobsQuery.isLoading && jobs.length === 0 && (
          <p className="text-center text-muted-foreground py-8">No scheduled jobs.</p>
        )}
      </div>
    </div>
  );
}
