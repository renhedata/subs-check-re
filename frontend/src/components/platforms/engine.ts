import { useMonaco } from "@monaco-editor/react";
import { useEffect } from "react";

export const RULE_TYPES = ["condition", "js", "ts", "tengo", "lua"] as const;
export type RuleType = (typeof RULE_TYPES)[number];

export const RULE_TYPE_LABELS: Record<RuleType, string> = {
	condition: "Condition",
	js: "JavaScript",
	ts: "TypeScript",
	tengo: "Tengo",
	lua: "Lua",
};

export const MONACO_LANG: Record<RuleType, string> = {
	condition: "plaintext",
	js: "javascript",
	ts: "typescript",
	tengo: "go",
	lua: "lua",
};

export const TYPE_COLORS: Record<RuleType, string> = {
	condition: "bg-blue-500/10 text-blue-400 border-blue-500/30",
	js: "bg-yellow-500/10 text-yellow-400 border-yellow-500/30",
	ts: "bg-blue-600/10 text-blue-300 border-blue-600/30",
	tengo: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
	lua: "bg-purple-500/10 text-purple-400 border-purple-500/30",
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

export function defaultDef(type: RuleType) {
	return type === "condition" ? { ...emptyCondition } : { ...emptyScript };
}

export const ENGINE_DOCS: Record<
	RuleType,
	{ sections: { h: string; body: string }[] }
> = {
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
				body: "import / require · fetch · async/await · Node.js",
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
				body: "import / export · npm packages · async/await",
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

const MONACO_DTS = `declare function http_get(
  url: string,
  opts?: { headers?: Record<string, string> }
): { readonly status: number; readonly body: string; readonly final_url: string };
`;

export function useMonacoSetup() {
	const monaco = useMonaco();
	useEffect(() => {
		if (!monaco) return;
		monaco.typescript.javascriptDefaults.addExtraLib(
			MONACO_DTS,
			"subs-check.d.ts",
		);
		monaco.typescript.typescriptDefaults.addExtraLib(
			MONACO_DTS,
			"subs-check.d.ts",
		);
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
