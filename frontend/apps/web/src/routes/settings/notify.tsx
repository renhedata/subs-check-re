import { Checkbox } from "@frontend/ui/components/checkbox";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";
// queries imported from queries/ — no direct @tanstack/react-query usage here
import { createFileRoute } from "@tanstack/react-router";
import cronstrue from "cronstrue";
import {
	AlertTriangle,
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
import { CronPicker } from "@/components/cron-picker";
import type { PlatformKey } from "@/components/platform-icons";
import { PlatformIcon } from "@/components/platform-icons";
import { isApiError } from "@/lib/client";
import type { JSONValue, notify } from "@/lib/client.gen";
import {
	useCreateNotifyChannel,
	useDeleteNotifyChannel,
	useNotifyChannels,
	useTestNotifyChannel,
	useUpdateNotifyChannel,
} from "@/queries";

type NotifyChannel = notify.Channel;

const ALL_PLATFORMS: { key: PlatformKey; label: string }[] = [
	{ key: "netflix", label: "Netflix" },
	{ key: "youtube", label: "YouTube" },
	{ key: "youtube_premium", label: "YouTube Premium" },
	{ key: "openai", label: "OpenAI" },
	{ key: "claude", label: "Claude" },
	{ key: "gemini", label: "Gemini" },
	{ key: "grok", label: "Grok" },
	{ key: "disney", label: "Disney+" },
	{ key: "tiktok", label: "TikTok" },
];

function cronToLabel(cron: string): string {
	if (!cron) return "disabled";
	try {
		return cronstrue.toString(cron, { use24HourTimeFormat: true });
	} catch {
		return cron;
	}
}

export const Route = createFileRoute("/settings/notify")({
	component: NotifyPage,
});

function NotifyPage() {
	const [adding, setAdding] = useState(false);

	// Create form state
	const [type, setType] = useState<"webhook" | "telegram" | "email">("webhook");
	const [name, setName] = useState("");
	const [webhookUrl, setWebhookUrl] = useState("");
	const [botToken, setBotToken] = useState("");
	const [chatId, setChatId] = useState("");
	const [onCheckComplete, setOnCheckComplete] = useState(false);
	const [unlockCron, setUnlockCron] = useState("");
	const [platformAlerts, setPlatformAlerts] = useState<string[]>([]);

	const channelsQuery = useNotifyChannels();
	const createMut = useCreateNotifyChannel();
	const deleteMut = useDeleteNotifyChannel();
	const updateMut = useUpdateNotifyChannel();
	const testMut = useTestNotifyChannel();

	const handleCreate = () => {
		const config: JSONValue =
			type === "webhook"
				? { url: webhookUrl, method: "POST" }
				: type === "telegram"
					? { bot_token: botToken, chat_id: chatId }
					: {};
		createMut.mutate(
			{
				name,
				type,
				config,
				on_check_complete: onCheckComplete,
				unlock_cron: unlockCron,
				platform_alerts: platformAlerts,
			},
			{
				onSuccess: () => {
					setAdding(false);
					setName("");
					setWebhookUrl("");
					setBotToken("");
					setChatId("");
					setOnCheckComplete(false);
					setUnlockCron("");
					setPlatformAlerts([]);
					toast.success("Channel added");
				},
				onError: (e) => toast.error(isApiError(e) ? e.message : "Failed"),
			},
		);
	};

	const handleDelete = (id: string) =>
		deleteMut.mutate(id, {
			onSuccess: () => toast.success("Removed"),
		});

	const handleUpdate = (id: string, data: notify.UpdateChannelParams) =>
		updateMut.mutate(
			{ id, params: data },
			{
				onSuccess: () => toast.success("Updated"),
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Update failed"),
			},
		);

	const handleTest = (id: string, reportType: string) =>
		testMut.mutate(
			{ id, params: { report_type: reportType } },
			{
				onSuccess: (resp) => {
					if (resp.ok) {
						toast.success("Test notification sent");
					} else {
						toast.error(`Test failed: ${resp.error ?? "unknown error"}`);
					}
				},
				onError: (e) => toast.error(isApiError(e) ? e.message : "Test failed"),
			},
		);

	const channels = channelsQuery.data?.channels ?? [];

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="font-semibold text-foreground text-lg">
						Notification Channels
					</h1>
					<p className="mt-0.5 text-muted-foreground text-xs">
						Receive webhook, Telegram, or email alerts for check results,
						scheduled unlock reports, and platform availability changes. Email
						SMTP settings are in{" "}
						<a
							href="/settings/general"
							className="underline underline-offset-2 hover:text-foreground"
						>
							General Settings
						</a>
						.
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
							onValueChange={(v) =>
								setType(v as "webhook" | "telegram" | "email")
							}
						>
							<SelectTrigger className="h-8 text-sm">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="webhook">Webhook</SelectItem>
								<SelectItem value="telegram">Telegram</SelectItem>
								<SelectItem value="email">Email</SelectItem>
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
					{type === "email" && (
						<p className="rounded-md bg-secondary/40 px-3 py-2 text-muted-foreground text-xs">
							Uses SMTP settings from{" "}
							<a
								href="/settings/general"
								className="underline underline-offset-2 hover:text-foreground"
							>
								General Settings
							</a>
							.
						</p>
					)}

					<ReportSettings
						onCheckComplete={onCheckComplete}
						setOnCheckComplete={setOnCheckComplete}
						unlockCron={unlockCron}
						setUnlockCron={setUnlockCron}
						platformAlerts={platformAlerts}
						setPlatformAlerts={setPlatformAlerts}
					/>

					<div className="flex gap-2">
						<button
							type="button"
							onClick={handleCreate}
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
							className="rounded-md border border-border px-3 py-1.5 text-muted-foreground text-sm transition-colors hover:bg-white/5"
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
						onSave={(data) => handleUpdate(ch.id, data)}
						onDelete={() => handleDelete(ch.id)}
						onTest={(reportType) => handleTest(ch.id, reportType)}
						savePending={
							updateMut.isPending && updateMut.variables?.id === ch.id
						}
						deletePending={deleteMut.isPending}
						testPending={testMut.isPending && testMut.variables?.id === ch.id}
					/>
				))}
				{!channelsQuery.isLoading && channels.length === 0 && (
					<p className="py-10 text-center text-muted-foreground text-sm">
						No channels configured.
					</p>
				)}
			</div>
		</div>
	);
}

// --- Platform alerts multi-select ---

function PlatformAlertsSelect({
	selected,
	onChange,
}: {
	selected: string[];
	onChange: (v: string[]) => void;
}) {
	function toggle(key: string) {
		onChange(
			selected.includes(key)
				? selected.filter((k) => k !== key)
				: [...selected, key],
		);
	}

	return (
		<div className="space-y-1.5">
			<div className="flex items-center gap-2">
				<AlertTriangle
					size={12}
					strokeWidth={1.5}
					className="text-muted-foreground"
				/>
				<Label className="text-muted-foreground text-xs">
					Platform unavailability alerts
				</Label>
			</div>
			<div className="flex flex-wrap gap-1.5">
				{ALL_PLATFORMS.map(({ key, label }) => {
					const active = selected.includes(key);
					return (
						<button
							key={key}
							type="button"
							onClick={() => toggle(key)}
							title={label}
							className="flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] transition-colors"
							style={{
								borderColor: active ? "var(--primary)" : "var(--border)",
								background: active
									? "color-mix(in srgb, var(--primary) 12%, transparent)"
									: "transparent",
								color: active ? "var(--primary)" : "var(--muted-foreground)",
								opacity: active ? 1 : 0.6,
							}}
						>
							<PlatformIcon platform={key} size={12} />
							<span>{label}</span>
						</button>
					);
				})}
			</div>
			<p className="text-[10px] text-muted-foreground/60">
				Alert fires when a selected platform drops from available to unavailable
				after a check.
			</p>
		</div>
	);
}

// --- Report settings ---

function ReportSettings({
	onCheckComplete,
	setOnCheckComplete,
	unlockCron,
	setUnlockCron,
	platformAlerts,
	setPlatformAlerts,
}: {
	onCheckComplete: boolean;
	setOnCheckComplete: (v: boolean) => void;
	unlockCron: string;
	setUnlockCron: (v: string) => void;
	platformAlerts: string[];
	setPlatformAlerts: (v: string[]) => void;
}) {
	return (
		<div className="space-y-3 rounded-md border border-border/60 bg-secondary/30 p-3">
			<p className="font-medium text-foreground text-xs">Report Types</p>

			<div className="space-y-1.5">
				<div className="flex items-center gap-2">
					<Clock
						size={12}
						strokeWidth={1.5}
						className="text-muted-foreground"
					/>
					<Label className="text-muted-foreground text-xs">
						Scheduled network unlock report
					</Label>
				</div>
				<CronPicker allowDisable value={unlockCron} onChange={setUnlockCron} />
				<p className="text-[10px] text-muted-foreground/70">
					Periodically reports which streaming platforms are accessible from
					this server's network.
				</p>
			</div>

			<div className="space-y-1">
				<label className="flex cursor-pointer select-none items-center gap-2">
					<Checkbox
						checked={onCheckComplete}
						onCheckedChange={(v) => setOnCheckComplete(v === true)}
					/>
					<Bell size={12} strokeWidth={1.5} className="text-muted-foreground" />
					<span className="text-muted-foreground text-xs">
						Notify on check completion
					</span>
				</label>
				<p className="ml-[18px] text-[10px] text-muted-foreground/70">
					Sends a detailed report when a subscription check finishes, including
					speed stats, platform unlocks, top nodes, and country breakdown.
				</p>
			</div>

			<PlatformAlertsSelect
				selected={platformAlerts}
				onChange={setPlatformAlerts}
			/>
		</div>
	);
}

// --- Channel row ---

function ChannelRow({
	ch,
	onSave,
	onDelete,
	onTest,
	savePending,
	deletePending,
	testPending,
}: {
	ch: NotifyChannel;
	onSave: (data: notify.UpdateChannelParams) => void;
	onDelete: () => void;
	onTest: (reportType: string) => void;
	savePending: boolean;
	deletePending: boolean;
	testPending: boolean;
}) {
	const [isEditing, setIsEditing] = useState(false);
	const [editName, setEditName] = useState("");
	const [editEnabled, setEditEnabled] = useState(true);
	const [editOnCheck, setEditOnCheck] = useState(false);
	const [editUnlockCron, setEditUnlockCron] = useState("");
	const [editPlatformAlerts, setEditPlatformAlerts] = useState<string[]>([]);

	function openEdit() {
		setEditName(ch.name);
		setEditEnabled(ch.enabled);
		setEditOnCheck(ch.on_check_complete);
		setEditUnlockCron(ch.unlock_cron);
		setEditPlatformAlerts(ch.platform_alerts ?? []);
		setIsEditing(true);
	}

	function handleSave() {
		onSave({
			name: editName,
			enabled: editEnabled,
			config: ch.config,
			on_check_complete: editOnCheck,
			unlock_cron: editUnlockCron,
			platform_alerts: editPlatformAlerts,
		});
		setIsEditing(false);
	}

	const tags: string[] = [];
	if (ch.on_check_complete) tags.push("check report");
	if (ch.unlock_cron) tags.push(cronToLabel(ch.unlock_cron));
	if ((ch.platform_alerts ?? []).length > 0) tags.push("platform alerts");

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
						<p className="mt-0.5 text-[11px] text-muted-foreground tracking-[0.4px]">
							<span className="uppercase">{ch.type}</span>
							{" · "}
							{ch.enabled ? "enabled" : "disabled"}
							{tags.length > 0 && ` · ${tags.join(" · ")}`}
						</p>
					</div>
				</div>
				<div className="flex items-center gap-1">
					{(ch.platform_alerts ?? []).length > 0 && (
						<div className="mr-1 flex items-center gap-0.5">
							{ch.platform_alerts.map((p) => (
								<span key={p} title={p} className="opacity-50">
									<PlatformIcon platform={p as PlatformKey} size={13} />
								</span>
							))}
						</div>
					)}
					<div className="flex gap-1">
						{ch.unlock_cron && (
							<TestButton
								label="Unlock"
								pending={testPending}
								onClick={() => onTest("unlock")}
							/>
						)}
						{ch.on_check_complete && (
							<TestButton
								label="Check"
								pending={testPending}
								onClick={() => onTest("check")}
							/>
						)}
						{(ch.platform_alerts ?? []).length > 0 && (
							<TestButton
								label="Alert"
								pending={testPending}
								onClick={() => onTest("platform_alert")}
							/>
						)}
					</div>
					<button
						type="button"
						onClick={() => (isEditing ? setIsEditing(false) : openEdit())}
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
				<div className="space-y-3 border-border border-t px-4 py-3">
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">Name</Label>
						<Input
							value={editName}
							onChange={(e) => setEditName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<label className="flex cursor-pointer select-none items-center gap-2">
						<Checkbox
							checked={editEnabled}
							onCheckedChange={(v) => setEditEnabled(v === true)}
						/>
						<span className="text-muted-foreground text-xs">Enabled</span>
					</label>
					<ReportSettings
						onCheckComplete={editOnCheck}
						setOnCheckComplete={setEditOnCheck}
						unlockCron={editUnlockCron}
						setUnlockCron={setEditUnlockCron}
						platformAlerts={editPlatformAlerts}
						setPlatformAlerts={setEditPlatformAlerts}
					/>
					<div className="flex gap-2">
						<button
							type="button"
							onClick={handleSave}
							disabled={savePending}
							className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
							style={{ background: "var(--color-btn-success)" }}
						>
							{savePending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Save"
							)}
						</button>
						<button
							type="button"
							onClick={() => setIsEditing(false)}
							className="rounded-md border border-border px-3 py-1.5 text-muted-foreground text-sm"
						>
							Cancel
						</button>
					</div>
				</div>
			)}
		</div>
	);
}

function TestButton({
	label,
	pending,
	onClick,
}: {
	label: string;
	pending: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			disabled={pending}
			className="flex items-center gap-1 rounded-md border border-border px-2 py-1 text-muted-foreground text-xs transition-colors hover:bg-white/5 disabled:opacity-50"
		>
			{pending ? (
				<Loader2 size={10} className="animate-spin" />
			) : (
				<FlaskConical size={10} strokeWidth={1.5} />
			)}
			{label}
		</button>
	);
}
