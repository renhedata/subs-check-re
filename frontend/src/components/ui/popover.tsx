import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import { cn } from "@/lib/utils";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;

function PopoverContent({
	className,
	sideOffset = 6,
	align = "end",
	...props
}: PopoverPrimitive.Popup.Props & {
	sideOffset?: number;
	align?: "start" | "center" | "end";
}) {
	return (
		<PopoverPrimitive.Portal>
			<PopoverPrimitive.Positioner sideOffset={sideOffset} align={align}>
				<PopoverPrimitive.Popup
					className={cn(
						"z-50 w-72 rounded-lg border border-border bg-popover p-3 text-popover-foreground shadow-[var(--shadow-popover)] outline-none",
						"transition-all duration-150 data-[ending-style]:scale-95 data-[starting-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0",
						className,
					)}
					{...props}
				/>
			</PopoverPrimitive.Positioner>
		</PopoverPrimitive.Portal>
	);
}

export { Popover, PopoverContent, PopoverTrigger };
