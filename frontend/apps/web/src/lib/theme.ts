import { useState } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "theme";

function resolveTheme(): Theme {
	const stored = localStorage.getItem(STORAGE_KEY);
	if (stored === "light" || stored === "dark") return stored;
	return window.matchMedia("(prefers-color-scheme: dark)").matches
		? "dark"
		: "light";
}

function applyTheme(theme: Theme) {
	document.documentElement.classList.toggle("dark", theme === "dark");
	localStorage.setItem(STORAGE_KEY, theme);
}

export function useTheme() {
	const [theme, setTheme] = useState<Theme>(resolveTheme);

	function toggle() {
		const next: Theme = theme === "dark" ? "light" : "dark";
		applyTheme(next);
		setTheme(next);
	}

	return { theme, toggle };
}
