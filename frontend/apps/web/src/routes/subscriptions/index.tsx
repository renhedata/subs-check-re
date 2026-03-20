import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Skeleton } from "@frontend/ui/components/skeleton";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Clock, Loader2, Pencil, Play, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { client, isApiError } from "@/lib/client";
import type { subscription } from "@/lib/client.gen";
import { PlatformIcon } from "@/components/platform-icons";
import type { PlatformKey } from "@/components/platform-icons";

type Subscription = subscription.Subscription;

const MEDIA_APPS = [
	"openai",
	"claude",
	"gemini",
	"grok",
	"netflix",
	"youtube",
	"disney",
	"tiktok",
] as const;

export const Route = createFileRoute("/subscriptions/")({
	component: SubscriptionsPage,
});

function SubscriptionsPage() {
	const navigate = useNavigate();
	const qc = useQueryClient();
	const [name, setName] = useState("");
	const [url, setUrl] = useState("");
	const [adding, setAdding] = useState(false);

	const { data, isLoading } = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => client.subscription.List(),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => client.subscription.Delete(id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["subscriptions"] });
			toast.success("Deleted");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Delete failed"),
	});

	const updateMut = useMutation({
		mutationFn: ({
			id,
			data,
		}: {
			id: string;
			data: { name?: string; url?: string };
		}) => {
			const current = subs.find((s) => s.id === id);
			return client.subscription.Update(id, {
				name: data.name ?? current?.name ?? "",
				url: data.url ?? current?.url ?? "",
				enabled: current?.enabled ?? true,
				cron_expr: current?.cron_expr ?? "",
				clear_cron_expr: false,
			});
		},
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["subscriptions"] });
			toast.success("Updated");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Update failed"),
	});

	const triggerMut = useMutation({
		mutationFn: ({
			id,
			opts,
		}: {
			id: string;
			opts: { speed_test: boolean; media_apps: string[] };
		}) => client.checker.TriggerCheck(id, opts),
		onSuccess: (resp, { id }) => {
			toast.success("Check started");
			navigate({
				to: "/subscriptions/$id",
				params: { id },
				search: { job: resp.job_id },
			});
		},
		onError: (e) =>
			toast.error(isApiError(e) ? e.message : "Failed to start check"),
	});

	const createMut = useMutation({
		mutationFn: () => client.subscription.Create({ name, url, cron_expr: "" }),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["subscriptions"] });
			setName("");
			setUrl("");
			setAdding(false);
			toast.success("Subscription added");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Failed to add"),
	});

	const subs = data?.subscriptions ?? [];

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-foreground text-lg">Subscriptions</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90"
					style={{ background: "var(--color-btn-success)" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add
				</button>
			</div>

			{adding && (
				<div className="space-y-3 rounded-lg border border-border bg-card p-4">
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">
							Name (optional)
						</Label>
						<Input
							placeholder="My Sub"
							value={name}
							onChange={(e) => setName(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="space-y-1.5">
						<Label className="text-muted-foreground text-xs">
							Subscription URL
						</Label>
						<Input
							placeholder="https://..."
							value={url}
							onChange={(e) => setUrl(e.target.value)}
							className="h-8 text-sm"
						/>
					</div>
					<div className="flex gap-2">
						<Button
							size="sm"
							onClick={() => createMut.mutate()}
							disabled={!url || createMut.isPending}
							style={{ background: "var(--color-btn-success)", color: "#fff" }}
							className="border-0"
						>
							{createMut.isPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Add"
							)}
						</Button>
						<Button
							size="sm"
							variant="outline"
							onClick={() => setAdding(false)}
						>
							Cancel
						</Button>
					</div>
				</div>
			)}

			<div className="space-y-2">
				{isLoading
					? Array.from({ length: 3 }).map((_, i) => (
							<div
								key={i}
								className="rounded-lg border border-border bg-card p-4"
							>
								<Skeleton className="mb-2 h-4 w-48" />
								<Skeleton className="h-3 w-72" />
							</div>
						))
					: subs.map((sub) => (
							<SubRow
								key={sub.id}
								sub={sub}
								deleteMut={deleteMut}
								updateMut={updateMut}
								triggerMut={triggerMut}
							/>
						))}

				{!isLoading && subs.length === 0 && (
					<p className="py-10 text-center text-sm text-muted-foreground">
						No subscriptions yet. Add one above.
					</p>
				)}
			</div>
		</div>
	);
}

function SubRow({
	sub,
	deleteMut,
	updateMut,
	triggerMut,
}: {
	sub: Subscription;
	deleteMut: { mutate: (id: string) => void; isPending: boolean };
	updateMut: {
		mutate: (args: {
			id: string;
			data: { name?: string; url?: string };
		}) => void;
		isPending: boolean;
	};
	triggerMut: {
		mutate: (args: {
			id: string;
			opts: { speed_test: boolean; media_apps: string[] };
		}) => void;
		isPending: boolean;
	};
}) {
	const [showOpts, setShowOpts] = useState(false);
	const [showEdit, setShowEdit] = useState(false);
	const [editName, setEditName] = useState(sub.name);
	const [editUrl, setEditUrl] = useState(sub.url);

	function handleSaveEdit() {
		updateMut.mutate({
			id: sub.id,
			data: {
				name: editName || undefined,
				url: editUrl || undefined,
			},
		});
		setShowEdit(false);
	}

	const [speedTest, setSpeedTest] = useState(true);
	const [mediaApps, setMediaApps] = useState<string[]>([...MEDIA_APPS]);

	function toggleApp(app: string) {
		setMediaApps((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);
	}

	function handleCheck() {
		triggerMut.mutate({
			id: sub.id,
			opts: { speed_test: speedTest, media_apps: mediaApps },
		});
		setShowOpts(false);
	}

	return (
		<div className="rounded-lg border border-border bg-card">
			<div className="flex items-center gap-3 px-4 py-3">
				{/* Info — entire left section navigates to detail */}
				<Link
					to="/subscriptions/$id"
					params={{ id: sub.id }}
					className="flex min-w-0 flex-1 items-center gap-3"
				>
					{/* Status dot */}
					<div
						className="h-2 w-2 flex-shrink-0 rounded-full"
						style={{
							background: sub.last_run_at
								? "var(--color-success)"
								: "var(--border)",
						}}
					/>
					<div className="min-w-0 flex-1">
						<p
							className="font-medium text-sm hover:underline"
							style={{ color: "var(--primary)" }}
						>
							{sub.name || sub.url}
						</p>
						{sub.name && (
							<p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
								{sub.url}
							</p>
						)}
						{sub.cron_expr && (
							<p
								className="mt-0.5 flex items-center gap-1 text-xs"
								style={{ color: "var(--color-dimmed)" }}
							>
								<Clock size={10} strokeWidth={1.5} />
								{sub.cron_expr}
							</p>
						)}
					</div>
				</Link>
				{/* Actions */}
				<div className="flex flex-shrink-0 items-center gap-2">
					<button
						type="button"
						onClick={() => setShowOpts(!showOpts)}
						className="flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-white/5"
					>
						<Play size={11} strokeWidth={1.5} />
						Check
					</button>
					<button
						type="button"
						onClick={() => {
							setEditName(sub.name);
							setEditUrl(sub.url);
							setShowEdit(!showEdit);
							setShowOpts(false);
						}}
						className="rounded-md p-1.5 transition-colors hover:bg-white/5"
						style={{ color: "var(--color-dimmed)" }}
					>
						<Pencil size={13} strokeWidth={1.5} />
					</button>
					<button
						type="button"
						onClick={() => deleteMut.mutate(sub.id)}
						disabled={deleteMut.isPending}
						className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
						style={{ color: "var(--color-dimmed)" }}
					>
						<Trash2 size={13} strokeWidth={1.5} />
					</button>
				</div>
			</div>

			{showOpts && (
				<div className="space-y-3 border-t border-border px-4 py-3">
					<label className="flex cursor-pointer select-none items-center gap-2">
						<input
							type="checkbox"
							checked={speedTest}
							onChange={(e) => setSpeedTest(e.target.checked)}
						/>
						<span className="text-xs text-muted-foreground">Speed test</span>
					</label>
					<div className="flex flex-wrap gap-2">
						{MEDIA_APPS.map((app) => (
							<label
								key={app}
								className="flex cursor-pointer select-none items-center gap-1"
							>
								<input
									type="checkbox"
									checked={mediaApps.includes(app)}
									onChange={() => toggleApp(app)}
								/>
								<PlatformIcon platform={app as PlatformKey} size={13} showLabel />
							</label>
						))}
					</div>
					<div className="flex gap-2">
						<button
							type="button"
							onClick={handleCheck}
							disabled={triggerMut.isPending}
							className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
							style={{ background: "var(--color-btn-success)" }}
						>
							{triggerMut.isPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Start"
							)}
						</button>
						<button
							type="button"
							onClick={() => setShowOpts(false)}
							className="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground"
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			{showEdit && (
				<div className="space-y-3 border-t border-border px-4 py-3">
					<div className="grid gap-3 sm:grid-cols-2">
						<div className="space-y-1.5">
							<Label className="text-muted-foreground text-xs">Name</Label>
							<Input
								value={editName}
								onChange={(e) => setEditName(e.target.value)}
								placeholder="My Subscription"
								className="h-8 text-sm"
							/>
						</div>
						<div className="space-y-1.5">
							<Label className="text-muted-foreground text-xs">URL</Label>
							<Input
								value={editUrl}
								onChange={(e) => setEditUrl(e.target.value)}
								placeholder="https://..."
								className="h-8 font-mono text-sm"
							/>
						</div>
					</div>
					<div className="flex gap-2">
						<button
							type="button"
							onClick={handleSaveEdit}
							disabled={updateMut.isPending}
							className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
							style={{ background: "var(--color-btn-success)" }}
						>
							{updateMut.isPending ? (
								<Loader2 size={13} className="animate-spin" />
							) : (
								"Save"
							)}
						</button>
						<button
							type="button"
							onClick={() => setShowEdit(false)}
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
