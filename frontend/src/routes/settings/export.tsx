import { createFileRoute } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { isApiError } from "@/lib/client";
import { useAPIKey, useRegenerateAPIKey, useSubscriptions } from "@/queries";

export const Route = createFileRoute("/settings/export")({
	component: ExportSettingsPage,
});

const FORMATS = ["clash", "base64", "routeros"] as const;

function ExportSettingsPage() {
	const apiKeyQuery = useAPIKey();
	const subsQuery = useSubscriptions();
	const regenMut = useRegenerateAPIKey();
	const [confirmOpen, setConfirmOpen] = useState(false);
	const [subId, setSubId] = useState<string>("all");
	const [format, setFormat] = useState<(typeof FORMATS)[number]>("clash");

	const apiKey = apiKeyQuery.data?.api_key ?? "";
	const subs = subsQuery.data?.subscriptions ?? [];
	const origin = typeof window !== "undefined" ? window.location.origin : "";
	const url =
		subId === "all"
			? `${origin}/api/export/all?token=${apiKey}&target=${format}`
			: `${origin}/api/export/${subId}?token=${apiKey}&target=${format}`;

	const handleRegenerate = () =>
		regenMut.mutate(undefined, {
			onSuccess: () => {
				toast.success("API key regenerated — old links stop working");
				setConfirmOpen(false);
			},
			onError: (e) =>
				toast.error(isApiError(e) ? e.message : "Failed to regenerate"),
		});

	return (
		<div className="space-y-4">
			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="font-semibold text-foreground text-sm">API Key</h2>
				<p className="mt-0.5 mb-3 text-muted-foreground text-xs">
					Authenticates export URLs. Regenerating invalidates all existing
					links.
				</p>
				{apiKeyQuery.isLoading ? (
					<Skeleton className="h-8 w-full" />
				) : (
					<div className="flex items-center gap-2">
						<code className="min-w-0 flex-1 truncate rounded-md border border-border bg-background px-3 py-1.5 font-mono text-foreground text-xs">
							{apiKey || "—"}
						</code>
						<CopyButton text={apiKey} />
						<Button
							variant="outline"
							size="sm"
							className="text-danger"
							onClick={() => setConfirmOpen(true)}
						>
							<RefreshCw size={12} /> Regenerate
						</Button>
					</div>
				)}
			</section>

			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="font-semibold text-foreground text-sm">
					Subscription URLs
				</h2>
				<p className="mt-0.5 mb-3 text-muted-foreground text-xs">
					Use these as subscription links in your proxy client.
				</p>
				<div className="flex flex-wrap items-center gap-2">
					<Select value={subId} onValueChange={(v) => v && setSubId(v)}>
						<SelectTrigger className="w-44">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All subscriptions</SelectItem>
							{subs.map((s) => (
								<SelectItem key={s.id} value={s.id}>
									{s.name || s.url}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<Select
						value={format}
						onValueChange={(v) => v && setFormat(v as (typeof FORMATS)[number])}
					>
						<SelectTrigger className="w-28">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{FORMATS.map((f) => (
								<SelectItem key={f} value={f}>
									{f}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<div className="mt-3 flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
					<code className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted-foreground">
						{url}
					</code>
					<CopyButton text={url} />
				</div>
			</section>

			<section className="rounded-lg border border-border bg-card p-4 md:p-5">
				<h2 className="mb-2 font-semibold text-foreground text-sm">
					Parameters
				</h2>
				<table className="w-full text-muted-foreground text-xs">
					<tbody>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">token</td>
							<td>Your API key (required)</td>
						</tr>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">target</td>
							<td>
								<code>clash</code> (default) · <code>base64</code> ·{" "}
								<code>routeros</code>
							</td>
						</tr>
						<tr>
							<td className="py-1 pr-4 font-mono text-primary">list</td>
							<td>
								RouterOS address-list name (default <code>clash_servers</code>)
							</td>
						</tr>
					</tbody>
				</table>
			</section>

			<ConfirmDialog
				open={confirmOpen}
				onOpenChange={setConfirmOpen}
				title="Regenerate API key?"
				description="Every existing export link stops working immediately. Proxy clients using the old key must be updated."
				confirmLabel="Regenerate"
				pending={regenMut.isPending}
				onConfirm={handleRegenerate}
			/>
		</div>
	);
}
