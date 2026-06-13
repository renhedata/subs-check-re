import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
	"inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 font-medium text-[11px] leading-4",
	{
		variants: {
			tone: {
				success: "border-success-line bg-success-muted text-success",
				danger: "border-danger-line bg-danger-muted text-danger",
				warning: "border-warning-line bg-warning-muted text-warning",
				info: "border-info-line bg-info-muted text-info",
				neutral: "border-border bg-secondary text-muted-foreground",
			},
		},
		defaultVariants: { tone: "neutral" },
	},
);

function Badge({
	className,
	tone,
	...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
	return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}

export { Badge, badgeVariants };
