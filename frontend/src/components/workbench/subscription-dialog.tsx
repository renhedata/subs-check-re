import { useEffect, useState } from "react";
import { toast } from "sonner";
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
import { isApiError } from "@/lib/client";
import type { subscription } from "@/lib/client.gen";
import { useCreateSubscription, useUpdateSubscription } from "@/queries";

type Subscription = subscription.Subscription;

function isValidHttpUrl(value: string): boolean {
	try {
		const u = new URL(value);
		return u.protocol === "http:" || u.protocol === "https:";
	} catch {
		return false;
	}
}

// One dialog for both create (sub == null) and edit (sub set).
export function SubscriptionDialog({
	open,
	onOpenChange,
	sub,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	sub?: Subscription | null;
}) {
	const editing = !!sub;
	const [name, setName] = useState("");
	const [url, setUrl] = useState("");
	const [enabled, setEnabled] = useState(true);
	const [urlError, setUrlError] = useState<string | null>(null);

	// Re-seed form whenever the dialog opens for a different target.
	useEffect(() => {
		if (open) {
			setName(sub?.name ?? "");
			setUrl(sub?.url ?? "");
			setEnabled(sub?.enabled ?? true);
			setUrlError(null);
		}
	}, [open, sub]);

	const createMut = useCreateSubscription();
	const updateMut = useUpdateSubscription();
	const pending = createMut.isPending || updateMut.isPending;

	function submit() {
		if (!isValidHttpUrl(url)) {
			setUrlError("Must be a valid http(s) URL");
			return;
		}
		setUrlError(null);
		const onError = (e: unknown) =>
			toast.error(isApiError(e) ? e.message : "Request failed");
		if (editing && sub) {
			updateMut.mutate(
				{
					id: sub.id,
					params: {
						name,
						url,
						enabled,
						cron_expr: sub.cron_expr ?? "",
						clear_cron_expr: false,
					},
				},
				{
					onSuccess: () => {
						toast.success("Subscription updated");
						onOpenChange(false);
					},
					onError,
				},
			);
		} else {
			createMut.mutate(
				{ name, url, cron_expr: "" },
				{
					onSuccess: () => {
						toast.success("Subscription added");
						onOpenChange(false);
					},
					onError,
				},
			);
		}
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogTitle>
					{editing ? "Edit subscription" : "Add subscription"}
				</DialogTitle>
				<DialogDescription>
					Paste a Clash/V2Ray subscription URL.
				</DialogDescription>

				<div className="mt-4 space-y-3">
					<div className="space-y-1.5">
						<Label htmlFor="sub-name" className="text-xs">
							Name <span className="text-muted-foreground">(optional)</span>
						</Label>
						<Input
							id="sub-name"
							value={name}
							placeholder="My provider"
							onChange={(e) => setName(e.target.value)}
						/>
					</div>
					<div className="space-y-1.5">
						<Label htmlFor="sub-url" className="text-xs">
							URL
						</Label>
						<Input
							id="sub-url"
							value={url}
							placeholder="https://…"
							className="font-mono"
							aria-invalid={!!urlError}
							onChange={(e) => {
								setUrl(e.target.value);
								if (urlError) setUrlError(null);
							}}
						/>
						{urlError ? (
							<p className="text-danger text-xs">⚠ {urlError}</p>
						) : null}
					</div>
					{editing ? (
						<label className="flex cursor-pointer items-center gap-2 text-sm">
							<Checkbox
								checked={enabled}
								onCheckedChange={(v) => setEnabled(v === true)}
							/>
							Enabled
						</label>
					) : null}
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Cancel
					</Button>
					<Button
						variant="success"
						loading={pending}
						disabled={!url}
						onClick={submit}
					>
						{editing ? "Save" : "Add"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
