import { cn } from "@/lib/utils";

export function Progress({
	value,
	className,
}: {
	value: number; // 0–100
	className?: string;
}) {
	const clamped = Math.max(0, Math.min(100, value));
	return (
		<div
			role="progressbar"
			aria-valuenow={Math.round(clamped)}
			aria-valuemin={0}
			aria-valuemax={100}
			className={cn(
				"h-1.5 w-full overflow-hidden rounded-full bg-secondary",
				className,
			)}
		>
			<div
				className="h-full rounded-full transition-[width] duration-300 ease-out"
				style={{ width: `${clamped}%`, background: "var(--color-progress)" }}
			/>
		</div>
	);
}
