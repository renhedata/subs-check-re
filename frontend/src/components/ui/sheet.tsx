import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { cn } from "@/lib/utils";

const Sheet = DialogPrimitive.Root;
const SheetTrigger = DialogPrimitive.Trigger;
const SheetClose = DialogPrimitive.Close;

function SheetContent({
	className,
	children,
	...props
}: DialogPrimitive.Popup.Props) {
	return (
		<DialogPrimitive.Portal>
			<DialogPrimitive.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
			<DialogPrimitive.Popup
				className={cn(
					"fixed top-0 right-0 z-50 flex h-screen w-full flex-col overflow-y-auto bg-popover p-5 text-popover-foreground outline-none",
					"sm:max-w-lg sm:border-border sm:border-l sm:shadow-[var(--shadow-dialog)]",
					"transition-transform duration-200 data-[ending-style]:translate-x-full data-[starting-style]:translate-x-full",
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

function SheetTitle({ className, ...props }: DialogPrimitive.Title.Props) {
	return (
		<DialogPrimitive.Title
			className={cn("font-semibold text-[15px] text-foreground", className)}
			{...props}
		/>
	);
}

function SheetDescription({
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

function SheetFooter({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			className={cn("mt-auto flex justify-end gap-2 pt-5", className)}
			{...props}
		/>
	);
}

export {
	Sheet,
	SheetClose,
	SheetContent,
	SheetDescription,
	SheetFooter,
	SheetTitle,
	SheetTrigger,
};
