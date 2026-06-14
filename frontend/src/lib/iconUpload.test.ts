import { describe, expect, it } from "vitest";
import { ICON_MAX_BYTES, validateIconFile } from "./iconUpload";

describe("validateIconFile", () => {
	it("accepts a small svg", () => {
		const f = new File(["<svg/>"], "i.svg", { type: "image/svg+xml" });
		expect(validateIconFile(f)).toBeNull();
	});
	it("rejects an unsupported type", () => {
		const f = new File(["x"], "i.gif", { type: "image/gif" });
		expect(validateIconFile(f)).toMatch(/type/i);
	});
	it("rejects an oversized file", () => {
		const big = new File([new Uint8Array(ICON_MAX_BYTES + 1)], "i.png", {
			type: "image/png",
		});
		expect(validateIconFile(big)).toMatch(/large|32/i);
	});
});
