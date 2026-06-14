import { Icon as IconifyIcon } from "@iconify/react";
import { Loader2, Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { isIconifyId } from "@/components/platform-icons";
import { RuleIcon } from "@/components/rule-icon";
import { readIconAsDataUrl, validateIconFile } from "@/lib/iconUpload";

const QUICK_SETS = [
	{ label: "Brands", prefix: "simple-icons" },
	{ label: "Logos", prefix: "logos" },
	{ label: "Generic", prefix: "lucide" },
];

export function IconDisplay({
	icon,
	name,
	size = "md",
}: {
	icon: string;
	name: string;
	size?: "sm" | "md";
}) {
	const px = size === "sm" ? 16 : 20;
	const dim = size === "sm" ? "h-5 w-5" : "h-7 w-7";

	if (!icon) {
		return (
			<span
				className={`flex flex-shrink-0 items-center justify-center rounded bg-secondary font-medium text-muted-foreground ${dim} text-sm`}
			>
				{name.charAt(0).toUpperCase()}
			</span>
		);
	}

	if (isIconifyId(icon)) {
		return (
			<span className={`flex flex-shrink-0 items-center justify-center ${dim}`}>
				<IconifyIcon icon={icon} width={px} height={px} />
			</span>
		);
	}

	const isUrl =
		icon.startsWith("http://") ||
		icon.startsWith("https://") ||
		icon.startsWith("data:");
	if (isUrl) {
		return (
			<img
				src={icon}
				alt={name}
				className={`flex-shrink-0 rounded object-contain ${dim}`}
				onError={(e) => {
					(e.currentTarget as HTMLImageElement).style.display = "none";
				}}
			/>
		);
	}

	return (
		<span
			className={`flex flex-shrink-0 items-center justify-center ${dim} text-base`}
			aria-hidden
		>
			{icon}
		</span>
	);
}

export function IconPickerInput({
	value,
	onChange,
	name,
}: {
	value: string;
	onChange: (v: string) => void;
	name: string;
}) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const [results, setResults] = useState<string[]>([]);
	const [searching, setSearching] = useState(false);
	const containerRef = useRef<HTMLDivElement>(null);
	const fileRef = useRef<HTMLInputElement>(null);

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

	useEffect(() => {
		function onDown(e: MouseEvent) {
			if (
				containerRef.current &&
				!containerRef.current.contains(e.target as Node)
			) {
				setOpen(false);
			}
		}
		document.addEventListener("mousedown", onDown);
		return () => document.removeEventListener("mousedown", onDown);
	}, []);

	useEffect(() => {
		if (!query.trim()) {
			setResults([]);
			return;
		}
		const id = setTimeout(async () => {
			setSearching(true);
			try {
				const res = await fetch(
					`https://api.iconify.design/search?query=${encodeURIComponent(query)}&limit=30`,
				);
				const data = (await res.json()) as { icons?: string[] };
				setResults(data.icons ?? []);
			} catch {
				setResults([]);
			} finally {
				setSearching(false);
			}
		}, 400);
		return () => clearTimeout(id);
	}, [query]);

	return (
		<div className="relative" ref={containerRef}>
			<div className="flex items-center gap-1">
				<RuleIcon icon={value} label={name || "?"} size={18} />
				<input
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder="emoji, URL, or icon:name"
					className="h-7 w-36 rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
				/>
				<button
					type="button"
					onClick={() => setOpen((o) => !o)}
					className="flex h-7 w-7 items-center justify-center rounded border border-border text-muted-foreground hover:bg-secondary"
					title="Search Iconify"
				>
					<Search size={11} />
				</button>
			</div>

			{open && (
				<div className="absolute top-9 left-0 z-50 w-80 rounded-lg border border-border bg-card p-3 shadow-xl">
					<input
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						placeholder="Search icons (e.g. netflix, youtube…)"
						className="mb-2 h-7 w-full rounded border border-border bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
						// biome-ignore lint/a11y/noAutofocus: intentional — picker opens on button click
						autoFocus
					/>

					<div className="mb-2 flex flex-wrap items-center gap-1">
						<button
							type="button"
							onClick={() => fileRef.current?.click()}
							className="rounded border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
						>
							Upload SVG/PNG
						</button>
						{QUICK_SETS.map((s) => (
							<button
								key={s.prefix}
								type="button"
								onClick={() => setQuery(`${s.label} `)}
								className="rounded border border-border px-2 py-1 text-muted-foreground text-xs hover:bg-secondary"
							>
								{s.label}
							</button>
						))}
						<input
							ref={fileRef}
							type="file"
							accept="image/svg+xml,image/png,image/jpeg,image/webp"
							className="hidden"
							onChange={(e) => onUpload(e.target.files?.[0])}
						/>
					</div>

					{searching && (
						<div className="flex justify-center py-3">
							<Loader2
								size={14}
								className="animate-spin text-muted-foreground"
							/>
						</div>
					)}

					{!searching && results.length > 0 && (
						<div className="grid max-h-48 grid-cols-6 gap-1 overflow-y-auto">
							{results.map((iconId) => (
								<button
									key={iconId}
									type="button"
									onClick={() => {
										onChange(iconId);
										setOpen(false);
									}}
									title={iconId}
									className="flex h-9 w-full items-center justify-center rounded hover:bg-secondary"
								>
									<IconifyIcon icon={iconId} width={20} height={20} />
								</button>
							))}
						</div>
					)}

					{!searching && results.length === 0 && query && (
						<p className="py-3 text-center text-muted-foreground text-xs">
							No results
						</p>
					)}

					{!query && (
						<p className="text-muted-foreground text-xs leading-relaxed">
							Powered by Iconify · 200k+ icons
							<br />
							<span className="text-foreground/50">
								Try: simple-icons:netflix · logos:youtube
							</span>
						</p>
					)}
				</div>
			)}
		</div>
	);
}
