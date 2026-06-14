import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RuleIcon } from "./rule-icon";

describe("RuleIcon", () => {
	it("renders an <img> for a data URL", () => {
		const { container } = render(
			<RuleIcon icon="data:image/png;base64,AAAA" label="X" />,
		);
		expect(container.querySelector("img")).not.toBeNull();
	});
	it("renders a letter badge when icon is empty", () => {
		const { getByText } = render(<RuleIcon icon="" label="netflix" />);
		expect(getByText("N")).toBeTruthy();
	});
	it("renders raw emoji text", () => {
		const { getByText } = render(<RuleIcon icon="🎬" label="X" />);
		expect(getByText("🎬")).toBeTruthy();
	});
});
