import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";

export function CopyButton({
	text,
	className,
}: {
	text: string;
	className?: string;
}) {
	const [copied, setCopied] = useState(false);
	return (
		<button
			type="button"
			aria-label="Copy to clipboard"
			onClick={() => {
				navigator.clipboard.writeText(text);
				setCopied(true);
				setTimeout(() => setCopied(false), 1500);
			}}
			className={cn(
				"shrink-0 rounded p-1 transition-colors hover:bg-secondary",
				copied ? "text-success" : "text-muted-foreground",
				className,
			)}
		>
			{copied ? <Check size={12} /> : <Copy size={12} />}
		</button>
	);
}
