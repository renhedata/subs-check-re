import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { ApiError, api, type NotifyChannel } from "@/lib/api";

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
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-[#f0f6fc] text-lg">
					Notification Channels
				</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Channel
				</button>
			</div>

			{adding && (
				<div
					className="space-y-3 rounded-lg border p-4"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Name</Label>
						<Input
							placeholder="My Channel"
							value={name}
							onChange={(e) => setName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="space-y-1.5">
						<Label className="text-[#8b949e] text-xs">Type</Label>
						<Select
							value={type}
							onValueChange={(v) => setType(v as "webhook" | "telegram")}
						>
							<SelectTrigger className="h-8 text-sm">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="webhook">Webhook</SelectItem>
								<SelectItem value="telegram">Telegram</SelectItem>
							</SelectContent>
						</Select>
					</div>
					{type === "webhook" && (
						<div className="space-y-1.5">
							<Label className="text-[#8b949e] text-xs">URL</Label>
							<Input
								placeholder="https://..."
								value={webhookUrl}
								onChange={(e) => setWebhookUrl(e.target.value)}
								className="h-8 text-sm"
							/>
						</div>
					)}
					{type === "telegram" && (
						<>
							<div className="space-y-1.5">
								<Label className="text-[#8b949e] text-xs">Bot Token</Label>
								<Input
									placeholder="123456:ABC..."
									value={botToken}
									onChange={(e) => setBotToken(e.target.value)}
									className="h-8 font-mono text-sm"
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-[#8b949e] text-xs">Chat ID</Label>
								<Input
									placeholder="-1001234567890"
									value={chatId}
									onChange={(e) => setChatId(e.target.value)}
									className="h-8 font-mono text-sm"
								/>
							</div>
						</>
					)}
					<div className="flex gap-2">
						<button
							type="button"
							onClick={() => createMut.mutate()}
							disabled={createMut.isPending}
							className="flex items-center gap-2 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90 disabled:opacity-50"
							style={{ background: "#238636" }}
						>
							{createMut.isPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Save"
							)}
						</button>
						<button
							type="button"
							onClick={() => setAdding(false)}
							className="rounded-md border px-3 py-1.5 text-sm transition-colors hover:bg-white/5"
							style={{ borderColor: "#30363d", color: "#8b949e" }}
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{channels.map((ch) => (
					<div
						key={ch.id}
						className="flex items-center justify-between rounded-lg border px-4 py-3"
						style={{ background: "#161b22", borderColor: "#30363d" }}
					>
						<div>
							<p className="font-medium text-[#f0f6fc] text-sm">
								{ch.name || ch.id}
							</p>
							<p
								className="mt-0.5 text-[11px] uppercase tracking-[0.4px]"
								style={{ color: "#8b949e" }}
							>
								{ch.type}
							</p>
						</div>
						<button
							type="button"
							onClick={() => deleteMut.mutate(ch.id)}
							className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149]"
							style={{ color: "#6e7681" }}
						>
							<Trash2 size={13} strokeWidth={1.5} />
						</button>
					</div>
				))}
				{!channelsQuery.isLoading && channels.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No channels configured.
					</p>
				)}
			</div>
		</div>
	);
}
