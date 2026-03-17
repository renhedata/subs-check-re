import { Button } from "@frontend/ui/components/button";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@frontend/ui/components/select";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
	ChevronRight,
	Clock,
	History,
	Loader2,
	Pencil,
	Plus,
	Trash2,
	Zap,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { client, isApiError } from "@/lib/client";
import type { checker, scheduler, subscription } from "@/lib/client.gen";

type CheckJob = checker.JobSummary;
type ScheduledJob = scheduler.ScheduledJob;
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

const SCHEDULE_PRESETS = [
	{ label: "1h", cron: "0 * * * *", desc: "Every hour" },
	{ label: "2h", cron: "0 */2 * * *", desc: "Every 2 hours" },
	{ label: "6h", cron: "0 */6 * * *", desc: "Every 6 hours" },
	{ label: "12h", cron: "0 */12 * * *", desc: "Every 12 hours" },
	{ label: "Daily", cron: "0 0 * * *", desc: "Once a day" },
	{ label: "Weekly", cron: "0 0 * * 0", desc: "Once a week" },
] as const;

function cronToLabel(cron: string): string {
	const preset = SCHEDULE_PRESETS.find((p) => p.cron === cron);
	return preset ? preset.desc : cron;
}

function statusColor(status: CheckJob["status"]): string {
	if (status === "completed") return "#3fb950";
	if (status === "failed") return "#f85149";
	return "#58a6ff";
}

export const Route = createFileRoute("/scheduler")({
	component: SchedulerPage,
});

function SchedulerPage() {
	const qc = useQueryClient();
	const [adding, setAdding] = useState(false);
	const [subId, setSubId] = useState("");
	const [selectedCron, setSelectedCron] = useState("");
	const [speedTest, setSpeedTest] = useState(true);
	const [mediaApps, setMediaApps] = useState<string[]>([...MEDIA_APPS]);

	const jobsQuery = useQuery({
		queryKey: ["scheduler"],
		queryFn: () => client.scheduler.List(),
	});

	const subsQuery = useQuery({
		queryKey: ["subscriptions"],
		queryFn: () => client.subscription.List(),
	});

	const createMut = useMutation({
		mutationFn: (params: {
			subscription_id: string;
			cron_expr: string;
			speed_test: boolean;
			media_apps: string[];
		}) =>
			client.scheduler.Create({
				subscription_id: params.subscription_id,
				cron_expr: params.cron_expr,
				options: {
					speed_test: params.speed_test,
					media_apps: params.media_apps,
				},
			}),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			setAdding(false);
			setSubId("");
			setSelectedCron("");
			setSpeedTest(true);
			setMediaApps([...MEDIA_APPS]);
			toast.success("Schedule created");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Failed"),
	});

	const deleteMut = useMutation({
		mutationFn: (id: string) => client.scheduler.Delete(id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["scheduler"] });
			toast.success("Removed");
		},
		onError: (e) => toast.error(isApiError(e) ? e.message : "Failed"),
	});

	const jobs = jobsQuery.data?.jobs ?? [];
	const subs = subsQuery.data?.subscriptions ?? [];

	function subName(id: string) {
		const s = subs.find((s) => s.id === id);
		return s ? s.name || s.url : `${id.slice(0, 8)}…`;
	}

	function toggleApp(app: string) {
		setMediaApps((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);
	}

	return (
		<div className="space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-[#f0f6fc] text-lg">Scheduler</h1>
				<button
					type="button"
					onClick={() => setAdding(!adding)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 font-medium text-sm text-white transition-opacity hover:opacity-90"
					style={{ background: "#238636" }}
				>
					<Plus size={13} strokeWidth={1.5} />
					Add Schedule
				</button>
			</div>

			{adding && (
				<ScheduleForm
					subs={subs}
					subId={subId}
					setSubId={setSubId}
					selectedCron={selectedCron}
					setSelectedCron={setSelectedCron}
					speedTest={speedTest}
					setSpeedTest={setSpeedTest}
					mediaApps={mediaApps}
					toggleApp={toggleApp}
					isPending={createMut.isPending}
					onSave={() =>
						createMut.mutate({
							subscription_id: subId,
							cron_expr: selectedCron,
							speed_test: speedTest,
							media_apps: mediaApps,
						})
					}
					onCancel={() => setAdding(false)}
				/>
			)}

			<div className="space-y-2">
				{jobs.map((job) => (
					<JobRow
						key={job.id}
						job={job}
						subName={subName(job.subscription_id)}
						subs={subs}
						onDelete={() => deleteMut.mutate(job.id)}
						deleting={deleteMut.isPending}
						onSaveEdit={(cron, st, apps) =>
							createMut.mutate({
								subscription_id: job.subscription_id,
								cron_expr: cron,
								speed_test: st,
								media_apps: apps,
							})
						}
						editPending={createMut.isPending}
					/>
				))}
				{!jobsQuery.isLoading && jobs.length === 0 && (
					<p className="py-10 text-center text-sm" style={{ color: "#8b949e" }}>
						No scheduled jobs.
					</p>
				)}
			</div>
		</div>
	);
}

// --- Shared schedule form (used for create and edit) ---

function ScheduleForm({
	subs,
	subId,
	setSubId,
	selectedCron,
	setSelectedCron,
	speedTest,
	setSpeedTest,
	mediaApps,
	toggleApp,
	isPending,
	onSave,
	onCancel,
	hideSubSelector,
}: {
	subs: Subscription[];
	subId: string;
	setSubId: (v: string) => void;
	selectedCron: string;
	setSelectedCron: (v: string) => void;
	speedTest: boolean;
	setSpeedTest: (v: boolean) => void;
	mediaApps: string[];
	toggleApp: (app: string) => void;
	isPending: boolean;
	onSave: () => void;
	onCancel: () => void;
	hideSubSelector?: boolean;
}) {
	return (
		<div
			className="space-y-4 rounded-lg border p-4"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			{!hideSubSelector && (
				<div className="space-y-1.5">
					<p className="text-[#8b949e] text-xs">Subscription</p>
					<Select value={subId} onValueChange={(v) => setSubId(v ?? "")}>
						<SelectTrigger className="h-8 text-sm">
							<SelectValue placeholder="Select subscription…" />
						</SelectTrigger>
						<SelectContent>
							{subs.map((s) => (
								<SelectItem key={s.id} value={s.id}>
									{s.name || s.url}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
			)}

			{/* Schedule presets */}
			<div className="space-y-1.5">
				<p className="text-[#8b949e] text-xs">Schedule</p>
				<div className="flex flex-wrap gap-2">
					{SCHEDULE_PRESETS.map((preset) => {
						const active = selectedCron === preset.cron;
						return (
							<button
								key={preset.cron}
								type="button"
								onClick={() => setSelectedCron(preset.cron)}
								title={preset.desc}
								className="rounded-md border px-3 py-1 text-sm transition-colors"
								style={{
									borderColor: active ? "#58a6ff" : "#30363d",
									color: active ? "#58a6ff" : "#8b949e",
									background: active ? "#1a2a3a" : "transparent",
								}}
							>
								{preset.label}
							</button>
						);
					})}
				</div>
				{selectedCron && (
					<p className="text-[11px]" style={{ color: "#6e7681" }}>
						{SCHEDULE_PRESETS.find((p) => p.cron === selectedCron)?.desc}
					</p>
				)}
			</div>

			{/* Check options */}
			<div className="space-y-2">
				<p className="text-[#8b949e] text-xs">Check options</p>
				<label className="flex cursor-pointer select-none items-center gap-2">
					<input
						type="checkbox"
						checked={speedTest}
						onChange={(e) => setSpeedTest(e.target.checked)}
						className="accent-[#58a6ff]"
					/>
					<span
						className="flex items-center gap-1 text-xs"
						style={{ color: "#8b949e" }}
					>
						<Zap size={11} strokeWidth={1.5} />
						Speed test
					</span>
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
								className="accent-[#58a6ff]"
							/>
							<span
								className="text-[11px] uppercase"
								style={{ color: "#8b949e" }}
							>
								{app}
							</span>
						</label>
					))}
				</div>
			</div>

			<div className="flex gap-2">
				<Button
					size="sm"
					onClick={onSave}
					disabled={(!hideSubSelector && !subId) || !selectedCron || isPending}
					style={{ background: "#238636", color: "#fff" }}
					className="border-0"
				>
					{isPending ? <Loader2 size={13} className="animate-spin" /> : "Save"}
				</Button>
				<Button size="sm" variant="outline" onClick={onCancel}>
					Cancel
				</Button>
			</div>
		</div>
	);
}

// --- Job row with inline edit + history ---

function JobRow({
	job,
	subName,
	subs,
	onDelete,
	deleting,
	onSaveEdit,
	editPending,
}: {
	job: ScheduledJob;
	subName: string;
	subs: Subscription[];
	onDelete: () => void;
	deleting: boolean;
	onSaveEdit: (cron: string, speedTest: boolean, mediaApps: string[]) => void;
	editPending: boolean;
}) {
	const [showEdit, setShowEdit] = useState(false);
	const [showHistory, setShowHistory] = useState(false);

	// Edit state initialized from current job values
	const [editCron, setEditCron] = useState(job.cron_expr);
	const [editSpeedTest, setEditSpeedTest] = useState(job.speed_test);
	const [editMediaApps, setEditMediaApps] = useState<string[]>(
		job.media_apps ?? [...MEDIA_APPS],
	);

	function toggleApp(app: string) {
		setEditMediaApps((prev) =>
			prev.includes(app) ? prev.filter((a) => a !== app) : [...prev, app],
		);
	}

	const historyQuery = useQuery({
		queryKey: ["scheduler-history", job.subscription_id],
		queryFn: () =>
			client.checker.ListJobs(job.subscription_id, { Limit: 8, Offset: 0 }),
		enabled: showHistory,
		staleTime: 15_000,
	});

	return (
		<div
			className="rounded-lg border"
			style={{ background: "#161b22", borderColor: "#30363d" }}
		>
			{/* Main row */}
			<div className="flex items-center gap-3 px-4 py-3">
				<Clock size={13} strokeWidth={1.5} style={{ color: "#8b949e" }} />
				<div className="min-w-0 flex-1">
					<Link
						to="/subscriptions/$id"
						params={{ id: job.subscription_id }}
						className="font-medium text-sm hover:underline"
						style={{ color: "#58a6ff" }}
					>
						{subName}
					</Link>
					<p className="mt-0.5 text-xs" style={{ color: "#8b949e" }}>
						{cronToLabel(job.cron_expr)}
					</p>
				</div>
				<div className="flex flex-shrink-0 items-center gap-1">
					<button
						type="button"
						onClick={() => {
							setShowHistory(!showHistory);
							setShowEdit(false);
						}}
						className="flex items-center gap-1 rounded-md p-1.5 text-xs transition-colors hover:bg-white/5"
						style={{ color: showHistory ? "#58a6ff" : "#6e7681" }}
						title="Execution history"
					>
						<History size={13} strokeWidth={1.5} />
					</button>
					<button
						type="button"
						onClick={() => {
							setEditCron(job.cron_expr);
							setEditSpeedTest(job.speed_test);
							setEditMediaApps(job.media_apps ?? [...MEDIA_APPS]);
							setShowEdit(!showEdit);
							setShowHistory(false);
						}}
						className="rounded-md p-1.5 transition-colors hover:bg-white/5"
						style={{ color: showEdit ? "#58a6ff" : "#6e7681" }}
						title="Edit schedule"
					>
						<Pencil size={13} strokeWidth={1.5} />
					</button>
					<button
						type="button"
						onClick={onDelete}
						disabled={deleting}
						className="rounded-md p-1.5 transition-colors hover:bg-[#f85149]/10 hover:text-[#f85149] disabled:opacity-50"
						style={{ color: "#6e7681" }}
					>
						<Trash2 size={13} strokeWidth={1.5} />
					</button>
				</div>
			</div>

			{/* Tags */}
			<div className="flex flex-wrap gap-1.5 px-4 pt-0 pb-3">
				{job.speed_test && (
					<span
						className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px]"
						style={{ background: "#1a2a3a", color: "#58a6ff" }}
					>
						<Zap size={9} strokeWidth={2} />
						Speed
					</span>
				)}
				{(job.media_apps ?? []).map((app) => (
					<span
						key={app}
						className="rounded px-1.5 py-0.5 text-[10px] uppercase"
						style={{ background: "#21262d", color: "#8b949e" }}
					>
						{app}
					</span>
				))}
			</div>

			{/* Inline edit panel */}
			{showEdit && (
				<div className="border-t" style={{ borderColor: "#30363d" }}>
					<ScheduleForm
						subs={subs}
						subId={job.subscription_id}
						setSubId={() => {}}
						selectedCron={editCron}
						setSelectedCron={setEditCron}
						speedTest={editSpeedTest}
						setSpeedTest={setEditSpeedTest}
						mediaApps={editMediaApps}
						toggleApp={toggleApp}
						isPending={editPending}
						onSave={() => {
							onSaveEdit(editCron, editSpeedTest, editMediaApps);
							setShowEdit(false);
						}}
						onCancel={() => setShowEdit(false)}
						hideSubSelector
					/>
				</div>
			)}

			{/* History panel */}
			{showHistory && (
				<div className="border-t px-4 py-3" style={{ borderColor: "#30363d" }}>
					<p className="mb-2 font-medium text-xs" style={{ color: "#8b949e" }}>
						Recent runs
					</p>
					{historyQuery.isLoading && (
						<p className="text-xs" style={{ color: "#6e7681" }}>
							Loading…
						</p>
					)}
					{!historyQuery.isLoading &&
						(historyQuery.data?.jobs.length ?? 0) === 0 && (
							<p className="text-xs" style={{ color: "#6e7681" }}>
								No runs yet.
							</p>
						)}
					<div className="space-y-1">
						{historyQuery.data?.jobs.map((j) => (
							<Link
								key={j.id}
								to="/subscriptions/$id"
								params={{ id: job.subscription_id }}
								search={{ job: j.id }}
								className="flex items-center justify-between rounded px-2 py-1.5 transition-colors hover:bg-white/5"
							>
								<div className="flex items-center gap-2">
									<span
										className="h-1.5 w-1.5 flex-shrink-0 rounded-full"
										style={{ background: statusColor(j.status) }}
									/>
									<span
										className="font-mono text-[11px]"
										style={{ color: "#c9d1d9" }}
									>
										{new Date(j.created_at).toLocaleString(undefined, {
											month: "short",
											day: "numeric",
											hour: "2-digit",
											minute: "2-digit",
										})}
									</span>
								</div>
								<div className="flex items-center gap-3">
									<span className="text-[11px]" style={{ color: "#8b949e" }}>
										{j.available}/{j.total} alive
									</span>
									<ChevronRight
										size={11}
										strokeWidth={1.5}
										style={{ color: "#6e7681" }}
									/>
								</div>
							</Link>
						))}
					</div>
				</div>
			)}
		</div>
	);
}
