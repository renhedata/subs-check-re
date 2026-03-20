import {
	SiAnthropic,
	SiGooglegemini,
	SiNetflix,
	SiOpenai,
	SiTiktok,
	SiYoutube,
} from "react-icons/si";

type PlatformKey =
	| "netflix"
	| "youtube"
	| "youtube_premium"
	| "openai"
	| "claude"
	| "gemini"
	| "grok"
	| "disney"
	| "tiktok";

interface IconProps {
	size?: number;
	color?: string;
}

// Disney+ icon (custom SVG — not in Simple Icons)
function DisneyPlusIcon({ size = 14, color = "#113CCF" }: IconProps) {
	return (
		<svg
			width={size}
			height={size}
			viewBox="0 0 24 24"
			fill={color}
			role="img"
		>
			<title>Disney+</title>
			<path d="M2.056 6.834C.447 7.97 0 8.845 0 9.48c0 .803.617 1.484 1.907 2.023.893.4 2.09.685 3.476.87l.15-.063c-.388-.282-.58-.597-.58-.966 0-.142.025-.29.084-.448-1.264-.202-2.264-.51-2.94-.877C1.52 9.72 1.2 9.436 1.2 9.17c0-.504.997-1.232 3.064-1.838.04-.025.068-.08.068-.16 0-.05-.017-.108-.044-.16-.738.183-1.492.43-2.232.822zm19.888 2.336c0-.266-.32-.55-.897-.849-.676.367-1.676.675-2.94.877.059.157.084.306.084.448 0 .37-.192.684-.58.966l.15.063c1.387-.185 2.583-.47 3.476-.87 1.29-.54 1.907-1.22 1.907-2.023 0-.635-.447-1.51-2.056-2.646-.74-.392-1.494-.639-2.232-.823a.303.303 0 0 0-.044.16c0 .08.028.136.068.16 2.067.607 3.064 1.335 3.064 1.838zM6.446 17.022c.428.382 1.143.564 2.168.564.57 0 1.172-.076 1.815-.218l-.01-.04c-.94-.134-1.705-.425-2.26-.883-.64-.567-.967-1.302-.967-2.174 0-.328.076-.722.2-1.15-1.37.727-2.037 1.59-2.037 2.393 0 .617.371 1.115 1.09 1.508zm6.957.346c.643.142 1.246.218 1.815.218 1.025 0 1.74-.182 2.168-.564.72-.393 1.09-.89 1.09-1.508 0-.803-.666-1.666-2.036-2.392.123.427.2.821.2 1.15 0 .87-.328 1.606-.968 2.173-.555.458-1.32.749-2.26.883l-.01.04zm5.135-7.862c-.625-.275-1.41-.51-2.364-.7a28.376 28.376 0 0 0-2.55-.35c.143-.31.214-.6.214-.863 0-.63-.345-1.153-.996-1.552-.227.285-.437.678-.616 1.168a21.36 21.36 0 0 0-1.685-.074c-.553 0-1.118.025-1.685.074-.18-.49-.39-.883-.617-1.168-.65.399-.995.922-.995 1.552 0 .264.07.553.213.864-.93.082-1.78.2-2.55.35-.955.19-1.74.424-2.364.699.893-.242 2.132-.434 3.704-.566a9.39 9.39 0 0 1 1.537-.31c.617-.075 1.198-.113 1.757-.113.559 0 1.14.038 1.757.113a9.39 9.39 0 0 1 1.537.31c1.572.132 2.811.324 3.703.566zM12 14.688c-.885 0-1.596-.128-2.142-.388-.64-.328-.963-.78-.963-1.364 0-.38.139-.725.406-1.025.328-.345.804-.588 1.436-.722.37-.084.79-.126 1.263-.126.474 0 .894.042 1.264.126.631.134 1.107.377 1.435.722.268.3.407.646.407 1.025 0 .583-.323 1.036-.964 1.364-.545.26-1.256.388-2.142.388z" />
		</svg>
	);
}

// Grok/xAI icon (custom SVG)
function GrokIcon({ size = 14, color = "#808080" }: IconProps) {
	return (
		<svg
			width={size}
			height={size}
			viewBox="0 0 24 24"
			fill={color}
			role="img"
		>
			<title>Grok</title>
			<path d="M13.982 10.622 20.54 3h-1.554l-5.693 6.618L8.745 3H3.5l6.876 10.007L3.5 21h1.554l6.012-6.989L15.868 21H21.1l-7.118-10.378Zm-2.128 2.474-.697-.997-5.543-7.93H8l4.474 6.4.697.996 5.815 8.318h-2.387l-4.745-6.787Z" />
		</svg>
	);
}

interface PlatformMeta {
	icon: React.ComponentType<IconProps>;
	color: string;
	label: string;
}

const PLATFORM_META: Record<PlatformKey, PlatformMeta> = {
	netflix: { icon: SiNetflix, color: "#E50914", label: "Netflix" },
	youtube: { icon: SiYoutube, color: "#FF0000", label: "YouTube" },
	youtube_premium: { icon: SiYoutube, color: "#FF0000", label: "YouTube Premium" },
	openai: { icon: SiOpenai, color: "#412991", label: "OpenAI" },
	claude: { icon: SiAnthropic, color: "#D97757", label: "Claude" },
	gemini: { icon: SiGooglegemini, color: "#8E75B2", label: "Gemini" },
	grok: { icon: GrokIcon, color: "#808080", label: "Grok" },
	disney: { icon: DisneyPlusIcon, color: "#113CCF", label: "Disney+" },
	tiktok: { icon: SiTiktok, color: "#00F2EA", label: "TikTok" },
};

export function PlatformIcon({
	platform,
	size = 14,
	showLabel = false,
}: {
	platform: PlatformKey;
	size?: number;
	showLabel?: boolean;
}) {
	const meta = PLATFORM_META[platform];
	if (!meta) return null;
	const Icon = meta.icon;
	const isPremium = platform === "youtube_premium";

	return (
		<span
			className="inline-flex items-center gap-1"
			title={meta.label}
		>
			<span className="relative inline-flex shrink-0">
				<Icon size={size} color={meta.color} />
				{isPremium && (
					<span
						className="-right-1 -top-1 absolute flex h-2.5 w-2.5 items-center justify-center rounded-full text-[7px] font-bold text-white"
						style={{ background: "#FFD700" }}
					>
						P
					</span>
				)}
			</span>
			{showLabel && (
				<span className="text-[10px] text-muted-foreground">{meta.label}</span>
			)}
		</span>
	);
}

export { PLATFORM_META };
export type { PlatformKey };
