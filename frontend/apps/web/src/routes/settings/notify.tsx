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
import {
	Bell,
	CheckCircle2,
	Clock,
	FlaskConical,
	Loader2,
	Pencil,
	Plus,
	Trash2,
	XCircle,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { client, isApiError } from "@/lib/client";
import type { JSONValue, notify } from "@/lib/client.gen";

type NotifyChannel = notify.Channel;

const CRON_PRESETS = [
	{ label: "Disabled", value: "" },
	{ label: "Every hour", value: "0 * * * *" },
	{ label: "Every 2 hours", value: "0 */2 * * *" },
	{ label: "Every 6 hours", value: "0 */6 * * *" },
	{ label: "Every 12 hours", value: "0 */12 * * *" },
	{ label: "Once a day", value: "0 0 * * *" },
] as const;

function cronToLabel(cron: string): string {
	const preset = CRON_PRESETS.find((p) => p.value === cron);
	return preset ? preset.label : cron;
}

export const Route = createFileRoute("/settings/notify")({
	component: NotifyPage,
});

function NotifyPage() {
	const qc = useQueryClient();
	const [adding, setAdding] = useState(false);

	// Create form state
	const [type, setType] = useState<"webhook" | "telegram">("webhook");
	const [name, setName] = useState("");
	const [webhookUrl, setWebhookUrl] = useState("");
	const [botToken, setBotToken] = useState("");
	const [chatId, setChatId] = useState("");
	const [onCheckComplete, setOnCheckComplete] = useState(false);
	const [unlockCron, setUnlockCron] = useState("");

	// Edit state
	const [editingId, setEditingId] = useState<string | null>(null);
	const [editName, setEditName] = useState("");
	const [editEnabled, setEditEnabled] = useState(true);
	const [editOnCheck, setEditOnCheck] = useState(false);
	const [editUnlockCron, setEditUnlockCron] = useState("");

	const channelsQuery = useQuery({
		queryKey: ["notify-channels"],
		queryFn: () => client.notify.ListChannels(),
	});

	const createMut = useMutation({
		mutationFn: () => {
			const config: JSONValue =
				type === "webhook"
					? { url: webhookUrl, method: "POST" }
					: { bot_token: botToken, chat_id: chatId };
			return client.notify.CreateChannel({
				name,
				type,
				config,
				on_check_complete: onCheckComplete,
				unlock_cron: unlockCron,
			});
		},
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["notify-channels"] });
			setAdding(false);
			setName("");
			setWebhookUrl("");
			setBotToken("");
			setChatId("");
			setOnCheckComplete(false);
			setUnlockCron("");
			toast.success("Channel added");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Failed"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => client.notify.DeleteChannel(id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["notify-channels"] });
			toast.success("Removed");
		},
	});

	const updateMut = useMutation({
		mutationFn: ({
			id,
			data,
		}: {
			id: string;
			data: notify.UpdateChannelParams;
		}) => client.notify.UpdateChannel(id, data),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["notify-channels"] });
			setEditingId(null);
			toast.success("Updated");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Update failed"),
	});

	const testMut = useMutation({
		mutationFn: ({ id, reportType }: { id: string; reportType: string }) =>
			client.notify.TestChannel(id, { report_type: reportType }),
		onSuccess: (resp) => {
			if (resp.ok) {
				toast.success("Test notification sent");
			} else {
				toast.error(`Test failed: ${resp.error ?? "unknown error"}`);
			}
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Test failed"),
	});

	const channels = channelsQuery.data?.channels ?? [];

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="font-semibold text-foreground text-lg">
						Notification Channels
					</h1>
					<p className="mt-0.5 text-xs text-muted-foreground">
						Configure webhook or Telegram notifications for unlock reports and
						check results.
					</p>
				</div>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90"
					style={{ background: "var(--color-btn-success)" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Channel
				</button>
			</div>

			{adding && (
				<div className="space-y-3 rounded-lg border border-border bg-card p-4">
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">Name</Label>
						<Input
							placeholder="My Channel"
							value={name}
							onChange={(e) => setName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">Type</Label>
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
							<Label className="text-muted-foreground text-xs">URL</Label>
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
								<Label className="text-muted-foreground text-xs">
									Bot Token
								</Label>
								<Input
									placeholder="123456:ABC..."
									value={botToken}
									onChange={(e) => setBotToken(e.target.value)}
									className="h-8 font-mono text-sm"
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-muted-foreground text-xs">Chat ID</Label>
								<Input
									placeholder="-1001234567890"
									value={chatId}
									onChange={(e) => setChatId(e.target.value)}
									className="h-8 font-mono text-sm"
								/>
							</div>
						</>
					)}

					<ReportSettings
						onCheckComplete={onCheckComplete}
						setOnCheckComplete={setOnCheckComplete}
						unlockCron={unlockCron}
						setUnlockCron={setUnlockCron}
					/>

					<div className="flex gap-2">
						<button
							type="button"
							onClick={() => createMut.mutate()}
							disabled={createMut.isPending}
							className="flex items-center gap-2 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90 disabled:opacity-50"
							style={{ background: "var(--color-btn-success)" }}
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
							className="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-white/5"
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{channels.map((ch) => (
					<ChannelRow
						key={ch.id}
						ch={ch}
						isEditing={editingId === ch.id}
						editName={editName}
						editEnabled={editEnabled}
						editOnCheck={editOnCheck}
						editUnlockCron={editUnlockCron}
						setEditName={setEditName}
						setEditEnabled={setEditEnabled}
						setEditOnCheck={setEditOnCheck}
						setEditUnlockCron={setEditUnlockCron}
						onEditOpen={() => {
							if (editingId === ch.id) {
								setEditingId(null);
							} else {
								setEditingId(ch.id);
								setEditName(ch.name);
								setEditEnabled(ch.enabled);
								setEditOnCheck(ch.on_check_complete);
								setEditUnlockCron(ch.unlock_cron);
							}
						}}
						onEditClose={() => setEditingId(null)}
						onSaveEdit={() =>
							updateMut.mutate({
								id: ch.id,
								data: {
									name: editName,
									enabled: editEnabled,
									config: ch.config,
									on_check_complete: editOnCheck,
									unlock_cron: editUnlockCron,
								},
							})
						}
						onDelete={() => deleteMut.mutate(ch.id)}
						onTest={(reportType) =>
							testMut.mutate({ id: ch.id, reportType })
						}
						editPending={updateMut.isPending}
						deletePending={deleteMut.isPending}
						testPending={
							testMut.isPending && testMut.variables?.id === ch.id
						}
					/>
				))}
				{!channelsQuery.isLoading && channels.length === 0 && (
					<p className="py-10 text-center text-sm text-muted-foreground">
						No channels configured.
					</p>
				)}
			</div>
		</div>
	);
}

// --- Report settings ---

function ReportSettings({
	onCheckComplete,
	setOnCheckComplete,
	unlockCron,
	setUnlockCron,
}: {
	onCheckComplete: boolean;
	setOnCheckComplete: (v: boolean) => void;
	unlockCron: string;
	setUnlockCron: (v: string) => void;
}) {
	return (
		<div className="space-y-3 rounded-md border border-border/60 bg-secondary/30 p-3">
			<p className="font-medium text-foreground text-xs">Report Types</p>

			{/* Scheduled unlock report */}
			<div className="space-y-1.5">
				<div className="flex items-center gap-2">
					<Clock size={12} strokeWidth={1.5} className="text-muted-foreground" />
					<Label className="text-xs text-muted-foreground">
						Scheduled network unlock report
					</Label>
				</div>
				<Select
					value={unlockCron || "__disabled__"}
					onValueChange={(v) => setUnlockCron(!v || v === "__disabled__" ? "" : v)}
				>
					<SelectTrigger className="h-7 w-48 text-xs">
						<SelectValue placeholder="Disabled" />
					</SelectTrigger>
					<SelectContent>
						{CRON_PRESETS.map((p) => (
							<SelectItem key={p.value || "__disabled__"} value={p.value || "__disabled__"}>
								{p.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<p className="text-[10px] text-muted-foreground/70">
					Periodically reports which streaming platforms are accessible from this
					server's network.
				</p>
			</div>

			{/* Check job report */}
			<div className="space-y-1">
				<label className="flex cursor-pointer select-none items-center gap-2">
					<input
						type="checkbox"
						checked={onCheckComplete}
						onChange={(e) => setOnCheckComplete(e.target.checked)}
					/>
					<Bell size={12} strokeWidth={1.5} className="text-muted-foreground" />
					<span className="text-xs text-muted-foreground">
						Notify on check completion
					</span>
				</label>
				<p className="ml-[18px] text-[10px] text-muted-foreground/70">
					Sends a detailed report when a subscription check finishes, including
					speed stats, platform unlocks, top nodes, and country breakdown.
				</p>
			</div>
		</div>
	);
}

// --- Channel row ---

function ChannelRow({
	ch,
	isEditing,
	editName,
	editEnabled,
	editOnCheck,
	editUnlockCron,
	setEditName,
	setEditEnabled,
	setEditOnCheck,
	setEditUnlockCron,
	onEditOpen,
	onEditClose,
	onSaveEdit,
	onDelete,
	onTest,
	editPending,
	deletePending,
	testPending,
}: {
	ch: NotifyChannel;
	isEditing: boolean;
	editName: string;
	editEnabled: boolean;
	editOnCheck: boolean;
	editUnlockCron: string;
	setEditName: (v: string) => void;
	setEditEnabled: (v: boolean) => void;
	setEditOnCheck: (v: boolean) => void;
	setEditUnlockCron: (v: string) => void;
	onEditOpen: () => void;
	onEditClose: () => void;
	onSaveEdit: () => void;
	onDelete: () => void;
	onTest: (reportType: string) => void;
	editPending: boolean;
	deletePending: boolean;
	testPending: boolean;
}) {
	// Build feature tags
	const tags: string[] = [];
	if (ch.on_check_complete) tags.push("check report");
	if (ch.unlock_cron) tags.push(cronToLabel(ch.unlock_cron));

	return (
		<div className="rounded-lg border border-border bg-card">
			<div className="flex items-center justify-between px-4 py-3">
				<div className="flex items-center gap-3">
					{ch.enabled ? (
						<CheckCircle2
							size={14}
							strokeWidth={1.5}
							style={{ color: "var(--color-success)" }}
						/>
					) : (
						<XCircle
							size={14}
							strokeWidth={1.5}
							style={{ color: "var(--color-dimmed)" }}
						/>
					)}
					<div>
						<p className="font-medium text-foreground text-sm">
							{ch.name || ch.id}
						</p>
						<p className="mt-0.5 text-[11px] tracking-[0.4px] text-muted-foreground">
							<span className="uppercase">{ch.type}</span>
							{" · "}
							{ch.enabled ? "enabled" : "disabled"}
							{tags.length > 0 && ` · ${tags.join(" · ")}`}
						</p>
					</div>
				</div>
				<div className="flex items-center gap-1">
					{/* Test buttons */}
					<div className="flex gap-1">
						{ch.unlock_cron && (
							<button
								type="button"
								onClick={() => onTest("unlock")}
								disabled={testPending}
								title="Test unlock report"
								className="flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-white/5 disabled:opacity-50"
							>
								{testPending ? (
									<Loader2 size={10} className="animate-spin" />
								) : (
									<FlaskConical size={10} strokeWidth={1.5} />
								)}
								Unlock
							</button>
						)}
						{ch.on_check_complete && (
							<button
								type="button"
								onClick={() => onTest("check")}
								disabled={testPending}
								title="Test check report"
								className="flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-white/5 disabled:opacity-50"
							>
								{testPending ? (
									<Loader2 size={10} className="animate-spin" />
								) : (
									<FlaskConical size={10} strokeWidth={1.5} />
								)}
								Check
							</button>
						)}
					</div>
					<button
						type="button"
						onClick={onEditOpen}
						className="rounded-md p-1.5 transition-colors hover:bg-white/5"
						style={{
							color: isEditing ? "var(--primary)" : "var(--color-dimmed)",
						}}
					>
						<Pencil size={13} strokeWidth={1.5} />
					</button>
					<button
						type="button"
						onClick={onDelete}
						disabled={deletePending}
						className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
						style={{ color: "var(--color-dimmed)" }}
					>
						<Trash2 size={13} strokeWidth={1.5} />
					</button>
				</div>
			</div>
			{isEditing && (
				<div className="space-y-3 border-t border-border px-4 py-3">
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">Name</Label>
						<Input
							value={editName}
							onChange={(e) => setEditName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<label className="flex cursor-pointer select-none items-center gap-2">
						<input
							type="checkbox"
							checked={editEnabled}
							onChange={(e) => setEditEnabled(e.target.checked)}
						/>
						<span className="text-xs text-muted-foreground">Enabled</span>
					</label>
					<ReportSettings
						onCheckComplete={editOnCheck}
						setOnCheckComplete={setEditOnCheck}
						unlockCron={editUnlockCron}
						setUnlockCron={setEditUnlockCron}
					/>
					<div className="flex gap-2">
						<button
							type="button"
							onClick={onSaveEdit}
							disabled={editPending}
							className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
							style={{ background: "var(--color-btn-success)" }}
						>
							{editPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Save"
							)}
						</button>
						<button
							type="button"
							onClick={onEditClose}
							className="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground"
						>
							Cancel
						</button>
					</div>
				</div>
			)}
		</div>
	);
}
