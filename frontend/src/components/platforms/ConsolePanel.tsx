import {
	Check,
	ChevronRight,
	Clock,
	Copy,
	Eye,
	Loader2,
	Search,
	X,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { checker } from "@/lib/client.gen";

type TestRuleResult = checker.TestRuleResult;
type DebugStep = checker.DebugStep;
type DebugTrace = checker.DebugTrace;
type StepType =
	| "http_request"
	| "http_response"
	| "variable"
	| "condition"
	| "log"
	| "error";
type Tab = "trace" | "body" | "rendered" | "headers";

interface Props {
	result: TestRuleResult | null;
	loading: boolean;
	nodeLabel: string;
}

export function ConsolePanel({ result, loading, nodeLabel }: Props) {
	const trace: DebugTrace | undefined = result?.trace;
	const steps = trace?.steps ?? [];

	const respHeaders = result?.response_headers ?? {};
	const headerEntries = useMemo(
		() => Object.entries(respHeaders).sort(([a], [b]) => a.localeCompare(b)),
		[respHeaders],
	);
	const contentType = useMemo(
		() => findHeader(respHeaders, "content-type"),
		[respHeaders],
	);

	const body = result?.body ?? "";
	const finalURL = result?.final_url ?? "";
	const looksLikeHTML = useMemo(
		() =>
			contentType.toLowerCase().includes("text/html") ||
			/<!DOCTYPE\s+html|<html[\s>]/i.test(body.slice(0, 4000)),
		[body, contentType],
	);

	const [tab, setTab] = useState<Tab>("trace");

	// Auto-switch back to trace when a new request runs.
	const resultStamp = result?.duration_ms ?? 0;
	useEffect(() => {
		setTab("trace");
	}, [resultStamp]);

	return (
		<div className="border-border border-t bg-[#1e1e1e] font-mono text-[#d4d4d4] text-xs">
			<StatusBar
				result={result}
				loading={loading}
				nodeLabel={nodeLabel}
				stepCount={steps.length}
			/>

			{result && (
				<>
					<TabBar
						tab={tab}
						setTab={setTab}
						counts={{
							trace: steps.length,
							body: body.length,
							headers: headerEntries.length,
						}}
						canRender={body.length > 0}
					/>
					<div className="min-h-[260px]">
						{tab === "trace" && <TraceView steps={steps} trace={trace} />}
						{tab === "body" && (
							<BodyView body={body} contentType={contentType} />
						)}
						{tab === "rendered" && (
							<RenderedView
								body={body}
								baseURL={finalURL}
								looksLikeHTML={looksLikeHTML}
							/>
						)}
						{tab === "headers" && <HeadersView entries={headerEntries} />}
					</div>
				</>
			)}
		</div>
	);
}

// ─── Top status bar ───────────────────────────────────────────────────────────

function StatusBar({
	result,
	loading,
	nodeLabel,
	stepCount,
}: {
	result: TestRuleResult | null;
	loading: boolean;
	nodeLabel: string;
	stepCount: number;
}) {
	return (
		<div className="flex flex-wrap items-center gap-3 border-white/5 border-b px-3 py-1.5">
			<span className="font-semibold text-[#858585] text-[10px] uppercase tracking-wider">
				Test
			</span>
			{loading && (
				<span className="flex items-center gap-1.5 text-[#858585]">
					<Loader2 size={11} className="animate-spin" />
					Running…
				</span>
			)}
			{!loading && result && (
				<>
					<span
						className={
							result.ok
								? "rounded bg-[#4ec9b0]/15 px-1.5 py-0.5 font-medium text-[#4ec9b0]"
								: "rounded bg-[#f14c4c]/15 px-1.5 py-0.5 font-medium text-[#f14c4c]"
						}
					>
						{result.ok ? "✓ PASS" : "✗ FAIL"}
					</span>
					{result.status_code > 0 && (
						<span
							className={
								result.status_code < 400 ? "text-[#4ec9b0]" : "text-[#f14c4c]"
							}
						>
							HTTP {result.status_code}
						</span>
					)}
					{result.duration_ms > 0 && (
						<span className="flex items-center gap-1 text-[#858585]">
							<Clock size={9} /> {result.duration_ms}ms total
						</span>
					)}
					<span className="text-[#858585]">{stepCount} steps</span>
					{result.final_url && (
						<span className="truncate text-[#9cdcfe]" title={result.final_url}>
							→ {result.final_url}
						</span>
					)}
				</>
			)}
			<span className="flex-1" />
			{nodeLabel && (
				<span className="rounded bg-white/5 px-1.5 py-0.5 text-[#858585] text-[10px]">
					via {nodeLabel}
				</span>
			)}
			{result?.error && (
				<span className="text-[#f14c4c]" title={result.error}>
					! {result.error}
				</span>
			)}
		</div>
	);
}

// ─── Tabs ──────────────────────────────────────────────────────────────────────

function TabBar({
	tab,
	setTab,
	counts,
	canRender,
}: {
	tab: Tab;
	setTab: (t: Tab) => void;
	counts: { trace: number; body: number; headers: number };
	canRender: boolean;
}) {
	const tabs: Array<{
		id: Tab;
		label: string;
		count?: number;
		disabled?: boolean;
	}> = [
		{ id: "trace", label: "Trace", count: counts.trace },
		{ id: "body", label: "Body", count: counts.body },
		{ id: "rendered", label: "Rendered", disabled: !canRender },
		{ id: "headers", label: "Headers", count: counts.headers },
	];

	return (
		<div className="flex items-center gap-0 border-white/5 border-b bg-white/[0.02] px-3">
			{tabs.map((t) => (
				<button
					key={t.id}
					type="button"
					disabled={t.disabled}
					onClick={() => setTab(t.id)}
					className={[
						"flex items-center gap-1.5 border-b-2 px-3 py-1.5 text-[11px] transition-colors",
						tab === t.id
							? "border-[#569cd6] text-[#d4d4d4]"
							: "border-transparent text-[#858585] hover:text-[#d4d4d4]",
						t.disabled ? "cursor-not-allowed opacity-40" : "",
					].join(" ")}
				>
					{t.label}
					{t.count != null && t.count > 0 && (
						<span className="text-[#569cd6] text-[10px]">
							{formatCount(t.count)}
						</span>
					)}
				</button>
			))}
		</div>
	);
}

function formatCount(n: number): string {
	if (n < 1000) return String(n);
	if (n < 1000_000) return `${(n / 1000).toFixed(1)}k`;
	return `${(n / 1_000_000).toFixed(1)}M`;
}

// ─── Trace view ────────────────────────────────────────────────────────────────

function TraceView({
	steps,
	trace,
}: {
	steps: DebugStep[];
	trace?: DebugTrace;
}) {
	const [selected, setSelected] = useState(0);
	useEffect(() => {
		setSelected(steps.length - 1 >= 0 ? steps.length - 1 : 0);
	}, [steps.length]);

	if (steps.length === 0) {
		return (
			<p className="px-3 py-6 text-center text-[#858585]">
				No debug steps captured.
			</p>
		);
	}

	return (
		<div className="flex flex-col sm:flex-row sm:max-h-[60vh]">
			{/* Step list */}
			<div className="w-full sm:w-2/5 overflow-y-auto border-white/5 border-b sm:border-b-0 sm:border-r max-h-48 sm:max-h-none">
				{trace && (
					<div className="border-white/5 border-b px-3 py-1.5">
						<span className="text-[#858585] text-[10px]">
							{trace.platform || "rule"} ·{" "}
						</span>
						<span
							className={
								trace.result
									? "font-medium text-[#4ec9b0]"
									: "font-medium text-[#f14c4c]"
							}
						>
							{trace.result ? "UNLOCKED" : "BLOCKED"}
						</span>
					</div>
				)}
				{steps.map((step, i) => (
					<StepRow
						key={i}
						step={step}
						index={i}
						selected={selected === i}
						onClick={() => setSelected(i)}
					/>
				))}
			</div>

			{/* Detail panel */}
			<div className="w-full sm:w-3/5 overflow-y-auto">
				{steps[selected] ? (
					<StepDetail step={steps[selected]} />
				) : (
					<p className="px-3 py-4 text-[#858585]">Select a step.</p>
				)}
			</div>
		</div>
	);
}

const stepStyle: Record<
	StepType,
	{ bg: string; color: string; label: string }
> = {
	http_request: {
		bg: "rgba(86, 156, 214, 0.2)",
		color: "#569cd6",
		label: "REQ",
	},
	http_response: {
		bg: "rgba(86, 156, 214, 0.2)",
		color: "#569cd6",
		label: "RES",
	},
	variable: { bg: "rgba(78, 201, 176, 0.2)", color: "#4ec9b0", label: "VAR" },
	condition: { bg: "rgba(220, 220, 170, 0.2)", color: "#dcdcaa", label: "IF" },
	log: { bg: "transparent", color: "#858585", label: "LOG" },
	error: { bg: "rgba(241, 76, 76, 0.2)", color: "#f14c4c", label: "ERR" },
};

function StepRow({
	step,
	index,
	selected,
	onClick,
}: {
	step: DebugStep;
	index: number;
	selected: boolean;
	onClick: () => void;
}) {
	const style = stepStyle[step.type as StepType] ?? stepStyle.log;
	const details = step.details as Record<string, unknown>;
	const duration =
		typeof details.duration_ms === "number" ? details.duration_ms : null;

	return (
		<button
			type="button"
			onClick={onClick}
			className={[
				"flex w-full items-center gap-2 px-3 py-1 text-left text-[11px] transition-colors",
				selected ? "bg-white/10" : "hover:bg-white/5",
			].join(" ")}
		>
			<span className="w-5 flex-shrink-0 text-right text-[#858585] text-[9px]">
				{index + 1}
			</span>
			<span
				className="rounded px-1 py-0.5 font-medium text-[9px]"
				style={{ background: style.bg, color: style.color }}
			>
				{style.label}
			</span>
			<span className="truncate text-[#d4d4d4]">{step.description}</span>
			{duration != null && (
				<span className="ml-auto flex-shrink-0 text-[#858585] text-[9px]">
					{duration}ms
				</span>
			)}
		</button>
	);
}

function StepDetail({ step }: { step: DebugStep }) {
	const details = (step.details ?? {}) as Record<string, unknown>;

	if (step.type === "http_request") {
		return <HTTPRequestDetail details={details} />;
	}
	if (step.type === "http_response") {
		return <HTTPResponseDetail details={details} />;
	}
	return <KeyValueDetail details={details} />;
}

function HTTPRequestDetail({ details }: { details: Record<string, unknown> }) {
	return (
		<div className="space-y-2 p-3">
			<DetailRow
				label="Method"
				value={String(details.method ?? "GET")}
				mono
				color="#569cd6"
			/>
			<DetailRow
				label="URL"
				value={String(details.url ?? "")}
				mono
				color="#9cdcfe"
			/>
			<HeadersBlock
				title="Request Headers"
				headers={details.headers as Record<string, string> | undefined}
			/>
			{typeof details.body === "string" && details.body.length > 0 && (
				<CollapsibleBody title="Request Body" body={details.body} defaultOpen />
			)}
		</div>
	);
}

function HTTPResponseDetail({ details }: { details: Record<string, unknown> }) {
	const status =
		typeof details.status_code === "number" ? details.status_code : 0;
	const duration =
		typeof details.duration_ms === "number" ? details.duration_ms : null;
	const size =
		typeof details.size_bytes === "number" ? details.size_bytes : null;
	const finalURL =
		typeof details.final_url === "string" ? details.final_url : "";
	const body = typeof details.body === "string" ? details.body : "";
	const headers = details.headers as Record<string, string> | undefined;

	return (
		<div className="space-y-2 p-3">
			<div className="flex flex-wrap gap-3">
				<DetailInline
					label="Status"
					value={status > 0 ? String(status) : "—"}
					color={
						status === 0 ? "#858585" : status < 400 ? "#4ec9b0" : "#f14c4c"
					}
				/>
				{duration != null && (
					<DetailInline
						label="Duration"
						value={`${duration} ms`}
						color="#858585"
					/>
				)}
				{size != null && (
					<DetailInline label="Size" value={humanBytes(size)} color="#858585" />
				)}
			</div>
			{finalURL && (
				<DetailRow label="Final URL" value={finalURL} mono color="#9cdcfe" />
			)}
			<HeadersBlock title="Response Headers" headers={headers} />
			{body.length > 0 && (
				<CollapsibleBody
					title="Response Body"
					body={body}
					defaultOpen={body.length < 5000}
				/>
			)}
		</div>
	);
}

function KeyValueDetail({ details }: { details: Record<string, unknown> }) {
	const keys = Object.keys(details);
	if (keys.length === 0) {
		return <p className="px-3 py-4 text-[#858585]">No details.</p>;
	}
	return (
		<div className="space-y-1.5 p-3">
			{keys.map((k) => {
				const v = details[k];
				const display = typeof v === "string" ? v : JSON.stringify(v, null, 2);
				return (
					<div key={k} className="rounded bg-white/[0.03] px-2 py-1">
						<div className="flex items-center gap-1.5">
							<span className="text-[#9cdcfe] text-[11px]">{k}</span>
							<span className="text-[#858585] text-[10px]">
								{describeValue(v)}
							</span>
						</div>
						<pre
							className="mt-0.5 overflow-auto whitespace-pre-wrap break-all text-[#ce9178] text-[11px] leading-relaxed"
							style={{ maxHeight: 200 }}
						>
							{display}
						</pre>
					</div>
				);
			})}
		</div>
	);
}

function describeValue(v: unknown): string {
	if (v === null) return "null";
	if (Array.isArray(v)) return `array[${v.length}]`;
	return typeof v;
}

function DetailRow({
	label,
	value,
	mono,
	color,
}: {
	label: string;
	value: string;
	mono?: boolean;
	color?: string;
}) {
	return (
		<div className="space-y-0.5">
			<p className="text-[#858585] text-[10px] uppercase tracking-wider">
				{label}
			</p>
			<p
				className={["break-all", mono ? "font-mono" : ""].join(" ")}
				style={{ color: color ?? "#d4d4d4" }}
			>
				{value}
			</p>
		</div>
	);
}

function DetailInline({
	label,
	value,
	color,
}: {
	label: string;
	value: string;
	color?: string;
}) {
	return (
		<div>
			<span className="text-[#858585] text-[10px] uppercase tracking-wider">
				{label}:{" "}
			</span>
			<span style={{ color: color ?? "#d4d4d4" }}>{value}</span>
		</div>
	);
}

function HeadersBlock({
	title,
	headers,
}: {
	title: string;
	headers?: Record<string, string>;
}) {
	const [open, setOpen] = useState(true);
	const entries = headers
		? Object.entries(headers).sort(([a], [b]) => a.localeCompare(b))
		: [];
	if (entries.length === 0) return null;
	return (
		<div className="rounded border border-white/5">
			<button
				type="button"
				onClick={() => setOpen((o) => !o)}
				className="flex w-full items-center gap-1.5 px-2 py-1 text-left text-[#858585] hover:text-[#d4d4d4]"
			>
				<ChevronRight
					size={10}
					className={
						open ? "rotate-90 transition-transform" : "transition-transform"
					}
				/>
				<span>{title}</span>
				<span className="ml-1 text-[#569cd6]">{entries.length}</span>
			</button>
			{open && (
				<div className="space-y-0.5 border-white/5 border-t px-2 py-1.5">
					{entries.map(([k, v]) => (
						<div key={k} className="flex gap-2 text-[11px]">
							<span className="flex-shrink-0 text-[#9cdcfe]">{k}:</span>
							<span className="break-all text-[#ce9178]">{v}</span>
						</div>
					))}
				</div>
			)}
		</div>
	);
}

function CollapsibleBody({
	title,
	body,
	defaultOpen,
}: {
	title: string;
	body: string;
	defaultOpen?: boolean;
}) {
	const [open, setOpen] = useState(!!defaultOpen);
	return (
		<div className="rounded border border-white/5">
			<div className="flex items-center justify-between border-white/5 border-b px-2 py-1">
				<button
					type="button"
					onClick={() => setOpen((o) => !o)}
					className="flex items-center gap-1.5 text-[#858585] hover:text-[#d4d4d4]"
				>
					<ChevronRight
						size={10}
						className={
							open ? "rotate-90 transition-transform" : "transition-transform"
						}
					/>
					<span>{title}</span>
					<span className="text-[#569cd6] text-[10px]">
						{humanBytes(body.length)}
					</span>
				</button>
				<CopyButton text={body} />
			</div>
			{open && (
				<pre
					className="max-h-72 overflow-auto whitespace-pre-wrap break-all p-2 text-[11px] leading-relaxed"
					style={{ color: "#d4d4d4" }}
				>
					{body}
				</pre>
			)}
		</div>
	);
}

// ─── Body view (full + searchable) ─────────────────────────────────────────────

function BodyView({
	body,
	contentType,
}: {
	body: string;
	contentType: string;
}) {
	const [q, setQ] = useState("");
	const [caseSensitive, setCaseSensitive] = useState(false);

	const { matchCount, html } = useMemo(
		() => renderBodyWithHighlight(body, q, caseSensitive),
		[body, q, caseSensitive],
	);

	if (body.length === 0) {
		return <p className="px-3 py-6 text-center text-[#858585]">No body.</p>;
	}

	return (
		<div>
			<div className="sticky top-0 z-10 flex items-center gap-2 border-white/5 border-b bg-[#1e1e1e] px-3 py-1.5">
				<Search size={11} className="text-[#858585]" />
				<input
					value={q}
					onChange={(e) => setQ(e.target.value)}
					placeholder="Search body…"
					className="h-6 flex-1 rounded border border-white/5 bg-[#252526] px-2 text-[#d4d4d4] text-[11px] focus:outline-none focus:ring-1 focus:ring-[#569cd6]"
				/>
				<button
					type="button"
					onClick={() => setCaseSensitive((c) => !c)}
					title="Case sensitive"
					className={[
						"flex h-6 w-6 items-center justify-center rounded text-[11px]",
						caseSensitive
							? "bg-[#569cd6]/20 text-[#569cd6]"
							: "border border-white/5 text-[#858585] hover:text-[#d4d4d4]",
					].join(" ")}
				>
					Aa
				</button>
				{q && (
					<span className="text-[#858585] text-[10px]">
						{matchCount} match{matchCount === 1 ? "" : "es"}
					</span>
				)}
				{q && (
					<button
						type="button"
						onClick={() => setQ("")}
						className="rounded p-0.5 text-[#858585] hover:bg-white/10 hover:text-[#d4d4d4]"
					>
						<X size={10} />
					</button>
				)}
				<span className="text-[#858585] text-[10px]">
					{humanBytes(body.length)}
				</span>
				{contentType && (
					<span className="rounded bg-white/5 px-1 py-0.5 text-[#858585] text-[10px]">
						{contentType.split(";")[0]}
					</span>
				)}
				<CopyButton text={body} />
			</div>
			<div
				className="overflow-auto p-3 text-[11px] leading-[1.6]"
				style={{ maxHeight: "55vh" }}
			>
				<pre
					className="m-0 whitespace-pre-wrap break-all"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: text is escaped in renderBodyWithHighlight
					dangerouslySetInnerHTML={{ __html: html }}
				/>
			</div>
		</div>
	);
}

const ESCAPE_HTML: Record<string, string> = {
	"&": "&amp;",
	"<": "&lt;",
	">": "&gt;",
	'"': "&quot;",
	"'": "&#39;",
};

function escapeHTML(s: string): string {
	return s.replace(/[&<>"']/g, (c) => ESCAPE_HTML[c]);
}

function escapeRegex(s: string): string {
	return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function renderBodyWithHighlight(
	body: string,
	q: string,
	caseSensitive: boolean,
): { matchCount: number; html: string } {
	const lines = body.split("\n");
	const lineNumDigits = String(lines.length).length;
	let total = 0;

	const re = q
		? new RegExp(`(${escapeRegex(q)})`, caseSensitive ? "g" : "gi")
		: null;

	const rendered = lines.map((line, i) => {
		const lineNum = String(i + 1).padStart(lineNumDigits, " ");
		const numHTML = `<span style="color:#5a5a5a;user-select:none;">${escapeHTML(lineNum)}  </span>`;
		if (!re) {
			return numHTML + escapeHTML(line);
		}
		const parts = line.split(re);
		const lineHTML = parts
			.map((part, j) => {
				const safe = escapeHTML(part);
				if (j % 2 === 1) {
					total++;
					return `<mark style="background:#5e3a00;color:#ffd700;border-radius:2px;padding:0 1px;">${safe}</mark>`;
				}
				return safe;
			})
			.join("");
		return numHTML + lineHTML;
	});

	return { matchCount: total, html: rendered.join("\n") };
}

function humanBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

// ─── Rendered view (sandboxed iframe) ──────────────────────────────────────────

function RenderedView({
	body,
	baseURL,
	looksLikeHTML,
}: {
	body: string;
	baseURL: string;
	looksLikeHTML: boolean;
}) {
	const [zoom, setZoom] = useState(100);
	const [showImages, setShowImages] = useState(true);
	const [runScripts, setRunScripts] = useState(false);

	const srcDoc = useMemo(
		() => buildPreviewDoc(body, baseURL, showImages, runScripts),
		[body, baseURL, showImages, runScripts],
	);

	if (!body) {
		return (
			<p className="px-3 py-6 text-center text-[#858585]">No body to render.</p>
		);
	}

	return (
		<div className="bg-white">
			<div className="flex flex-wrap items-center gap-2 border-white/5 border-b bg-[#1e1e1e] px-3 py-1.5">
				<Eye size={11} className="text-[#858585]" />
				<span className="text-[#858585] text-[10px]">
					{runScripts
						? "Sandboxed preview · scripts ON (opaque origin)"
						: "Sandboxed preview · scripts OFF"}
				</span>
				{!looksLikeHTML && (
					<span className="rounded bg-amber-500/15 px-1.5 py-0.5 text-[10px] text-amber-400">
						No HTML detected — showing as plain text
					</span>
				)}
				{baseURL && (
					<span
						className="hidden truncate text-[#858585] text-[10px] sm:inline"
						title={baseURL}
						style={{ maxWidth: 220 }}
					>
						base: {baseURL}
					</span>
				)}
				<span className="flex-1" />
				<label
					className="flex cursor-pointer items-center gap-1 text-[10px]"
					style={{ color: runScripts ? "#f14c4c" : "#858585" }}
					title="Untrusted scripts run in an opaque origin (cannot touch this app). Still: only enable when you trust the page."
				>
					<input
						type="checkbox"
						checked={runScripts}
						onChange={(e) => setRunScripts(e.target.checked)}
						className="h-3 w-3 cursor-pointer"
					/>
					Scripts
					{runScripts && <span className="text-[9px]">⚠</span>}
				</label>
				<label className="flex cursor-pointer items-center gap-1 text-[#858585] text-[10px]">
					<input
						type="checkbox"
						checked={showImages}
						onChange={(e) => setShowImages(e.target.checked)}
						className="h-3 w-3 cursor-pointer"
					/>
					Images
				</label>
				<button
					type="button"
					onClick={() => setZoom((z) => Math.max(25, z - 25))}
					className="rounded border border-white/5 px-2 py-0.5 text-[#858585] text-[10px] hover:text-[#d4d4d4]"
				>
					−
				</button>
				<span className="text-[#858585] text-[10px]">{zoom}%</span>
				<button
					type="button"
					onClick={() => setZoom((z) => Math.min(200, z + 25))}
					className="rounded border border-white/5 px-2 py-0.5 text-[#858585] text-[10px] hover:text-[#d4d4d4]"
				>
					+
				</button>
			</div>
			{/*
			 * Scripts OFF (default): sandbox="allow-same-origin" — JS blocked,
			 *   relative URLs resolve via <base href> so images/CSS load.
			 * Scripts ON: sandbox="allow-scripts" — JS runs in opaque origin
			 *   (cannot touch parent app), but relative URLs no longer resolve
			 *   (browser treats it as cross-origin). Most assets still load if
			 *   the page uses absolute URLs.
			 */}
			<iframe
				key={runScripts ? "scripts-on" : "scripts-off"}
				sandbox={runScripts ? "allow-scripts" : "allow-same-origin"}
				srcDoc={srcDoc}
				className="block w-full bg-white"
				style={{
					height: "60vh",
					transform: `scale(${zoom / 100})`,
					transformOrigin: "top left",
					width: zoom === 100 ? "100%" : `${10000 / zoom}%`,
				}}
				title="Response preview"
			/>
		</div>
	);
}

// buildPreviewDoc prepares the HTML for the sandboxed iframe:
// - injects a <base href="<finalURL>"> so relative image/CSS/font URLs resolve
//   against the original site (when sandbox permits — scripts-off mode)
// - when runScripts is OFF, strips <script> blocks + inline event handlers to
//   silence console errors (the sandbox would block them anyway)
// - when runScripts is ON, leaves scripts intact — the iframe sandbox is set to
//   `allow-scripts` (without allow-same-origin) so scripts run in an opaque
//   origin and cannot touch the host app
// - optionally strips <img> tags when the user disables image loading
//
// Non-HTML bodies (JSON/text) are wrapped in a <pre> so they're still visible.
function buildPreviewDoc(
	body: string,
	baseURL: string,
	showImages: boolean,
	runScripts: boolean,
): string {
	const isHTML = /<!DOCTYPE\s+html|<html[\s>]/i.test(body.slice(0, 4000));
	const baseTag = baseURL ? `<base href="${escapeAttr(baseURL)}">` : "";

	let html = body;
	if (!isHTML) {
		const escaped = escapeHTML(body);
		html = `<!DOCTYPE html><html><head><meta charset="utf-8">${baseTag}<style>body{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;padding:16px;color:#222;}pre{white-space:pre-wrap;word-break:break-all;margin:0;}</style></head><body><pre>${escaped}</pre></body></html>`;
		return html;
	}

	// Inject <base> right after <head> opens (or before </head>).
	if (baseTag) {
		if (/<head[^>]*>/i.test(html)) {
			html = html.replace(/<head[^>]*>/i, (m) => `${m}${baseTag}`);
		} else if (/<html[^>]*>/i.test(html)) {
			html = html.replace(/<html[^>]*>/i, (m) => `${m}<head>${baseTag}</head>`);
		} else {
			html = `${baseTag}${html}`;
		}
	}

	if (!runScripts) {
		// Strip scripts + inline event handlers when scripts are disabled.
		// The sandbox would block them anyway; removing avoids console noise.
		html = html.replace(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, "");
		html = html.replace(/\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, "");
	}

	if (!showImages) {
		html = html.replace(/<img\b[^>]*>/gi, "");
	}

	return html;
}

function escapeAttr(s: string): string {
	return s.replace(/[&<>"']/g, (c) => ESCAPE_HTML[c]);
}

function findHeader(headers: Record<string, string>, key: string): string {
	const lower = key.toLowerCase();
	for (const [k, v] of Object.entries(headers)) {
		if (k.toLowerCase() === lower) return v;
	}
	return "";
}

// ─── Headers view (table) ──────────────────────────────────────────────────────

function HeadersView({ entries }: { entries: [string, string][] }) {
	if (entries.length === 0) {
		return <p className="px-3 py-6 text-center text-[#858585]">No headers.</p>;
	}
	return (
		<div className="overflow-auto" style={{ maxHeight: "55vh" }}>
			<table className="w-full border-collapse text-[11px]">
				<tbody>
					{entries.map(([k, v], i) => (
						<tr
							key={k}
							className={i % 2 === 0 ? "bg-white/[0.02]" : ""}
							style={{ borderBottom: "1px solid rgba(255,255,255,0.04)" }}
						>
							<td className="w-1/3 px-3 py-1.5 align-top text-[#9cdcfe]">
								{k}
							</td>
							<td className="break-all px-3 py-1.5 align-top text-[#ce9178]">
								{v}
							</td>
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}

// ─── Copy button ───────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
	const [copied, setCopied] = useState(false);
	return (
		<button
			type="button"
			onClick={() => {
				navigator.clipboard.writeText(text);
				setCopied(true);
				setTimeout(() => setCopied(false), 1500);
			}}
			className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[#858585] text-[10px] hover:bg-white/10 hover:text-[#d4d4d4]"
		>
			{copied ? <Check size={9} /> : <Copy size={9} />}
			{copied ? "Copied" : "Copy"}
		</button>
	);
}
