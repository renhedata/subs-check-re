import Editor from "@monaco-editor/react";
import { MONACO_LANG, type RuleType } from "./engine";

export function ScriptEditorArea({
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
			<div className="flex items-center gap-0 border-border border-b bg-secondary/30 px-3 pt-1">
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
						{tab === "prelude" && !!def?.prelude && (
							<span className="ml-1.5 inline-block h-1.5 w-1.5 rounded-full bg-blue-400" />
						)}
					</button>
				))}
				<span className="ml-auto pb-1.5 text-muted-foreground text-xs">
					{activeTab === "prelude"
						? "Shared helpers — define functions, import modules"
						: returnHint}
				</span>
			</div>

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
