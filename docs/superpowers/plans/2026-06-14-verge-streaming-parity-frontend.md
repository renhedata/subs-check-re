# Verge Streaming Parity — Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consume the new `NodeResult.platforms` map (`{unlocked, status, region}`) across the workbench — show region flags + status, add the 6 new platforms' icons, and migrate every consumer off the removed boolean fields / `extra_platforms`.

**Architecture:** The shared constants (`PLATFORM_META`, `BUILTIN_PLATFORMS`, `MEDIA_APPS`) and the `nodeFilters` helpers are the choke points; expanding/retyping them propagates to most consumers. Two display surfaces change shape: the node-table unlock icons and the node-detail platform matrix.

**Tech Stack:** React 19 + TanStack, Tailwind, Biome (tabs), `react-icons/si`, Iconify. Run checks: `bun check-types` and `bun check` and `bun run build` from `frontend/`.

**Prerequisite:** The backend plan (`2026-06-14-verge-streaming-parity-backend.md`) is merged and `frontend/src/lib/client.gen.ts` has been regenerated so `checker.NodeResult` has `platforms: { [key: string]: checker.PlatformOutcome }` and no boolean platform fields / `extra_platforms`.

---

## Conventions

- All commands run from `frontend/`.
- Biome enforces **tab** indentation — match surrounding files.
- After each task: `bun check-types` must pass (it won't fully pass until Task 7; intermediate tasks may show errors only in not-yet-migrated files — that's expected and called out).

---

## Task 0: Confirm regenerated client

**Files:** none (verification only)

- [ ] **Step 1: Verify the generated types**

Run: `grep -n "PlatformOutcome\|platforms" src/lib/client.gen.ts | head`
Expected: a `PlatformOutcome` interface (`unlocked: boolean; status: string; region?: string`) and `NodeResult.platforms: { [key: string]: PlatformOutcome }`. If absent, run the backend plan's Task 15 first.

---

## Task 1: `countryToFlag` util

**Files:**
- Create: `src/lib/countryToFlag.ts`
- Test: `src/lib/countryToFlag.test.ts`

- [ ] **Step 1: Write the failing test**

`src/lib/countryToFlag.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { countryToFlag } from "./countryToFlag";

describe("countryToFlag", () => {
	it("converts a 2-letter code to a flag emoji", () => {
		expect(countryToFlag("US")).toBe("🇺🇸");
		expect(countryToFlag("hk")).toBe("🇭🇰");
	});
	it("returns empty string for non-2-letter input", () => {
		expect(countryToFlag("")).toBe("");
		expect(countryToFlag("CHN")).toBe("");
		expect(countryToFlag("1")).toBe("");
	});
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test:unit countryToFlag`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement**

`src/lib/countryToFlag.ts`:

```ts
// Converts a 2-letter ISO country code to a flag emoji (regional indicators).
// Returns "" for anything that is not exactly two ASCII letters.
export function countryToFlag(code: string): string {
	if (!/^[A-Za-z]{2}$/.test(code)) return "";
	const upper = code.toUpperCase();
	const base = 0x1f1e6;
	const a = base + (upper.charCodeAt(0) - 65);
	const b = base + (upper.charCodeAt(1) - 65);
	return String.fromCodePoint(a) + String.fromCodePoint(b);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test:unit countryToFlag`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/lib/countryToFlag.ts src/lib/countryToFlag.test.ts
git commit -m "feat(frontend): countryToFlag util"
```

---

## Task 2: Expand `PLATFORM_META` + `PlatformKey` (6 new platforms)

**Files:**
- Modify: `src/components/platform-icons.tsx`

- [ ] **Step 1: Add icon imports and new keys**

In the `react-icons/si` import (top of file), add `SiBilibili`, `SiSpotify`, `SiPrimevideo`:

```tsx
import {
	SiAnthropic,
	SiBilibili,
	SiGooglegemini,
	SiNetflix,
	SiOpenai,
	SiPrimevideo,
	SiSpotify,
	SiTiktok,
	SiYoutube,
} from "react-icons/si";
```

Extend the `PlatformKey` union (after `tiktok`):

```tsx
	| "tiktok"
	| "chatgpt_ios"
	| "bilibili_cn"
	| "bilibili_hkmctw"
	| "bahamut"
	| "spotify"
	| "prime_video";
```

- [ ] **Step 2: Add a Bahamut icon component**

Bahamut has no Simple Icons brand glyph; add a small letter-badge component near `GrokIcon`:

```tsx
// Bahamut Anime icon (no brand glyph in Simple Icons — letter badge).
function BahamutIcon({ size = 14, color = "#FF7800" }: IconProps) {
	return (
		<span
			className="inline-flex items-center justify-center rounded font-bold text-white"
			style={{ width: size, height: size, background: color, fontSize: Math.round(size * 0.7) }}
		>
			巴
		</span>
	);
}
```

- [ ] **Step 3: Add the 6 entries to `PLATFORM_META`**

Update `openai`'s label to `ChatGPT Web` and append the new entries:

```tsx
	openai: { icon: SiOpenai, color: "#412991", label: "ChatGPT Web" },
```

and before the closing brace of `PLATFORM_META`, after `tiktok`:

```tsx
	chatgpt_ios: { icon: SiOpenai, color: "#10A37F", label: "ChatGPT iOS" },
	bilibili_cn: { icon: SiBilibili, color: "#00A1D6", label: "哔哩哔哩大陆" },
	bilibili_hkmctw: { icon: SiBilibili, color: "#00A1D6", label: "哔哩哔哩港澳台" },
	bahamut: { icon: BahamutIcon, color: "#FF7800", label: "巴哈姆特动画疯" },
	spotify: { icon: SiSpotify, color: "#1DB954", label: "Spotify" },
	prime_video: { icon: SiPrimevideo, color: "#00A8E1", label: "Prime Video" },
```

- [ ] **Step 4: Verify**

Run: `bun check-types`
Expected: `platform-icons.tsx` has no errors (other files may still error until later tasks).

- [ ] **Step 5: Commit**

```bash
git add src/components/platform-icons.tsx
git commit -m "feat(frontend): platform icons/meta for chatgpt_ios, bilibili x2, bahamut, spotify, prime video"
```

---

## Task 3: `nodeFilters` — platforms map model

**Files:**
- Modify: `src/lib/nodeFilters.ts`
- Test: `src/lib/nodeFilters.test.ts` (extend if it exists; else create)

- [ ] **Step 1: Write/extend the failing test**

Create or append to `src/lib/nodeFilters.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { type NodeLike, nodeHasPlatform } from "./nodeFilters";

const node = (platforms: NodeLike["platforms"]): NodeLike => ({
	node_id: "1",
	node_name: "n",
	alive: true,
	latency_ms: 10,
	speed_kbps: 100,
	platforms,
});

describe("nodeHasPlatform (platforms map)", () => {
	it("reads unlocked from the platforms map for builtin and custom keys", () => {
		const n = node({
			netflix: { unlocked: true, status: "Yes", region: "US" },
			spotify: { unlocked: false, status: "No" },
		});
		expect(nodeHasPlatform(n, "netflix")).toBe(true);
		expect(nodeHasPlatform(n, "spotify")).toBe(false);
		expect(nodeHasPlatform(n, "absent")).toBe(false);
	});
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test:unit nodeFilters`
Expected: FAIL — `NodeLike` still has `extra_platforms` + bool fields, no `platforms`.

- [ ] **Step 3: Implement**

In `src/lib/nodeFilters.ts`:

1. Expand `BUILTIN_PLATFORMS`:

```ts
export const BUILTIN_PLATFORMS = [
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
```

2. Replace `NodeLike` and `nodeHasPlatform`:

```ts
export type PlatformOutcomeLike = {
	unlocked: boolean;
	status: string;
	region?: string;
};

// Structural subset of checker.NodeResult that the helpers need. The real
// NodeResult satisfies it.
export type NodeLike = {
	node_id: string;
	node_name: string;
	alive: boolean;
	latency_ms: number;
	speed_kbps: number;
	platforms: Record<string, PlatformOutcomeLike>;
};
```

```ts
export function nodeHasPlatform(n: NodeLike, platform: string): boolean {
	return n.platforms?.[platform]?.unlocked === true;
}
```

(Remove the old `& Record<BuiltinPlatform, boolean>` intersection and the `extra_platforms` field. `BuiltinPlatform` type export can stay.)

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test:unit nodeFilters`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/lib/nodeFilters.ts src/lib/nodeFilters.test.ts
git commit -m "feat(frontend): nodeFilters uses platforms map + 15 builtin keys"
```

---

## Task 4: Node table unlock icons from `platforms`

**Files:**
- Modify: `src/components/workbench/node-table.tsx`

- [ ] **Step 1: Rewrite `UnlockIcons`**

Replace the `UnlockIcons` function body:

```tsx
function UnlockIcons({
	r,
	ruleByKey,
}: {
	r: NodeResult;
	ruleByKey: Record<string, PlatformRule>;
}) {
	const entries = Object.entries(r.platforms ?? {}).filter(
		([, o]) => o?.unlocked,
	);
	// youtube_premium supersedes the plain youtube icon.
	const hasPremium = entries.some(([k]) => k === "youtube_premium");
	return (
		<div className="flex flex-wrap items-center gap-1.5">
			{entries.map(([key]) => {
				if (key === "youtube" && hasPremium) return null;
				const rule = ruleByKey[key];
				return (
					<PlatformIconAny
						key={key}
						platformKey={key}
						icon={rule?.icon}
						label={rule?.name ?? key}
					/>
				);
			})}
		</div>
	);
}
```

> `PlatformIconAny` already routes builtin keys (now 15) to brand icons and custom keys to Iconify/letter fallback, so the explicit per-field `<PlatformIcon platform="netflix"/>` list is no longer needed. The unused `PlatformIcon` import can be dropped if nothing else uses it (it isn't elsewhere in this file).

- [ ] **Step 2: Drop the now-unused import**

Change line 3 to import only `PlatformIconAny`:

```tsx
import { PlatformIconAny } from "@/components/platform-icons";
```

- [ ] **Step 3: Verify**

Run: `bun check-types`
Expected: `node-table.tsx` clean.

- [ ] **Step 4: Commit**

```bash
git add src/components/workbench/node-table.tsx
git commit -m "feat(frontend): node-table unlock icons iterate platforms map"
```

---

## Task 5: Node-detail platform matrix — status + region

**Files:**
- Modify: `src/components/workbench/node-detail-dialog.tsx`

- [ ] **Step 1: Rewrite `platformRows` to carry status + region**

Replace `platformRows`:

```tsx
function platformRows(
	r: NodeResult,
	rules: PlatformRule[],
): Array<{ key: string; label: string; unlocked: boolean; status: string; region: string }> {
	const ruleByKey = Object.fromEntries(rules.map((x) => [x.key, x]));
	const platforms = r.platforms ?? {};
	const seen = new Set<string>();
	const rows = BUILTIN_PLATFORMS.map((key) => {
		seen.add(key);
		const o = platforms[key];
		return {
			key,
			label: PLATFORM_META[key as PlatformKey]?.label ?? key,
			unlocked: o?.unlocked === true,
			status: o?.status ?? "",
			region: o?.region ?? "",
		};
	});
	const extra = Object.entries(platforms)
		.filter(([key]) => !seen.has(key))
		.map(([key, o]) => ({
			key,
			label: ruleByKey[key]?.name ?? key,
			unlocked: o?.unlocked === true,
			status: o?.status ?? "",
			region: o?.region ?? "",
		}));
	return [...rows, ...extra];
}
```

- [ ] **Step 2: Add the flag import**

Add to the imports:

```tsx
import { countryToFlag } from "@/lib/countryToFlag";
```

- [ ] **Step 3: Render status pill + region**

Replace the Platforms `<div className="flex flex-wrap gap-1.5">…</div>` block:

```tsx
								<div className="flex flex-col gap-1">
									{platformRows(result, rules).map((p) => (
										<div
											key={p.key}
											className="flex items-center justify-between gap-2 text-xs"
										>
											<span className="truncate text-foreground">{p.label}</span>
											<span className="flex shrink-0 items-center gap-1.5">
												{p.region ? (
													<span className="text-muted-foreground">
														{countryToFlag(p.region)} {p.region}
													</span>
												) : null}
												<span
													className={cn(
														"inline-flex items-center rounded-full border px-2 py-0.5",
														p.unlocked
															? "border-success-line bg-success-muted text-success"
															: "border-border text-muted-foreground",
													)}
												>
													{p.status || (p.unlocked ? "Yes" : "No")}
												</span>
											</span>
										</div>
									))}
								</div>
```

- [ ] **Step 4: Verify**

Run: `bun check-types`
Expected: `node-detail-dialog.tsx` clean.

- [ ] **Step 5: Commit**

```bash
git add src/components/workbench/node-detail-dialog.tsx
git commit -m "feat(frontend): node-detail platform matrix shows verge status + region flag"
```

---

## Task 6: Expand `MEDIA_APPS` (run-check options + notify alerts)

**Files:**
- Modify: `src/lib/checkOptions.ts`

- [ ] **Step 1: Expand the constant**

Replace `MEDIA_APPS`:

```ts
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
```

> This drives both `check-options-fields.tsx` (run-check media selector) and `notify-channel-dialog.tsx` (platform-alert chips); both map `app as PlatformKey` → all entries are now valid keys (Task 2). No changes needed in those two files.

- [ ] **Step 2: Verify**

Run: `bun check-types`
Expected: `check-options-fields.tsx` and `notify-channel-dialog.tsx` clean.

- [ ] **Step 3: Commit**

```bash
git add src/lib/checkOptions.ts
git commit -m "feat(frontend): MEDIA_APPS includes all 15 platforms"
```

---

## Task 7: Export-tags — exclude `chatgpt_ios` from tag rows

**Files:**
- Modify: `src/routes/settings/export-tags.tsx`

- [ ] **Step 1: Exclude non-tag builtins**

The backend `defaultExportTags` gives no standalone tag to `youtube_premium` or `chatgpt_ios`. Mirror that on the editor. Change the `builtinKeys` filter (line ~50):

```tsx
	// youtube_premium → folded into the "YT+" modifier; chatgpt_ios → display-only.
	// Neither gets its own editable export-tag row.
	const NON_TAG_BUILTINS = new Set(["youtube_premium", "chatgpt_ios"]);
	const builtinKeys: string[] = BUILTIN_PLATFORMS.filter(
		(k) => !NON_TAG_BUILTINS.has(k),
	);
```

(The `allBuiltinSet`/`customKeys` logic below already filters custom rules against the full `BUILTIN_PLATFORMS`, so the new builtin keys won't leak in as custom.)

- [ ] **Step 2: Verify**

Run: `bun check-types`
Expected: full project type-checks now (all consumers migrated).

- [ ] **Step 3: Commit**

```bash
git add src/routes/settings/export-tags.tsx
git commit -m "feat(frontend): export-tags lists new builtin platforms, excludes chatgpt_ios row"
```

---

## Task 8: Full verification

**Files:** none

- [ ] **Step 1: Type-check, lint, unit tests, build**

Run: `bun check-types`
Expected: PASS.

Run: `bun check`
Expected: PASS (Biome — tabs/format clean).

Run: `bun run test:unit`
Expected: PASS (countryToFlag + nodeFilters).

Run: `bun run build`
Expected: PASS.

- [ ] **Step 2: Browser smoke test**

Run backend (`encore run`) + frontend (`bun dev`), trigger a check on a subscription with live nodes, then:
- Workbench table: unlocked platform icons render, including the new platforms; YouTube Premium shows the "P" badge and suppresses the plain YouTube icon.
- Click a node → detail dialog: each platform row shows a status pill (Yes/No/Soon/…) and a region flag + code where present (e.g. Netflix 🇺🇸 US).
- Settings → Export Tags: new platforms (Spotify/Prime/Bahamut/Bilibili) appear as rows; no `chatgpt_ios`/`youtube_premium` row.
- Notify channel dialog: platform-alert chips include all 15 platforms.

Verify rendered colors/flags in the browser (Tailwind utilities can silently no-op — confirm the pills are actually tinted).

- [ ] **Step 3: Commit (if any lint autofixes)**

```bash
git add -A
git commit -m "chore(frontend): lint/format pass for verge streaming parity"
```

---

## Self-Review Notes

- **Spec coverage:** countryToFlag (T1) · icons/meta (T2) · filters model (T3) · table icons (T4) · detail status+region (T5) · MEDIA_APPS (T6) · export-tags (T7) · verify (T8). `check-options-fields.tsx` + `results-section.tsx` ride on the shared constants/helpers — no direct edits (noted in T3/T6).
- **Type consistency:** `PlatformOutcomeLike`/`NodeLike.platforms` (T3) is the structural mirror of generated `checker.PlatformOutcome`; `nodeHasPlatform` reads `.unlocked` uniformly for builtin + custom (no BUILTIN_PLATFORMS branch).
- **No leftover refs:** the boolean fields and `extra_platforms` are removed from `NodeLike` (T3) and from `node-table` (T4) and `node-detail` (T5); `bun check-types` in T7/T8 is the backstop that nothing else references them.
- **Removed special-casing:** the per-field `<PlatformIcon platform="netflix"/>` list (T4) collapses to a map iteration; YouTube-premium dedup preserved.
```
