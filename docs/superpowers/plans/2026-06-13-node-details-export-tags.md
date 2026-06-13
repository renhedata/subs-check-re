# Node Detail View + Global Export Tag Scheme — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-node detail dialog (identity / metrics / platform matrix / raw config) to the workbench results table, and a global "Export Tags" Settings tab that customizes the country / platform / speed tags appended to node names in exports (built-in **and** custom `extra_platforms`).

**Architecture:** F1 extends `checker.GetResults`' `NodeResult` with `server`/`port`/`config` and adds a frontend dialog. F2 stores an `ExportTagConfig` on `user_settings` (JSONB), exposes it via an internal settings endpoint, and rewrites `taggedName` in the checker export path to honor it; a new Settings tab edits it. The two features are independent — F1 (Tasks 1–3) and F2 (Tasks 4–6) can ship/review separately.

**Tech Stack:** Go + Encore (checker, settings services), TanStack Start + React 19, Base UI, Tailwind v4, Bun, Biome.

**Spec:** `docs/superpowers/specs/2026-06-13-node-details-export-tags-design.md`

---

## Prerequisites & Conventions

1. **Branch:** `feat/node-details-export-tags` (already created from `feat/ui-redesign`; do NOT switch).
2. **Backend tests:** `encore test ./services/...` **without `-race`** (known harness hang).
3. **Frontend:** from `frontend/`: `bun check-types`, `bun check` (Biome, tabs/double-quotes, auto-fix), `bun run test:unit`, `bun run build`.
4. **Client regen after backend API/type changes:** `cd frontend && bun run gen:client` (writes `src/lib/client.gen.ts`).
5. Conventional commits, no attribution footers. Commit at each task boundary.

---

## File Map

**Backend — modify:**
- `services/checker/checker.go` — `NodeResult` (+server/port/config), `GetResults` CTE/scan (F1)
- `services/checker/latest_test.go` or new `services/checker/results_test.go` — GetResults field test (F1)
- `services/settings/settings.go` — `ExportTagConfig`/`PlatformTag` types, `defaultExportTags()`, `mergeExportTags()`, `UserSettings.ExportTags`, GetSettings/UpdateSettings, internal `GetExportTagsForUser` (F2)
- `services/settings/settings_test.go` (new) — defaults/merge test (F2)
- `services/checker/export_data.go` — `taggedName` rewrite, `loadJobProxies` query + signature, `latestUsableProxies*` signatures (F2)
- `services/checker/export.go` — fetch config in `loadExportProxies`, thread down (F2)
- `services/checker/export_test.go` (new or existing) — `taggedName` table test (F2)

**Backend — create:**
- `services/settings/migrations/6_add_export_tags.up.sql` / `.down.sql` (F2)

**Frontend — create:**
- `src/components/workbench/node-detail-dialog.tsx` (F1)
- `src/routes/settings/export-tags.tsx` (F2)

**Frontend — modify:**
- `src/components/workbench/node-table.tsx` — clickable rows + dialog state (F1)
- `src/routes/settings.tsx` — add 5th tab (F2)
- `src/lib/client.gen.ts` — regenerated (F1 + F2)

---

## Task 1: F1 backend — NodeResult + GetResults returns server/port/config

**Files:**
- Modify: `services/checker/checker.go`
- Create: `services/checker/results_test.go`

- [ ] **Step 1: Write the failing test**

Create `services/checker/results_test.go`:

```go
// services/checker/results_test.go
package checker

import (
	"context"
	"testing"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func resultsCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func TestGetResultsReturnsServerPortConfig(t *testing.T) {
	userID := "res-user-" + uuid.New().String()
	subID := "res-sub-" + uuid.New().String()
	jobID := uuid.New().String()
	nodeID := uuid.New().String()
	ctx := resultsCtx(userID)

	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1,$2,$3,'completed',1,1,$4,$4)
	`, jobID, subID, userID, time.Now()); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO nodes (id, subscription_id, name, type, server, port, config, enabled)
		VALUES ($1,$2,'N1','vmess','example.com',443,'{"type":"vmess","server":"example.com","port":443}'::jsonb,true)
	`, nodeID, subID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, alive, latency_ms, country, ip)
		VALUES ($1,$2,$3,'N1','vmess',true,42,'HK','1.2.3.4')
	`, uuid.New().String(), jobID, nodeID); err != nil {
		t.Fatalf("seed result: %v", err)
	}

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobID})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Server != "example.com" {
		t.Errorf("server: want example.com got %q", r.Server)
	}
	if r.Port != 443 {
		t.Errorf("port: want 443 got %d", r.Port)
	}
	if r.Config == "" || r.Config[0] != '{' {
		t.Errorf("config: want JSON object string, got %q", r.Config)
	}
	// regression: existing fields still populate
	if r.NodeName != "N1" || r.LatencyMs != 42 || r.Country != "HK" {
		t.Errorf("existing fields wrong: %+v", r)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
encore test ./services/checker/ -run TestGetResultsReturnsServerPortConfig -v
```

Expected: FAIL — `r.Server`/`r.Port`/`r.Config` undefined (compile error).

- [ ] **Step 3: Add fields to NodeResult**

In `services/checker/checker.go`, in the `NodeResult` struct, after the `IP` field add:

```go
	Server          string `json:"server"`
	Port            int    `json:"port"`
	Config          string `json:"config"`
```

- [ ] **Step 4: Add columns to the GetResults CTE and scan**

In `GetResults`, in the `WITH r AS (SELECT ...)` block, immediately after the line `COALESCE(n.enabled, true) AS enabled,` insert:

```sql
			       COALESCE(n.server, '') AS server,
			       COALESCE(n.port, 0) AS port,
			       COALESCE(n.config, cr.node_config)::text AS config,
```

Then in the `rows.Scan(...)` call, immediately after `&r.Enabled,` insert:

```go
			&r.Server, &r.Port, &r.Config,
```

> The query ends `SELECT * FROM r`, so the CTE column order is the scan order. Inserting all three in the same position in both places keeps alignment.

- [ ] **Step 5: Run to verify pass + full checker suite**

```bash
encore test ./services/checker/ -run TestGetResultsReturnsServerPortConfig -v
encore test ./services/checker/
```

Expected: PASS, suite green.

- [ ] **Step 6: Regenerate client + commit**

```bash
cd frontend && bun run gen:client && cd ..
git add services/checker/checker.go services/checker/results_test.go frontend/src/lib/client.gen.ts
git commit -m "feat(checker): return server/port/config in node results"
```

Expected: `client.gen.ts` diff shows `server`/`port`/`config` on the `NodeResult` interface.

---

## Task 2: F1 frontend — NodeDetailDialog component

**Files:**
- Create: `frontend/src/components/workbench/node-detail-dialog.tsx`

- [ ] **Step 1: Create the dialog**

Create `frontend/src/components/workbench/node-detail-dialog.tsx`:

```tsx
import { CopyButton } from "@/components/copy-button";
import { PLATFORM_META, type PlatformKey } from "@/components/platform-icons";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog";
import type { checker } from "@/lib/client.gen";
import { formatBytes } from "@/lib/format";
import { BUILTIN_PLATFORMS, latencyTone } from "@/lib/nodeFilters";
import { cn } from "@/lib/utils";

type NodeResult = checker.NodeResult;
type PlatformRule = checker.PlatformRule;

const toneText: Record<string, string> = {
	success: "text-success",
	warning: "text-warning",
	danger: "text-danger",
};

function Row({ label, children }: { label: string; children: React.ReactNode }) {
	return (
		<div className="flex items-baseline justify-between gap-3 py-1">
			<span className="shrink-0 text-muted-foreground text-xs">{label}</span>
			<span className="min-w-0 truncate text-right text-foreground text-sm tabular-nums">
				{children}
			</span>
		</div>
	);
}

function formatSpeed(kbps: number): string {
	return kbps >= 1024 ? `${(kbps / 1024).toFixed(1)} MB/s` : `${kbps} KB/s`;
}

// Builds the [key,label,unlocked] rows for the platform matrix: every built-in
// plus every key present in extra_platforms. Labels: PLATFORM_META for builtins,
// the matching rule name (or key) for custom platforms.
function platformRows(
	r: NodeResult,
	rules: PlatformRule[],
): Array<{ key: string; label: string; unlocked: boolean }> {
	const ruleByKey = Object.fromEntries(rules.map((x) => [x.key, x]));
	const builtin = BUILTIN_PLATFORMS.map((key) => ({
		key,
		label: PLATFORM_META[key as PlatformKey]?.label ?? key,
		unlocked: (r as unknown as Record<string, boolean>)[key] === true,
	}));
	const extra = Object.entries(r.extra_platforms ?? {}).map(([key, v]) => ({
		key,
		label: ruleByKey[key]?.name ?? key,
		unlocked: v === true,
	}));
	return [...builtin, ...extra];
}

function prettyConfig(raw: string): string {
	try {
		return JSON.stringify(JSON.parse(raw), null, 2);
	} catch {
		return raw;
	}
}

export function NodeDetailDialog({
	result,
	rules = [],
	open,
	onOpenChange,
}: {
	result: NodeResult | null;
	rules?: PlatformRule[];
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				{result ? (
					<>
						<DialogTitle className="truncate pr-6 font-mono">
							{result.node_name}
						</DialogTitle>
						<DialogDescription>
							{result.alive ? "Alive" : "Dead"} · {result.node_type || "—"}
						</DialogDescription>

						<div className="mt-4 max-h-[60vh] space-y-4 overflow-y-auto pr-1">
							<section>
								<p className="mb-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									Identity
								</p>
								<Row label="Protocol">{result.node_type || "—"}</Row>
								<Row label="Server">
									<span className="font-mono">
										{result.server ? `${result.server}:${result.port}` : "—"}
									</span>
								</Row>
								<Row label="Exit IP">
									<span className="font-mono">{result.ip || "—"}</span>
								</Row>
								<Row label="Country">{result.country || "—"}</Row>
							</section>

							<section>
								<p className="mb-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									Performance
								</p>
								<Row label="Latency">
									{result.alive ? (
										<span className={toneText[latencyTone(result.latency_ms)]}>
											{result.latency_ms}ms
										</span>
									) : (
										"—"
									)}
								</Row>
								<Row label="Download">
									{result.alive && result.speed_kbps
										? formatSpeed(result.speed_kbps)
										: "—"}
								</Row>
								<Row label="Upload">
									{result.alive && result.upload_speed_kbps
										? formatSpeed(result.upload_speed_kbps)
										: "—"}
								</Row>
								<Row label="Traffic">{formatBytes(result.traffic_bytes)}</Row>
							</section>

							<section>
								<p className="mb-1.5 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
									Platforms
								</p>
								<div className="flex flex-wrap gap-1.5">
									{platformRows(result, rules).map((p) => (
										<span
											key={p.key}
											className={cn(
												"inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs",
												p.unlocked
													? "border-success-line bg-success-muted text-success"
													: "border-border text-muted-foreground",
											)}
										>
											{p.unlocked ? "✓" : "✗"} {p.label}
										</span>
									))}
								</div>
							</section>

							<section>
								<details>
									<summary className="cursor-pointer font-medium text-[11px] text-muted-foreground uppercase tracking-[0.4px]">
										Raw config
									</summary>
									<div className="mt-2 flex items-start gap-2">
										<pre className="min-w-0 flex-1 overflow-x-auto rounded-md bg-secondary p-2 font-mono text-[11px] text-foreground">
											{prettyConfig(result.config || "")}
										</pre>
										<CopyButton text={result.config || ""} />
									</div>
								</details>
							</section>
						</div>
					</>
				) : null}
			</DialogContent>
		</Dialog>
	);
}
```

> If `PLATFORM_META[key].label` doesn't exist on the meta shape, open `src/components/platform-icons.tsx` and use the correct property (it exports `PLATFORM_META`); fall back to the key. The `(r as unknown as Record<string, boolean>)[key]` cast reads built-in unlock booleans by key name — they exist on `NodeResult` (netflix, youtube, …).

- [ ] **Step 2: Verify compile + commit**

```bash
cd frontend && bun check-types && bun check
git add src/components/workbench/node-detail-dialog.tsx
git commit -m "feat(frontend): node detail dialog (identity, metrics, platform matrix, raw config)"
```

---

## Task 3: F1 frontend — open the dialog from the node table

**Files:**
- Modify: `frontend/src/components/workbench/node-table.tsx`

- [ ] **Step 1: Add dialog state + render it**

In `frontend/src/components/workbench/node-table.tsx`:

a) Add imports at the top (with the other imports):

```tsx
import { useState } from "react";
import { NodeDetailDialog } from "@/components/workbench/node-detail-dialog";
```

b) Inside the `NodeTable` function, after `const ruleByKey = ...` (or at the top of the function body), add:

```tsx
	const [detail, setDetail] = useState<NodeResult | null>(null);
```

c) Make the desktop row's **name cell** open the dialog. Find the desktop `<td>` rendering `{r.node_name}` and wrap its content in a button:

```tsx
								<td
									className={cn(
										"max-w-52 truncate px-3 py-1.5 font-mono text-[11px]",
										r.alive ? "text-foreground" : "text-muted-foreground/70",
									)}
								>
									<button
										type="button"
										onClick={() => setDetail(r)}
										className="truncate text-left hover:text-primary hover:underline"
									>
										{r.node_name}
									</button>
								</td>
```

d) Make the mobile card's name open the dialog. Find the mobile card's name `<span>` rendering `{r.node_name}` and replace it with:

```tsx
							<button
								type="button"
								onClick={() => setDetail(r)}
								className="min-w-0 flex-1 truncate text-left font-mono text-foreground text-xs hover:text-primary"
							>
								{r.node_name}
							</button>
```

e) Just before the final closing `</>` of the component's returned fragment, add:

```tsx
			<NodeDetailDialog
				result={detail}
				rules={rules}
				open={!!detail}
				onOpenChange={(o) => !o && setDetail(null)}
			/>
```

> The enable toggle (`EnableToggle`, the `●`/`○` button) already has its own `onClick`; it's in a separate cell/element so clicking it does not trigger the name button. No `stopPropagation` needed because the row itself has no click handler — only the name button and the toggle do.

- [ ] **Step 2: Verify in browser**

```bash
cd frontend && bun check-types && bun check
```

Run `encore run` + `bun dev`; open a subscription with results → click a node name → dialog shows identity/perf/platform matrix/raw-config; toggle still works without opening the dialog; mobile (375px) dialog is full-screen.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/workbench/node-table.tsx
git commit -m "feat(frontend): open node detail dialog from results table"
```

---

## Task 4: F2 backend — settings ExportTagConfig (TDD)

**Files:**
- Create: `services/settings/migrations/6_add_export_tags.up.sql`, `6_add_export_tags.down.sql`
- Modify: `services/settings/settings.go`
- Create: `services/settings/settings_test.go`

- [ ] **Step 1: Migration**

Create `services/settings/migrations/6_add_export_tags.up.sql`:

```sql
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS export_tags JSONB;
```

Create `services/settings/migrations/6_add_export_tags.down.sql`:

```sql
ALTER TABLE user_settings DROP COLUMN IF EXISTS export_tags;
```

- [ ] **Step 2: Write the failing test**

Create `services/settings/settings_test.go`:

```go
// services/settings/settings_test.go
package settings

import "testing"

func TestDefaultExportTagsMatchesLegacy(t *testing.T) {
	d := defaultExportTags()
	if !d.ShowSpeed {
		t.Error("ShowSpeed should default true")
	}
	if d.ShowCountry {
		t.Error("ShowCountry should default false (preserve current export names)")
	}
	want := map[string]string{
		"netflix": "NF", "openai": "GPT", "gemini": "GM", "claude": "CL",
		"grok": "GK", "youtube": "YT", "disney": "D+", "tiktok": "TK",
	}
	got := map[string]string{}
	for _, p := range d.Platforms {
		if !p.Enabled {
			t.Errorf("default platform %q should be enabled", p.Key)
		}
		got[p.Key] = p.Label
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("default label for %q: want %q got %q", k, v, got[k])
		}
	}
}

func TestMergeExportTagsOverridesAndKeepsCustom(t *testing.T) {
	stored := ExportTagConfig{
		ShowCountry: true,
		ShowSpeed:   false,
		Platforms: []PlatformTag{
			{Key: "netflix", Label: "Netflix", Enabled: false}, // override builtin
			{Key: "spotify", Label: "Spotify", Enabled: true},  // custom, must survive
		},
	}
	m := mergeExportTags(stored)
	if !m.ShowCountry || m.ShowSpeed {
		t.Errorf("scalar flags not carried: %+v", m)
	}
	byKey := map[string]PlatformTag{}
	for _, p := range m.Platforms {
		byKey[p.Key] = p
	}
	if byKey["netflix"].Label != "Netflix" || byKey["netflix"].Enabled {
		t.Errorf("netflix override lost: %+v", byKey["netflix"])
	}
	if byKey["openai"].Label != "GPT" || !byKey["openai"].Enabled {
		t.Errorf("untouched builtin openai should keep default: %+v", byKey["openai"])
	}
	if byKey["spotify"].Label != "Spotify" || !byKey["spotify"].Enabled {
		t.Errorf("custom spotify dropped: %+v", byKey["spotify"])
	}
}
```

- [ ] **Step 3: Run to verify it fails**

```bash
encore test ./services/settings/ -run "TestDefaultExportTags|TestMergeExportTags" -v
```

Expected: FAIL — `defaultExportTags`/`mergeExportTags`/`ExportTagConfig`/`PlatformTag` undefined.

- [ ] **Step 4: Add types + helpers**

In `services/settings/settings.go`, after the `EmailConfig` struct add:

```go
// PlatformTag is one platform's export-tag rule. Key is a built-in platform
// (netflix, openai, …) or a custom rule key (e.g. spotify).
type PlatformTag struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// ExportTagConfig controls the tags appended to node names in exports.
type ExportTagConfig struct {
	ShowCountry bool          `json:"show_country"`
	ShowSpeed   bool          `json:"show_speed"`
	Platforms   []PlatformTag `json:"platforms"`
}

// defaultExportTags reproduces the legacy taggedName behavior: built-in short
// tags, speed on, country off.
func defaultExportTags() ExportTagConfig {
	return ExportTagConfig{
		ShowCountry: false,
		ShowSpeed:   true,
		Platforms: []PlatformTag{
			{Key: "netflix", Label: "NF", Enabled: true},
			{Key: "openai", Label: "GPT", Enabled: true},
			{Key: "gemini", Label: "GM", Enabled: true},
			{Key: "claude", Label: "CL", Enabled: true},
			{Key: "grok", Label: "GK", Enabled: true},
			{Key: "youtube", Label: "YT", Enabled: true},
			{Key: "disney", Label: "D+", Enabled: true},
			{Key: "tiktok", Label: "TK", Enabled: true},
		},
	}
}

// mergeExportTags overlays a stored config onto the defaults: built-in entries
// take the stored label/enabled when present (defaults otherwise, in default
// order), then any custom (non-built-in) stored entries are appended in their
// stored order. Scalar flags come straight from the stored config.
func mergeExportTags(stored ExportTagConfig) ExportTagConfig {
	storedByKey := map[string]PlatformTag{}
	for _, p := range stored.Platforms {
		storedByKey[p.Key] = p
	}
	out := ExportTagConfig{ShowCountry: stored.ShowCountry, ShowSpeed: stored.ShowSpeed}
	builtinKeys := map[string]bool{}
	for _, def := range defaultExportTags().Platforms {
		builtinKeys[def.Key] = true
		if s, ok := storedByKey[def.Key]; ok {
			out.Platforms = append(out.Platforms, s)
		} else {
			out.Platforms = append(out.Platforms, def)
		}
	}
	for _, p := range stored.Platforms {
		if !builtinKeys[p.Key] {
			out.Platforms = append(out.Platforms, p)
		}
	}
	return out
}
```

> A freshly-stored `ExportTagConfig{}` (zero value) has `ShowSpeed:false` — but that only happens after an explicit save. For never-saved users, `GetSettings` (Step 6) substitutes `defaultExportTags()` wholesale, so the zero value never reaches merge.

- [ ] **Step 5: Add `ExportTags` to `UserSettings`**

In the `UserSettings` struct add:

```go
	ExportTags ExportTagConfig `json:"export_tags"`
```

- [ ] **Step 6: Wire GetSettings / UpdateSettings**

In `GetSettings`, change the query + scan to also read `export_tags`, and substitute defaults when absent:

Replace the query string's column list `COALESCE(email_config, 'null'::jsonb)` with `COALESCE(email_config, 'null'::jsonb), COALESCE(export_tags, 'null'::jsonb)` and add a `var exportTagsJSON []byte` scanned at the end. After the `email_config` unmarshal block add:

```go
	if len(exportTagsJSON) > 0 && string(exportTagsJSON) != "null" {
		var stored ExportTagConfig
		if json.Unmarshal(exportTagsJSON, &stored) == nil {
			s.ExportTags = mergeExportTags(stored)
		} else {
			s.ExportTags = defaultExportTags()
		}
	} else {
		s.ExportTags = defaultExportTags()
	}
```

Also, in the early `ErrNoRows` return (`return &UserSettings{}, nil`), change it to:

```go
		if errors.Is(err, sqldb.ErrNoRows) {
			return &UserSettings{ExportTags: defaultExportTags()}, nil
		}
```

In `UpdateSettings`, marshal and persist export_tags. Add before the Exec:

```go
	exportTagsJSON, _ := json.Marshal(p.ExportTags)
```

Change the INSERT column list / VALUES / DO UPDATE to include `export_tags`:

```go
	if _, err := db.Exec(ctx, `
		INSERT INTO user_settings (user_id, speed_test_url, upload_test_url, latency_test_url, email_config, export_tags)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		  SET speed_test_url   = EXCLUDED.speed_test_url,
		      upload_test_url  = EXCLUDED.upload_test_url,
		      latency_test_url = EXCLUDED.latency_test_url,
		      email_config     = EXCLUDED.email_config,
		      export_tags      = EXCLUDED.export_tags
	`, claims.UserID, p.SpeedTestURL, p.UploadTestURL, p.LatencyTestURL, emailConfigJSON, exportTagsJSON); err != nil {
```

- [ ] **Step 7: Add the internal getter for the export path**

After `GetEmailConfigForUser`, add:

```go
// GetExportTagsForUser is an internal helper used by the checker export path
// (token-auth, no user claims). Returns the user's merged tag config, or
// defaults when unset.
//
//encore:api private method=GET path=/internal/settings/:userID/export-tags
func GetExportTagsForUser(ctx context.Context, userID string) (*ExportTagConfig, error) {
	var exportTagsJSON []byte
	err := db.QueryRow(ctx,
		`SELECT COALESCE(export_tags, 'null'::jsonb) FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&exportTagsJSON)
	if err != nil || len(exportTagsJSON) == 0 || string(exportTagsJSON) == "null" {
		d := defaultExportTags()
		return &d, nil
	}
	var stored ExportTagConfig
	if json.Unmarshal(exportTagsJSON, &stored) != nil {
		d := defaultExportTags()
		return &d, nil
	}
	merged := mergeExportTags(stored)
	return &merged, nil
}
```

- [ ] **Step 8: Run tests + full suite**

```bash
encore test ./services/settings/ -run "TestDefaultExportTags|TestMergeExportTags" -v
encore test ./services/settings/
```

Expected: PASS, suite green.

- [ ] **Step 9: Commit**

```bash
git add services/settings/
git commit -m "feat(settings): export tag config with defaults, merge, and internal getter"
```

---

## Task 5: F2 backend — apply tag config in the export path (TDD)

**Files:**
- Modify: `services/checker/export_data.go`, `services/checker/export.go`
- Create: `services/checker/export_test.go`

- [ ] **Step 1: Write the failing test for `taggedName`**

Create `services/checker/export_test.go`:

```go
// services/checker/export_test.go
package checker

import (
	"testing"

	settingssvc "subs-check-re/services/settings"
)

func legacyCfg() settingssvc.ExportTagConfig {
	return settingssvc.ExportTagConfig{
		ShowCountry: false,
		ShowSpeed:   true,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "NF", Enabled: true},
			{Key: "openai", Label: "GPT", Enabled: true},
			{Key: "youtube", Label: "YT", Enabled: true},
		},
	}
}

func TestTaggedNameLegacyDefault(t *testing.T) {
	flags := unlockFlags{Netflix: true, OpenAI: true}
	got := taggedName("HK-01", "HK", flags, nil, 1536, legacyCfg())
	if got != "HK-01|NF|GPT|1.5MB" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCountryAndPremiumAndDisabled(t *testing.T) {
	cfg := legacyCfg()
	cfg.ShowCountry = true
	cfg.Platforms[1].Enabled = false // disable openai
	flags := unlockFlags{Netflix: true, OpenAI: true, YouTube: true, YouTubePremium: true}
	got := taggedName("JP-1", "JP", flags, nil, 0, cfg)
	// country on; openai disabled; youtube premium => YT+; speed 0 => no speed tag
	if got != "JP-1|JP|NF|YT+" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCustomLabelAndExtraSorted(t *testing.T) {
	cfg := settingssvc.ExportTagConfig{
		ShowSpeed: false,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "Netflix", Enabled: true},
			{Key: "spotify", Label: "Spotify", Enabled: true},
		},
	}
	flags := unlockFlags{Netflix: true}
	extra := map[string]bool{"zlib": true, "spotify": true, "off": false}
	got := taggedName("US", "US", flags, extra, 999, cfg)
	// builtin Netflix first, then extras sorted by key (spotify, zlib);
	// "off" is false so excluded; spotify uses configured label, zlib defaults to key
	if got != "US|Netflix|Spotify|zlib" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
encore test ./services/checker/ -run TestTaggedName -v
```

Expected: FAIL — `unlockFlags` undefined and `taggedName` signature mismatch.

- [ ] **Step 3: Rewrite `taggedName`**

In `services/checker/export_data.go`, add the import `settingssvc "subs-check-re/services/settings"` and `"sort"` (already imported). Replace the entire existing `taggedName` function with:

```go
// unlockFlags carries a node's built-in platform unlock booleans.
type unlockFlags struct {
	Netflix        bool
	YouTube        bool
	YouTubePremium bool
	OpenAI         bool
	Claude         bool
	Gemini         bool
	Grok           bool
	Disney         bool
	TikTok         bool
}

// builtinUnlocked maps a built-in platform key to whether this node unlocked it.
func (f unlockFlags) builtinUnlocked(key string) bool {
	switch key {
	case "netflix":
		return f.Netflix
	case "youtube":
		return f.YouTube
	case "openai":
		return f.OpenAI
	case "claude":
		return f.Claude
	case "gemini":
		return f.Gemini
	case "grok":
		return f.Grok
	case "disney":
		return f.Disney
	case "tiktok":
		return f.TikTok
	default:
		return false
	}
}

// taggedName appends country / platform / speed tags to a node name per cfg.
// Order: country, built-in platforms (cfg order), custom extra_platforms
// (sorted by key), speed. Returns the bare name when no tags apply.
func taggedName(name, country string, f unlockFlags, extra map[string]bool, speedKbps int, cfg settingssvc.ExportTagConfig) string {
	tags := []string{}

	if cfg.ShowCountry && country != "" {
		tags = append(tags, country)
	}

	builtin := map[string]bool{
		"netflix": true, "youtube": true, "openai": true, "claude": true,
		"gemini": true, "grok": true, "disney": true, "tiktok": true,
	}
	cfgByKey := map[string]settingssvc.PlatformTag{}
	for _, p := range cfg.Platforms {
		cfgByKey[p.Key] = p
	}

	for _, p := range cfg.Platforms {
		if !builtin[p.Key] || !p.Enabled {
			continue
		}
		if !f.builtinUnlocked(p.Key) {
			continue
		}
		if p.Key == "youtube" && f.YouTubePremium {
			tags = append(tags, p.Label+"+")
		} else {
			tags = append(tags, p.Label)
		}
	}

	// Custom (extra) platforms, deterministic order.
	keys := make([]string, 0, len(extra))
	for k, v := range extra {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if builtin[k] {
			continue // a custom rule shadowing a builtin key is handled above
		}
		if p, ok := cfgByKey[k]; ok {
			if !p.Enabled {
				continue
			}
			tags = append(tags, p.Label)
		} else {
			tags = append(tags, k) // unconfigured custom key: default enabled, label=key
		}
	}

	if cfg.ShowSpeed && speedKbps > 0 {
		if speedKbps >= 1024 {
			tags = append(tags, fmt.Sprintf("%.1fMB", float64(speedKbps)/1024))
		} else {
			tags = append(tags, fmt.Sprintf("%dKB", speedKbps))
		}
	}

	if len(tags) == 0 {
		return name
	}
	return name + "|" + strings.Join(tags, "|")
}
```

- [ ] **Step 4: Update `loadJobProxies` query, scan, and call site**

In `loadJobProxies`:

a) Signature gains the config:

```go
func loadJobProxies(ctx context.Context, jobID, subscriptionID, subNamePrefix string, cfg settingssvc.ExportTagConfig) ([]map[string]any, error) {
```

b) In the CTE `SELECT`, after `cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,` add `cr.country, cr.extra_platforms,` and ensure the outer `SELECT` lists them. Concretely the inner select adds `cr.country`, `cr.extra_platforms`; add both to the outer `SELECT config, node_name, …` column list after `tiktok,` → `…, tiktok, country, extra_platforms, speed_kbps, latency_ms`.

c) Scan: add `country string` and `extraJSON []byte` vars; extend the `rows.Scan(...)` to include `&country, &extraJSON` in the matching position (after `&tiktok`). After scanning, decode extra:

```go
		var extra map[string]bool
		if len(extraJSON) > 0 {
			_ = json.Unmarshal(extraJSON, &extra)
		}
```

d) Replace the `taggedName(...)` call:

```go
		tagged := taggedName(name, country,
			unlockFlags{
				Netflix: netflix, YouTube: youtube, YouTubePremium: youtubePremium,
				OpenAI: openai, Claude: claude, Gemini: gemini, Grok: grok,
				Disney: disney, TikTok: tiktok,
			}, extra, speedKbps, cfg)
```

- [ ] **Step 5: Thread cfg through the callers**

`latestUsableProxies` and `latestUsableProxiesAcrossAllSubs` each gain a `cfg settingssvc.ExportTagConfig` parameter and pass it to `loadJobProxies(...)`. Their signatures become:

```go
func latestUsableProxies(ctx context.Context, subscriptionID, userID string, cfg settingssvc.ExportTagConfig) ([]map[string]any, error) {
	// ...unchanged body, but:
	return loadJobProxies(ctx, jobID, subscriptionID, "", cfg)
}

func latestUsableProxiesAcrossAllSubs(ctx context.Context, userID string, cfg settingssvc.ExportTagConfig) ([]map[string]any, error) {
	// ...unchanged until the loop:
	proxies, _ := loadJobProxies(ctx, js.jobID, js.subscriptionID, subName, cfg)
	// ...
}
```

In `services/checker/export.go`, `loadExportProxies` fetches the config once and routes it:

```go
func loadExportProxies(ctx context.Context, subID, userID string) ([]map[string]any, error) {
	cfg, err := settingssvc.GetExportTagsForUser(ctx, userID)
	if err != nil || cfg == nil {
		d := settingssvc.ExportTagConfig{ShowSpeed: true} // safe fallback
		cfg = &d
	}
	if subID == "all" {
		return latestUsableProxiesAcrossAllSubs(ctx, userID, *cfg)
	}
	return latestUsableProxies(ctx, subID, userID, *cfg)
}
```

> `export.go` already imports `settingssvc`. The fallback only triggers on a settings-service error; `GetExportTagsForUser` already returns merged defaults for the no-row case.

- [ ] **Step 6: Run taggedName tests + full checker suite**

```bash
encore test ./services/checker/ -run TestTaggedName -v
encore test ./services/checker/
```

Expected: the three taggedName tests PASS; suite green (the existing export tests, if any, still pass — they now exercise the default config path).

- [ ] **Step 7: Commit**

```bash
git add services/checker/export_data.go services/checker/export.go services/checker/export_test.go
git commit -m "feat(checker): apply per-user export tag config (country, custom labels, extra platforms)"
```

---

## Task 6: F2 frontend — Export Tags settings tab

**Files:**
- Create: `frontend/src/routes/settings/export-tags.tsx`
- Modify: `frontend/src/routes/settings.tsx`

- [ ] **Step 1: Add the tab to the Settings layout**

In `frontend/src/routes/settings.tsx`, add to the `TABS` array (after the Export API entry):

```tsx
	{ to: "/settings/export-tags", label: "Export Tags" },
```

- [ ] **Step 2: Create the Export Tags route**

Create `frontend/src/routes/settings/export-tags.tsx`:

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { PLATFORM_META, type PlatformKey } from "@/components/platform-icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isApiError } from "@/lib/client";
import type { settings } from "@/lib/client.gen";
import { BUILTIN_PLATFORMS } from "@/lib/nodeFilters";
import { useRules, useSettings, useUpdateSettings } from "@/queries";

export const Route = createFileRoute("/settings/export-tags")({
	component: ExportTagsPage,
});

type PlatformTag = settings.PlatformTag;

function ExportTagsPage() {
	const settingsQuery = useSettings();
	const rulesQuery = useRules();
	const updateMut = useUpdateSettings();

	const [showCountry, setShowCountry] = useState(false);
	const [showSpeed, setShowSpeed] = useState(true);
	const [tags, setTags] = useState<Record<string, PlatformTag>>({});

	const loaded = settingsQuery.data;
	const rules = rulesQuery.data?.rules ?? [];

	// Seed form state from settings + custom rules once both have loaded.
	// biome-ignore lint/correctness/useExhaustiveDependencies: seed on data identity
	useEffect(() => {
		if (!loaded) return;
		const cfg = loaded.export_tags;
		setShowCountry(cfg?.show_country ?? false);
		setShowSpeed(cfg?.show_speed ?? true);
		const byKey: Record<string, PlatformTag> = {};
		for (const p of cfg?.platforms ?? []) byKey[p.key] = { ...p };
		// Ensure every custom rule has a row (default enabled, label = rule name).
		for (const r of rules) {
			if (!byKey[r.key]) {
				byKey[r.key] = { key: r.key, label: r.name || r.key, enabled: true };
			}
		}
		setTags(byKey);
	}, [loaded, rules]);

	const builtinKeys: string[] = BUILTIN_PLATFORMS.filter(
		(k) => k !== "youtube_premium",
	);
	const builtinSet = new Set(builtinKeys);
	const customKeys = rules.map((r) => r.key).filter((k) => !builtinSet.has(k));

	const setTag = (key: string, patch: Partial<PlatformTag>) =>
		setTags((prev) => ({
			...prev,
			[key]: { key, label: "", enabled: true, ...prev[key], ...patch },
		}));

	const labelFor = (key: string) =>
		PLATFORM_META[key as PlatformKey]?.label ??
		rules.find((r) => r.key === key)?.name ??
		key;

	function buildPreview(): string {
		const parts = ["HK-01"];
		if (showCountry) parts.push("HK");
		const sample = new Set(["netflix", "openai", ...customKeys.slice(0, 1)]);
		for (const k of builtinKeys) {
			const t = tags[k];
			if (t?.enabled && sample.has(k)) parts.push(t.label || k);
		}
		for (const k of [...customKeys].sort()) {
			const t = tags[k];
			if (t?.enabled && sample.has(k)) parts.push(t.label || k);
		}
		if (showSpeed) parts.push("10.5MB");
		return parts.join("｜");
	}

	function save() {
		if (!loaded) return;
		const platforms: PlatformTag[] = [
			...builtinKeys.map((k) => tags[k]).filter(Boolean),
			...customKeys.map((k) => tags[k]).filter(Boolean),
		];
		const next: settings.UserSettings = {
			...loaded,
			export_tags: { show_country: showCountry, show_speed: showSpeed, platforms },
		};
		updateMut.mutate(next, {
			onSuccess: () => toast.success("Export tags saved"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Failed to save"),
		});
	}

	if (settingsQuery.isLoading) {
		return <Skeleton className="h-64 w-full" />;
	}

	return (
		<div className="space-y-4">
			<p className="text-muted-foreground text-xs">
				Tags appended to node names in every export, e.g.{" "}
				<code className="rounded bg-secondary px-1 font-mono">{buildPreview()}</code>
			</p>

			<section className="space-y-3 rounded-lg border border-border bg-card p-4">
				<label className="flex items-center justify-between gap-3 text-sm">
					Detected country
					<Switch
						checked={showCountry}
						onCheckedChange={(v) => setShowCountry(v === true)}
					/>
				</label>
				<label className="flex items-center justify-between gap-3 text-sm">
					Speed
					<Switch
						checked={showSpeed}
						onCheckedChange={(v) => setShowSpeed(v === true)}
					/>
				</label>
			</section>

			<section className="rounded-lg border border-border bg-card p-4">
				<p className="mb-3 font-medium text-foreground text-sm">Platforms</p>
				<div className="space-y-2">
					{[...builtinKeys, ...customKeys].map((key) => {
						const t = tags[key];
						return (
							<div key={key} className="flex items-center gap-3">
								<Switch
									checked={t?.enabled ?? true}
									onCheckedChange={(v) => setTag(key, { enabled: v === true })}
								/>
								<Label className="w-28 shrink-0 truncate text-xs">
									{labelFor(key)}
								</Label>
								<Input
									value={t?.label ?? ""}
									placeholder={key}
									onChange={(e) => setTag(key, { label: e.target.value })}
									className="h-7 max-w-40 text-xs"
								/>
							</div>
						);
					})}
				</div>
			</section>

			<div className="flex justify-end">
				<Button variant="success" loading={updateMut.isPending} onClick={save}>
					Save export tags
				</Button>
			</div>
		</div>
	);
}
```

> `settings.PlatformTag` / `settings.UserSettings.export_tags` come from the regenerated client (Task 4 changed the Go types; regen happens in Step 3 below). If `PLATFORM_META[key].label` isn't the right property, match `platform-icons.tsx` (same note as Task 2). `useRules` returns `{ rules: PlatformRule[] }`.

- [ ] **Step 3: Regenerate client (picks up settings type changes) + verify**

```bash
cd frontend && bun run gen:client && bun check-types && bun check
```

Expected: `client.gen.ts` has `settings.PlatformTag` and `export_tags` on `UserSettings`; tsc clean.

- [ ] **Step 4: Browser verification**

`encore run` + `bun dev`. Settings → Export Tags: toggle country on, rename `NF`→`Netflix`, disable one platform, see a custom rule row, toggle speed; the preview line updates live; Save shows loading + toast. Then open an export URL (Settings → Export API) and confirm node names reflect the config.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): Export Tags settings tab"
```

---

## Task 7: Full verification

- [ ] **Step 1: Automated gate**

```bash
cd frontend && bun check-types && bun check && bun run test:unit && bun run build
cd .. && encore test ./services/...
```

All green.

- [ ] **Step 2: End-to-end browser walkthrough (1440 + 375 px)**

With `encore run` + `bun dev`:
1. Open a subscription with results → click a node name → detail dialog (identity/perf/platform matrix/raw config + copy); enable toggle still works independently; mobile full-screen.
2. Settings → Export Tags → country on, custom label, disable a platform, enable a custom-rule tag, save.
3. Open the clash + base64 export URLs → node names show the configured tags (country present, renamed/omitted built-ins, custom platform sorted in, premium `YT+`, speed per toggle).
4. Reset Export Tags to defaults (country off, default labels, speed on) → export names match pre-feature output.

- [ ] **Step 3: Final commit (if any walkthrough fixes)**

```bash
git add -A && git commit -m "fix(node-details-export-tags): walkthrough fixes"
```

(Skip if nothing changed.)

---

## Execution order

```
F1  Task 1  backend NodeResult/GetResults (+test, regen)
    Task 2  NodeDetailDialog
    Task 3  wire into node-table
F2  Task 4  settings ExportTagConfig (migration, types, merge, getter; TDD)
    Task 5  checker export taggedName + threading (TDD)
    Task 6  Export Tags settings tab (regen)
    Task 7  full verification
```
