import { Icon as IconifyIcon } from "@iconify/react";
import Editor, { useMonaco } from "@monaco-editor/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import {
	BookOpen,
	Check,
	ChevronDown,
	ChevronRight,
	ChevronUp,
	Clock,
	Copy,
	Loader2,
	Play,
	Plus,
	Search,
	Trash2,
	X,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { isIconifyId } from "@/components/platform-icons";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";

type PlatformRule = checker.PlatformRule;
type CreateRuleParams = checker.CreateRuleParams;
type UpdateRuleParams = checker.UpdateRuleParams;
type TestRuleResult = checker.TestRuleResult;
type NodeSummary = checker.NodeSummary;

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

// ─── Icon display ─────────────────────────────────────────────────────────────

function IconDisplay({
	icon,
	name,
	size = "md",
}: { icon: string; name: string; size?: "sm" | "md" }) {
	const px = size === "sm" ? 16 : 20;
	const dim = size === "sm" ? "h-5 w-5" : "h-7 w-7";

	if (!icon) {
		return (
			<span
				className={`flex flex-shrink-0 items-center justify-center rounded bg-secondary font-medium text-muted-foreground ${dim} text-sm`}
			>
				{name.charAt(0).toUpperCase()}
			</span>
		);
	}

	if (isIconifyId(icon)) {
		return (
			<span className={`flex flex-shrink-0 items-center justify-center ${dim}`}>
				<IconifyIcon icon={icon} width={px} height={px} />
			</span>
		);
	}

	const isUrl = icon.startsWith("http://") || icon.startsWith("https://") || icon.startsWith("data:");
	if (isUrl) {
		return (
			<img
				src={icon}
				alt={name}
				className={`flex-shrink-0 rounded object-contain ${dim}`}
				onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = "none"; }}
			/>
		);
	}

	return (
		<span className={`flex flex-shrink-0 items-center justify-center ${dim} text-base`} aria-hidden>
			{icon}
		</span>
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
				icon: rule.icon,
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
			{/* Toggle: h-5/w-9 track, h-4/w-4 thumb, 2px padding, on=translate-x-[18px] = 36-16-2 */}
			<button
				type="button"
				onClick={() => toggleMut.mutate(!rule.enabled)}
				disabled={toggleMut.isPending}
				className={[
					"relative h-5 w-9 flex-shrink-0 rounded-full transition-colors",
					rule.enabled ? "bg-green-500" : "bg-muted",
				].join(" ")}
				aria-label="Toggle"
			>
				<span
					className={[
						"absolute left-0 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform",
						rule.enabled ? "translate-x-[18px]" : "translate-x-0.5",
					].join(" ")}
				/>
			</button>

			<IconDisplay icon={rule.icon} name={rule.name} />

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

// ─── Icon picker ──────────────────────────────────────────────────────────────

function IconPickerInput({
	value,
	onChange,
	name,
}: { value: string; onChange: (v: string) => void; name: string }) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [results, setResults] = useState<string[]>([]);
	const [searching, setSearching] = useState(false);
	const containerRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		function onDown(e: MouseEvent) {
			if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
				setOpen(false);
			}
		}
		document.addEventListener("mousedown", onDown);
		return () => document.removeEventListener("mousedown", onDown);
	}, []);

	useEffect(() => {
		if (!query.trim()) {
			setResults([]);
			return;
		}
		const id = setTimeout(async () => {
			setSearching(true);
			try {
				const res = await fetch(
					`https://api.iconify.design/search?query=${encodeURIComponent(query)}&limit=30`,
				);
				const data = (await res.json()) as { icons?: string[] };
				setResults(data.icons ?? []);
			} catch {
				setResults([]);
			} finally {
				setSearching(false);
			}
		}, 400);
		return () => clearTimeout(id);
	}, [query]);

	return (
		<div className="relative" ref={containerRef}>
			<div className="flex items-center gap-1">
				<IconDisplay icon={value} name={name || "?"} size="sm" />
				<input
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder="emoji, URL, or icon:name"
					className="h-7 w-36 rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
				/>
				<button
					type="button"
					onClick={() => setOpen((o) => !o)}
					className="flex h-7 w-7 items-center justify-center rounded border border-border text-muted-foreground hover:bg-secondary"
					title="Search Iconify"
				>
					<Search size={11} />
				</button>
			</div>

			{open && (
				<div className="absolute left-0 top-9 z-50 w-80 rounded-lg border border-border bg-card p-3 shadow-xl">
					<input
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						placeholder="Search icons (e.g. netflix, youtube…)"
						className="mb-2 h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
						// biome-ignore lint/a11y/noAutofocus: intentional — picker opens on button click
						autoFocus
					/>

					{searching && (
						<div className="flex justify-center py-3">
							<Loader2 size={14} className="animate-spin text-muted-foreground" />
						</div>
					)}

					{!searching && results.length > 0 && (
						<div className="grid max-h-48 grid-cols-6 gap-1 overflow-y-auto">
							{results.map((iconId) => (
								<button
									key={iconId}
									type="button"
									onClick={() => {
										onChange(iconId);
										setOpen(false);
									}}
									title={iconId}
									className="flex h-9 w-full items-center justify-center rounded hover:bg-secondary"
								>
									<IconifyIcon icon={iconId} width={20} height={20} />
								</button>
							))}
						</div>
					)}

					{!searching && results.length === 0 && query && (
						<p className="py-3 text-center text-muted-foreground text-xs">No results</p>
					)}

					{!query && (
						<p className="text-muted-foreground text-xs leading-relaxed">
							Powered by Iconify · 200k+ icons
							<br />
							<span className="text-foreground/50">
								Try: simple-icons:netflix · logos:youtube
							</span>
						</p>
					)}
				</div>
			)}
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
	const [icon, setIcon] = useState(rule?.icon ?? "");
	const [ruleType, setRuleType] = useState<RuleType>((rule?.rule_type as RuleType) ?? "js");
	const [def, setDef] = useState<Record<string, unknown>>(
		(rule?.definition as Record<string, unknown>) ?? defaultDef("js"),
	);
	const [showDocs, setShowDocs] = useState(false);
	const [activeTab, setActiveTab] = useState<"prelude" | "code">("code");
	const [testResult, setTestResult] = useState<TestRuleResult | null>(null);
	const [testing, setTesting] = useState(false);
	const [testNodeId, setTestNodeId] = useState("");
	const consoleRef = useRef<HTMLDivElement>(null);

	const nodesQuery = useQuery({
		queryKey: ["test-nodes"],
		queryFn: () => client.checker.ListTestNodes(),
		staleTime: 30_000,
	});
	const testNodes: NodeSummary[] = nodesQuery.data?.nodes ?? [];

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
				node_id: testNodeId || "",
			});
			setTestResult(res);
			setTimeout(() => consoleRef.current?.scrollIntoView({ behavior: "smooth" }), 100);
		} catch {
			setTestResult({ ok: false, error: "Request failed", duration_ms: 0, status_code: 0, final_url: "", body: "", response_headers: {}, node_name: "" });
		} finally {
			setTesting(false);
		}
	}

	const saveMut = useMutation({
		mutationFn: () => {
			if (isEdit && rule) {
				const p: UpdateRuleParams = {
					name,
					icon,
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
				icon,
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
					<IconPickerInput value={icon} onChange={setIcon} name={name} />
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

					<select
						value={testNodeId}
						onChange={(e) => setTestNodeId(e.target.value)}
						className="h-7 max-w-[160px] rounded border border-border bg-background px-2 text-xs text-muted-foreground focus:outline-none"
						title="Node to test through"
					>
						<option value="">Direct (no proxy)</option>
						{testNodes.map((n) => (
							<option key={n.id} value={n.id}>
								{n.name}
							</option>
						))}
					</select>

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
								<ConsolePanel
									result={testResult}
									loading={testing}
									nodeLabel={testResult?.node_name ?? (testNodeId ? (testNodes.find((n) => n.id === testNodeId)?.name ?? "") : "")}
								/>
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
			className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[#858585] hover:bg-white/10 hover:text-[#d4d4d4]"
		>
			{copied ? <Check size={10} /> : <Copy size={10} />}
			{copied ? "Copied" : "Copy"}
		</button>
	);
}

function ConsolePanel({
	result,
	loading,
	nodeLabel,
}: { result: TestRuleResult | null; loading: boolean; nodeLabel: string }) {
	const [headersOpen, setHeadersOpen] = useState(false);

	const headerEntries = result?.response_headers
		? Object.entries(result.response_headers).sort(([a], [b]) => a.localeCompare(b))
		: [];

	return (
		<div className="border-t border-border bg-[#1e1e1e] font-mono text-xs">
			{/* Title bar */}
			<div className="flex items-center gap-2 border-b border-white/5 px-3 py-1.5">
				<span className="text-[#858585]">Console</span>
				{loading && <Loader2 size={10} className="animate-spin text-[#858585]" />}
				{!loading && result && (
					<span className={result.ok ? "text-[#4ec9b0]" : "text-[#f14c4c]"}>
						{result.ok ? "✓ PASS" : "✗ FAIL"}
					</span>
				)}
				<span className="flex-1" />
				{nodeLabel && (
					<span className="rounded bg-white/5 px-1.5 py-0.5 text-[#858585] text-[10px]">
						via {nodeLabel}
					</span>
				)}
			</div>

			<div className="space-y-2 px-3 py-2.5">
				{loading && (
					<p className="text-[#858585]">
						<span className="text-[#569cd6]">&gt;</span>{" "}
						Running{nodeLabel ? ` through ${nodeLabel}` : " direct"}…
					</p>
				)}

				{result && (
					<>
						{/* Status row */}
						<div className="flex flex-wrap items-center gap-3">
							{result.duration_ms != null && result.duration_ms > 0 && (
								<span className="flex items-center gap-1 text-[#858585]">
									<Clock size={9} /> {result.duration_ms}ms
								</span>
							)}
							{result.status_code != null && result.status_code > 0 && (
								<span className={result.status_code < 400 ? "text-[#4ec9b0]" : "text-[#f14c4c]"}>
									HTTP {result.status_code}
								</span>
							)}
							{result.final_url && (
								<span className="text-[#858585]">
									<span className="text-[#569cd6]">→</span>{" "}
									<span className="text-[#9cdcfe]">{result.final_url}</span>
								</span>
							)}
						</div>

						{/* Error */}
						{result.error && (
							<p className="text-[#f14c4c]">
								<span className="text-[#569cd6]">!</span> {result.error}
							</p>
						)}

						{/* Response headers */}
						{headerEntries.length > 0 && (
							<div className="rounded border border-white/5">
								<button
									type="button"
									onClick={() => setHeadersOpen((o) => !o)}
									className="flex w-full items-center gap-1.5 px-2 py-1 text-left text-[#858585] hover:text-[#d4d4d4]"
								>
									<ChevronRight
										size={10}
										className={headersOpen ? "rotate-90 transition-transform" : "transition-transform"}
									/>
									<span>Response Headers</span>
									<span className="ml-1 text-[#569cd6]">{headerEntries.length}</span>
								</button>
								{headersOpen && (
									<div className="border-t border-white/5 px-2 pb-1.5">
										{headerEntries.map(([k, v]) => (
											<div key={k} className="flex gap-2 py-0.5">
												<span className="shrink-0 text-[#9cdcfe]">{k}:</span>
												<span className="break-all text-[#ce9178]">{v}</span>
											</div>
										))}
									</div>
								)}
							</div>
						)}

						{/* Body */}
						{result.body && (
							<div className="rounded border border-white/5">
								<div className="flex items-center justify-between border-b border-white/5 px-2 py-1">
									<span className="text-[#858585]">
										Body{" "}
										<span className="text-[#569cd6]">
											{result.body.length.toLocaleString()} chars
										</span>
									</span>
									<CopyButton text={result.body} />
								</div>
								<pre
									className="max-h-72 overflow-auto whitespace-pre-wrap break-all p-2 text-[#d4d4d4] leading-relaxed"
									style={{ fontSize: 11 }}
								>
									{result.body}
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
