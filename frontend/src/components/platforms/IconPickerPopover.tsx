import { Icon as IconifyIcon } from "@iconify/react";
import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { RuleIcon } from "@/components/rule-icon";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { readIconAsDataUrl, validateIconFile } from "@/lib/iconUpload";
import { cn } from "@/lib/utils";

const QUICK = [
	{ label: "Brands", q: "simple-icons:" },
	{ label: "Logos", q: "logos:" },
	{ label: "Generic", q: "lucide:" },
];

export function IconPickerPopover({
	value,
	onChange,
	name,
	size = 40,
}: {
	value: string;
	onChange: (v: string) => void;
	name: string;
	size?: number;
}) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [results, setResults] = useState<string[]>([]);
	const [searching, setSearching] = useState(false);
	const fileRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (!query.trim()) {
			setResults([]);
			return;
		}
		const id = setTimeout(async () => {
			setSearching(true);
			try {
				const res = await fetch(
					`https://api.iconify.design/search?query=${encodeURIComponent(query)}&limit=48`,
				);
				const data = (await res.json()) as { icons?: string[] };
				setResults(data.icons ?? []);
			} catch {
				setResults([]);
			} finally {
				setSearching(false);
			}
		}, 350);
		return () => clearTimeout(id);
	}, [query]);

	const pick = (v: string) => {
		onChange(v);
		setOpen(false);
	};

	const onUpload = async (file: File | undefined) => {
		if (!file) return;
		const err = validateIconFile(file);
		if (err) {
			toast.error(err);
			return;
		}
		try {
			onChange(await readIconAsDataUrl(file));
			setOpen(false);
		} catch {
			toast.error("Could not read file");
		}
	};

	return (
		<Popover open={open} onOpenChange={(v) => setOpen(v)}>
			<PopoverTrigger
				className="group relative flex items-center justify-center rounded-[10px] border border-border bg-card transition-colors hover:border-primary/60"
				style={{ width: size, height: size }}
				aria-label="Change icon"
			>
				<RuleIcon
					icon={value}
					label={name || "?"}
					size={Math.round(size * 0.55)}
				/>
				<span className="absolute -right-1 -bottom-1 flex size-4 items-center justify-center rounded-full border border-border bg-popover text-[9px] text-muted-foreground">
					✎
				</span>
			</PopoverTrigger>
			<PopoverContent align="start" className="w-80 p-3">
				<input
					value={query}
					onChange={(e) => setQuery(e.target.value)}
					placeholder="Search 200k+ icons…"
					className="mb-2 h-8 w-full rounded-md border border-border bg-background px-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
				/>
				<div className="mb-2 flex flex-wrap gap-1.5">
					{QUICK.map((s) => (
						<button
							key={s.q}
							type="button"
							onClick={() => setQuery(s.q)}
							className="rounded-md border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
						>
							{s.label}
						</button>
					))}
					<button
						type="button"
						onClick={() => fileRef.current?.click()}
						className="rounded-md border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
					>
						⬆ Upload
					</button>
					<input
						ref={fileRef}
						type="file"
						accept="image/svg+xml,image/png,image/jpeg,image/webp"
						className="hidden"
						onChange={(e) => onUpload(e.target.files?.[0])}
					/>
				</div>
				{searching ? (
					<div className="flex justify-center py-4">
						<Loader2 size={16} className="animate-spin text-muted-foreground" />
					</div>
				) : results.length > 0 ? (
					<div className="grid max-h-52 grid-cols-8 gap-1 overflow-y-auto">
						{results.map((id) => (
							<button
								key={id}
								type="button"
								title={id}
								onClick={() => pick(id)}
								className="flex aspect-square items-center justify-center rounded-md hover:bg-secondary"
							>
								<IconifyIcon icon={id} width={18} height={18} />
							</button>
						))}
					</div>
				) : (
					<p className="px-1 py-2 text-muted-foreground text-xs leading-relaxed">
						Search Iconify, pick a quick set, paste an emoji below, or upload an
						SVG/PNG (≤32 KB).
					</p>
				)}
				<input
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder="emoji · simple-icons:netflix · https://… · data:…"
					className={cn(
						"mt-2 h-8 w-full rounded-md border border-border bg-background px-2.5 font-mono text-xs",
						"focus:outline-none focus:ring-1 focus:ring-ring",
					)}
				/>
			</PopoverContent>
		</Popover>
	);
}
