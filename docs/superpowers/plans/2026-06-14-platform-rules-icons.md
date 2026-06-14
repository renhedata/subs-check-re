# Platform Rules — Rule-Driven Unlock + De-Hardcoded Icons (Plan 1 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the server network-unlock probe run the user's *enabled* rules, and make every platform icon in the UI render from its rule's `icon` field instead of the hardcoded `PLATFORM_META` brand glyphs — plus add SVG/PNG upload to the icon picker.

**Architecture:** One backend change (`GetLocalUnlock` loads enabled DB rules, runs them concurrently). On the frontend, introduce a single rule-driven icon renderer (`RuleIcon` + `usePlatformDisplay` + `RulePlatformIcon`), migrate all 6 consumers to it, then delete `PLATFORM_META`/`PlatformIcon`/`PlatformIconAny`. The icon picker gains upload/quick/emoji.

**Tech Stack:** Go + Encore; React + TanStack; `@iconify/react` (already a dep, exports `loadIcon`); Biome (tabs). Plan 2 (`2026-06-14-platform-rules-page.md`) does the sidebar move + page redesign and depends on this plan's picker.

**Spec:** `docs/superpowers/specs/2026-06-14-platform-rules-experience-design.md`. **Branch:** `feat/platform-rules-experience`.

---

## Conventions

- Go tests: `encore test ./services/checker/` (NO `-race`).
- Frontend (from `frontend/`): `bun check-types`, `bun run test:unit`, `bun run build`.
- The build stays green after every task: `RuleIcon` is added alongside the old icons (Task 2), consumers migrate (Task 3), then the old icons are deleted (Task 4).

## File Structure

| File | Responsibility |
|------|----------------|
| `services/checker/local_check.go` | `GetLocalUnlock` runs enabled user rules (Task 1) |
| `frontend/src/components/rule-icon.tsx` | NEW — `RuleIcon`, `usePlatformDisplay`, `RulePlatformIcon` (Task 2) |
| `frontend/src/components/workbench/node-table.tsx`, `node-detail-dialog.tsx`, `workbench/unlock-strip.tsx`, `check-options-fields.tsx`, `notify-channel-dialog.tsx`, `routes/settings/export-tags.tsx` | Migrate to RuleIcon (Task 3) |
| `frontend/src/components/platform-icons.tsx` | Strip to `isIconifyId` only (Task 4) |
| `frontend/src/components/platforms/IconPicker.tsx` | Upload + quick + emoji (Task 5) |

---

## Task 1: Backend — `GetLocalUnlock` runs enabled rules

**Files:**
- Modify: `services/checker/local_check.go`
- Test: `services/checker/local_check_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/local_check_test.go`:

```go
package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func TestGetLocalUnlock_OnlyEnabledRules(t *testing.T) {
	userID := "lu-user-" + uuid.New().String()
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Enabled condition rule -> should be probed and report unlocked.
	mkRule := func(key string, enabled bool) {
		def := []byte(`{"url":"` + srv.URL + `","status_code":200}`)
		if _, err := db.Exec(ctx, `
			INSERT INTO platform_rules (id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'',$5,'condition',$6,false,0,NOW(),NOW())
		`, uuid.New().String(), userID, key, key, enabled, def); err != nil {
			t.Fatalf("seed rule %s: %v", key, err)
		}
	}
	mkRule("alpha_on", true)
	mkRule("beta_off", false)

	res, err := GetLocalUnlock(ctx)
	if err != nil {
		t.Fatalf("GetLocalUnlock: %v", err)
	}
	if got, ok := res.Platforms["alpha_on"]; !ok || !got.Unlocked {
		t.Fatalf("alpha_on should be present and unlocked: %+v", res.Platforms)
	}
	if _, ok := res.Platforms["beta_off"]; ok {
		t.Fatalf("beta_off (disabled) must NOT be probed: %+v", res.Platforms)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestGetLocalUnlock_OnlyEnabledRules`
Expected: FAIL — current `GetLocalUnlock` runs the hardcoded `defaultRules`, so `alpha_on`/`beta_off` never appear.

- [ ] **Step 3: Implement**

In `services/checker/local_check.go`, replace `GetLocalUnlock` and `runDefaultRulesAgainst` (and the `ruleResult` struct) with:

```go
// GetLocalUnlock checks which platforms are accessible from the server's own
// network, running the current user's ENABLED rules (not the hardcoded defaults).
//
//encore:api auth method=GET path=/network-unlock
func GetLocalUnlock(ctx context.Context) (*LocalUnlockResult, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	client := &http.Client{Timeout: 15 * time.Second}
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rules, err := loadUserRules(checkCtx, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to load rules").Err()
	}
	enabled := make([]*PlatformRule, 0, len(rules))
	for _, r := range rules {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}

	res := LocalUnlockResult{Platforms: runRulesAgainst(checkCtx, client, enabled)}
	res.IP, res.Country = getProxyInfo(checkCtx, client)

	if err := checkCtx.Err(); err != nil {
		return nil, errs.B().Code(errs.DeadlineExceeded).Msg("check timed out").Err()
	}
	return &res, nil
}

// runRulesAgainst evaluates the given rules concurrently against the HTTP client
// and returns a map of rule key -> outcome.
func runRulesAgainst(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]PlatformOutcome {
	type kv struct {
		key     string
		outcome PlatformOutcome
	}
	out := make(chan kv, len(rules))
	var wg sync.WaitGroup
	for _, rule := range rules {
		wg.Add(1)
		go func(r *PlatformRule) {
			defer wg.Done()
			defer func() { _ = recover() }()
			outcome, _ := runRule(ctx, client, r, nil)
			out <- kv{key: r.Key, outcome: outcome}
		}(rule)
	}
	wg.Wait()
	close(out)

	result := make(map[string]PlatformOutcome, len(rules))
	for e := range out {
		result[e.key] = e.outcome
	}
	return result
}
```

Update the imports in `local_check.go`: add `encauth "encore.dev/beta/auth"` and `authsvc "subs-check-re/services/auth"`; keep `context`, `net/http`, `sync`, `time`, `encore.dev/beta/errs`. Remove `encoding/json` if it is now unused (the old `runDefaultRulesAgainst` marshalled `d.def`; the new code doesn't). The `LocalUnlockResult` struct stays as-is (`{Platforms map[string]PlatformOutcome, IP, Country}`).

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestGetLocalUnlock_OnlyEnabledRules`
Expected: PASS. Then `encore test ./services/checker/` (full) green.

- [ ] **Step 5: Commit**

```bash
git add services/checker/local_check.go services/checker/local_check_test.go
git commit -m "feat(checker): network-unlock runs the user's enabled rules, concurrently"
```

---

## Task 2: Frontend — `RuleIcon` renderer (additive)

**Files:**
- Create: `frontend/src/components/rule-icon.tsx`
- Test: `frontend/src/components/rule-icon.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/rule-icon.test.tsx`:

```tsx
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RuleIcon } from "./rule-icon";

describe("RuleIcon", () => {
	it("renders an <img> for a data URL", () => {
		const { container } = render(
			<RuleIcon icon="data:image/png;base64,AAAA" label="X" />,
		);
		expect(container.querySelector("img")).not.toBeNull();
	});
	it("renders a letter badge when icon is empty", () => {
		const { getByText } = render(<RuleIcon icon="" label="netflix" />);
		expect(getByText("N")).toBeTruthy();
	});
	it("renders raw emoji text", () => {
		const { getByText } = render(<RuleIcon icon="🎬" label="X" />);
		expect(getByText("🎬")).toBeTruthy();
	});
});
```

> If `@testing-library/react` is not installed, run `bun add -d @testing-library/react @testing-library/dom` first (vitest is already the test runner). Check `frontend/package.json` for `vitest` config; tests run via `bun run test:unit`.

- [ ] **Step 2: Run test to verify it fails**

Run (from `frontend/`): `bun run test:unit rule-icon`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement**

Create `frontend/src/components/rule-icon.tsx`:

```tsx
import { Icon as IconifyIcon, loadIcon } from "@iconify/react";
import { useEffect, useState } from "react";
import { isIconifyId } from "@/components/platform-icons";
import { usePlatformRules } from "@/components/platforms/platform-rules-context";

function LetterBadge({ label, size }: { label: string; size: number }) {
	return (
		<span
			className="inline-flex flex-shrink-0 items-center justify-center rounded bg-secondary font-medium text-muted-foreground"
			style={{ width: size, height: size, fontSize: Math.round(size * 0.6) }}
		>
			{(label || "?").charAt(0).toUpperCase()}
		</span>
	);
}

function IconifyOrFallback({
	icon,
	label,
	size,
}: {
	icon: string;
	label: string;
	size: number;
}) {
	const [failed, setFailed] = useState(false);
	useEffect(() => {
		let active = true;
		setFailed(false);
		loadIcon(icon).catch(() => {
			if (active) setFailed(true);
		});
		return () => {
			active = false;
		};
	}, [icon]);
	if (failed) return <LetterBadge label={label} size={size} />;
	return (
		<span
			className="inline-flex flex-shrink-0 items-center justify-center"
			style={{ width: size, height: size }}
		>
			<IconifyIcon icon={icon} width={size} height={size} />
		</span>
	);
}

// RuleIcon renders a platform icon from a free-form icon string (Iconify id,
// http/data URL, or emoji), falling back to a first-letter badge when empty or
// when an Iconify id does not resolve.
export function RuleIcon({
	icon,
	label,
	size = 14,
	showLabel = false,
}: {
	icon: string;
	label: string;
	size?: number;
	showLabel?: boolean;
}) {
	let glyph: React.ReactNode;
	if (!icon) {
		glyph = <LetterBadge label={label} size={size} />;
	} else if (isIconifyId(icon)) {
		glyph = <IconifyOrFallback icon={icon} label={label} size={size} />;
	} else if (
		icon.startsWith("http://") ||
		icon.startsWith("https://") ||
		icon.startsWith("data:")
	) {
		glyph = (
			<img
				src={icon}
				alt={label}
				className="flex-shrink-0 rounded object-contain"
				style={{ width: size, height: size }}
				onError={(e) => {
					(e.currentTarget as HTMLImageElement).style.display = "none";
				}}
			/>
		);
	} else {
		glyph = (
			<span
				className="inline-flex flex-shrink-0 items-center justify-center"
				style={{ width: size, height: size, fontSize: size }}
				aria-hidden
			>
				{icon}
			</span>
		);
	}

	return (
		<span className="inline-flex items-center gap-1" title={label}>
			{glyph}
			{showLabel && (
				<span className="text-[10px] text-muted-foreground">{label}</span>
			)}
		</span>
	);
}

// usePlatformDisplay resolves a platform key to its rule-defined icon + label.
export function usePlatformDisplay(key: string): { icon: string; label: string } {
	const rules = usePlatformRules();
	const rule = rules.get(key);
	return { icon: rule?.icon ?? "", label: rule?.name ?? key };
}

// RulePlatformIcon renders the icon for a platform key, resolved from the rules
// context. Use when you have a key but not the rule object.
export function RulePlatformIcon({
	platformKey,
	size = 14,
	showLabel = false,
}: {
	platformKey: string;
	size?: number;
	showLabel?: boolean;
}) {
	const { icon, label } = usePlatformDisplay(platformKey);
	return <RuleIcon icon={icon} label={label} size={size} showLabel={showLabel} />;
}
```

> Verify the import path/shape of the rules context: it is `usePlatformRules()` from `@/components/platforms/platform-rules-context`, returning a `Map<string, PlatformRule>`-like object with `.get(key)?.icon` / `.name` (the old `platform-icons.tsx` used `usePlatformRules().get(platform)?.icon`). If `.get` is not how it exposes rules, adapt this hook to its actual API (read that file first).

- [ ] **Step 4: Run test to verify it passes**

Run (from `frontend/`): `bun run test:unit rule-icon`
Expected: PASS. Then `bun check-types` (RuleIcon is additive — nothing else changed yet).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/rule-icon.tsx frontend/src/components/rule-icon.test.tsx frontend/package.json frontend/bun.lock
git commit -m "feat(frontend): RuleIcon — rule-driven icon renderer (additive)"
```

---

## Task 3: Frontend — migrate all icon consumers to `RuleIcon`

**Files (modify):**
- `frontend/src/components/workbench/node-table.tsx`
- `frontend/src/components/workbench/node-detail-dialog.tsx`
- `frontend/src/components/workbench/unlock-strip.tsx`
- `frontend/src/components/check-options-fields.tsx`
- `frontend/src/components/notify-channel-dialog.tsx`
- `frontend/src/routes/settings/export-tags.tsx`

No new tests (behavior preserved; `bun check-types` + `bun run build` are the gate). Do each edit, then run `bun check-types` after all six.

- [ ] **Step 1: `node-table.tsx` `UnlockIcons`**

Replace the `PlatformIconAny` import (line 3) with `import { RuleIcon } from "@/components/rule-icon";`. In `UnlockIcons`, replace the `entries.map(...)` icon render:

```tsx
			{entries.map(([key]) => {
				if (key === "youtube" && hasPremium) return null;
				const rule = ruleByKey[key];
				return (
					<RuleIcon
						key={key}
						icon={rule?.icon ?? ""}
						label={rule?.name ?? key}
						size={14}
					/>
				);
			})}
```

- [ ] **Step 2: `node-detail-dialog.tsx` — label from rule + show icon**

Replace `import { PLATFORM_META, type PlatformKey } from "@/components/platform-icons";` with `import { RuleIcon } from "@/components/rule-icon";`. In `platformRows`, change the builtin label source from `PLATFORM_META[key as PlatformKey]?.label ?? key` to `ruleByKey[key]?.name ?? key`:

```tsx
	const rows = BUILTIN_PLATFORMS.map((key) => {
		seen.add(key);
		const o = platforms[key];
		return {
			key,
			label: ruleByKey[key]?.name ?? key,
			icon: ruleByKey[key]?.icon ?? "",
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
			icon: ruleByKey[key]?.icon ?? "",
			unlocked: o?.unlocked === true,
			status: o?.status ?? "",
			region: o?.region ?? "",
		}));
```

(`platformRows` return type gains `icon: string`.) In the row JSX, prepend the icon before the label:

```tsx
											<span className="flex min-w-0 items-center gap-1.5 text-foreground">
												<RuleIcon icon={p.icon} label={p.label} size={14} />
												<span className="truncate">{p.label}</span>
											</span>
```

(replacing the existing `<span className="truncate text-foreground">{p.label}</span>`).

- [ ] **Step 3: `unlock-strip.tsx` — drive from returned platforms (enabled rules)**

Replace lines 2-3 imports with `import { RulePlatformIcon } from "@/components/rule-icon";`. Remove the `PLATFORM_KEYS` const (lines 13-23). Derive keys from the probe result:

```tsx
	const keys = data ? Object.keys(data.platforms ?? {}) : [];
	const unlockCount = keys.filter(
		(k) => data?.platforms?.[k]?.unlocked === true,
	).length;
```

In the badge list, iterate `keys` and render `RulePlatformIcon`:

```tsx
					<div className="flex flex-wrap gap-2">
						{keys.length === 0 ? (
							<span className="text-muted-foreground text-xs">
								No enabled rules.
							</span>
						) : (
							keys.map((k) => {
								const available = data?.platforms?.[k]?.unlocked === true;
								return (
									<span
										key={k}
										className={cn(
											"inline-flex items-center gap-1.5 rounded-md bg-secondary px-2 py-1",
											available ? "" : "opacity-35",
										)}
									>
										<RulePlatformIcon platformKey={k} size={14} showLabel />
										{available ? (
											<CheckCircle size={10} className="text-success" />
										) : null}
									</span>
								);
							})
						)}
					</div>
```

- [ ] **Step 4: `check-options-fields.tsx`**

Replace lines 1-2 imports with `import { RulePlatformIcon } from "@/components/rule-icon";`. Replace the `<PlatformIcon platform={app as PlatformKey} size={12} showLabel />` (lines 88-92) with:

```tsx
								<RulePlatformIcon platformKey={app} size={12} showLabel />
```

- [ ] **Step 5: `notify-channel-dialog.tsx`**

Replace the two platform-icon imports (lines 4-5) with `import { RulePlatformIcon } from "@/components/rule-icon";`. Replace the `<PlatformIcon platform={app as PlatformKey} size={12} showLabel />` (lines 279-283) with:

```tsx
											<RulePlatformIcon platformKey={app} size={12} showLabel />
```

- [ ] **Step 6: `export-tags.tsx` — label from rule name**

Remove the `PLATFORM_META, type PlatformKey` import (line 4). Change `labelFor` (lines 66-69) to drop the `PLATFORM_META` lookup:

```tsx
	const labelFor = (key: string) =>
		rules.find((r) => r.key === key)?.name ?? key;
```

- [ ] **Step 7: Verify**

Run (from `frontend/`): `bun check-types`
Expected: PASS for all six (the old `PlatformIcon`/`PlatformIconAny`/`PLATFORM_META` still exist, so `platform-icons.tsx` is unchanged and compiles; the migrated files now use `RuleIcon`).

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/workbench/node-table.tsx frontend/src/components/workbench/node-detail-dialog.tsx frontend/src/components/workbench/unlock-strip.tsx frontend/src/components/check-options-fields.tsx frontend/src/components/notify-channel-dialog.tsx frontend/src/routes/settings/export-tags.tsx
git commit -m "feat(frontend): migrate all platform-icon consumers to RuleIcon"
```

---

## Task 4: Frontend — delete hardcoded `PLATFORM_META` / `PlatformIcon` / `PlatformIconAny`

**Files:**
- Modify: `frontend/src/components/platform-icons.tsx`

- [ ] **Step 1: Strip the file to `isIconifyId`**

Replace the entire contents of `frontend/src/components/platform-icons.tsx` with just the still-used helper (`isIconifyId` is imported by `rule-icon.tsx` and `IconPicker.tsx`):

```tsx
// Returns true for Iconify IDs like "simple-icons:netflix" or "mdi:home".
export function isIconifyId(icon: string): boolean {
	const parts = icon.split(":");
	return (
		parts.length === 2 &&
		/^[a-z][\w-]*$/i.test(parts[0]) &&
		/^[a-z][\w-]*$/i.test(parts[1])
	);
}
```

This deletes `PLATFORM_META`, `PlatformIcon`, `PlatformIconAny`, `PlatformKey`, `renderCustomIcon`, the `react-icons/si` import, and the custom SVG/letter-badge components.

- [ ] **Step 2: Verify nothing references the removed exports**

Run (from `frontend/`): `grep -rnE 'PlatformIconAny|PLATFORM_META|from "@/components/platform-icons"' src | grep -v 'isIconifyId'`
Expected: only `rule-icon.tsx` and `platforms/IconPicker.tsx` importing `isIconifyId` (those are fine). Any other hit (e.g. a stray `PlatformKey` import) must be fixed — replace per Task 3's pattern.

Run: `bun check-types`
Expected: PASS. Run `bun run build` — PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/platform-icons.tsx
git commit -m "refactor(frontend): delete hardcoded PLATFORM_META/PlatformIcon; icons are rule-driven"
```

---

## Task 5: Frontend — icon picker gains upload, quick-pick, emoji

**Files:**
- Modify: `frontend/src/components/platforms/IconPicker.tsx`
- Test: `frontend/src/lib/iconUpload.ts` + `frontend/src/lib/iconUpload.test.ts` (new — the upload helper)

- [ ] **Step 1: Write the failing test for the upload helper**

Create `frontend/src/lib/iconUpload.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { ICON_MAX_BYTES, validateIconFile } from "./iconUpload";

describe("validateIconFile", () => {
	it("accepts a small svg", () => {
		const f = new File(["<svg/>"], "i.svg", { type: "image/svg+xml" });
		expect(validateIconFile(f)).toBeNull();
	});
	it("rejects an unsupported type", () => {
		const f = new File(["x"], "i.gif", { type: "image/gif" });
		expect(validateIconFile(f)).toMatch(/type/i);
	});
	it("rejects an oversized file", () => {
		const big = new File([new Uint8Array(ICON_MAX_BYTES + 1)], "i.png", {
			type: "image/png",
		});
		expect(validateIconFile(big)).toMatch(/large|32/i);
	});
});
```

- [ ] **Step 2: Run test to verify it fails**

Run (from `frontend/`): `bun run test:unit iconUpload`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the upload helper**

Create `frontend/src/lib/iconUpload.ts`:

```ts
export const ICON_MAX_BYTES = 32 * 1024;
const ALLOWED = [
	"image/svg+xml",
	"image/png",
	"image/jpeg",
	"image/webp",
];

// validateIconFile returns an error message, or null if the file is acceptable.
export function validateIconFile(file: File): string | null {
	if (!ALLOWED.includes(file.type)) {
		return "Unsupported type — use SVG, PNG, JPEG, or WebP";
	}
	if (file.size > ICON_MAX_BYTES) {
		return "Too large — max 32 KB (prefer SVG)";
	}
	return null;
}

// readIconAsDataUrl resolves to a data: URL for the file.
export function readIconAsDataUrl(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => resolve(String(reader.result));
		reader.onerror = () => reject(reader.error);
		reader.readAsDataURL(file);
	});
}
```

- [ ] **Step 4: Run test to verify it passes**

Run (from `frontend/`): `bun run test:unit iconUpload`
Expected: PASS.

- [ ] **Step 5: Wire upload + quick-pick into `IconPickerInput`**

In `frontend/src/components/platforms/IconPicker.tsx`:
- Add imports: `import { useRef } from "react";` (extend existing), `import { toast } from "sonner";`, `import { readIconAsDataUrl, validateIconFile } from "@/lib/iconUpload";`.
- Add a quick-pick constant near the top:

```tsx
const QUICK_SETS = [
	{ label: "Brands", prefix: "simple-icons" },
	{ label: "Logos", prefix: "logos" },
	{ label: "Generic", prefix: "lucide" },
];
```

- Inside `IconPickerInput`, add a hidden file input + handler. After the existing `containerRef` declaration add:

```tsx
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
```

- In the open popover (inside the `{open && (...)}` block), add an **Upload** button + **quick-pick** chips above the search results. Insert right after the search `<input>` (line ~142):

```tsx
						<div className="mb-2 flex flex-wrap items-center gap-1">
							<button
								type="button"
								onClick={() => fileRef.current?.click()}
								className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-secondary"
							>
								Upload SVG/PNG
							</button>
							{QUICK_SETS.map((s) => (
								<button
									key={s.prefix}
									type="button"
									onClick={() => setQuery(`${s.label} `)}
									className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-secondary"
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
```

- Replace the search-grid icon render `<IconifyIcon icon={iconId} ... />` is fine to keep (search results are valid ids). Update the picker's preview at line 116 (`<IconDisplay icon={value} name={name || "?"} size="sm" />`) to use the new renderer for consistency: change the import at the top to `import { RuleIcon } from "@/components/rule-icon";` and replace that line with `<RuleIcon icon={value} label={name || "?"} size={18} />`. (The `IconDisplay` export can stay for now; Plan 2's editor uses `RuleIcon` directly.)

> Quick-pick uses the existing Iconify search by seeding the query with the set name; it is a lightweight "browse" without a separate collections API call. Good enough for this plan.

- [ ] **Step 6: Verify**

Run (from `frontend/`): `bun check-types`, then `bun run test:unit`, then `bun run build`.
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/platforms/IconPicker.tsx frontend/src/lib/iconUpload.ts frontend/src/lib/iconUpload.test.ts
git commit -m "feat(frontend): icon picker — SVG/PNG upload (data URL, 32KB cap) + quick-pick"
```

---

## Self-Review Notes

- **Spec coverage:** Section 1 → Task 1. Section 2 (de-hardcode + unified renderer) → Tasks 2-4. Section 3 (picker upload/quick/emoji) → Task 5. Section 5 (consumers + tests) → Task 3 + the unit tests in Tasks 1/2/5. Section 4 (sidebar + page redesign) is **Plan 2**, not this plan.
- **Build-green ordering:** Task 2 additive → Task 3 migrates consumers (old icons still present) → Task 4 deletes old (no consumers left). No broken-build window.
- **Type consistency:** `RuleIcon({icon,label,size?,showLabel?})`, `usePlatformDisplay(key)→{icon,label}`, `RulePlatformIcon({platformKey,size?,showLabel?})` used identically across Tasks 2-5. `runRulesAgainst(ctx,client,rules)→map[string]PlatformOutcome` (Task 1).
- **Pre-flight to verify during execution:** the `usePlatformRules()` context API shape (`.get(key)`); whether `@testing-library/react` is installed (Task 2 note); `sonner`'s `toast` import path (already used elsewhere, e.g. export-tags).
- **Known follow-on:** `IconDisplay` in `IconPicker.tsx` remains until Plan 2 fully reworks the editor; not dead (still used by `IconPickerInput`'s other call sites if any) — leave it.
