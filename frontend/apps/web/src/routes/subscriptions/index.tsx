// frontend/apps/web/src/routes/subscriptions/index.tsx
import { useState } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Plus, Play, Trash2 } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Card, CardContent } from "@frontend/ui/components/card";

import { api, ApiError, type Subscription } from "@/lib/api";

export const Route = createFileRoute("/subscriptions/")({
  component: SubscriptionsPage,
});

function SubscriptionsPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [adding, setAdding] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["subscriptions"],
    queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.delete(`/subscriptions/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["subscriptions"] });
      toast.success("Deleted");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Delete failed"),
  });

  const triggerMut = useMutation({
    mutationFn: (id: string) => api.post<{ job_id: string }>(`/check/${id}`),
    onSuccess: (resp, id) => {
      toast.success("Check started");
      navigate({ to: "/subscriptions/$id", params: { id }, search: { job: resp.job_id } });
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed to start check"),
  });

  const createMut = useMutation({
    mutationFn: () => api.post<Subscription>("/subscriptions", { name, url }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["subscriptions"] });
      setName("");
      setUrl("");
      setAdding(false);
      toast.success("Subscription added");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed to add"),
  });

  const subs = data?.subscriptions ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Subscriptions</h1>
        <Button onClick={() => setAdding(!adding)} size="sm">
          <Plus className="mr-1 h-4 w-4" /> Add
        </Button>
      </div>

      {adding && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label>Name (optional)</Label>
              <Input placeholder="My Sub" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label>Subscription URL</Label>
              <Input placeholder="https://..." value={url} onChange={(e) => setUrl(e.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button onClick={() => createMut.mutate()} disabled={!url || createMut.isPending}>
                {createMut.isPending ? "Adding..." : "Add"}
              </Button>
              <Button variant="outline" onClick={() => setAdding(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {isLoading && <p className="text-muted-foreground">Loading...</p>}

      <div className="space-y-3">
        {subs.map((sub) => (
          <Card key={sub.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div>
                <Link
                  to="/subscriptions/$id"
                  params={{ id: sub.id }}
                  className="font-medium hover:underline"
                >
                  {sub.name || sub.url}
                </Link>
                {sub.name && (
                  <p className="text-sm text-muted-foreground truncate max-w-md">{sub.url}</p>
                )}
                {sub.cron_expr && (
                  <p className="text-xs text-muted-foreground">⏱ {sub.cron_expr}</p>
                )}
              </div>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => triggerMut.mutate(sub.id)}
                  disabled={triggerMut.isPending}
                >
                  <Play className="h-3 w-3 mr-1" /> Check
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => deleteMut.mutate(sub.id)}
                  disabled={deleteMut.isPending}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
        {!isLoading && subs.length === 0 && (
          <p className="text-center text-muted-foreground py-8">
            No subscriptions yet. Add one above.
          </p>
        )}
      </div>
    </div>
  );
}
