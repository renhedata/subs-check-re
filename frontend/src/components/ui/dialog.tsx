import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

function DialogContent({
	className,
	children,
	...props
}: DialogPrimitive.Popup.Props) {
	return (
		<DialogPrimitive.Portal>
			<DialogPrimitive.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
			<DialogPrimitive.Popup
				className={cn(
					// Mobile: full screen. ≥sm: centered card.
					"fixed z-50 flex flex-col bg-popover text-popover-foreground outline-none",
					"inset-0 overflow-y-auto p-5",
					"sm:inset-auto sm:top-1/2 sm:left-1/2 sm:max-h-[85vh] sm:w-full sm:max-w-md sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:border sm:border-border sm:shadow-[var(--shadow-dialog)]",
					"transition-all duration-200 data-[ending-style]:scale-95 data-[starting-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0",
					className,
				)}
				{...props}
			>
				{children}
				<DialogPrimitive.Close
					aria-label="Close"
					className="absolute top-4 right-4 rounded-md p-1 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					<XIcon className="size-4" />
				</DialogPrimitive.Close>
			</DialogPrimitive.Popup>
		</DialogPrimitive.Portal>
	);
}

function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props) {
	return (
		<DialogPrimitive.Title
			className={cn("font-semibold text-[15px] text-foreground", className)}
			{...props}
		/>
	);
}

function DialogDescription({
	className,
	...props
}: DialogPrimitive.Description.Props) {
	return (
		<DialogPrimitive.Description
			className={cn("mt-0.5 text-muted-foreground text-xs", className)}
			{...props}
		/>
	);
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div className={cn("mt-5 flex justify-end gap-2", className)} {...props} />
	);
}

export {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogTitle,
	DialogTrigger,
};
