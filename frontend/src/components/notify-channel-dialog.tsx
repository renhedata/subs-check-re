import { useEffect, useState } from "react";
import { toast } from "sonner";
import { CronPicker } from "@/components/cron-picker";
import { RulePlatformIcon } from "@/components/rule-icon";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { MEDIA_APPS } from "@/lib/checkOptions";
import { isApiError } from "@/lib/client";
import type { JSONValue, notify } from "@/lib/client.gen";
import { cn } from "@/lib/utils";
import { useCreateNotifyChannel, useUpdateNotifyChannel } from "@/queries";

// notify.Channel is the generated type name (not notify.NotifyChannel)
type NotifyChannel = notify.Channel;

const TYPES = [
	{ value: "webhook", label: "Webhook" },
	{ value: "telegram", label: "Telegram" },
	{ value: "email", label: "Email" },
] as const;

// Config JSON keys must match services/notify/senders.go:
// webhook {url, method, headers} · telegram {bot_token, chat_id} · email {to_email}
interface ConfigState {
	url: string;
	bot_token: string;
	chat_id: string;
	to_email: string;
}

const EMPTY_CONFIG: ConfigState = {
	url: "",
	bot_token: "",
	chat_id: "",
	to_email: "",
};

function configFromChannel(ch: NotifyChannel | null | undefined): ConfigState {
	if (!ch) return { ...EMPTY_CONFIG };
	const cfg = (ch.config ?? {}) as Record<string, unknown>;
	return {
		url: typeof cfg.url === "string" ? cfg.url : "",
		bot_token: typeof cfg.bot_token === "string" ? cfg.bot_token : "",
		chat_id: typeof cfg.chat_id === "string" ? cfg.chat_id : "",
		to_email: typeof cfg.to_email === "string" ? cfg.to_email : "",
	};
}

function buildConfig(type: string, c: ConfigState): Record<string, unknown> {
	switch (type) {
		case "webhook":
			return { url: c.url, method: "POST", headers: {} };
		case "telegram":
			return { bot_token: c.bot_token, chat_id: c.chat_id };
		case "email":
			return { to_email: c.to_email };
		default:
			return {};
	}
}

export function NotifyChannelDialog({
	open,
	onOpenChange,
	editing,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	editing?: NotifyChannel | null;
}) {
	const [name, setName] = useState("");
	const [type, setType] = useState<string>("webhook");
	const [config, setConfig] = useState<ConfigState>(EMPTY_CONFIG);
	const [onCheckComplete, setOnCheckComplete] = useState(true);
	const [unlockCron, setUnlockCron] = useState("");
	const [platformAlerts, setPlatformAlerts] = useState<string[]>([]);

	useEffect(() => {
		if (open) {
			setName(editing?.name ?? "");
			setType(editing?.type ?? "webhook");
			setConfig(configFromChannel(editing));
			setOnCheckComplete(editing?.on_check_complete ?? true);
			setUnlockCron(editing?.unlock_cron ?? "");
			setPlatformAlerts(editing?.platform_alerts ?? []);
		}
	}, [open, editing]);

	const createMut = useCreateNotifyChannel();
	const updateMut = useUpdateNotifyChannel();
	const pending = createMut.isPending || updateMut.isPending;

	const configValid =
		(type === "webhook" && config.url.startsWith("http")) ||
		(type === "telegram" && !!config.bot_token && !!config.chat_id) ||
		(type === "email" && config.to_email.includes("@"));

	function submit() {
		const common = {
			name,
			config: buildConfig(type, config) as JSONValue,
			on_check_complete: onCheckComplete,
			unlock_cron: unlockCron,
			platform_alerts: platformAlerts,
		};
		const onError = (e: unknown) =>
			toast.error(isApiError(e) ? e.message : "Failed to save channel");
		if (editing) {
			updateMut.mutate(
				{
					id: editing.id,
					params: { ...common, enabled: editing.enabled },
				},
				{
					onSuccess: () => {
						toast.success("Channel updated");
						onOpenChange(false);
					},
					onError,
				},
			);
		} else {
			createMut.mutate(
				{ ...common, type },
				{
					onSuccess: () => {
						toast.success("Channel created");
						onOpenChange(false);
					},
					onError,
				},
			);
		}
	}

	const togglePlatform = (app: string) =>
		setPlatformAlerts((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				<DialogTitle>{editing ? "Edit channel" : "Add channel"}</DialogTitle>
				<DialogDescription>
					Where and when to send check reports.
				</DialogDescription>

				<div className="mt-4 space-y-4">
					<div className="grid gap-3 sm:grid-cols-2">
						<div className="space-y-1.5">
							<Label className="text-xs">Name</Label>
							<Input
								value={name}
								placeholder="My alerts"
								onChange={(e) => setName(e.target.value)}
							/>
						</div>
						<div className="space-y-1.5">
							<Label className="text-xs">Type</Label>
							<Select
								value={type}
								onValueChange={(v) => v && setType(v)}
								disabled={!!editing}
							>
								<SelectTrigger className="w-full">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{TYPES.map((t) => (
										<SelectItem key={t.value} value={t.value}>
											{t.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
					</div>

					{type === "webhook" ? (
						<div className="space-y-1.5">
							<Label className="text-xs">Webhook URL</Label>
							<Input
								value={config.url}
								className="font-mono"
								placeholder="https://…"
								onChange={(e) => setConfig({ ...config, url: e.target.value })}
							/>
						</div>
					) : null}
					{type === "telegram" ? (
						<div className="grid gap-3 sm:grid-cols-2">
							<div className="space-y-1.5">
								<Label className="text-xs">Bot token</Label>
								<Input
									value={config.bot_token}
									className="font-mono"
									onChange={(e) =>
										setConfig({ ...config, bot_token: e.target.value })
									}
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-xs">Chat ID</Label>
								<Input
									value={config.chat_id}
									className="font-mono"
									onChange={(e) =>
										setConfig({ ...config, chat_id: e.target.value })
									}
								/>
							</div>
						</div>
					) : null}
					{type === "email" ? (
						<div className="space-y-1.5">
							<Label className="text-xs">Recipients</Label>
							<Input
								value={config.to_email}
								placeholder="a@example.com, b@example.com"
								onChange={(e) =>
									setConfig({ ...config, to_email: e.target.value })
								}
							/>
							<p className="text-muted-foreground text-xs">
								SMTP server is configured in Settings → General.
							</p>
						</div>
					) : null}

					<label className="flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={onCheckComplete}
							onCheckedChange={(v) => setOnCheckComplete(v === true)}
						/>
						Notify when a check completes
					</label>

					<div className="space-y-1.5">
						<Label className="text-xs">
							Platform alerts{" "}
							<span className="text-muted-foreground">
								(alert when a platform loses all unlocked nodes)
							</span>
						</Label>
						<div className="flex flex-wrap gap-1.5">
							{MEDIA_APPS.map((app) => {
								const active = platformAlerts.includes(app);
								return (
									<button
										key={app}
										type="button"
										aria-pressed={active}
										onClick={() => togglePlatform(app)}
										className={cn(
											"inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors",
											active
												? "border-info-line bg-info-muted text-info"
												: "border-border text-muted-foreground hover:bg-secondary",
										)}
									>
										<RulePlatformIcon platformKey={app} size={12} showLabel />
									</button>
								);
							})}
						</div>
					</div>

					<div className="space-y-1.5">
						<Label className="text-xs">
							Scheduled unlock report{" "}
							<span className="text-muted-foreground">(optional)</span>
						</Label>
						<CronPicker
							value={unlockCron}
							onChange={setUnlockCron}
							allowDisable
						/>
					</div>
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={pending}
						disabled={!name || !configValid}
						onClick={submit}
					>
						{editing ? "Save" : "Create"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
