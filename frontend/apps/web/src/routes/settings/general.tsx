import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { api, type UserSettings } from "@/lib/api";

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
		if (settingsQuery.data) reset(settingsQuery.data);
	}, [settingsQuery.data, reset]);

	const saveMutation = useMutation({
		mutationFn: (data: UserSettings) =>
			api.put<UserSettings>("/settings", data),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["settings"] });
			toast.success("Settings saved");
		},
		onError: () => toast.error("Failed to save settings"),
	});

	return (
		<div className="max-w-lg space-y-5">
			<h1 className="font-semibold text-[#f0f6fc] text-lg">General Settings</h1>

			<div
				className="rounded-lg border p-5"
				style={{ background: "#161b22", borderColor: "#30363d" }}
			>
				<form
					onSubmit={handleSubmit((d) => saveMutation.mutate(d))}
					className="space-y-4"
				>
					<div className="space-y-1.5">
						<Label htmlFor="speed_test_url" className="text-[#8b949e] text-xs">
							Speed Test URL
						</Label>
						<Input
							id="speed_test_url"
							placeholder={DEFAULT_SPEED_TEST_URL}
							{...register("speed_test_url")}
							className="h-8 font-mono text-sm"
						/>
						<p className="text-xs" style={{ color: "#6e7681" }}>
							URL used to measure download speed. Leave blank to use default.
						</p>
					</div>

					<button
						type="submit"
						disabled={saveMutation.isPending}
						className="flex items-center gap-2 rounded-md px-4 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90 disabled:opacity-50"
						style={{ background: "#238636" }}
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
