import { ChevronDown, ChevronUp } from "lucide-react";
import { useState } from "react";
import { ENGINE_DOCS, RULE_TYPE_LABELS, type RuleType } from "./engine";

export function DocsPanel({ ruleType }: { ruleType: RuleType }) {
	const docs = ENGINE_DOCS[ruleType];
	const [open, setOpen] = useState<Record<string, boolean>>({});
	const toggle = (h: string) =>
		setOpen((p) => ({ ...p, [h]: !(p[h] ?? true) }));

	return (
		<div className="space-y-1 p-3">
			<p className="mb-2 font-semibold text-foreground text-xs">
				{RULE_TYPE_LABELS[ruleType]} — API Reference
			</p>
			{docs.sections.map((s) => {
				const isOpen = open[s.h] ?? true; // sections expanded by default
				return (
					<div
						key={s.h}
						className="overflow-hidden rounded border border-border"
					>
						<button
							type="button"
							onClick={() => toggle(s.h)}
							className="flex w-full items-center justify-between px-2.5 py-1.5 text-left text-muted-foreground text-xs hover:bg-secondary/50"
						>
							<span className="font-medium text-foreground">{s.h}</span>
							{isOpen ? <ChevronUp size={11} /> : <ChevronDown size={11} />}
						</button>
						{isOpen && (
							<pre className="whitespace-pre-wrap border-border border-t bg-secondary/30 px-2.5 py-2 font-mono text-muted-foreground text-xs leading-relaxed">
								{s.body}
							</pre>
						)}
					</div>
				);
			})}
		</div>
	);
}
