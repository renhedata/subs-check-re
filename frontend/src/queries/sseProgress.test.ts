import { describe, expect, it } from "vitest";
import { isPhaseEvent, isResultEvent } from "./sseProgress";

describe("sse event classification", () => {
	it("treats an event with a phase as an in-flight phase event", () => {
		const e = { node_id: "n1", node_name: "x", phase: "streaming" };
		expect(isPhaseEvent(e)).toBe(true);
		expect(isResultEvent(e)).toBe(false);
	});

	it("treats a node event without a phase as a completed result", () => {
		const e = { node_id: "n1", node_name: "x", alive: true };
		expect(isResultEvent(e)).toBe(true);
		expect(isPhaseEvent(e)).toBe(false);
	});

	it("treats a counter/snapshot event (no node, no phase) as neither", () => {
		const e = { progress: 5, total: 10 };
		expect(isPhaseEvent(e)).toBe(false);
		expect(isResultEvent(e)).toBe(false);
	});
});
