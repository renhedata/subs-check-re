import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { createFileRoute } from "@tanstack/react-router";
import { Loader2, Mail } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import type { settings } from "@/lib/client.gen";
import { useSettings, useUpdateSettings } from "@/queries";

type UserSettings = settings.UserSettings;

export const Route = createFileRoute("/settings/general")({
	component: GeneralSettingsPage,
});

const DEFAULT_SPEED_TEST_URL =
	"https://speed.cloudflare.com/__down?bytes=204800";
const DEFAULT_LATENCY_TEST_URL = "http://www.gstatic.com/generate_204";

function GeneralSettingsPage() {
	const settingsQuery = useSettings();

	const { register, handleSubmit, reset } = useForm<UserSettings>({
		defaultValues: {
			speed_test_url: "",
			upload_test_url: "",
			latency_test_url: "",
			email_config: {
				smtp_host: "",
				smtp_port: 587,
				smtp_user: "",
				smtp_pass: "",
				from: "",
				to: "",
			},
		},
	});

	useEffect(() => {
		if (settingsQuery.data) reset(settingsQuery.data);
	}, [settingsQuery.data, reset]);

	const saveMutation = useUpdateSettings();
	const handleSave = (data: UserSettings) =>
		saveMutation.mutate(data, {
			onSuccess: () => toast.success("Settings saved"),
			onError: () => toast.error("Failed to save settings"),
		});

	return (
		<div className="max-w-lg space-y-5">
			<h1 className="font-semibold text-foreground text-lg">
				General Settings
			</h1>

			<form onSubmit={handleSubmit(handleSave)} className="space-y-5">
				{/* Latency test */}
				<div className="rounded-lg border border-border bg-card p-5">
					<p className="mb-3 font-medium text-foreground text-sm">
						Latency Test
					</p>
					<div className="space-y-1.5">
						<Label
							htmlFor="latency_test_url"
							className="text-muted-foreground text-xs"
						>
							Latency Test URL
						</Label>
						<Input
							id="latency_test_url"
							placeholder={DEFAULT_LATENCY_TEST_URL}
							{...register("latency_test_url")}
							className="h-8 font-mono text-sm"
						/>
						<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
							URL used to measure round-trip latency and check if a node is
							alive. Leave blank to use default ({DEFAULT_LATENCY_TEST_URL}).
						</p>
					</div>
				</div>

				{/* Speed test */}
				<div className="rounded-lg border border-border bg-card p-5">
					<p className="mb-3 font-medium text-foreground text-sm">Speed Test</p>
					<div className="space-y-3">
						<div className="space-y-1.5">
							<Label
								htmlFor="speed_test_url"
								className="text-muted-foreground text-xs"
							>
								↓ Download URL
							</Label>
							<Input
								id="speed_test_url"
								placeholder={DEFAULT_SPEED_TEST_URL}
								{...register("speed_test_url")}
								className="h-8 font-mono text-sm"
							/>
							<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
								Leave blank to use default ({DEFAULT_SPEED_TEST_URL}).
							</p>
						</div>
						<div className="space-y-1.5">
							<Label
								htmlFor="upload_test_url"
								className="text-muted-foreground text-xs"
							>
								↑ Upload URL
							</Label>
							<Input
								id="upload_test_url"
								placeholder="https://speed.cloudflare.com/__up"
								{...register("upload_test_url")}
								className="h-8 font-mono text-sm"
							/>
							<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
								POST endpoint that accepts an upload payload. Leave blank to
								auto-derive from the download URL (replaces path with /__up).
							</p>
						</div>
					</div>
				</div>

				{/* Email / SMTP */}
				<div className="rounded-lg border border-border bg-card p-5">
					<div className="mb-3 flex items-center gap-1.5">
						<Mail
							size={13}
							strokeWidth={1.5}
							className="text-muted-foreground"
						/>
						<p className="font-medium text-foreground text-sm">
							Email Notifications (SMTP)
						</p>
					</div>
					<div className="space-y-3">
						<div className="grid grid-cols-3 gap-3">
							<div className="col-span-2 space-y-1.5">
								<Label className="text-muted-foreground text-xs">
									SMTP Host
								</Label>
								<Input
									placeholder="smtp.gmail.com"
									{...register("email_config.smtp_host")}
									className="h-8 font-mono text-sm"
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-muted-foreground text-xs">Port</Label>
								<Input
									placeholder="587"
									type="number"
									{...register("email_config.smtp_port", {
										valueAsNumber: true,
									})}
									className="h-8 font-mono text-sm"
								/>
							</div>
						</div>
						<div className="grid grid-cols-2 gap-3">
							<div className="space-y-1.5">
								<Label className="text-muted-foreground text-xs">
									Username
								</Label>
								<Input
									placeholder="user@example.com"
									{...register("email_config.smtp_user")}
									className="h-8 text-sm"
								/>
							</div>
							<div className="space-y-1.5">
								<Label className="text-muted-foreground text-xs">
									Password
								</Label>
								<Input
									type="password"
									placeholder="••••••••"
									{...register("email_config.smtp_pass")}
									className="h-8 text-sm"
								/>
							</div>
						</div>
						<div className="space-y-1.5">
							<Label className="text-muted-foreground text-xs">
								From address
							</Label>
							<Input
								placeholder="alerts@example.com"
								{...register("email_config.from")}
								className="h-8 text-sm"
							/>
						</div>
						<div className="space-y-1.5">
							<Label className="text-muted-foreground text-xs">
								To address(es){" "}
								<span className="opacity-60">(comma-separated)</span>
							</Label>
							<Input
								placeholder="you@example.com, team@example.com"
								{...register("email_config.to")}
								className="h-8 text-sm"
							/>
						</div>
						<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
							Port 465 = SSL/TLS · Port 587 = STARTTLS. Leave blank to disable
							email notifications.
						</p>
					</div>
				</div>

				<button
					type="submit"
					disabled={saveMutation.isPending}
					className="flex items-center gap-2 rounded-md px-4 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90 disabled:opacity-50"
					style={{ background: "var(--color-btn-success)" }}
				>
					{saveMutation.isPending ? (
						<Loader2 size={13} className="animate-spin" />
					) : (
						"Save"
					)}
				</button>
			</form>

			<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
				After saving, add an <b>Email</b> channel in{" "}
				<a
					href="/settings/notify"
					className="underline underline-offset-2 hover:text-foreground"
				>
					Notification Channels
				</a>{" "}
				and use its Test button to verify delivery.
			</p>
		</div>
	);
}
