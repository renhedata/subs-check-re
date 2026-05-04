import Editor, { useMonaco } from "@monaco-editor/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import {
	BookOpen,
	ChevronDown,
	ChevronUp,
	Clock,
	Loader2,
	Play,
	Plus,
	Trash2,
	X,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";

type PlatformRule = checker.PlatformRule;
type CreateRuleParams = checker.CreateRuleParams;
type UpdateRuleParams = checker.UpdateRuleParams;
type TestRuleResult = checker.TestRuleResult;

export const Route = createFileRoute("/settings/platforms")({
	component: PlatformsPage,
});

const RULE_TYPES = ["condition", "js", "ts", "tengo", "lua"] as const;
type RuleType = (typeof RULE_TYPES)[number];

const RULE_TYPE_LABELS: Record<RuleType, string> = {
	condition: "Condition",
	js: "JavaScript",
	ts: "TypeScript",
	tengo: "Tengo",
	lua: "Lua",
};

const MONACO_LANG: Record<RuleType, string> = {
	condition: "plaintext",
	js: "javascript",
	ts: "typescript",
	tengo: "go",
	lua: "lua",
};

const TYPE_COLORS: Record<RuleType, string> = {
	condition: "bg-blue-500/10 text-blue-400 border-blue-500/30",
	js: "bg-yellow-500/10 text-yellow-400 border-yellow-500/30",
	ts: "bg-blue-600/10 text-blue-300 border-blue-600/30",
	tengo: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
	lua: "bg-purple-500/10 text-purple-400 border-purple-500/30",
};

// ─── Engine docs ──────────────────────────────────────────────────────────────

const ENGINE_DOCS: Record<RuleType, { sections: { h: string; body: string }[] }> = {
	condition: {
		sections: [
			{
				h: "Fields",
				body: `url                    string   required
method                 string   GET | HEAD | POST
status_code            int      0 = any status
body_contains          []string all must match
body_contains_any      []string at least one must match
body_not_contains      []string none may match
final_url_contains     string   after redirect
final_url_not_contains string   after redirect`,
			},
		],
	},
	js: {
		sections: [
			{
				h: "http_get(url, opts?)",
				body: `const r = http_get("https://example.com", {
  headers: { "Accept": "application/json" }
})
r.status     // number
r.body       // string
r.final_url  // string (after redirects)`,
			},
			{
				h: "Globals",
				body: `JSON · Math · parseInt · parseFloat
encodeURIComponent · Array · RegExp · Date`,
			},
			{
				h: "Return",
				body: `// last expression or explicit return:
return r.status === 200 && r.body.includes("OK")`,
			},
			{
				h: "Not available",
				body: `import / require · fetch · async/await · Node.js`,
			},
		],
	},
	ts: {
		sections: [
			{
				h: "http_get declaration",
				body: `declare function http_get(
  url: string,
  opts?: { headers?: Record<string, string> }
): { status: number; body: string; final_url: string };`,
			},
			{
				h: "Supported",
				body: `types · interfaces · generics · enums
arrow functions · optional chaining · nullish coalescing
const / let · destructuring · spread`,
			},
			{
				h: "Return",
				body: `return r.status === 200 && r.body.includes("OK")`,
			},
			{
				h: "Not available",
				body: `import / export · npm packages · async/await`,
			},
		],
	},
	tengo: {
		sections: [
			{
				h: "http_get variable",
				body: `r := http_get("https://example.com")
r.status     // int
r.body       // string
r.final_url  // string
r.error      // string (empty on success)`,
			},
			{
				h: 'stdlib — import("name")',
				body: `"fmt"    fmt.sprintf, fmt.println
"text"   text.contains, text.has_prefix, text.split …
"json"   json.encode, json.decode
"math"   math.abs, math.floor, math.sqrt
"base64" base64.encode, base64.decode
"times"  times.now, times.format`,
			},
			{
				h: "Result",
				body: `// assign bool to pre-declared output var:
output = r.status == 200`,
			},
		],
	},
	lua: {
		sections: [
			{
				h: "http_get(url, opts?)",
				body: `local r = http_get("https://example.com", {
  headers = { ["User-Agent"] = "bot" }
})
-- r.status / r.body / r.final_url / r.error`,
			},
			{
				h: "Standard libraries",
				body: `string  string.find, string.match, string.gsub …
table   table.insert, table.concat …
math    math.abs, math.floor, math.random …
os      os.time, os.date`,
			},
			{
				h: "Return",
				body: `return r.status == 200 and
       r.body:find("currentMember") ~= nil`,
			},
		],
	},
};

// ─── Monaco setup ─────────────────────────────────────────────────────────────

function useMonacoSetup() {
	const monaco = useMonaco();
	useEffect(() => {
		if (!monaco) return;
		const dts = `declare function http_get(
  url: string,
  opts?: { headers?: Record<string, string> }
): { readonly status: number; readonly body: string; readonly final_url: string };
`;
		monaco.typescript.javascriptDefaults.addExtraLib(dts, "subs-check.d.ts");
		monaco.typescript.typescriptDefaults.addExtraLib(dts, "subs-check.d.ts");
		monaco.typescript.javascriptDefaults.setCompilerOptions({
			target: monaco.typescript.ScriptTarget.ES2015,
			allowNonTsExtensions: true,
		});
		monaco.typescript.typescriptDefaults.setCompilerOptions({
			target: monaco.typescript.ScriptTarget.ES2015,
			strict: false,
			allowNonTsExtensions: true,
		});
	}, [monaco]);
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const emptyCondition = {
	url: "",
	method: "GET",
	status_code: 0,
	body_contains: [] as string[],
	body_contains_any: [] as string[],
	body_not_contains: [] as string[],
	final_url_contains: "",
	final_url_not_contains: "",
};

const emptyScript = { prelude: "", code: "" };

function defaultDef(type: RuleType) {
	return type === "condition" ? { ...emptyCondition } : { ...emptyScript };
}

// ─── Page ─────────────────────────────────────────────────────────────────────

function PlatformsPage() {
	const qc = useQueryClient();
	useMonacoSetup();

	const [editingRule, setEditingRule] = useState<PlatformRule | null>(null);
	const [addOpen, setAddOpen] = useState(false);

	const { data, isLoading } = useQuery({
		queryKey: ["platform-rules"],
		queryFn: () => client.checker.ListRules(),
	});

	const rules = data?.rules ?? [];

	return (
		<div className="max-w-2xl space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-foreground text-lg">Platform Detection Rules</h1>
				<button
					type="button"
					onClick={() => setAddOpen(true)}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white"
					style={{ background: "var(--color-btn-success)" }}
				>
					<Plus size={13} /> Add Rule
				</button>
			</div>

			<p className="text-muted-foreground text-xs">
				Rules run during each proxy check. Built-in rules are seeded on first visit.
				Custom keys store results in{" "}
				<code className="rounded bg-secondary px-1 font-mono">extra_platforms</code>.
			</p>

			{isLoading ? (
				<div className="flex justify-center py-8">
					<Loader2 size={18} className="animate-spin text-muted-foreground" />
				</div>
			) : (
				<div className="space-y-2">
					{rules.map((rule) => (
						<RuleCard
							key={rule.id}
							rule={rule}
							qc={qc}
							onEdit={() => setEditingRule(rule)}
						/>
					))}
					{rules.length === 0 && (
						<p className="py-6 text-center text-muted-foreground text-sm">No rules yet.</p>
					)}
				</div>
			)}

			{addOpen && (
				<RuleEditorDialog
					qc={qc}
					onClose={() => setAddOpen(false)}
				/>
			)}

			{editingRule && (
				<RuleEditorDialog
					rule={editingRule}
					qc={qc}
					onClose={() => setEditingRule(null)}
				/>
			)}
		</div>
	);
}

// ─── Rule card ────────────────────────────────────────────────────────────────

function RuleCard({
	rule,
	qc,
	onEdit,
}: {
	rule: PlatformRule;
	qc: ReturnType<typeof useQueryClient>;
	onEdit: () => void;
}) {
	const ruleType = rule.rule_type as RuleType;

	const deleteMut = useMutation({
		mutationFn: () => client.checker.DeleteRule(rule.id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success("Rule deleted");
		},
		onError: () => toast.error("Failed to delete"),
	});

	const toggleMut = useMutation({
		mutationFn: (enabled: boolean) =>
			client.checker.UpdateRule(rule.id, {
				name: rule.name,
				enabled,
				rule_type: rule.rule_type,
				definition: rule.definition,
				sort_order: rule.sort_order,
			}),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["platform-rules"] }),
		onError: () => toast.error("Failed to update"),
	});

	return (
		<div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
			<button
				type="button"
				onClick={() => toggleMut.mutate(!rule.enabled)}
				disabled={toggleMut.isPending}
				className={[
					"relative h-4 w-7 flex-shrink-0 rounded-full transition-colors",
					rule.enabled ? "bg-green-500" : "bg-muted",
				].join(" ")}
				aria-label="Toggle"
			>
				<span
					className={[
						"absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform",
						rule.enabled ? "translate-x-3" : "translate-x-0.5",
					].join(" ")}
				/>
			</button>

			<div className="min-w-0 flex-1">
				<div className="flex flex-wrap items-center gap-2">
					<span className="font-medium text-foreground text-sm">{rule.name}</span>
					<span className="rounded bg-secondary px-1.5 py-0.5 font-mono text-muted-foreground text-xs">
						{rule.key}
					</span>
					<span
						className={[
							"rounded border px-1.5 py-0.5 text-xs",
							TYPE_COLORS[ruleType] ?? "bg-secondary text-muted-foreground border-border",
						].join(" ")}
					>
						{RULE_TYPE_LABELS[ruleType] ?? rule.rule_type}
					</span>
					{rule.is_default && (
						<span className="text-muted-foreground text-xs opacity-50">default</span>
					)}
				</div>
			</div>

			<div className="flex items-center gap-1">
				<button
					type="button"
					onClick={onEdit}
					className="rounded px-2 py-1 text-muted-foreground text-xs hover:bg-secondary hover:text-foreground"
				>
					Edit
				</button>
				<button
					type="button"
					onClick={() => deleteMut.mutate()}
					disabled={deleteMut.isPending}
					className="rounded p-1 text-muted-foreground hover:text-red-500 disabled:opacity-40"
				>
					{deleteMut.isPending ? (
						<Loader2 size={12} className="animate-spin" />
					) : (
						<Trash2 size={12} />
					)}
				</button>
			</div>
		</div>
	);
}

// ─── IDE Editor Dialog ────────────────────────────────────────────────────────

function RuleEditorDialog({
	rule,
	qc,
	onClose,
}: {
	rule?: PlatformRule;
	qc: ReturnType<typeof useQueryClient>;
	onClose: () => void;
}) {
	const { theme } = useTheme();
	const monacoTheme = theme === "dark" ? "vs-dark" : "vs";
	const isEdit = !!rule;

	const [name, setName] = useState(rule?.name ?? "");
	const [key, setKey] = useState(rule?.key ?? "");
	const [ruleType, setRuleType] = useState<RuleType>((rule?.rule_type as RuleType) ?? "js");
	const [def, setDef] = useState<Record<string, unknown>>(
		(rule?.definition as Record<string, unknown>) ?? defaultDef("js"),
	);
	const [showDocs, setShowDocs] = useState(false);
	const [activeTab, setActiveTab] = useState<"prelude" | "code">("code");
	const [testResult, setTestResult] = useState<TestRuleResult | null>(null);
	const [testing, setTesting] = useState(false);
	const consoleRef = useRef<HTMLDivElement>(null);

	function handleTypeChange(t: RuleType) {
		setRuleType(t);
		setDef(defaultDef(t));
		setTestResult(null);
	}

	async function handleTest() {
		setTesting(true);
		setTestResult(null);
		try {
			const res = await client.checker.TestRule({
				rule_type: ruleType,
				definition: def as never,
			});
			setTestResult(res);
			setTimeout(() => consoleRef.current?.scrollIntoView({ behavior: "smooth" }), 100);
		} catch {
			setTestResult({ ok: false, error: "Request failed", duration_ms: 0, status_code: 0, final_url: "", body_preview: "" });
		} finally {
			setTesting(false);
		}
	}

	const saveMut = useMutation({
		mutationFn: () => {
			if (isEdit && rule) {
				const p: UpdateRuleParams = {
					name,
					enabled: rule.enabled,
					rule_type: ruleType,
					definition: def as never,
					sort_order: rule.sort_order,
				};
				return client.checker.UpdateRule(rule.id, p);
			}
			const p: CreateRuleParams = {
				name,
				key,
				enabled: true,
				rule_type: ruleType,
				definition: def as never,
				sort_order: 100,
			};
			return client.checker.CreateRule(p);
		},
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success(isEdit ? "Rule saved" : "Rule created");
			onClose();
		},
		onError: () => toast.error(isEdit ? "Failed to save" : "Failed to create"),
	});

	const canSave = name.trim() && (isEdit || key.trim());

	return (
		<div className="fixed inset-0 z-50 flex items-stretch justify-center bg-black/70 p-4">
			<div
				className="flex w-full max-w-5xl flex-col rounded-xl border border-border bg-card shadow-2xl"
				style={{ maxHeight: "94vh" }}
			>
				{/* ── Toolbar ── */}
				<div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-2.5">
					<input
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="Rule name"
						className="h-7 min-w-0 flex-1 rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
						style={{ maxWidth: 200 }}
					/>
					{!isEdit && (
						<input
							value={key}
							onChange={(e) => setKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))}
							placeholder="key"
							className="h-7 w-28 rounded border border-border bg-background px-2 font-mono text-sm focus:outline-none focus:ring-1 focus:ring-ring"
						/>
					)}
					{isEdit && (
						<span className="rounded bg-secondary px-2 py-0.5 font-mono text-muted-foreground text-xs">
							{rule?.key}
						</span>
					)}
					<select
						value={ruleType}
						onChange={(e) => handleTypeChange(e.target.value as RuleType)}
						className="h-7 rounded border border-border bg-background px-2 text-sm focus:outline-none"
					>
						{RULE_TYPES.map((t) => (
							<option key={t} value={t}>
								{RULE_TYPE_LABELS[t]}
							</option>
						))}
					</select>

					<div className="flex-1" />

					<button
						type="button"
						onClick={() => setShowDocs(!showDocs)}
						className={[
							"flex h-7 items-center gap-1.5 rounded border px-2 text-xs transition-colors",
							showDocs
								? "border-blue-500/50 bg-blue-500/10 text-blue-400"
								: "border-border text-muted-foreground hover:bg-secondary",
						].join(" ")}
					>
						<BookOpen size={11} /> Docs
					</button>

					<button
						type="button"
						onClick={handleTest}
						disabled={testing}
						className="flex h-7 items-center gap-1.5 rounded border border-emerald-500/40 bg-emerald-500/10 px-3 text-emerald-400 text-xs hover:bg-emerald-500/20 disabled:opacity-50"
					>
						{testing ? (
							<Loader2 size={11} className="animate-spin" />
						) : (
							<Play size={11} />
						)}
						{testing ? "Running…" : "Test"}
					</button>

					<button
						type="button"
						onClick={() => saveMut.mutate()}
						disabled={saveMut.isPending || !canSave}
						className="flex h-7 items-center gap-1.5 rounded px-3 text-sm text-white disabled:opacity-50"
						style={{ background: "var(--color-btn-success)" }}
					>
						{saveMut.isPending && <Loader2 size={11} className="animate-spin" />}
						{isEdit ? "Save" : "Create"}
					</button>

					<button
						type="button"
						onClick={onClose}
						className="rounded p-1 text-muted-foreground hover:text-foreground"
					>
						<X size={15} />
					</button>
				</div>

				{/* ── Main area ── */}
				<div className="flex min-h-0 flex-1 overflow-hidden">
					{/* Editor */}
					<div className="flex min-h-0 flex-1 flex-col overflow-hidden">
						{ruleType === "condition" ? (
							<div className="flex-1 overflow-y-auto p-4">
								<ConditionEditor def={def} onChange={setDef} />
							</div>
						) : (
							<ScriptEditorArea
								def={def}
								onChange={setDef}
								lang={ruleType}
								monacoTheme={monacoTheme}
								activeTab={activeTab}
								onTabChange={setActiveTab}
							/>
						)}

						{/* Console */}
						<div ref={consoleRef}>
							{(testResult || testing) && (
								<ConsolePanel result={testResult} loading={testing} />
							)}
						</div>
					</div>

					{/* Docs sidebar */}
					{showDocs && (
						<div className="w-72 flex-shrink-0 overflow-y-auto border-l border-border bg-background/50">
							<DocsPanel ruleType={ruleType} />
						</div>
					)}
				</div>
			</div>
		</div>
	);
}

// ─── Script editor with tabs ──────────────────────────────────────────────────

function ScriptEditorArea({
	def,
	onChange,
	lang,
	monacoTheme,
	activeTab,
	onTabChange,
}: {
	def: Record<string, unknown>;
	onChange: (d: Record<string, unknown>) => void;
	lang: RuleType;
	monacoTheme: string;
	activeTab: "prelude" | "code";
	onTabChange: (t: "prelude" | "code") => void;
}) {
	const monacoLang = MONACO_LANG[lang];
	const editorOpts = {
		minimap: { enabled: false },
		scrollBeyondLastLine: false,
		fontSize: 13,
		lineNumbers: "on" as const,
		wordWrap: "on" as const,
		padding: { top: 12, bottom: 12 },
	};

	const returnHint =
		lang === "tengo"
			? "Assign result to output (bool)"
			: lang === "lua"
				? "Must return true or false"
				: "Must return a boolean";

	return (
		<div className="flex min-h-0 flex-1 flex-col">
			{/* Tabs */}
			<div className="flex items-center gap-0 border-b border-border bg-secondary/30 px-3 pt-1">
				{(["prelude", "code"] as const).map((tab) => (
					<button
						key={tab}
						type="button"
						onClick={() => onTabChange(tab)}
						className={[
							"rounded-t border-b-2 px-3 py-1.5 text-xs transition-colors",
							activeTab === tab
								? "border-foreground text-foreground"
								: "border-transparent text-muted-foreground hover:text-foreground",
						].join(" ")}
					>
						{tab === "prelude" ? "Prelude" : "Code"}
						{tab === "prelude" && !!(def?.prelude) && (
							<span className="ml-1.5 h-1.5 w-1.5 rounded-full bg-blue-400 inline-block" />
						)}
					</button>
				))}
				<span className="ml-auto pb-1.5 text-muted-foreground text-xs">
					{activeTab === "prelude"
						? "Shared helpers — define functions, import modules"
						: returnHint}
				</span>
			</div>

			{/* Editor */}
			<div className="min-h-0 flex-1">
				{activeTab === "prelude" ? (
					<Editor
						height="100%"
						language={monacoLang}
						value={(def?.prelude as string) ?? ""}
						theme={monacoTheme}
						onChange={(v) => onChange({ ...def, prelude: v ?? "" })}
						options={editorOpts}
					/>
				) : (
					<Editor
						height="100%"
						language={monacoLang}
						value={(def?.code as string) ?? ""}
						theme={monacoTheme}
						onChange={(v) => onChange({ ...def, code: v ?? "" })}
						options={editorOpts}
					/>
				)}
			</div>
		</div>
	);
}

// ─── Console panel ────────────────────────────────────────────────────────────

function ConsolePanel({
	result,
	loading,
}: { result: TestRuleResult | null; loading: boolean }) {
	return (
		<div className="border-t border-border bg-[#1e1e1e] font-mono text-xs">
			<div className="flex items-center gap-2 border-b border-border/50 px-3 py-1.5">
				<span className="text-[#858585]">Console</span>
				{loading && <Loader2 size={10} className="animate-spin text-[#858585]" />}
			</div>

			<div className="space-y-0.5 px-3 py-2">
				{loading && (
					<div className="text-[#858585]">
						<span className="text-[#569cd6]">&gt;</span> Running test… (direct HTTP, no proxy)
					</div>
				)}

				{result && (
					<>
						<div className="flex items-center gap-3">
							{result.ok ? (
								<span className="text-[#4ec9b0]">✓ PASS</span>
							) : (
								<span className="text-[#f14c4c]">✗ FAIL</span>
							)}
							{result.duration_ms != null && result.duration_ms > 0 && (
								<span className="flex items-center gap-1 text-[#858585]">
									<Clock size={9} />
									{result.duration_ms}ms
								</span>
							)}
							{result.status_code != null && result.status_code > 0 && (
								<span
									className={
										result.status_code < 400 ? "text-[#4ec9b0]" : "text-[#f14c4c]"
									}
								>
									HTTP {result.status_code}
								</span>
							)}
						</div>

						{result.final_url && (
							<div className="text-[#858585]">
								<span className="text-[#569cd6]">→</span>{" "}
								<span className="text-[#9cdcfe]">{result.final_url}</span>
							</div>
						)}

						{result.error && (
							<div className="text-[#f14c4c]">
								<span className="text-[#569cd6]">!</span> {result.error}
							</div>
						)}

						{result.body_preview && (
							<div className="mt-1">
								<div className="text-[#858585]">Body preview:</div>
								<pre
									className="mt-0.5 max-h-28 overflow-auto whitespace-pre-wrap rounded bg-black/30 p-2 text-[#d4d4d4] text-xs leading-relaxed"
									style={{ wordBreak: "break-all" }}
								>
									{result.body_preview}
								</pre>
							</div>
						)}
					</>
				)}
			</div>
		</div>
	);
}

// ─── Docs panel ───────────────────────────────────────────────────────────────

function DocsPanel({ ruleType }: { ruleType: RuleType }) {
	const docs = ENGINE_DOCS[ruleType];
	const [open, setOpen] = useState<Record<string, boolean>>({});
	const toggle = (h: string) => setOpen((p) => ({ ...p, [h]: !p[h] }));

	return (
		<div className="p-3 space-y-1">
			<p className="mb-2 font-semibold text-foreground text-xs">
				{RULE_TYPE_LABELS[ruleType]} — API Reference
			</p>
			{docs.sections.map((s) => (
				<div key={s.h} className="rounded border border-border overflow-hidden">
					<button
						type="button"
						onClick={() => toggle(s.h)}
						className="flex w-full items-center justify-between px-2.5 py-1.5 text-left text-muted-foreground text-xs hover:bg-secondary/50"
					>
						<span className="font-medium text-foreground">{s.h}</span>
						{open[s.h] ? <ChevronUp size={11} /> : <ChevronDown size={11} />}
					</button>
					{open[s.h] && (
						<pre className="border-t border-border bg-secondary/30 px-2.5 py-2 font-mono text-xs leading-relaxed text-muted-foreground whitespace-pre-wrap">
							{s.body}
						</pre>
					)}
				</div>
			))}
		</div>
	);
}

// ─── Condition editor ─────────────────────────────────────────────────────────

function ConditionEditor({
	def,
	onChange,
}: {
	def: Record<string, unknown>;
	onChange: (d: Record<string, unknown>) => void;
}) {
	const set = (k: string, v: unknown) => onChange({ ...def, [k]: v });
	const listVal = (v: unknown) => (Array.isArray(v) ? (v as string[]).join(", ") : "");
	const parseList = (s: string) => s.split(",").map((x) => x.trim()).filter(Boolean);
	const inp =
		"h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring";

	return (
		<div className="space-y-3 max-w-lg">
			<FL label="URL (required)">
				<input
					value={(def?.url as string) ?? ""}
					onChange={(e) => set("url", e.target.value)}
					placeholder="https://example.com/api"
					className={inp}
				/>
			</FL>
			<div className="grid grid-cols-2 gap-3">
				<FL label="Method">
					<select
						value={(def?.method as string) ?? "GET"}
						onChange={(e) => set("method", e.target.value)}
						className={inp}
					>
						{["GET", "HEAD", "POST"].map((m) => (
							<option key={m}>{m}</option>
						))}
					</select>
				</FL>
				<FL label="Expected status (0 = any)">
					<input
						type="number"
						value={(def?.status_code as number) ?? 0}
						onChange={(e) => set("status_code", Number(e.target.value))}
						className={inp}
					/>
				</FL>
			</div>
			<FL label="Body contains ALL (comma-separated)">
				<input
					value={listVal(def?.body_contains)}
					onChange={(e) => set("body_contains", parseList(e.target.value))}
					placeholder="keyword1, keyword2"
					className={inp}
				/>
			</FL>
			<FL label="Body contains ANY">
				<input
					value={listVal(def?.body_contains_any)}
					onChange={(e) => set("body_contains_any", parseList(e.target.value))}
					placeholder="alt1, alt2"
					className={inp}
				/>
			</FL>
			<FL label="Body must NOT contain">
				<input
					value={listVal(def?.body_not_contains)}
					onChange={(e) => set("body_not_contains", parseList(e.target.value))}
					placeholder="blocked, unavailable"
					className={inp}
				/>
			</FL>
			<FL label="Final URL contains">
				<input
					value={(def?.final_url_contains as string) ?? ""}
					onChange={(e) => set("final_url_contains", e.target.value)}
					className={inp}
				/>
			</FL>
			<FL label="Final URL must NOT contain">
				<input
					value={(def?.final_url_not_contains as string) ?? ""}
					onChange={(e) => set("final_url_not_contains", e.target.value)}
					className={inp}
				/>
			</FL>
		</div>
	);
}

function FL({ label, children }: { label: string; children: React.ReactNode }) {
	return (
		<div className="space-y-1">
			<label className="text-muted-foreground text-xs">{label}</label>
			{children}
		</div>
	);
}
