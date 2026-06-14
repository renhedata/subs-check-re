import { describe, expect, it } from "vitest";
import {
	DEFAULT_CHECK_OPTIONS,
	hasStoredCheckOptions,
	loadCheckOptions,
	reconcileMediaApps,
	saveCheckOptions,
} from "./checkOptions";

function fakeStorage(): Storage {
	const map = new Map<string, string>();
	return {
		getItem: (k) => map.get(k) ?? null,
		setItem: (k, v) => void map.set(k, v),
		removeItem: (k) => void map.delete(k),
		clear: () => map.clear(),
		key: () => null,
		get length() {
			return map.size;
		},
	} as Storage;
}

describe("checkOptions persistence", () => {
	it("returns defaults when nothing stored", () => {
		const s = fakeStorage();
		expect(loadCheckOptions("sub1", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});

	it("round-trips saved options per subscription", () => {
		const s = fakeStorage();
		const opts = {
			speed_test: false,
			upload_speed_test: true,
			media_apps: ["netflix"],
			debug: false,
		};
		saveCheckOptions("sub1", opts, s);
		expect(loadCheckOptions("sub1", s)).toEqual(opts);
		expect(loadCheckOptions("sub2", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});

	it("falls back to defaults on corrupt JSON", () => {
		const s = fakeStorage();
		s.setItem("check-options:sub1", "{nope");
		expect(loadCheckOptions("sub1", s)).toEqual(DEFAULT_CHECK_OPTIONS);
	});
});

describe("checkOptions rule reconciliation", () => {
	it("hasStoredCheckOptions reflects whether prefs were saved", () => {
		const s = fakeStorage();
		expect(hasStoredCheckOptions("sub1", s)).toBe(false);
		saveCheckOptions("sub1", DEFAULT_CHECK_OPTIONS, s);
		expect(hasStoredCheckOptions("sub1", s)).toBe(true);
	});

	it("reconcileMediaApps drops keys not in the available set", () => {
		expect(
			reconcileMediaApps(
				["netflix", "gone", "disney"],
				["netflix", "disney", "max"],
			),
		).toEqual(["netflix", "disney"]);
		expect(reconcileMediaApps([], ["netflix"])).toEqual([]);
	});
});
