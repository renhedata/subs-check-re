import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import type * as React from "react";

export function Tooltip({
	content,
	children,
	side = "top",
}: {
	content: React.ReactNode;
	children: React.ReactElement;
	side?: "top" | "bottom" | "left" | "right";
}) {
	return (
		<TooltipPrimitive.Root>
			<TooltipPrimitive.Trigger delay={300} render={children} />
			<TooltipPrimitive.Portal>
				<TooltipPrimitive.Positioner side={side} sideOffset={6}>
					<TooltipPrimitive.Popup className="z-50 rounded-md border border-border bg-popover px-2 py-1 text-popover-foreground text-xs shadow-[var(--shadow-popover)]">
						{content}
					</TooltipPrimitive.Popup>
				</TooltipPrimitive.Positioner>
			</TooltipPrimitive.Portal>
		</TooltipPrimitive.Root>
	);
}
