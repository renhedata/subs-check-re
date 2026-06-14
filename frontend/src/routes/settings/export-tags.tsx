import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { settings } from "@/lib/client.gen";
import { useRules, useSettings, useUpdateSettings } from "@/queries";

export const Route = createFileRoute("/settings/export-tags")({
	component: ExportTagsPage,
});

type PlatformTag = settings.PlatformTag;

function ExportTagsPage() {
	const settingsQuery = useSettings();
	const rulesQuery = useRules();
	const updateMut = useUpdateSettings();

	const [showCountry, setShowCountry] = useState(false);
	const [showSpeed, setShowSpeed] = useState(true);
	const [tags, setTags] = useState<Record<string, PlatformTag>>({});

	const loaded = settingsQuery.data;
	const rules = rulesQuery.data?.rules ?? [];

	useEffect(() => {
		if (!loaded) return;
		const cfg = loaded.export_tags;
		setShowCountry(cfg?.show_country ?? false);
		setShowSpeed(cfg?.show_speed ?? true);
		const byKey: Record<string, PlatformTag> = {};
		for (const p of cfg?.platforms ?? []) byKey[p.key] = { ...p };
		for (const r of rules) {
			if (!byKey[r.key]) {
				byKey[r.key] = { key: r.key, label: r.name || r.key, enabled: true };
			}
		}
		setTags(byKey);
	}, [loaded, rules]);

	// Every rule is taggable, in rule order. youtube_premium folds into YouTube's
	// "YT+" modifier at export time, so it has no standalone tag row.
	const tagKeys: string[] = rules
		.map((r) => r.key)
		.filter((k) => k !== "youtube_premium");

	const setTag = (key: string, patch: Partial<PlatformTag>) =>
		setTags((prev) => ({
			...prev,
			[key]: { ...{ key, label: "", enabled: true }, ...prev[key], ...patch },
		}));

	const labelFor = (key: string) =>
		rules.find((r) => r.key === key)?.name ?? key;

	function buildPreview(): string {
		const parts = ["HK-01"];
		if (showCountry) parts.push("HK");
		for (const k of tagKeys.filter((k) => tags[k]?.enabled).slice(0, 3)) {
			parts.push(tags[k]?.label || labelFor(k));
		}
		if (showSpeed) parts.push("10.5MB");
		return parts.join("|");
	}

	function save() {
		if (!loaded) return;
		const platforms: PlatformTag[] = tagKeys
			.map((k) => tags[k])
			.filter(Boolean);
		const next: settings.UserSettings = {
			...loaded,
			export_tags: {
				show_country: showCountry,
				show_speed: showSpeed,
				platforms,
			},
		};
		updateMut.mutate(next, {
			onSuccess: () => toast.success("Export tags saved"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Failed to save"),
		});
	}

	if (settingsQuery.isLoading) {
		return <Skeleton className="h-64 w-full" />;
	}

	return (
		<div className="space-y-4">
			<p className="text-muted-foreground text-xs">
				Tags appended to node names in every export, e.g.{" "}
				<code className="rounded bg-secondary px-1 font-mono">
					{buildPreview()}
				</code>
			</p>

			<section className="space-y-3 rounded-lg border border-border bg-card p-4">
				<div className="flex items-center justify-between gap-3 text-sm">
					<span>Detected country</span>
					<Switch
						checked={showCountry}
						onCheckedChange={(v) => setShowCountry(v === true)}
					/>
				</div>
				<div className="flex items-center justify-between gap-3 text-sm">
					<span>Speed</span>
					<Switch
						checked={showSpeed}
						onCheckedChange={(v) => setShowSpeed(v === true)}
					/>
				</div>
			</section>

			<section className="rounded-lg border border-border bg-card p-4">
				<p className="mb-3 font-medium text-foreground text-sm">Platforms</p>
				<div className="space-y-2">
					{tagKeys.map((key) => {
						const t = tags[key];
						return (
							<div key={key} className="flex items-center gap-3">
								<Switch
									checked={t?.enabled ?? true}
									onCheckedChange={(v) => setTag(key, { enabled: v === true })}
								/>
								<Label className="w-28 shrink-0 truncate text-xs">
									{labelFor(key)}
								</Label>
								<Input
									value={t?.label ?? ""}
									placeholder={key}
									onChange={(e) => setTag(key, { label: e.target.value })}
									className="h-7 max-w-40 text-xs"
								/>
							</div>
						);
					})}
				</div>
			</section>

			<div className="flex justify-end">
				<Button variant="success" loading={updateMut.isPending} onClick={save}>
					Save export tags
				</Button>
			</div>
		</div>
	);
}
