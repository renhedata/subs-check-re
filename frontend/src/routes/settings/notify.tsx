import { createFileRoute } from "@tanstack/react-router";
import { BellOff, Plus, Send } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { NotifyChannelDialog } from "@/components/notify-channel-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { notify } from "@/lib/client.gen";
import {
	useDeleteNotifyChannel,
	useNotifyChannels,
	useTestNotifyChannel,
	useUpdateNotifyChannel,
} from "@/queries";

// notify.Channel is the generated type name
type NotifyChannel = notify.Channel;

export const Route = createFileRoute("/settings/notify")({
	component: NotifySettingsPage,
});

function NotifySettingsPage() {
	const [dialogOpen, setDialogOpen] = useState(false);
	const [editing, setEditing] = useState<NotifyChannel | null>(null);
	const [deleting, setDeleting] = useState<NotifyChannel | null>(null);

	const channelsQuery = useNotifyChannels();
	const channels = channelsQuery.data?.channels ?? [];

	const updateMut = useUpdateNotifyChannel();
	const deleteMut = useDeleteNotifyChannel();
	const testMut = useTestNotifyChannel();

	const handleToggle = (ch: NotifyChannel, enabled: boolean) =>
		updateMut.mutate(
			{
				id: ch.id,
				params: {
					name: ch.name,
					config: ch.config,
					enabled,
					on_check_complete: ch.on_check_complete,
					unlock_cron: ch.unlock_cron,
					platform_alerts: ch.platform_alerts,
				},
			},
			{
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to update"),
			},
		);

	const handleTest = (ch: NotifyChannel) =>
		testMut.mutate(
			{ id: ch.id, params: { report_type: "check" } },
			{
				onSuccess: () => toast.success(`Test sent to "${ch.name}"`),
				onError: (e) => toast.error(isApiError(e) ? e.message : "Test failed"),
			},
		);

	const handleDelete = () => {
		if (!deleting) return;
		deleteMut.mutate(deleting.id, {
			onSuccess: () => {
				toast.success("Channel deleted");
				setDeleting(null);
			},
			onError: (e) => toast.error(isApiError(e) ? e.message : "Delete failed"),
		});
	};

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<p className="text-muted-foreground text-xs">
					Channels receive check reports, platform alerts and scheduled unlock
					summaries.
				</p>
				<Button
					variant="success"
					size="sm"
					onClick={() => {
						setEditing(null);
						setDialogOpen(true);
					}}
				>
					<Plus size={13} /> Add channel
				</Button>
			</div>

			{channelsQuery.isLoading ? (
				<div className="space-y-2">
					<Skeleton className="h-14 w-full" />
					<Skeleton className="h-14 w-full" />
				</div>
			) : channels.length === 0 ? (
				<div className="rounded-lg border border-border">
					<EmptyState
						icon={BellOff}
						title="No channels yet"
						description="Add a webhook, Telegram bot or email recipient to get notified after checks."
						action={
							<Button variant="success" onClick={() => setDialogOpen(true)}>
								Add channel
							</Button>
						}
					/>
				</div>
			) : (
				<div className="space-y-2">
					{channels.map((ch) => (
						<div
							key={ch.id}
							className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-card px-4 py-3"
						>
							<Switch
								checked={ch.enabled}
								onCheckedChange={(v) => handleToggle(ch, v === true)}
								disabled={updateMut.isPending}
							/>
							<button
								type="button"
								onClick={() => {
									setEditing(ch);
									setDialogOpen(true);
								}}
								className="min-w-0 flex-1 text-left"
							>
								<span className="block truncate font-medium text-foreground text-sm hover:underline">
									{ch.name}
								</span>
								<span className="text-muted-foreground text-xs">
									{ch.on_check_complete ? "check reports" : ""}
									{ch.platform_alerts?.length
										? ` · ${ch.platform_alerts.length} platform alerts`
										: ""}
									{ch.unlock_cron ? " · scheduled report" : ""}
								</span>
							</button>
							<Badge tone="info">{ch.type}</Badge>
							<Button
								variant="outline"
								size="sm"
								loading={testMut.isPending && testMut.variables?.id === ch.id}
								onClick={() => handleTest(ch)}
							>
								<Send size={12} /> Test
							</Button>
							<Button
								variant="ghost"
								size="sm"
								className="text-danger"
								onClick={() => setDeleting(ch)}
							>
								Delete
							</Button>
						</div>
					))}
				</div>
			)}

			<NotifyChannelDialog
				open={dialogOpen}
				onOpenChange={setDialogOpen}
				editing={editing}
			/>
			<ConfirmDialog
				open={!!deleting}
				onOpenChange={(o) => !o && setDeleting(null)}
				title={`Delete channel "${deleting?.name ?? ""}"?`}
				description="This channel stops receiving all notifications. This cannot be undone."
				pending={deleteMut.isPending}
				onConfirm={handleDelete}
			/>
		</div>
	);
}
