import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ChevronDown, ChevronUp, Loader2, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";

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

const emptyCondition = {
	url: "",
	method: "GET",
	status_code: 0,
	body_contains: [],
	body_contains_any: [],
	body_not_contains: [],
	final_url_contains: "",
	final_url_not_contains: "",
};

const emptyScript = { code: "" };

function defaultDefinition(type: RuleType) {
	return type === "condition" ? emptyCondition : emptyScript;
}

function PlatformsPage() {
	const qc = useQueryClient();

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
				Rules run during each check to detect platform availability. Built-in
				rules are seeded automatically and can be edited. Custom rules store
				results in the node's extra platforms field.
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
						<span className="font-medium text-foreground text-sm">
							{rule.name}
						</span>
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
							<span className="text-muted-foreground text-xs opacity-60">
								default
							</span>
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
						<EditRuleForm rule={rule} qc={qc} onDone={() => setEditing(false)} />
					) : (
						<RuleDefinitionView rule={rule} onEdit={() => setEditing(true)} />
					)}
				</div>
			)}
		</div>
	);
}

function RuleDefinitionView({
	rule,
	onEdit,
}: { rule: PlatformRule; onEdit: () => void }) {
	const [testResult, setTestResult] = useState<{
		ok: boolean;
		error?: string;
	} | null>(null);
	const [testing, setTesting] = useState(false);

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

	const def = rule.definition as Record<string, any>;

	return (
		<div className="space-y-3">
			{rule.rule_type === "condition" ? (
				<div className="space-y-1 text-xs" style={{ color: "var(--color-dimmed)" }}>
					<div>
						<span className="font-medium text-foreground">URL:</span>{" "}
						{def?.url || "—"}
					</div>
					{def?.status_code ? (
						<div>
							<span className="font-medium text-foreground">Status:</span>{" "}
							{def.status_code}
						</div>
					) : null}
					{def?.body_contains?.length ? (
						<div>
							<span className="font-medium text-foreground">Contains all:</span>{" "}
							{def.body_contains.join(", ")}
						</div>
					) : null}
					{def?.body_contains_any?.length ? (
						<div>
							<span className="font-medium text-foreground">Contains any:</span>{" "}
							{def.body_contains_any.join(", ")}
						</div>
					) : null}
					{def?.body_not_contains?.length ? (
						<div>
							<span className="font-medium text-foreground">Excludes:</span>{" "}
							{def.body_not_contains.join(", ")}
						</div>
					) : null}
					{def?.final_url_contains ? (
						<div>
							<span className="font-medium text-foreground">
								Final URL contains:
							</span>{" "}
							{def.final_url_contains}
						</div>
					) : null}
					{def?.final_url_not_contains ? (
						<div>
							<span className="font-medium text-foreground">
								Final URL excludes:
							</span>{" "}
							{def.final_url_not_contains}
						</div>
					) : null}
				</div>
			) : (
				<pre className="max-h-48 overflow-auto rounded bg-secondary p-3 text-xs">
					{def?.code || ""}
				</pre>
			)}

			<div className="flex items-center gap-2">
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
					Test (direct)
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
		</div>
	);
}

function EditRuleForm({
	rule,
	qc,
	onDone,
}: {
	rule: PlatformRule;
	qc: ReturnType<typeof useQueryClient>;
	onDone: () => void;
}) {
	const [name, setName] = useState(rule.name);
	const [ruleType, setRuleType] = useState<RuleType>(
		rule.rule_type as RuleType,
	);
	const [def, setDef] = useState<Record<string, any>>(
		rule.definition as Record<string, any>,
	);

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
			<div className="flex gap-3">
				<div className="flex-1 space-y-1">
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
						className="h-7 rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
					>
						{RULE_TYPES.map((t) => (
							<option key={t} value={t}>
								{RULE_TYPE_LABELS[t]}
							</option>
						))}
					</select>
				</div>
			</div>

			{ruleType === "condition" ? (
				<ConditionEditor def={def} onChange={setDef} />
			) : (
				<ScriptEditor def={def} onChange={setDef} lang={ruleType} />
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

function ConditionEditor({
	def,
	onChange,
}: { def: Record<string, any>; onChange: (d: Record<string, any>) => void }) {
	const update = (key: string, val: unknown) =>
		onChange({ ...def, [key]: val });
	const listVal = (v: unknown) =>
		Array.isArray(v) ? (v as string[]).join(", ") : "";
	const parseList = (s: string) =>
		s
			.split(",")
			.map((x) => x.trim())
			.filter(Boolean);

	const inputCls =
		"h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring";

	return (
		<div className="space-y-2.5">
			<FieldLabel label="URL">
				<input
					value={def?.url ?? ""}
					onChange={(e) => update("url", e.target.value)}
					placeholder="https://example.com/api"
					className={inputCls}
				/>
			</FieldLabel>
			<div className="grid grid-cols-2 gap-3">
				<FieldLabel label="Method">
					<select
						value={def?.method ?? "GET"}
						onChange={(e) => update("method", e.target.value)}
						className={inputCls}
					>
						{["GET", "HEAD", "POST"].map((m) => (
							<option key={m}>{m}</option>
						))}
					</select>
				</FieldLabel>
				<FieldLabel label="Expected status (0 = any)">
					<input
						type="number"
						value={def?.status_code ?? 0}
						onChange={(e) => update("status_code", Number(e.target.value))}
						className={inputCls}
					/>
				</FieldLabel>
			</div>
			<FieldLabel label="Body contains all (comma-separated)">
				<input
					value={listVal(def?.body_contains)}
					onChange={(e) => update("body_contains", parseList(e.target.value))}
					placeholder="keyword1, keyword2"
					className={inputCls}
				/>
			</FieldLabel>
			<FieldLabel label="Body contains any (comma-separated)">
				<input
					value={listVal(def?.body_contains_any)}
					onChange={(e) =>
						update("body_contains_any", parseList(e.target.value))
					}
					placeholder="alt1, alt2"
					className={inputCls}
				/>
			</FieldLabel>
			<FieldLabel label="Body must NOT contain (comma-separated)">
				<input
					value={listVal(def?.body_not_contains)}
					onChange={(e) =>
						update("body_not_contains", parseList(e.target.value))
					}
					placeholder="blocked, unavailable"
					className={inputCls}
				/>
			</FieldLabel>
			<FieldLabel label="Final URL contains">
				<input
					value={def?.final_url_contains ?? ""}
					onChange={(e) => update("final_url_contains", e.target.value)}
					className={inputCls}
				/>
			</FieldLabel>
			<FieldLabel label="Final URL must NOT contain">
				<input
					value={def?.final_url_not_contains ?? ""}
					onChange={(e) => update("final_url_not_contains", e.target.value)}
					className={inputCls}
				/>
			</FieldLabel>
		</div>
	);
}

function ScriptEditor({
	def,
	onChange,
	lang,
}: {
	def: Record<string, any>;
	onChange: (d: Record<string, any>) => void;
	lang: RuleType;
}) {
	const placeholders: Partial<Record<RuleType, string>> = {
		js: "// http_get(url, {headers?: {}}) → {status, body, final_url}\nvar r = http_get(\"https://example.com\");\nreturn r.status === 200;",
		ts: "const r = http_get(\"https://example.com\");\nreturn r.status === 200;",
		tengo: "// Assign result to output\nr := http_get(\"https://example.com\")\noutput = r.status == 200",
		lua: "-- Return a boolean\nlocal r = http_get(\"https://example.com\")\nreturn r.status == 200",
	};

	return (
		<div className="space-y-1">
			<label className="text-muted-foreground text-xs">
				{RULE_TYPE_LABELS[lang]} code
			</label>
			<textarea
				value={def?.code ?? ""}
				onChange={(e) => onChange({ ...def, code: e.target.value })}
				placeholder={placeholders[lang]}
				rows={10}
				className="w-full rounded border border-border bg-background px-3 py-2 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-ring"
				spellCheck={false}
			/>
		</div>
	);
}

function FieldLabel({
	label,
	children,
}: { label: string; children: React.ReactNode }) {
	return (
		<div className="space-y-1">
			<label className="text-muted-foreground text-xs">{label}</label>
			{children}
		</div>
	);
}

function AddRuleDialog({ qc }: { qc: ReturnType<typeof useQueryClient> }) {
	const [open, setOpen] = useState(false);
	const [name, setName] = useState("");
	const [key, setKey] = useState("");
	const [ruleType, setRuleType] = useState<RuleType>("condition");
	const [def, setDef] = useState<Record<string, any>>(emptyCondition);

	const createMutation = useMutation({
		mutationFn: (p: CreateRuleParams) => client.checker.CreateRule(p),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["platform-rules"] });
			toast.success("Rule created");
			setOpen(false);
			setName("");
			setKey("");
			setRuleType("condition");
			setDef(emptyCondition);
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
		<div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
			<div className="w-full max-w-lg overflow-y-auto rounded-xl border border-border bg-card p-5 shadow-xl" style={{ maxHeight: "90vh" }}>
				<h2 className="mb-4 font-semibold text-foreground">
					New Platform Rule
				</h2>

				<div className="space-y-3">
					<div className="grid grid-cols-2 gap-3">
						<FieldLabel label="Name">
							<input
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="My Platform"
								className={inputCls}
							/>
						</FieldLabel>
						<FieldLabel label="Key (unique identifier)">
							<input
								value={key}
								onChange={(e) =>
									setKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))
								}
								placeholder="my_platform"
								className={`${inputCls} font-mono`}
							/>
						</FieldLabel>
					</div>
					<FieldLabel label="Type">
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
					</FieldLabel>

					{ruleType === "condition" ? (
						<ConditionEditor def={def} onChange={setDef} />
					) : (
						<ScriptEditor def={def} onChange={setDef} lang={ruleType} />
					)}
				</div>

				<div className="mt-4 flex gap-2">
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
