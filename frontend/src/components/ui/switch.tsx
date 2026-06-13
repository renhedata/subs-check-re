import { Switch as SwitchPrimitive } from "@base-ui/react/switch";
import { cn } from "@/lib/utils";

function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
	return (
		<SwitchPrimitive.Root
			className={cn(
				"relative h-[18px] w-8 shrink-0 rounded-full border border-transparent bg-secondary outline-none transition-colors",
				"focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
				"data-checked:bg-solid-success",
				className,
			)}
			{...props}
		>
			<SwitchPrimitive.Thumb className="block size-3.5 translate-x-0.5 rounded-full bg-white shadow-sm transition-transform data-checked:translate-x-[15px]" />
		</SwitchPrimitive.Root>
	);
}

export { Switch };
