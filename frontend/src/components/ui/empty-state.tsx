import type { LucideIcon } from "lucide-react";
import type * as React from "react";

export function EmptyState({
	icon: Icon,
	title,
	description,
	action,
}: {
	icon: LucideIcon;
	title: string;
	description?: string;
	action?: React.ReactNode;
}) {
	return (
		<div className="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
			<div className="flex size-10 items-center justify-center rounded-full bg-secondary">
				<Icon className="size-5 text-muted-foreground" strokeWidth={1.5} />
			</div>
			<p className="font-medium text-foreground text-sm">{title}</p>
			{description ? (
				<p className="max-w-xs text-muted-foreground text-xs">{description}</p>
			) : null}
			{action ? <div className="mt-2">{action}</div> : null}
		</div>
	);
}
