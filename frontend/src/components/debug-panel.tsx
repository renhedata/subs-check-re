import { ChevronDown, ChevronRight } from "lucide-react";
import { useState } from "react";

export interface DebugStep {
	type:
		| "http_request"
		| "http_response"
		| "variable"
		| "condition"
		| "log"
		| "error";
	description: string;
	details: Record<string, unknown>;
}

export interface DebugTrace {
	platform: string;
	result: boolean;
	steps: DebugStep[];
}

export interface NodeDebug {
	node_id: string;
	node_name: string;
	traces: DebugTrace[];
}

function DebugStepView({ step }: { step: DebugStep }) {
	const [open, setOpen] = useState(false);

	return (
		<div className="border-border border-l-2 py-1 pl-3 font-mono text-[11px]">
			<div className="flex items-center gap-2">
				<span
					className="rounded px-1 py-0.5 font-medium text-[10px] uppercase"
					style={{
						background:
							step.type === "error"
								? "var(--color-badge-danger-bg)"
								: step.type === "http_request" || step.type === "http_response"
									? "var(--color-badge-info-bg)"
									: step.type === "variable"
										? "var(--color-badge-success-bg)"
										: step.type === "condition"
											? "var(--color-badge-warning-bg)"
											: "transparent",
						color:
							step.type === "error"
								? "var(--destructive)"
								: step.type === "http_request" || step.type === "http_response"
									? "var(--color-badge-info)"
									: step.type === "variable"
										? "var(--color-badge-success)"
										: step.type === "condition"
											? "var(--legacy-warning)"
											: "var(--muted-foreground)",
					}}
				>
					{step.type === "http_request"
						? "REQ"
						: step.type === "http_response"
							? "RES"
							: step.type === "variable"
								? "VAR"
								: step.type === "condition"
									? "IF"
									: step.type === "log"
										? "LOG"
										: step.type === "error"
											? "ERR"
											: step.type}
				</span>
				<span style={{ color: "var(--color-code)" }}>{step.description}</span>
				{Object.keys(step.details).length > 0 && (
					<button
						type="button"
						onClick={() => setOpen(!open)}
						className="ml-auto flex-shrink-0 rounded p-0.5 hover:bg-white/5"
					>
						{open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
					</button>
				)}
			</div>
			{open && Object.keys(step.details).length > 0 && (
				<div
					className="mt-1 ml-4 space-y-0.5 text-[10px]"
					style={{ color: "var(--color-dimmed)" }}
				>
					{Object.entries(step.details).map(([k, v]) => (
						<div key={k} className="flex gap-2">
							<span className="flex-shrink-0">{k}:</span>
							<span
								className="break-all"
								style={{ color: "var(--color-code)" }}
							>
								{typeof v === "string" ? v : JSON.stringify(v)}
							</span>
						</div>
					))}
				</div>
			)}
		</div>
	);
}

function DebugPlatformEntry({ trace }: { trace: DebugTrace }) {
	const [open, setOpen] = useState(false);

	return (
		<div>
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs transition-colors hover:bg-white/5"
			>
				{open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
				<span className="font-medium">{trace.platform}</span>
				<span
					className="ml-auto rounded px-1.5 py-0.5 font-medium text-[10px]"
					style={{
						background: trace.result
							? "var(--color-badge-success-bg)"
							: "var(--color-badge-danger-bg)",
						color: trace.result
							? "var(--color-badge-success)"
							: "var(--color-badge-danger)",
					}}
				>
					{trace.result ? "✓ UNLOCKED" : "✗ BLOCKED"}
				</span>
			</button>
			{open && (
				<div className="ml-3 space-y-0.5">
					{trace.steps.map((step, i) => (
						<DebugStepView key={i} step={step} />
					))}
				</div>
			)}
		</div>
	);
}

function DebugNodeEntry({ node }: { node: NodeDebug }) {
	const [open, setOpen] = useState(false);

	return (
		<div className="rounded border border-border">
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="flex w-full items-center gap-2 px-3 py-2 text-left font-medium text-xs transition-colors hover:bg-white/5"
			>
				{open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
				<span style={{ color: "var(--color-code)" }}>{node.node_name}</span>
				<span className="text-muted-foreground">
					({node.traces.length} platforms)
				</span>
			</button>
			{open && (
				<div className="border-border border-t pt-1 pb-1">
					{node.traces.map((trace) => (
						<DebugPlatformEntry key={trace.platform} trace={trace} />
					))}
				</div>
			)}
		</div>
	);
}

export function DebugPanel({ data }: { data: NodeDebug[] }) {
	const [open, setOpen] = useState(true);

	if (data.length === 0) return null;

	return (
		<div className="rounded-lg border border-border bg-card">
			<button
				type="button"
				onClick={() => setOpen(!open)}
				className="flex w-full items-center gap-2 px-3 py-2.5 text-left font-medium text-sm transition-colors hover:bg-white/[0.02]"
			>
				{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
				<span>Debug</span>
				<span className="font-normal text-muted-foreground text-xs">
					({data.length} nodes, {data.reduce((n, d) => n + d.traces.length, 0)}{" "}
					traces)
				</span>
			</button>
			{open && (
				<div
					className="max-h-[600px] space-y-1.5 overflow-y-auto border-border border-t p-2 text-xs"
					style={{ scrollbarWidth: "thin" }}
				>
					{data.map((node) => (
						<DebugNodeEntry key={node.node_id || node.node_name} node={node} />
					))}
				</div>
			)}
		</div>
	);
}
