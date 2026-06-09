import { Dialog } from "@base-ui/react/dialog";
import { Menu } from "lucide-react";
import { useState } from "react";
import { SidebarNav } from "@/components/sidebar";

export function MobileNav() {
	const [open, setOpen] = useState(false);

	return (
		<Dialog.Root open={open} onOpenChange={setOpen}>
			<header className="flex h-12 flex-shrink-0 items-center gap-3 border-border border-b bg-card px-4 md:hidden">
				<Dialog.Trigger
					aria-label="Open navigation menu"
					className="-ml-1 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-white/5 hover:text-foreground"
				>
					<Menu size={18} strokeWidth={1.5} />
				</Dialog.Trigger>
				<div className="flex items-center gap-2">
					<div
						className="flex h-6 w-6 items-center justify-center rounded-md font-bold text-xs"
						style={{
							background: "var(--primary)",
							color: "var(--primary-foreground)",
						}}
					>
						S
					</div>
					<span className="font-semibold text-foreground text-sm">
						subs-check
					</span>
				</div>
			</header>

			<Dialog.Portal>
				<Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
				<Dialog.Popup className="fixed inset-y-0 left-0 z-50 w-[280px] max-w-[85vw] border-border border-r bg-card shadow-xl transition-transform duration-200 data-[ending-style]:-translate-x-full data-[starting-style]:-translate-x-full">
					<Dialog.Title className="sr-only">Navigation</Dialog.Title>
					<SidebarNav onNavigate={() => setOpen(false)} />
				</Dialog.Popup>
			</Dialog.Portal>
		</Dialog.Root>
	);
}
