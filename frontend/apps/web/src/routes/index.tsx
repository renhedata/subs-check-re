// frontend/apps/web/src/routes/index.tsx
import { createFileRoute, redirect } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@frontend/ui/components/card";

import { api, type Subscription } from "@/lib/api";
import { isAuthenticated } from "@/lib/auth";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    if (!isAuthenticated()) throw redirect({ to: "/login" });
  },
  component: DashboardPage,
});

function DashboardPage() {
  const { data } = useQuery({
    queryKey: ["subscriptions"],
    queryFn: () => api.get<{ subscriptions: Subscription[] }>("/subscriptions"),
  });

  const subs = data?.subscriptions ?? [];
  const enabled = subs.filter((s) => s.enabled).length;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>
      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Subscriptions
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold">{subs.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Active</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold">{enabled}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Scheduled</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold">{subs.filter((s) => s.cron_expr).length}</p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
