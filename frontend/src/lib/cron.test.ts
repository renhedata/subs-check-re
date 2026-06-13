import { describe, expect, it } from "vitest";
import { describeCron, formatUntil, nextRun } from "./cron";

describe("nextRun", () => {
	it("computes the next fire time for an every-6h cron", () => {
		const from = new Date("2026-06-12T03:30:00Z");
		const next = nextRun("0 */6 * * *", from);
		expect(next?.toISOString()).toBe("2026-06-12T06:00:00.000Z");
	});

	it("returns null for an invalid expression", () => {
		expect(nextRun("not-a-cron", new Date())).toBeNull();
	});
});

describe("formatUntil", () => {
	const now = new Date("2026-06-12T00:00:00Z");
	it("formats hours and minutes", () => {
		expect(formatUntil(new Date("2026-06-12T02:14:00Z"), now)).toBe(
			"in 2h 14m",
		);
	});
	it("formats days", () => {
		expect(formatUntil(new Date("2026-06-14T12:00:00Z"), now)).toBe(
			"in 2d 12h",
		);
	});
	it("formats sub-minute as now", () => {
		expect(formatUntil(new Date("2026-06-12T00:00:30Z"), now)).toBe("in <1m");
	});
});

describe("describeCron", () => {
	it("describes a valid cron", () => {
		expect(describeCron("0 4 * * *")).toMatch(/4:00 AM/);
	});
	it("returns the raw expression when unparseable", () => {
		expect(describeCron("???")).toBe("???");
	});
});
