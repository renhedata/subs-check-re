// Converts a 2-letter ISO country code to a flag emoji (regional indicators).
// Returns "" for anything that is not exactly two ASCII letters.
export function countryToFlag(code: string): string {
	if (!/^[A-Za-z]{2}$/.test(code)) return "";
	const upper = code.toUpperCase();
	const base = 0x1f1e6;
	const a = base + (upper.charCodeAt(0) - 65);
	const b = base + (upper.charCodeAt(1) - 65);
	return String.fromCodePoint(a) + String.fromCodePoint(b);
}
