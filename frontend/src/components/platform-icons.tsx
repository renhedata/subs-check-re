// Returns true for Iconify IDs like "simple-icons:netflix" or "mdi:home".
export function isIconifyId(icon: string): boolean {
	const parts = icon.split(":");
	return (
		parts.length === 2 &&
		/^[a-z][\w-]*$/i.test(parts[0]) &&
		/^[a-z][\w-]*$/i.test(parts[1])
	);
}
