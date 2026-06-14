import { ChevronLeft, Loader2, Play, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";
import { IconPickerPopover } from "@/components/platforms/IconPickerPopover";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { client } from "@/lib/client";
import type { checker } from "@/lib/client.gen";
import { useTheme } from "@/lib/theme";
import {
	useCreateRule,
	useDeleteRule,
	useResetRule,
	useTestNodes,
	useUpdateRule,
} from "@/queries";
import { cn } from "@/lib/utils";
import { ConditionEditor } from "./ConditionEditor";
import { ConsolePanel } from "./ConsolePanel";
import {
	defaultDef,
	RULE_TYPE_LABELS,
	RULE_TYPES,
	type RuleType,
} from "./engine";
import { ScriptEditorArea } from "./ScriptEditorArea";

type PlatformRule = checker.PlatformRule;
type TestRuleResult = checker.TestRuleResult;

export function RuleInspector({
	rule,
	onClose,
	onMobileBack,
}: {
	// rule === undefined => draft (create); else edit
	rule?: PlatformRule;
	onClose: () => void;
	onMobileBack?: () => void;
}) {
	const isEdit = !!rule;
	const { theme } = useTheme();
	const monacoTheme = theme === "dark" ? "vs-dark" : "vs";

	const [name, setName] = useState(rule?.name ?? "");
	const [ruleKey, setRuleKey] = useState(rule?.key ?? "");
	const [icon, setIcon] = useState(rule?.icon ?? "");
	const [enabled, setEnabled] = useState(rule?.enabled ?? true);
	const [ruleType, setRuleType] = useState<RuleType>(
		(rule?.rule_type as RuleType) ?? "js",
	);
	const [def, setDef] = useState<Record<string, unknown>>(
		(rule?.definition as Record<string, unknown>) ?? defaultDef("js"),
	);
	const [activeTab, setActiveTab] = useState<"prelude" | "code">("code");
	const [testResult, setTestResult] = useState<TestRuleResult | null>(null);
	const [testing, setTesting] = useState(false);
	const [testNodeId, setTestNodeId] = useState("");
	const consoleRef = useRef<HTMLDivElement>(null);

	const testNodes = useTestNodes().data?.nodes ?? [];
	const createMut = useCreateRule();
	const updateMut = useUpdateRule();
	const deleteMut = useDeleteRule();
	const resetMut = useResetRule();
	const saving = createMut.isPending || updateMut.isPending;

	function changeType(t: RuleType) {
		setRuleType(t);
		setDef(defaultDef(t));
		setTestResult(null);
	}

	async function runTest() {
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
				80,
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

	function save() {
		const onSuccess = () => {
			toast.success(isEdit ? "Rule saved" : "Rule created");
			onClose();
		};
		const onError = () => toast.error(isEdit ? "Failed to save" : "Failed to create");
		if (isEdit && rule) {
			updateMut.mutate(
				{
					id: rule.id,
					params: {
						name,
						icon,
						enabled,
						rule_type: ruleType,
						definition: def as never,
						sort_order: rule.sort_order,
					},
				},
				{ onSuccess, onError },
			);
		} else {
			createMut.mutate(
				{
					name,
					key: ruleKey,
					icon,
					enabled,
					rule_type: ruleType,
					definition: def as never,
					sort_order: 1000,
				},
				{ onSuccess, onError },
			);
		}
	}

	function remove() {
		if (!rule) return;
		if (!confirm(`Delete rule "${rule.name}"?`)) return;
		deleteMut.mutate(rule.id, {
			onSuccess: () => {
				toast.success("Rule deleted");
				onClose();
			},
			onError: () => toast.error("Failed to delete"),
		});
	}

	const canSave = name.trim() && (isEdit || ruleKey.trim());

	return (
		<div className="flex h-full min-w-0 flex-col">
			{/* identity header */}
			<div className="flex items-center gap-3 border-border border-b px-4 py-3">
				{onMobileBack && (
					<button
						type="button"
						onClick={onMobileBack}
						className="-ml-1 rounded p-1 text-muted-foreground hover:text-foreground lg:hidden"
						aria-label="Back to list"
					>
						<ChevronLeft size={18} />
					</button>
				)}
				<IconPickerPopover value={icon} onChange={setIcon} name={name} size={40} />
				<div className="min-w-0">
					<input
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="Rule name"
						className="w-full bg-transparent font-semibold text-base focus:outline-none"
					/>
					<div className="mt-0.5">
						{isEdit ? (
							<span className="rounded bg-secondary px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
								{rule?.key}
							</span>
						) : (
							<input
								value={ruleKey}
								onChange={(e) =>
									setRuleKey(e.target.value.toLowerCase().replace(/\s+/g, "_"))
								}
								placeholder="key"
								className="h-6 w-32 rounded border border-border bg-background px-1.5 font-mono text-[11px] focus:outline-none focus:ring-1 focus:ring-ring"
							/>
						)}
					</div>
				</div>
				<div className="ml-auto flex items-center gap-2.5">
					<div className="flex rounded-lg border border-border bg-background p-0.5">
						{RULE_TYPES.map((t) => (
							<button
								key={t}
								type="button"
								onClick={() => changeType(t)}
								className={cn(
									"rounded-md px-2 py-1 text-[11px] transition-colors",
									ruleType === t
										? "bg-secondary text-foreground"
										: "text-muted-foreground hover:text-foreground",
								)}
							>
								{RULE_TYPE_LABELS[t]}
							</button>
						))}
					</div>
					<Switch checked={enabled} onCheckedChange={(v) => setEnabled(v === true)} />
					{isEdit && (
						<button
							type="button"
							onClick={remove}
							className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-danger"
							aria-label="Delete rule"
						>
							<Trash2 size={15} />
						</button>
					)}
				</div>
			</div>

			{/* definition */}
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
			</div>

			{/* test */}
			<div className="border-border border-t bg-card/40">
				<div className="flex items-center gap-2 px-4 py-2.5">
					<select
						value={testNodeId}
						onChange={(e) => setTestNodeId(e.target.value)}
						className="h-7 max-w-[170px] rounded-lg border border-border bg-background px-2 text-muted-foreground text-xs focus:outline-none"
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
						onClick={runTest}
						disabled={testing}
						className="flex h-7 items-center gap-1.5 rounded-lg bg-primary px-3 font-medium text-primary-foreground text-xs hover:opacity-90 disabled:opacity-50"
					>
						{testing ? <Loader2 size={11} className="animate-spin" /> : <Play size={11} />}
						{testing ? "Running…" : "Run test"}
					</button>
				</div>
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

			{/* footer */}
			<div className="flex items-center gap-2 border-border border-t px-4 py-3">
				{isEdit && rule?.is_default && rule.customized && (
					<>
						<span className="rounded bg-warning-muted px-1.5 py-0.5 text-[11px] text-warning">
							Modified
						</span>
						<button
							type="button"
							onClick={() =>
								resetMut.mutate(rule.id, {
									onSuccess: () => {
										toast.success("Reset to default");
										onClose();
									},
									onError: () => toast.error("Failed to reset"),
								})
							}
							disabled={resetMut.isPending}
							className="rounded-md border border-border px-2.5 py-1 text-muted-foreground text-xs hover:bg-secondary disabled:opacity-50"
						>
							Reset to default
						</button>
					</>
				)}
				<Button
					variant="success"
					size="sm"
					className="ml-auto"
					onClick={save}
					disabled={saving || !canSave}
				>
					{saving && <Loader2 size={11} className="animate-spin" />}
					{isEdit ? "Save changes" : "Create rule"}
				</Button>
			</div>
		</div>
	);
}
