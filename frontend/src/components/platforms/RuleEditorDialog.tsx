import { BookOpen, Loader2, Play, X } from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";
import { useCreateRule, useTestNodes, useUpdateRule } from "@/queries";
import { ConditionEditor } from "./ConditionEditor";
import { ConsolePanel } from "./ConsolePanel";
import { DocsPanel } from "./DocsPanel";
import {
	defaultDef,
	RULE_TYPE_LABELS,
	RULE_TYPES,
	type RuleType,
} from "./engine";
import { IconPickerInput } from "./IconPicker";
import { ScriptEditorArea } from "./ScriptEditorArea";

type PlatformRule = checker.PlatformRule;
type CreateRuleParams = checker.CreateRuleParams;
type UpdateRuleParams = checker.UpdateRuleParams;
type TestRuleResult = checker.TestRuleResult;
type NodeSummary = checker.NodeSummary;

export function RuleEditorDialog({
	rule,
	onClose,
}: {
	rule?: PlatformRule;
	onClose: () => void;
}) {
	const { theme } = useTheme();
	const monacoTheme = theme === "dark" ? "vs-dark" : "vs";
	const isEdit = !!rule;

	const [name, setName] = useState(rule?.name ?? "");
	const [key, setKey] = useState(rule?.key ?? "");
	const [icon, setIcon] = useState(rule?.icon ?? "");
	const [ruleType, setRuleType] = useState<RuleType>(
		(rule?.rule_type as RuleType) ?? "js",
	);
	const [def, setDef] = useState<Record<string, unknown>>(
		(rule?.definition as Record<string, unknown>) ?? defaultDef("js"),
	);
	const [showDocs, setShowDocs] = useState(false);
	const [activeTab, setActiveTab] = useState<"prelude" | "code">("code");
	const [testResult, setTestResult] = useState<TestRuleResult | null>(null);
	const [testing, setTesting] = useState(false);
	const [testNodeId, setTestNodeId] = useState("");
	const consoleRef = useRef<HTMLDivElement>(null);

	const nodesQuery = useTestNodes();
	const testNodes: NodeSummary[] = nodesQuery.data?.nodes ?? [];

	const createMut = useCreateRule();
	const updateMut = useUpdateRule();
	const saveMut = isEdit ? updateMut : createMut;

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
			setTimeout(
				() => consoleRef.current?.scrollIntoView({ behavior: "smooth" }),
				100,
			);
		} catch {
			setTestResult({
				ok: false,
				error: "Request failed",
				duration_ms: 0,
				status_code: 0,
				final_url: "",
				body: "",
				response_headers: {},
				node_name: "",
				trace: { platform: "", result: false, steps: [] },
			});
		} finally {
			setTesting(false);
		}
	}

	function handleSave() {
		const successMsg = isEdit ? "Rule saved" : "Rule created";
		const errorMsg = isEdit ? "Failed to save" : "Failed to create";
		const onSuccess = () => {
			toast.success(successMsg);
			onClose();
		};
		const onError = () => toast.error(errorMsg);

		if (isEdit && rule) {
			const p: UpdateRuleParams = {
				name,
				icon,
				enabled: rule.enabled,
				rule_type: ruleType,
				definition: def as never,
				sort_order: rule.sort_order,
			};
			updateMut.mutate({ id: rule.id, params: p }, { onSuccess, onError });
		} else {
			const p: CreateRuleParams = {
				name,
				key,
				icon,
				enabled: true,
				rule_type: ruleType,
				definition: def as never,
				sort_order: 100,
			};
			createMut.mutate(p, { onSuccess, onError });
		}
	}

	const canSave = name.trim() && (isEdit || key.trim());

	return (
		<div className="fixed inset-0 z-50 flex items-stretch justify-center bg-black/70 p-0 md:p-4">
			<div className="flex h-full max-h-screen w-full max-w-none flex-col rounded-none border border-border bg-card shadow-2xl md:h-auto md:max-h-[94vh] md:max-w-5xl md:rounded-xl">
				<div className="flex flex-wrap items-center gap-2 border-border border-b px-4 py-2.5">
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
							onChange={(e) =>
								setKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))
							}
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
						className="h-7 max-w-[160px] rounded border border-border bg-background px-2 text-muted-foreground text-xs focus:outline-none"
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

					<Button
						variant="success"
						size="sm"
						onClick={handleSave}
						disabled={saveMut.isPending || !canSave}
					>
						{saveMut.isPending && (
							<Loader2 size={11} className="animate-spin" />
						)}
						{isEdit ? "Save" : "Create"}
					</Button>

					<button
						type="button"
						onClick={onClose}
						className="rounded p-1 text-muted-foreground hover:text-foreground"
					>
						<X size={15} />
					</button>
				</div>

				<div className="flex min-h-0 flex-1 flex-col overflow-hidden md:flex-row">
					<div className="flex min-h-0 flex-1 flex-col overflow-hidden max-md:min-h-[280px]">
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

						<div ref={consoleRef}>
							{(testResult || testing) && (
								<ConsolePanel
									result={testResult}
									loading={testing}
									nodeLabel={
										testResult?.node_name ??
										(testNodeId
											? (testNodes.find((n) => n.id === testNodeId)?.name ?? "")
											: "")
									}
								/>
							)}
						</div>
					</div>

					{showDocs && (
						<div className="max-h-[40vh] w-full flex-shrink-0 overflow-y-auto border-border border-t bg-background/50 md:max-h-none md:w-72 md:border-t-0 md:border-l">
							<DocsPanel ruleType={ruleType} />
						</div>
					)}
				</div>
			</div>
		</div>
	);
}
