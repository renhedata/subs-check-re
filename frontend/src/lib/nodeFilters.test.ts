import { describe, expect, it } from "vitest";
import {
	filterNodes,
	latencyTone,
	type NodeLike,
	nodeHasPlatform,
	sortNodes,
} from "./nodeFilters";

function node(partial: Partial<NodeLike>): NodeLike {
	return {
		node_id: partial.node_id ?? "id",
		node_name: partial.node_name ?? "node",
		alive: partial.alive ?? true,
		latency_ms: partial.latency_ms ?? 100,
		speed_kbps: partial.speed_kbps ?? 0,
		platforms: partial.platforms ?? {},
	};
}

describe("sortNodes", () => {
	it("keeps dead nodes last regardless of sort key", () => {
		const sorted = sortNodes(
			[
				node({ node_id: "dead", alive: false, latency_ms: 1 }),
				node({ node_id: "slow", latency_ms: 300 }),
				node({ node_id: "fast", latency_ms: 20 }),
			],
			"latency",
			"asc",
		);
		expect(sorted.map((n) => n.node_id)).toEqual(["fast", "slow", "dead"]);
	});

	it("sorts by speed descending", () => {
		const sorted = sortNodes(
			[
				node({ node_id: "a", speed_kbps: 100 }),
				node({ node_id: "b", speed_kbps: 9000 }),
			],
			"speed",
			"desc",
		);
		expect(sorted[0].node_id).toBe("b");
	});
});

describe("filterNodes", () => {
	const nodes = [
		node({
			node_id: "hk",
			node_name: "HK-IEPL-01",
			platforms: { netflix: { unlocked: true, status: "Yes", region: "HK" } },
		}),
		node({ node_id: "jp", node_name: "JP Tokyo", alive: false }),
		node({
			node_id: "sg",
			node_name: "SG Marina",
			platforms: { spotify: { unlocked: true, status: "Yes" } },
		}),
	];

	it("filters by text, case-insensitive", () => {
		expect(filterNodes(nodes, { text: "iepl" }).map((n) => n.node_id)).toEqual([
			"hk",
		]);
	});

	it("filters alive only", () => {
		expect(filterNodes(nodes, { aliveOnly: true })).toHaveLength(2);
	});

	it("filters by built-in and extra platforms", () => {
		expect(
			filterNodes(nodes, { platforms: ["netflix"] }).map((n) => n.node_id),
		).toEqual(["hk"]);
		expect(
			filterNodes(nodes, { platforms: ["spotify"] }).map((n) => n.node_id),
		).toEqual(["sg"]);
	});
});

describe("nodeHasPlatform (platforms map)", () => {
	it("reads unlocked from the platforms map for builtin and custom keys", () => {
		const n = node({
			platforms: {
				netflix: { unlocked: true, status: "Yes", region: "US" },
				spotify: { unlocked: false, status: "No" },
			},
		});
		expect(nodeHasPlatform(n, "netflix")).toBe(true);
		expect(nodeHasPlatform(n, "spotify")).toBe(false);
		expect(nodeHasPlatform(n, "absent")).toBe(false);
	});
});

describe("latencyTone", () => {
	it("maps thresholds", () => {
		expect(latencyTone(49)).toBe("success");
		expect(latencyTone(200)).toBe("warning");
		expect(latencyTone(201)).toBe("danger");
	});
});
