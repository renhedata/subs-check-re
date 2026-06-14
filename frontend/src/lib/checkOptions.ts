// Persists the last-used Run Check options per subscription so the primary
// button can re-run with one click. Storage is injectable for tests.

export interface CheckFormOptions {
	speed_test: boolean;
	upload_speed_test: boolean;
	media_apps: string[];
	debug: boolean;
}

export const MEDIA_APPS = [
	"netflix",
	"youtube",
	"youtube_premium",
	"openai",
	"chatgpt_ios",
	"claude",
	"gemini",
	"grok",
	"disney",
	"tiktok",
	"bilibili_cn",
	"bilibili_hkmctw",
	"bahamut",
	"spotify",
	"prime_video",
] as const;

export const DEFAULT_CHECK_OPTIONS: CheckFormOptions = {
	speed_test: true,
	upload_speed_test: false,
	media_apps: [...MEDIA_APPS],
	debug: false,
};

const keyFor = (subscriptionId: string) => `check-options:${subscriptionId}`;

export function loadCheckOptions(
	subscriptionId: string,
	storage: Storage = localStorage,
): CheckFormOptions {
	try {
		const raw = storage.getItem(keyFor(subscriptionId));
		if (!raw) return { ...DEFAULT_CHECK_OPTIONS };
		const parsed = JSON.parse(raw) as Partial<CheckFormOptions>;
		return {
			speed_test: parsed.speed_test ?? DEFAULT_CHECK_OPTIONS.speed_test,
			upload_speed_test:
				parsed.upload_speed_test ?? DEFAULT_CHECK_OPTIONS.upload_speed_test,
			media_apps: Array.isArray(parsed.media_apps)
				? parsed.media_apps
				: [...DEFAULT_CHECK_OPTIONS.media_apps],
			debug: parsed.debug ?? DEFAULT_CHECK_OPTIONS.debug,
		};
	} catch {
		return { ...DEFAULT_CHECK_OPTIONS };
	}
}

export function saveCheckOptions(
	subscriptionId: string,
	opts: CheckFormOptions,
	storage: Storage = localStorage,
): void {
	try {
		storage.setItem(keyFor(subscriptionId), JSON.stringify(opts));
	} catch {
		// Quota/security errors are non-fatal; next run just uses defaults.
	}
}
