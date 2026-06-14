export const ICON_MAX_BYTES = 32 * 1024;
const ALLOWED = ["image/svg+xml", "image/png", "image/jpeg", "image/webp"];

// validateIconFile returns an error message, or null if the file is acceptable.
export function validateIconFile(file: File): string | null {
	if (!ALLOWED.includes(file.type)) {
		return "Unsupported type — use SVG, PNG, JPEG, or WebP";
	}
	if (file.size > ICON_MAX_BYTES) {
		return "Too large — max 32 KB (prefer SVG)";
	}
	return null;
}

// readIconAsDataUrl resolves to a data: URL for the file.
export function readIconAsDataUrl(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => resolve(String(reader.result));
		reader.onerror = () => reject(reader.error);
		reader.readAsDataURL(file);
	});
}
