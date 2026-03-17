# Export All Subscriptions Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/export/all` endpoint that combines alive nodes from all subscriptions into one link, and surface it on the dashboard.

**Architecture:** Three independent changes: (1) a new private endpoint in the subscription service to resolve subscription names by ID, (2) a new raw public endpoint in the checker service that queries the latest completed job per subscription, calls the private endpoint for names, collects all alive nodes, prefixes them with the subscription name, sorts by speed, and renders; (3) a frontend card above the per-subscription export list showing the combined URL.

**Tech Stack:** Go + Encore (backend), React 19 + TanStack Query (frontend), Biome (linting)

---

## Files Changed

| File | Action | Responsibility |
|------|--------|----------------|
| `services/subscription/subscription.go` | Modify | Add `GetSubscriptionNames` private endpoint + its param/response types |
| `services/subscription/subscription_test.go` | Modify | Tests for the new endpoint |
| `services/checker/export.go` | Modify | Add `ExportAll` raw endpoint; add `"sort"` import and `subsvc` import |
| `services/checker/checker_test.go` | Modify | Test for `ExportAll` missing-token guard |
| `frontend/apps/web/src/routes/index.tsx` | Modify | Add "All Subscriptions" export card above per-subscription list |

No database migrations needed.

---

## Chunk 1: Backend

### Task 1: GetSubscriptionNames private endpoint

**Files:**
- Modify: `services/subscription/subscription.go`
- Modify: `services/subscription/subscription_test.go`

The subscription service has its own Encore-managed DB. The checker service cannot query it directly. This private endpoint lets the checker resolve subscription names by ID + user_id.

- [ ] **Step 1: Write failing tests**

Add to `services/subscription/subscription_test.go` after `TestDeleteSubscription`:

```go
func TestGetSubscriptionNamesEmpty(t *testing.T) {
	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "nonexistent-user",
		IDs:    []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Names) != 0 {
		t.Errorf("expected empty names map, got %v", resp.Names)
	}
}

func TestGetSubscriptionNamesResolvesNames(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{
		Name: "MyProxy",
		URL:  "https://example.com/sub-names.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "test-user-id",
		IDs:    []string{created.ID},
	})
	if err != nil {
		t.Fatalf("GetSubscriptionNames failed: %v", err)
	}
	if resp.Names[created.ID] != "MyProxy" {
		t.Errorf("expected name %q, got %q", "MyProxy", resp.Names[created.ID])
	}
}

func TestGetSubscriptionNamesWrongUser(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{
		Name: "SomeProxy",
		URL:  "https://example.com/sub-wrong.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "different-user-id",
		IDs:    []string{created.ID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := resp.Names[created.ID]; found {
		t.Error("expected no name returned for wrong user")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
encore test -run TestGetSubscriptionNames ./services/subscription/...
```

Expected: FAIL — `GetSubscriptionNames`, `GetSubscriptionNamesParams`, `GetSubscriptionNamesResponse` undefined.

- [ ] **Step 3: Implement the endpoint**

Add to the end of `services/subscription/subscription.go` (before the final `}`):

```go
// GetSubscriptionNamesParams is the request for the internal name-lookup endpoint.
type GetSubscriptionNamesParams struct {
	UserID string   `json:"user_id"`
	IDs    []string `json:"ids"`
}

// GetSubscriptionNamesResponse maps subscription ID → name.
type GetSubscriptionNamesResponse struct {
	Names map[string]string `json:"names"`
}

// GetSubscriptionNames resolves subscription names by ID for internal service calls.
//
//encore:api private method=POST path=/internal/subscriptions/names
func GetSubscriptionNames(ctx context.Context, p *GetSubscriptionNamesParams) (*GetSubscriptionNamesResponse, error) {
	if len(p.IDs) == 0 {
		return &GetSubscriptionNamesResponse{Names: map[string]string{}}, nil
	}
	rows, err := db.Query(ctx, `
		SELECT id, name FROM subscriptions
		WHERE id = ANY($1) AND user_id = $2
	`, p.IDs, p.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db query failed").Err()
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		names[id] = name
	}
	return &GetSubscriptionNamesResponse{Names: names}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
encore test -run TestGetSubscriptionNames ./services/subscription/...
```

Expected: PASS — all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/subscription/subscription.go services/subscription/subscription_test.go
git commit -m "feat(subscription): add GetSubscriptionNames private endpoint"
```

---

### Task 2: ExportAll endpoint

**Files:**
- Modify: `services/checker/export.go`
- Modify: `services/checker/checker_test.go`

This raw endpoint collects the latest completed job per subscription for the user (via DISTINCT ON in the checker DB), resolves subscription names via the private endpoint added in Task 1, prefixes each node's name with the subscription name, sorts all nodes by speed DESC / latency ASC, and renders.

- [ ] **Step 1: Write failing test**

Add to `services/checker/checker_test.go` after `TestListJobsEmpty`:

```go
func TestExportAllMissingToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/export/all", nil)
	w := httptest.NewRecorder()
	ExportAll(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
```

Also ensure `"net/http/httptest"` is in the imports (it was already added for `TestCountingTransport`).

- [ ] **Step 2: Run test to verify it fails**

```bash
encore test -run TestExportAllMissingToken ./services/checker/...
```

Expected: FAIL — `ExportAll` undefined.

- [ ] **Step 3: Add ExportAll to export.go**

Update the import block in `services/checker/export.go` — add `"sort"` and the subscription service import:

```go
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	settingssvc "subs-check-re/services/settings"
	subsvc "subs-check-re/services/subscription"
	"gopkg.in/yaml.v3"
)
```

Then add the `ExportAll` function after the `Export` function (before `clientIP`):

```go
// ExportAll generates a combined subscription link from alive nodes across all subscriptions.
//
//encore:api public raw method=GET path=/export/all
func ExportAll(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	token := req.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}
	target := req.URL.Query().Get("target")
	if target == "" {
		target = "clash"
	}

	// Resolve token → user_id.
	userResp, err := settingssvc.GetUserIDByAPIKey(ctx, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID := userResp.UserID

	// Get the latest completed job per subscription owned by this user.
	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (subscription_id) id AS job_id, subscription_id
		FROM check_jobs
		WHERE user_id = $1 AND status = 'completed'
		ORDER BY subscription_id, created_at DESC
	`, userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type jobSub struct {
		jobID          string
		subscriptionID string
	}
	var jobs []jobSub
	var subIDs []string
	for rows.Next() {
		var js jobSub
		if err := rows.Scan(&js.jobID, &js.subscriptionID); err != nil {
			continue
		}
		jobs = append(jobs, js)
		subIDs = append(subIDs, js.subscriptionID)
	}

	if len(jobs) == 0 {
		switch target {
		case "base64":
			renderBase64(w, nil)
		default:
			renderClash(w, nil)
		}
		return
	}

	// Resolve subscription names via subscription service.
	namesResp, err := subsvc.GetSubscriptionNames(ctx, &subsvc.GetSubscriptionNamesParams{
		UserID: userID,
		IDs:    subIDs,
	})
	if err != nil {
		http.Error(w, "name lookup failed", http.StatusInternalServerError)
		return
	}
	subNames := namesResp.Names

	// Collect alive nodes from all jobs, prefix names with subscription name.
	type rankedNode struct {
		config    map[string]any
		speedKbps int
		latencyMs int
	}
	var allNodes []rankedNode

	for _, js := range jobs {
		subName := subNames[js.subscriptionID]
		if subName == "" {
			subName = js.subscriptionID[:8]
		}

		func() {
			nodeRows, err := db.Query(ctx, `
				SELECT COALESCE(n.config, cr.node_config) AS config,
				       COALESCE(n.name, cr.node_name) AS node_name,
				       cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
				       cr.speed_kbps, cr.latency_ms
				FROM check_results cr
				LEFT JOIN nodes n ON n.id = cr.node_id
				WHERE cr.job_id = $1 AND cr.alive = true
			`, js.jobID)
			if err != nil {
				return
			}
			defer nodeRows.Close()

			for nodeRows.Next() {
				var (
					configJSON                                                                     []byte
					name                                                                           string
					netflix, youtube, youtubePremium, openai, claude, gemini, grok, disney, tiktok bool
					speedKbps, latencyMs                                                           int
				)
				if err := nodeRows.Scan(&configJSON, &name,
					&netflix, &youtube, &youtubePremium, &openai, &claude, &gemini, &grok, &disney, &tiktok,
					&speedKbps, &latencyMs); err != nil {
					continue
				}
				if len(configJSON) == 0 {
					continue
				}
				var cfg map[string]any
				if err := json.Unmarshal(configJSON, &cfg); err != nil {
					continue
				}
				tagged := taggedName(name, netflix, youtube, youtubePremium, openai, claude, gemini, grok, disney, tiktok, speedKbps)
				cfg["name"] = subName + "|" + tagged
				allNodes = append(allNodes, rankedNode{config: cfg, speedKbps: speedKbps, latencyMs: latencyMs})
			}
		}()
	}

	// Sort all nodes by speed DESC, latency ASC.
	sort.Slice(allNodes, func(i, j int) bool {
		if allNodes[i].speedKbps != allNodes[j].speedKbps {
			return allNodes[i].speedKbps > allNodes[j].speedKbps
		}
		return allNodes[i].latencyMs < allNodes[j].latencyMs
	})

	proxies := make([]map[string]any, len(allNodes))
	for i, n := range allNodes {
		proxies[i] = n.config
	}

	// Log export (best effort).
	ip := clientIP(req)
	db.Exec(ctx, `
		INSERT INTO export_logs (id, subscription_id, user_id, ip, requested_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New().String(), "all", userID, ip, time.Now()) //nolint:errcheck

	switch target {
	case "base64":
		renderBase64(w, proxies)
	default:
		renderClash(w, proxies)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
encore test -run TestExportAllMissingToken ./services/checker/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/export.go services/checker/checker_test.go
git commit -m "feat(checker): add ExportAll endpoint combining all subscriptions"
```

---

## Chunk 2: Frontend

### Task 3: All Subscriptions export card on dashboard

**Files:**
- Modify: `frontend/apps/web/src/routes/index.tsx`

Add a card for the combined export URL above the per-subscription list. Only render it when `subs.length > 0 && apiKey`. Use the same visual style as the per-subscription cards.

- [ ] **Step 1: Add the "All Subscriptions" card**

In `frontend/apps/web/src/routes/index.tsx`, find the `{/* Per-subscription URLs */}` comment block and add the "All Subscriptions" card **above** it.

Replace:
```tsx
			{/* Per-subscription URLs */}
			{subs.length > 0 && apiKey && (
```

With:
```tsx
			{/* All Subscriptions combined export */}
			{subs.length > 0 && apiKey && (
				<div
					className="space-y-2 rounded-lg border p-4"
					style={{ background: "#161b22", borderColor: "#30363d" }}
				>
					<p className="font-medium text-[#f0f6fc] text-sm">
						All Subscriptions
					</p>
					<div className="flex flex-col gap-1.5">
						{(["clash", "base64"] as const).map((t) => {
							const url = `${origin}/api/export/all?token=${apiKey}&target=${t}`;
							return (
								<div key={t} className="flex items-center gap-2">
									<code className="flex-1 truncate rounded bg-[#0d1117] px-2 py-1 font-mono text-[#8b949e] text-[11px]">
										{url}
									</code>
									<button
										type="button"
										onClick={() => {
											navigator.clipboard.writeText(url);
											toast.success("Copied");
										}}
										className="flex-shrink-0 rounded border px-2 py-1 text-[11px] hover:bg-white/5"
										style={{ borderColor: "#30363d", color: "#6e7681" }}
									>
										{t}
									</button>
								</div>
							);
						})}
					</div>
				</div>
			)}

			{/* Per-subscription URLs */}
			{subs.length > 0 && apiKey && (
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && bun check-types
```

Expected: exits 0 with no errors.

- [ ] **Step 3: Lint**

```bash
cd frontend && bun check
```

Expected: exits 0 (Biome may auto-fix minor style issues; that's fine).

- [ ] **Step 4: Commit**

```bash
git add frontend/apps/web/src/routes/index.tsx
git commit -m "feat(dashboard): add All Subscriptions combined export card"
```
