import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { client } from "@/lib/client";
import type { settings } from "@/lib/client.gen";

type UserSettings = settings.UserSettings;

export const Route = createFileRoute("/settings/general")({
	component: GeneralSettingsPage,
});

const DEFAULT_SPEED_TEST_URL =
	"https://speed.cloudflare.com/__down?bytes=204800";

function GeneralSettingsPage() {
	const qc = useQueryClient();

	const settingsQuery = useQuery({
		queryKey: ["settings"],
		queryFn: () => client.settings.GetSettings(),
	});

	const { register, handleSubmit, reset } = useForm<UserSettings>({
		defaultValues: { speed_test_url: "" },
	});

	useEffect(() => {
		if (settingsQuery.data) reset(settingsQuery.data);
	}, [settingsQuery.data, reset]);

	const saveMutation = useMutation({
		mutationFn: (data: UserSettings) => client.settings.UpdateSettings(data),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["settings"] });
			toast.success("Settings saved");
		},
		onError: () => toast.error("Failed to save settings"),
	});

	return (
		<div className="max-w-lg space-y-5">
			<h1 className="font-semibold text-foreground text-lg">
				General Settings
			</h1>

			<div className="rounded-lg border border-border bg-card p-5">
				<form
					onSubmit={handleSubmit((d) => saveMutation.mutate(d))}
					className="space-y-4"
				>
					<div className="space-y-1.5">
						<Label
							htmlFor="speed_test_url"
							className="text-muted-foreground text-xs"
						>
							Speed Test URL
						</Label>
						<Input
							id="speed_test_url"
							placeholder={DEFAULT_SPEED_TEST_URL}
							{...register("speed_test_url")}
							className="h-8 font-mono text-sm"
						/>
						<p
							className="text-xs"
							style={{ color: "var(--color-dimmed)" }}
						>
							URL used to measure download speed. Leave blank to use default.
						</p>
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
			</div>
		</div>
	);
}
