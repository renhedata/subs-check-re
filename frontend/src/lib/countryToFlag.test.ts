import { describe, expect, it } from "vitest";
import { countryToFlag } from "./countryToFlag";

describe("countryToFlag", () => {
	it("converts a 2-letter code to a flag emoji", () => {
		expect(countryToFlag("US")).toBe("🇺🇸");
		expect(countryToFlag("hk")).toBe("🇭🇰");
	});
	it("returns empty string for non-2-letter input", () => {
		expect(countryToFlag("")).toBe("");
		expect(countryToFlag("CHN")).toBe("");
		expect(countryToFlag("1")).toBe("");
	});
});
