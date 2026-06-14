import { Icon as IconifyIcon, loadIcon } from "@iconify/react";
import { useEffect, useState } from "react";
import { isIconifyId } from "@/components/platform-icons";
import { usePlatformRules } from "@/components/platform-rules-context";

function LetterBadge({ label, size }: { label: string; size: number }) {
	return (
		<span
			className="inline-flex flex-shrink-0 items-center justify-center rounded bg-secondary font-medium text-muted-foreground"
			style={{ width: size, height: size, fontSize: Math.round(size * 0.6) }}
		>
			{(label || "?").charAt(0).toUpperCase()}
		</span>
	);
}

function IconifyOrFallback({
	icon,
	label,
	size,
}: {
	icon: string;
	label: string;
	size: number;
}) {
	const [failed, setFailed] = useState(false);
	useEffect(() => {
		let active = true;
		setFailed(false);
		loadIcon(icon).catch(() => {
			if (active) setFailed(true);
		});
		return () => {
			active = false;
		};
	}, [icon]);
	if (failed) return <LetterBadge label={label} size={size} />;
	return (
		<span
			className="inline-flex flex-shrink-0 items-center justify-center"
			style={{ width: size, height: size }}
		>
			<IconifyIcon icon={icon} width={size} height={size} />
		</span>
	);
}

// RuleIcon renders a platform icon from a free-form icon string (Iconify id,
// http/data URL, or emoji), falling back to a first-letter badge when empty or
// when an Iconify id does not resolve.
export function RuleIcon({
	icon,
	label,
	size = 14,
	showLabel = false,
}: {
	icon: string;
	label: string;
	size?: number;
	showLabel?: boolean;
}) {
	let glyph: React.ReactNode;
	if (!icon) {
		glyph = <LetterBadge label={label} size={size} />;
	} else if (isIconifyId(icon)) {
		glyph = <IconifyOrFallback icon={icon} label={label} size={size} />;
	} else if (
		icon.startsWith("http://") ||
		icon.startsWith("https://") ||
		icon.startsWith("data:")
	) {
		glyph = (
			<img
				src={icon}
				alt={label}
				className="flex-shrink-0 rounded object-contain"
				style={{ width: size, height: size }}
				onError={(e) => {
					(e.currentTarget as HTMLImageElement).style.display = "none";
				}}
			/>
		);
	} else {
		glyph = (
			<span
				className="inline-flex flex-shrink-0 items-center justify-center"
				style={{ width: size, height: size, fontSize: size }}
				aria-hidden
			>
				{icon}
			</span>
		);
	}

	return (
		<span className="inline-flex items-center gap-1" title={label}>
			{glyph}
			{showLabel && (
				<span className="text-[10px] text-muted-foreground">{label}</span>
			)}
		</span>
	);
}

// usePlatformDisplay resolves a platform key to its rule-defined icon + label.
export function usePlatformDisplay(key: string): { icon: string; label: string } {
	const rules = usePlatformRules();
	const rule = rules.get(key);
	return { icon: rule?.icon ?? "", label: rule?.name ?? key };
}

// RulePlatformIcon renders the icon for a platform key, resolved from the rules
// context. Use when you have a key but not the rule object.
export function RulePlatformIcon({
	platformKey,
	size = 14,
	showLabel = false,
}: {
	platformKey: string;
	size?: number;
	showLabel?: boolean;
}) {
	const { icon, label } = usePlatformDisplay(platformKey);
	return <RuleIcon icon={icon} label={label} size={size} showLabel={showLabel} />;
}
