// frontend/apps/web/src/routes/settings/notify.tsx
import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Trash2, Plus } from "lucide-react";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Card, CardContent } from "@frontend/ui/components/card";

import { api, ApiError, type NotifyChannel } from "@/lib/api";

export const Route = createFileRoute("/settings/notify")({
  component: NotifyPage,
});

function NotifyPage() {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [type, setType] = useState<"webhook" | "telegram">("webhook");
  const [name, setName] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [botToken, setBotToken] = useState("");
  const [chatId, setChatId] = useState("");

  const channelsQuery = useQuery({
    queryKey: ["notify-channels"],
    queryFn: () => api.get<{ channels: NotifyChannel[] }>("/notify/channels"),
  });

  const createMut = useMutation({
    mutationFn: () => {
      const config =
        type === "webhook"
          ? { url: webhookUrl, method: "POST" }
          : { bot_token: botToken, chat_id: chatId };
      return api.post("/notify/channels", { name, type, config });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["notify-channels"] });
      setAdding(false);
      setName("");
      setWebhookUrl("");
      setBotToken("");
      setChatId("");
      toast.success("Channel added");
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : "Failed"),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.delete(`/notify/channels/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["notify-channels"] });
      toast.success("Removed");
    },
  });

  const channels = channelsQuery.data?.channels ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Notification Channels</h1>
        <Button size="sm" onClick={() => setAdding(!adding)}>
          <Plus className="mr-1 h-4 w-4" /> Add Channel
        </Button>
      </div>

      {adding && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="space-y-1">
              <Label>Name</Label>
              <Input
                placeholder="My Channel"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label>Type</Label>
              <select
                className="w-full rounded border px-3 py-2 text-sm bg-background"
                value={type}
                onChange={(e) => setType(e.target.value as "webhook" | "telegram")}
              >
                <option value="webhook">Webhook</option>
                <option value="telegram">Telegram</option>
              </select>
            </div>
            {type === "webhook" && (
              <div className="space-y-1">
                <Label>URL</Label>
                <Input
                  placeholder="https://..."
                  value={webhookUrl}
                  onChange={(e) => setWebhookUrl(e.target.value)}
                />
              </div>
            )}
            {type === "telegram" && (
              <>
                <div className="space-y-1">
                  <Label>Bot Token</Label>
                  <Input
                    placeholder="123456:ABC..."
                    value={botToken}
                    onChange={(e) => setBotToken(e.target.value)}
                  />
                </div>
                <div className="space-y-1">
                  <Label>Chat ID</Label>
                  <Input
                    placeholder="-1001234567890"
                    value={chatId}
                    onChange={(e) => setChatId(e.target.value)}
                  />
                </div>
              </>
            )}
            <div className="flex gap-2">
              <Button onClick={() => createMut.mutate()} disabled={createMut.isPending}>
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
        {channels.map((ch) => (
          <Card key={ch.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div>
                <p className="font-medium">{ch.name || ch.id}</p>
                <p className="text-xs text-muted-foreground uppercase">{ch.type}</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => deleteMut.mutate(ch.id)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </CardContent>
          </Card>
        ))}
        {!channelsQuery.isLoading && channels.length === 0 && (
          <p className="text-center text-muted-foreground py-8">No channels configured.</p>
        )}
      </div>
    </div>
  );
}
