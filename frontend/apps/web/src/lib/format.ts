// frontend/apps/web/src/lib/format.ts
export function formatBytes(b: number): string {
	if (b === 0) return "—";
	if (b < 1024) return `${b} B`;
	if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
	return `${(b / (1024 * 1024)).toFixed(2)} MB`;
}
