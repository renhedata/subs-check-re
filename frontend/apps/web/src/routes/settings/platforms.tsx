import Editor, { useMonaco } from "@monaco-editor/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import {
	BookOpen,
	ChevronDown,
	ChevronUp,
	Loader2,
	Plus,
	Trash2,
	X,
} from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";

type PlatformRule = checker.PlatformRule;
type CreateRuleParams = checker.CreateRuleParams;
type UpdateRuleParams = checker.UpdateRuleParams;

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

// Per-engine API documentation
const ENGINE_DOCS: Record<
	RuleType,
	{ title: string; sections: { heading: string; content: string }[] }
> = {
	condition: {
		title: "HTTP Condition — no scripting required",
		sections: [
			{
				heading: "Fields",
				content: `url                  string    — Target URL (required)
method               string    — HTTP method (default: GET)
status_code          int       — Expected status (0 = any)
body_contains        []string  — Body must contain ALL of these
body_contains_any    []string  — Body must contain ANY of these
body_not_contains    []string  — Body must contain NONE of these
final_url_contains   string    — Redirect target must contain this
final_url_not_contains string  — Redirect target must NOT contain this`,
			},
		],
	},
	js: {
		title: "JavaScript — goja ES5.1+ runtime",
		sections: [
			{
				heading: "Built-in: http_get()",
				content: `http_get(url, opts?)
  opts: { headers: { "Key": "Value" } }

Returns:
  { status: number, body: string, final_url: string }`,
			},
			{
				heading: "Available globals",
				content: `JSON        — JSON.parse, JSON.stringify
Math        — Math.abs, Math.floor, Math.random …
parseInt, parseFloat, isNaN, isFinite
encodeURIComponent, decodeURIComponent
Array, Object, String, RegExp, Date`,
			},
			{
				heading: "NOT available",
				content: `import / require / npm packages
fetch / XMLHttpRequest / Node.js APIs
async / await (use sync http_get instead)`,
			},
			{
				heading: "Prelude — define helpers once, use in Code",
				content: `// Prelude
function contains(s, sub) {
  return s.indexOf(sub) !== -1;
}

// Code
var r = http_get("https://example.com");
return contains(r.body, "Welcome");`,
			},
			{
				heading: "Return value",
				content: "Must evaluate to true (accessible) or false.",
			},
		],
	},
	ts: {
		title: "TypeScript — transpiled by esbuild then run in goja",
		sections: [
			{
				heading: "Built-in: http_get()",
				content: `declare function http_get(
  url: string,
  opts?: { headers?: Record<string, string> }
): { status: number; body: string; final_url: string };`,
			},
			{
				heading: "Supported TS features",
				content: `Type annotations, interfaces, type aliases
Generics, enums, namespaces
Arrow functions, destructuring, spread
Optional chaining (?.), nullish coalescing (??)
const / let / var`,
			},
			{
				heading: "NOT supported",
				content: `import / export statements
npm / Deno packages
async / await
Decorators`,
			},
			{
				heading: "Return value",
				content: "Must return a boolean.",
			},
		],
	},
	tengo: {
		title: "Tengo — Go-like embedded scripting language",
		sections: [
			{
				heading: "Built-in: http_get",
				content: `r := http_get("https://example.com")
r.status     // int
r.body       // string
r.final_url  // string
r.error      // string (empty on success)`,
			},
			{
				heading: "Available stdlib — import(\"name\")",
				content: `"fmt"    — fmt.sprintf, fmt.println
"math"   — math.abs, math.floor, math.sqrt, math.pow …
"text"   — text.contains, text.has_prefix, text.has_suffix,
           text.join, text.split, text.to_lower …
"times"  — times.now, times.format, times.parse …
"rand"   — rand.int, rand.float
"json"   — json.encode, json.decode
"base64" — base64.encode, base64.decode
"hex"    — hex.encode, hex.decode
"enum"   — enum.all, enum.any, enum.filter, enum.map …`,
			},
			{
				heading: "Prelude example",
				content: `// Prelude
text := import("text")
json := import("json")

// Code
r := http_get("https://api.example.com/status")
data := json.decode(r.body)
output = text.contains(data["status"], "ok")`,
			},
			{
				heading: "Result",
				content: 'Assign bool to `output` (pre-declared false):\n  output = r.status == 200',
			},
		],
	},
	lua: {
		title: "Lua 5.1 — gopher-lua runtime",
		sections: [
			{
				heading: "Built-in: http_get()",
				content: `local r = http_get("https://example.com", {
  headers = { ["User-Agent"] = "my-bot" }
})
-- r.status     number
-- r.body       string
-- r.final_url  string
-- r.error      string`,
			},
			{
				heading: "Available standard libraries",
				content: `string  — string.find, string.match, string.gsub,
          string.sub, string.len, string.format …
table   — table.insert, table.remove, table.concat …
math    — math.abs, math.floor, math.random …
os      — os.time, os.date
io      — io.write (stdout only)`,
			},
			{
				heading: "Prelude example",
				content: `-- Prelude: shared helpers
function contains(s, sub)
  return s:find(sub, 1, true) ~= nil
end

-- Code
local r = http_get("https://example.com")
return contains(r.body, "Welcome")`,
			},
			{
				heading: "Return value",
				content: "Must return true or false.",
			},
		],
	},
};

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

function defaultDefinition(type: RuleType) {
	return type === "condition" ? { ...emptyCondition } : { ...emptyScript };
}

// Register http_get() type declarations in Monaco for JS and TS
function useMonacoSetup() {
	const monaco = useMonaco();
	useEffect(() => {
		if (!monaco) return;
		const dts = `
declare function http_get(
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

// ─── Page ─────────────────────────────────────────────────────────────────────

function PlatformsPage() {
	const qc = useQueryClient();
	useMonacoSetup();

	const { data, isLoading } = useQuery({
		queryKey: ["platform-rules"],
		queryFn: () => client.checker.ListRules(),
	});

	const rules = data?.rules ?? [];

	return (
		<div className="max-w-2xl space-y-5">
			<div className="flex items-center justify-between">
				<h1 className="font-semibold text-foreground text-lg">
					Platform Detection Rules
				</h1>
				<AddRuleDialog qc={qc} />
			</div>

			<p className="text-xs" style={{ color: "var(--color-dimmed)" }}>
				Rules run during each proxy check. Built-in rules are seeded on first
				visit and can be edited. Custom rules with new keys store results in{" "}
				<code className="rounded bg-secondary px-1 font-mono">extra_platforms</code>{" "}
				per node result.
			</p>

			{isLoading ? (
				<div className="flex justify-center py-8">
					<Loader2 size={18} className="animate-spin text-muted-foreground" />
				</div>
			) : (
				<div className="space-y-2">
					{rules.map((rule) => (
						<RuleRow key={rule.id} rule={rule} qc={qc} />
					))}
					{rules.length === 0 && (
						<p className="py-6 text-center text-muted-foreground text-sm">
							No rules yet.
						</p>
					)}
				</div>
			)}
		</div>
	);
}

// ─── Rule row ─────────────────────────────────────────────────────────────────

function RuleRow({
	rule,
	qc,
}: { rule: PlatformRule; qc: ReturnType<typeof useQueryClient> }) {
	const [expanded, setExpanded] = useState(false);
	const [editing, setEditing] = useState(false);

	const deleteMutation = useMutation({
		mutationFn: () => client.checker.DeleteRule(rule.id),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success("Rule deleted");
		},
		onError: () => toast.error("Failed to delete rule"),
	});

	const toggleMutation = useMutation({
		mutationFn: (enabled: boolean) =>
			client.checker.UpdateRule(rule.id, {
				name: rule.name,
				enabled,
				rule_type: rule.rule_type,
				definition: rule.definition,
				sort_order: rule.sort_order,
			}),
		onSuccess: () => qc.invalidateQueries({ queryKey: ["platform-rules"] }),
		onError: () => toast.error("Failed to update rule"),
	});

	return (
		<div className="rounded-lg border border-border bg-card">
			<div className="flex items-center gap-3 px-4 py-3">
				<button
					type="button"
					onClick={() => toggleMutation.mutate(!rule.enabled)}
					disabled={toggleMutation.isPending}
					className={[
						"relative h-4 w-7 flex-shrink-0 rounded-full transition-colors",
						rule.enabled ? "bg-green-500" : "bg-muted",
					].join(" ")}
					aria-label="Toggle enabled"
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
							className="rounded px-1.5 py-0.5 text-xs"
							style={{
								background: "var(--color-active-bg)",
								color: "var(--color-active-border)",
							}}
						>
							{RULE_TYPE_LABELS[rule.rule_type as RuleType] ?? rule.rule_type}
						</span>
						{rule.is_default && (
							<span className="text-muted-foreground text-xs opacity-60">default</span>
						)}
					</div>
				</div>

				<div className="flex items-center gap-1">
					<button
						type="button"
						onClick={() => {
							setExpanded(!expanded);
							setEditing(false);
						}}
						className="rounded p-1 text-muted-foreground hover:text-foreground"
					>
						{expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
					</button>
					<button
						type="button"
						onClick={() => deleteMutation.mutate()}
						disabled={deleteMutation.isPending}
						className="rounded p-1 text-muted-foreground hover:text-red-500 disabled:opacity-40"
					>
						{deleteMutation.isPending ? (
							<Loader2 size={12} className="animate-spin" />
						) : (
							<Trash2 size={12} />
						)}
					</button>
				</div>
			</div>

			{expanded && (
				<div className="border-t border-border px-4 py-3">
					{editing ? (
						<EditRuleForm
							rule={rule}
							qc={qc}
							onDone={() => setEditing(false)}
						/>
					) : (
						<RuleDefinitionView
							rule={rule}
							onEdit={() => setEditing(true)}
						/>
					)}
				</div>
			)}
		</div>
	);
}

// ─── Read-only view ───────────────────────────────────────────────────────────

function RuleDefinitionView({
	rule,
	onEdit,
}: { rule: PlatformRule; onEdit: () => void }) {
	const { theme } = useTheme();
	const [testResult, setTestResult] = useState<{
		ok: boolean;
		error?: string;
	} | null>(null);
	const [testing, setTesting] = useState(false);
	const [showDocs, setShowDocs] = useState(false);

	const ruleType = rule.rule_type as RuleType;
	const def = rule.definition as Record<string, any>;
	const monacoTheme = theme === "dark" ? "vs-dark" : "vs";

	async function handleTest() {
		setTesting(true);
		setTestResult(null);
		try {
			const res = await client.checker.TestRule({
				rule_type: rule.rule_type,
				definition: rule.definition,
			});
			setTestResult(res);
		} catch {
			setTestResult({ ok: false, error: "Request failed" });
		} finally {
			setTesting(false);
		}
	}

	return (
		<div className="space-y-3">
			{ruleType === "condition" ? (
				<ConditionReadView def={def} />
			) : (
				<div className="space-y-2">
					{def?.prelude && (
						<div>
							<p className="mb-1 text-muted-foreground text-xs">Prelude</p>
							<Editor
								height={80}
								language={MONACO_LANG[ruleType]}
								value={def.prelude}
								theme={monacoTheme}
								options={{
									readOnly: true,
									minimap: { enabled: false },
									lineNumbers: "off",
									scrollBeyondLastLine: false,
									fontSize: 12,
								}}
							/>
						</div>
					)}
					<div>
						<p className="mb-1 text-muted-foreground text-xs">Code</p>
						<Editor
							height={160}
							language={MONACO_LANG[ruleType]}
							value={def?.code ?? ""}
							theme={monacoTheme}
							options={{
								readOnly: true,
								minimap: { enabled: false },
								scrollBeyondLastLine: false,
								fontSize: 12,
							}}
						/>
					</div>
				</div>
			)}

			<div className="flex flex-wrap items-center gap-2">
				<button
					type="button"
					onClick={onEdit}
					className="rounded-md border border-border px-3 py-1 text-xs hover:bg-secondary"
				>
					Edit
				</button>
				<button
					type="button"
					onClick={handleTest}
					disabled={testing}
					className="flex items-center gap-1 rounded-md border border-border px-3 py-1 text-xs hover:bg-secondary disabled:opacity-50"
				>
					{testing && <Loader2 size={10} className="animate-spin" />}
					Test (direct HTTP)
				</button>
				<button
					type="button"
					onClick={() => setShowDocs(!showDocs)}
					className="flex items-center gap-1 rounded-md border border-border px-3 py-1 text-xs hover:bg-secondary"
				>
					<BookOpen size={10} />
					{showDocs ? "Hide" : "API"} docs
				</button>
				{testResult && (
					<span
						className={
							testResult.ok ? "text-green-500 text-xs" : "text-red-500 text-xs"
						}
					>
						{testResult.ok
							? "✓ Accessible"
							: `✗ ${testResult.error || "Not accessible"}`}
					</span>
				)}
			</div>

			{showDocs && <DocsPanel ruleType={ruleType} />}
		</div>
	);
}

function ConditionReadView({ def }: { def: Record<string, any> }) {
	return (
		<div className="space-y-1 text-xs" style={{ color: "var(--color-dimmed)" }}>
			<DocRow label="URL">{def?.url || "—"}</DocRow>
			{def?.status_code ? <DocRow label="Status">{def.status_code}</DocRow> : null}
			{def?.body_contains?.length ? (
				<DocRow label="Contains all">{(def.body_contains as string[]).join(", ")}</DocRow>
			) : null}
			{def?.body_contains_any?.length ? (
				<DocRow label="Contains any">{(def.body_contains_any as string[]).join(", ")}</DocRow>
			) : null}
			{def?.body_not_contains?.length ? (
				<DocRow label="Excludes">{(def.body_not_contains as string[]).join(", ")}</DocRow>
			) : null}
			{def?.final_url_contains ? (
				<DocRow label="Final URL contains">{def.final_url_contains}</DocRow>
			) : null}
			{def?.final_url_not_contains ? (
				<DocRow label="Final URL excludes">{def.final_url_not_contains}</DocRow>
			) : null}
		</div>
	);
}

function DocRow({ label, children }: { label: string; children: React.ReactNode }) {
	return (
		<div>
			<span className="font-medium text-foreground">{label}:</span> {children}
		</div>
	);
}

// ─── Docs panel ───────────────────────────────────────────────────────────────

function DocsPanel({ ruleType }: { ruleType: RuleType }) {
	const docs = ENGINE_DOCS[ruleType];
	return (
		<div className="rounded-lg border border-border bg-secondary/40 p-4 space-y-3">
			<p className="font-semibold text-foreground text-xs">{docs.title}</p>
			{docs.sections.map((s) => (
				<div key={s.heading} className="space-y-1">
					<p className="font-medium text-muted-foreground text-xs">{s.heading}</p>
					<pre
						className="whitespace-pre-wrap rounded bg-background p-2.5 font-mono text-xs leading-relaxed"
						style={{ color: "var(--color-dimmed)" }}
					>
						{s.content}
					</pre>
				</div>
			))}
		</div>
	);
}

// ─── Edit form ────────────────────────────────────────────────────────────────

function EditRuleForm({
	rule,
	qc,
	onDone,
}: {
	rule: PlatformRule;
	qc: ReturnType<typeof useQueryClient>;
	onDone: () => void;
}) {
	const { theme } = useTheme();
	const [name, setName] = useState(rule.name);
	const [ruleType, setRuleType] = useState<RuleType>(rule.rule_type as RuleType);
	const [def, setDef] = useState<Record<string, any>>(
		rule.definition as Record<string, any>,
	);
	const [showDocs, setShowDocs] = useState(false);

	const updateMutation = useMutation({
		mutationFn: (p: UpdateRuleParams) =>
			client.checker.UpdateRule(rule.id, p),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success("Rule saved");
			onDone();
		},
		onError: () => toast.error("Failed to save rule"),
	});

	function handleTypeChange(t: RuleType) {
		setRuleType(t);
		setDef(defaultDefinition(t));
	}

	return (
		<div className="space-y-3">
			<div className="flex flex-wrap gap-3">
				<div className="flex-1 space-y-1" style={{ minWidth: 160 }}>
					<label className="text-muted-foreground text-xs">Name</label>
					<input
						value={name}
						onChange={(e) => setName(e.target.value)}
						className="h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
					/>
				</div>
				<div className="space-y-1">
					<label className="text-muted-foreground text-xs">Type</label>
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
				</div>
				<div className="flex items-end">
					<button
						type="button"
						onClick={() => setShowDocs(!showDocs)}
						className="flex h-7 items-center gap-1 rounded border border-border px-2 text-xs hover:bg-secondary"
					>
						<BookOpen size={11} />
						Docs
					</button>
				</div>
			</div>

			{showDocs && <DocsPanel ruleType={ruleType} />}

			{ruleType === "condition" ? (
				<ConditionEditor def={def} onChange={setDef} />
			) : (
				<ScriptEditor
					def={def}
					onChange={setDef}
					lang={ruleType}
					monacoTheme={theme === "dark" ? "vs-dark" : "vs"}
				/>
			)}

			<div className="flex gap-2">
				<button
					type="button"
					onClick={() =>
						updateMutation.mutate({
							name,
							enabled: rule.enabled,
							rule_type: ruleType,
							definition: def,
							sort_order: rule.sort_order,
						})
					}
					disabled={updateMutation.isPending}
					className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white disabled:opacity-50"
					style={{ background: "var(--color-btn-success)" }}
				>
					{updateMutation.isPending && (
						<Loader2 size={12} className="animate-spin" />
					)}
					Save
				</button>
				<button
					type="button"
					onClick={onDone}
					className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-secondary"
				>
					Cancel
				</button>
			</div>
		</div>
	);
}

// ─── Script editor (Monaco) ───────────────────────────────────────────────────

function ScriptEditor({
	def,
	onChange,
	lang,
	monacoTheme,
}: {
	def: Record<string, any>;
	onChange: (d: Record<string, any>) => void;
	lang: RuleType;
	monacoTheme: string;
}) {
	const [showPrelude, setShowPrelude] = useState(!!(def?.prelude));
	const monacoLang = MONACO_LANG[lang];

	const editorOpts = {
		minimap: { enabled: false },
		scrollBeyondLastLine: false,
		fontSize: 12,
		lineNumbers: "on" as const,
		wordWrap: "on" as const,
	};

	return (
		<div className="space-y-2">
			{/* Prelude toggle — Windmill-style "shared helpers" section */}
			<div className="flex items-center gap-2">
				<button
					type="button"
					onClick={() => {
						const next = !showPrelude;
						setShowPrelude(next);
						if (!next) onChange({ ...def, prelude: "" });
					}}
					className={[
						"rounded border px-2 py-0.5 text-xs transition-colors",
						showPrelude
							? "border-blue-500/60 bg-blue-500/10 text-blue-400"
							: "border-border text-muted-foreground hover:bg-secondary",
					].join(" ")}
				>
					{showPrelude ? "− Prelude" : "+ Prelude"}
				</button>
				<span className="text-muted-foreground text-xs">
					{showPrelude
						? "Shared helpers available to main script — define functions, import modules"
						: "Add shared helper code / imports that run before the main script"}
				</span>
			</div>

			{showPrelude && (
				<div className="space-y-1">
					<label className="text-muted-foreground text-xs">
						Prelude — runs first, defines helpers &amp; imports
					</label>
					<div className="overflow-hidden rounded border border-dashed border-border">
						<Editor
							height={120}
							language={monacoLang}
							value={def?.prelude ?? ""}
							theme={monacoTheme}
							onChange={(v) => onChange({ ...def, prelude: v ?? "" })}
							options={editorOpts}
						/>
					</div>
				</div>
			)}

			<div className="space-y-1">
				<label className="text-muted-foreground text-xs">
					{lang === "tengo"
						? "Script — assign result to `output` (bool)"
						: "Script — must return a boolean"}
				</label>
				<div className="overflow-hidden rounded border border-border">
					<Editor
						height={220}
						language={monacoLang}
						value={def?.code ?? ""}
						theme={monacoTheme}
						onChange={(v) => onChange({ ...def, code: v ?? "" })}
						options={editorOpts}
					/>
				</div>
			</div>
		</div>
	);
}

// ─── Condition editor (form) ──────────────────────────────────────────────────

function ConditionEditor({
	def,
	onChange,
}: {
	def: Record<string, any>;
	onChange: (d: Record<string, any>) => void;
}) {
	const update = (key: string, val: unknown) => onChange({ ...def, [key]: val });
	const listVal = (v: unknown) =>
		Array.isArray(v) ? (v as string[]).join(", ") : "";
	const parseList = (s: string) =>
		s.split(",").map((x) => x.trim()).filter(Boolean);
	const inputCls =
		"h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring";

	return (
		<div className="space-y-2.5">
			<FL label="URL (required)">
				<input
					value={def?.url ?? ""}
					onChange={(e) => update("url", e.target.value)}
					placeholder="https://example.com/api"
					className={inputCls}
				/>
			</FL>
			<div className="grid grid-cols-2 gap-3">
				<FL label="Method">
					<select
						value={def?.method ?? "GET"}
						onChange={(e) => update("method", e.target.value)}
						className={inputCls}
					>
						{["GET", "HEAD", "POST"].map((m) => (
							<option key={m}>{m}</option>
						))}
					</select>
				</FL>
				<FL label="Expected status (0 = any)">
					<input
						type="number"
						value={def?.status_code ?? 0}
						onChange={(e) => update("status_code", Number(e.target.value))}
						className={inputCls}
					/>
				</FL>
			</div>
			<FL label="Body contains ALL (comma-separated)">
				<input
					value={listVal(def?.body_contains)}
					onChange={(e) => update("body_contains", parseList(e.target.value))}
					placeholder="keyword1, keyword2"
					className={inputCls}
				/>
			</FL>
			<FL label="Body contains ANY (comma-separated)">
				<input
					value={listVal(def?.body_contains_any)}
					onChange={(e) => update("body_contains_any", parseList(e.target.value))}
					placeholder="alt1, alt2"
					className={inputCls}
				/>
			</FL>
			<FL label="Body must NOT contain (comma-separated)">
				<input
					value={listVal(def?.body_not_contains)}
					onChange={(e) => update("body_not_contains", parseList(e.target.value))}
					placeholder="blocked, unavailable"
					className={inputCls}
				/>
			</FL>
			<FL label="Final URL contains">
				<input
					value={def?.final_url_contains ?? ""}
					onChange={(e) => update("final_url_contains", e.target.value)}
					className={inputCls}
				/>
			</FL>
			<FL label="Final URL must NOT contain">
				<input
					value={def?.final_url_not_contains ?? ""}
					onChange={(e) => update("final_url_not_contains", e.target.value)}
					className={inputCls}
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

// ─── Add rule dialog ──────────────────────────────────────────────────────────

function AddRuleDialog({ qc }: { qc: ReturnType<typeof useQueryClient> }) {
	const { theme } = useTheme();
	const [open, setOpen] = useState(false);
	const [name, setName] = useState("");
	const [key, setKey] = useState("");
	const [ruleType, setRuleType] = useState<RuleType>("js");
	const [def, setDef] = useState<Record<string, any>>({ ...emptyScript });
	const [showDocs, setShowDocs] = useState(false);

	const createMutation = useMutation({
		mutationFn: (p: CreateRuleParams) => client.checker.CreateRule(p),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success("Rule created");
			setOpen(false);
			setName("");
			setKey("");
			setRuleType("js");
			setDef({ ...emptyScript });
		},
		onError: () => toast.error("Failed to create rule"),
	});

	function handleTypeChange(t: RuleType) {
		setRuleType(t);
		setDef(defaultDefinition(t));
	}

	if (!open) {
		return (
			<button
				type="button"
				onClick={() => setOpen(true)}
				className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-white"
				style={{ background: "var(--color-btn-success)" }}
			>
				<Plus size={13} />
				Add Rule
			</button>
		);
	}

	const inputCls =
		"h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring";

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
			<div
				className="flex w-full max-w-2xl flex-col rounded-xl border border-border bg-card shadow-2xl"
				style={{ maxHeight: "92vh" }}
			>
				{/* Header */}
				<div className="flex items-center justify-between border-b border-border px-5 py-3">
					<h2 className="font-semibold text-foreground">New Platform Rule</h2>
					<button
						type="button"
						onClick={() => setOpen(false)}
						className="text-muted-foreground hover:text-foreground"
					>
						<X size={16} />
					</button>
				</div>

				{/* Body */}
				<div className="flex-1 space-y-4 overflow-y-auto p-5">
					<div className="grid grid-cols-3 gap-3">
						<FL label="Name">
							<input
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="My Platform"
								className={inputCls}
							/>
						</FL>
						<FL label="Key (unique identifier)">
							<input
								value={key}
								onChange={(e) =>
									setKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))
								}
								placeholder="my_platform"
								className={`${inputCls} font-mono`}
							/>
						</FL>
						<FL label="Type">
							<select
								value={ruleType}
								onChange={(e) => handleTypeChange(e.target.value as RuleType)}
								className={inputCls}
							>
								{RULE_TYPES.map((t) => (
									<option key={t} value={t}>
										{RULE_TYPE_LABELS[t]}
									</option>
								))}
							</select>
						</FL>
					</div>

					<div className="flex justify-end">
						<button
							type="button"
							onClick={() => setShowDocs(!showDocs)}
							className="flex items-center gap-1 text-muted-foreground text-xs hover:text-foreground"
						>
							<BookOpen size={11} />
							{showDocs ? "Hide" : "Show"} API docs for{" "}
							{RULE_TYPE_LABELS[ruleType]}
						</button>
					</div>

					{showDocs && <DocsPanel ruleType={ruleType} />}

					{ruleType === "condition" ? (
						<ConditionEditor def={def} onChange={setDef} />
					) : (
						<ScriptEditor
							def={def}
							onChange={setDef}
							lang={ruleType}
							monacoTheme={theme === "dark" ? "vs-dark" : "vs"}
						/>
					)}
				</div>

				{/* Footer */}
				<div className="flex gap-2 border-t border-border px-5 py-3">
					<button
						type="button"
						onClick={() =>
							createMutation.mutate({
								name,
								key,
								enabled: true,
								rule_type: ruleType,
								definition: def,
								sort_order: 100,
							})
						}
						disabled={createMutation.isPending || !name || !key}
						className="flex items-center gap-1.5 rounded-md px-4 py-1.5 text-sm text-white disabled:opacity-50"
						style={{ background: "var(--color-btn-success)" }}
					>
						{createMutation.isPending && (
							<Loader2 size={12} className="animate-spin" />
						)}
						Create
					</button>
					<button
						type="button"
						onClick={() => setOpen(false)}
						className="rounded-md border border-border px-4 py-1.5 text-sm hover:bg-secondary"
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	);
}
