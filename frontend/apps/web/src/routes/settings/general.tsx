// frontend/apps/web/src/routes/settings/general.tsx
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { useEffect } from "react";
import { toast } from "sonner";

import { api, type UserSettings } from "@/lib/api";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";

export const Route = createFileRoute("/settings/general")({
  component: GeneralSettingsPage,
});

const DEFAULT_SPEED_TEST_URL =
  "https://speed.cloudflare.com/__down?bytes=204800";

function GeneralSettingsPage() {
  const qc = useQueryClient();

  const settingsQuery = useQuery({
    queryKey: ["settings"],
    queryFn: () => api.get<UserSettings>("/settings"),
  });

  const { register, handleSubmit, reset } = useForm<UserSettings>({
    defaultValues: { speed_test_url: "" },
  });

  useEffect(() => {
    if (settingsQuery.data) {
      reset(settingsQuery.data);
    }
  }, [settingsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: (data: UserSettings) => api.put<UserSettings>("/settings", data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["settings"] });
      toast.success("Settings saved");
    },
    onError: () => toast.error("Failed to save settings"),
  });

  return (
    <div className="space-y-6 max-w-lg">
      <h1 className="text-2xl font-bold">General Settings</h1>

      <form onSubmit={handleSubmit((d) => saveMutation.mutate(d))} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="speed_test_url">Speed Test URL</Label>
          <Input
            id="speed_test_url"
            placeholder={DEFAULT_SPEED_TEST_URL}
            {...register("speed_test_url")}
          />
          <p className="text-xs text-muted-foreground">
            URL used to measure download speed for each node. Leave blank to use the default ({DEFAULT_SPEED_TEST_URL}).
          </p>
        </div>

        <Button type="submit" disabled={saveMutation.isPending}>
          {saveMutation.isPending ? "Saving…" : "Save"}
        </Button>
      </form>
    </div>
  );
}
