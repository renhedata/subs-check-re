// Pure sort/filter helpers for the workbench node table. Kept free of React
// so they unit-test in node and stay reusable between table and chips.

export const BUILTIN_PLATFORMS = [
	"netflix",
	"youtube",
	"youtube_premium",
	"openai",
	"chatgpt_ios",
	"claude",
	"gemini",
	"grok",
	"disney",
	"tiktok",
	"bilibili_cn",
	"bilibili_hkmctw",
	"bahamut",
	"spotify",
	"prime_video",
] as const;

export type BuiltinPlatform = (typeof BUILTIN_PLATFORMS)[number];

export type PlatformOutcomeLike = {
	unlocked: boolean;
	status: string;
	region?: string;
};

// Structural subset of checker.NodeResult that the helpers need. The real
// NodeResult satisfies it.
export type NodeLike = {
	node_id: string;
	node_name: string;
	alive: boolean;
	latency_ms: number;
	speed_kbps: number;
	platforms: Record<string, PlatformOutcomeLike>;
};

export type SortKey = "latency" | "speed" | "name";
export type SortDir = "asc" | "desc";

export interface NodeFilter {
	text?: string;
	aliveOnly?: boolean;
	platforms?: string[];
}

export function nodeHasPlatform(n: NodeLike, platform: string): boolean {
	return n.platforms?.[platform]?.unlocked === true;
}

export function filterNodes<T extends NodeLike>(
	nodes: T[],
	filter: NodeFilter,
): T[] {
	const text = filter.text?.trim().toLowerCase();
	return nodes.filter((n) => {
		if (filter.aliveOnly && !n.alive) return false;
		if (text && !n.node_name.toLowerCase().includes(text)) return false;
		if (filter.platforms && filter.platforms.length > 0) {
			if (!filter.platforms.some((p) => nodeHasPlatform(n, p))) return false;
		}
		return true;
	});
}

export function sortNodes<T extends NodeLike>(
	nodes: T[],
	key: SortKey,
	dir: SortDir,
): T[] {
	const mul = dir === "asc" ? 1 : -1;
	return [...nodes].sort((a, b) => {
		// Dead nodes always sink to the bottom.
		if (a.alive !== b.alive) return a.alive ? -1 : 1;
		switch (key) {
			case "latency":
				return (a.latency_ms - b.latency_ms) * mul;
			case "speed":
				return (a.speed_kbps - b.speed_kbps) * mul;
			case "name":
				return a.node_name.localeCompare(b.node_name) * mul;
			default:
				return 0;
		}
	});
}

export type Tone = "success" | "warning" | "danger";

export function latencyTone(ms: number): Tone {
	if (ms < 50) return "success";
	if (ms <= 200) return "warning";
	return "danger";
}
