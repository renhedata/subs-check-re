import { cn } from "@/lib/utils";

export type DotTone = "success" | "danger" | "info" | "neutral";

const toneClass: Record<DotTone, string> = {
	success: "bg-success",
	danger: "bg-danger",
	info: "bg-info",
	neutral: "bg-muted-foreground/50",
};

export function StatusDot({
	tone,
	pulse = false,
	className,
}: {
	tone: DotTone;
	pulse?: boolean;
	className?: string;
}) {
	return (
		<span
			aria-hidden
			className={cn(
				"inline-block size-2 shrink-0 rounded-full",
				toneClass[tone],
				pulse && "animate-pulse",
				className,
			)}
		/>
	);
}
