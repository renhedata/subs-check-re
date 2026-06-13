# Verge Streaming Parity — Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the checker's boolean per-platform unlock model with verge-parity `{unlocked, status, region}` outcomes stored in a unified `platforms jsonb` column, extend the JS rule engine (POST/headers/cookies/object-return), rewrite the 15 default rules to clash-verge-rev logic, add a default-rule sync, and switch latency to verge's single Cloudflare probe.

**Architecture:** Rules stay the mechanism. `runRule` returns a `PlatformOutcome`. Every check_results consumer (write, read, summary, export, notify, local probe) moves to the jsonb map. The spec is `docs/superpowers/specs/2026-06-14-verge-streaming-parity-design.md`.

**Tech Stack:** Go + Encore, goja (JS), PostgreSQL (Encore migrations), `encore test` (run WITHOUT `-race` — known harness hang in this repo).

**Frontend is a separate plan** (`2026-06-14-verge-streaming-parity-frontend.md`); the contract between them is the regenerated `client.gen.ts` produced by Task 15.

---

## Conventions

- Run Go tests with: `encore test ./services/checker/... ./services/notify/... ./services/settings/...` (NO `-race`).
- All work on branch `feat/verge-streaming-parity` (already created).
- After each task, the build must compile: `encore build` is heavy; prefer `go build ./services/...` from repo root to typecheck.

---

## Task 1: `PlatformOutcome` type + engine returns it

**Files:**
- Modify: `services/checker/rules.go` (add type near `builtinKeys`)
- Modify: `services/checker/engine.go` (`runRule`, `runUserRules`, `runUserRulesWithDebug`)
- Modify: `services/checker/rule_eval.go` (`evaluateRuleForNode` uses `.Unlocked`)
- Modify: `services/checker/local_check.go` (`ruleResult.outcome`, `runDefaultRulesAgainst`)
- Test: `services/checker/engine_outcome_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/engine_outcome_test.go`:

```go
package checker

import (
	"context"
	"net/http"
	"testing"
)

// roundTripFunc lets tests serve canned responses for any URL a rule requests.
type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

// mockClient returns an *http.Client whose responses are keyed by exact request URL.
// Unmatched URLs return 404 with an empty body.
func mockClient(byURL map[string]mockResp) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) *http.Response {
		m, ok := byURL[r.URL.String()]
		if !ok {
			return &http.Response{StatusCode: 404, Body: http.NoBody, Request: r, Header: http.Header{}}
		}
		return m.toResponse(r)
	})}
}

func TestRunRule_ConditionNormalizesToOutcome(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"https://example.com/": {status: 200, body: "hello world"},
	})
	rule := &PlatformRule{
		RuleType:   "condition",
		Key:        "demo",
		Enabled:    true,
		Definition: []byte(`{"url":"https://example.com/","status_code":200,"body_contains":["hello"]}`),
	}
	out, err := runRule(context.Background(), client, rule, nil)
	if err != nil {
		t.Fatalf("runRule error: %v", err)
	}
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v, want Unlocked=true Status=Yes", out)
	}
}
```

Also create the shared `mockResp` helper `services/checker/mockresp_test.go`:

```go
package checker

import (
	"io"
	"net/http"
	"strings"
)

type mockResp struct {
	status  int
	body    string
	headers map[string]string
	// finalURL, when set, becomes resp.Request.URL so rules see a post-redirect URL.
	finalURL string
}

func (m mockResp) toResponse(r *http.Request) *http.Response {
	h := http.Header{}
	for k, v := range m.headers {
		h.Set(k, v)
	}
	req := r
	if m.finalURL != "" {
		u := *r
		parsed := r.URL
		if p, err := parsed.Parse(m.finalURL); err == nil {
			u.URL = p
		}
		req = &u
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     h,
		Request:    req,
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestRunRule_ConditionNormalizesToOutcome`
Expected: FAIL — `runRule` currently returns `(bool, error)`, so `out.Unlocked` does not compile.

- [ ] **Step 3: Add the type and change signatures**

In `services/checker/rules.go`, add after the `builtinKeys` block (line ~28):

```go
// PlatformOutcome is the result of evaluating one platform rule for one node.
type PlatformOutcome struct {
	Unlocked bool   `json:"unlocked"`
	Status   string `json:"status"`
	Region   string `json:"region,omitempty"`
}

// boolOutcome normalizes a bare boolean rule result into a PlatformOutcome.
func boolOutcome(ok bool) PlatformOutcome {
	if ok {
		return PlatformOutcome{Unlocked: true, Status: "Yes"}
	}
	return PlatformOutcome{Unlocked: false, Status: "No"}
}
```

In `services/checker/engine.go`, replace `runRule`, `runUserRules`, `runUserRulesWithDebug`:

```go
// runRule dispatches a PlatformRule to the correct engine and normalizes the result.
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (PlatformOutcome, error) {
	if dr != nil {
		dr.Variable("rule_name", rule.Name)
		dr.Variable("rule_type", rule.RuleType)
		dr.Variable("rule_key", rule.Key)
	}
	switch rule.RuleType {
	case "condition":
		ok, err := runConditionRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	case "js", "ts":
		return runJSRule(ctx, client, rule.RuleType, rule.Definition, dr)
	case "tengo":
		ok, err := runTengoRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	case "lua":
		ok, err := runLuaRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	default:
		err := fmt.Errorf("unknown rule_type: %s", rule.RuleType)
		if dr != nil {
			dr.Error(err)
		}
		return PlatformOutcome{}, err
	}
}

// runUserRules runs all enabled rules against the provided HTTP client.
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]PlatformOutcome {
	return runUserRulesWithDebug(ctx, client, rules, nil)
}

// runUserRulesWithDebug runs all enabled rules and optionally collects per-rule debug traces.
func runUserRulesWithDebug(ctx context.Context, client *http.Client, rules []*PlatformRule, results map[string]*DebugRecorder) map[string]PlatformOutcome {
	out := make(map[string]PlatformOutcome, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		var dr *DebugRecorder
		if results != nil {
			dr = &DebugRecorder{}
			results[rule.Key] = dr
		}
		outcome, _ := runRule(ctx, client, rule, dr)
		out[rule.Key] = outcome
	}
	return out
}
```

In `services/checker/engine_script.go`, change `runJSRule`'s signature and return so it compiles now (object-return added in Task 2). For this task, keep the bool semantics but return an outcome:

```go
func runJSRule(ctx context.Context, client *http.Client, ruleType string, defRaw json.RawMessage, dr *DebugRecorder) (PlatformOutcome, error) {
```

and replace its three `return false, ...` / `return result, nil` tails:
- early-error returns become `return PlatformOutcome{}, err`
- the final block becomes:

```go
	result := val.ToBoolean()
	if dr != nil {
		dr.Variable("return_value", result)
	}
	return boolOutcome(result), nil
```

In `services/checker/rule_eval.go`, `evaluateRuleForNode` uses `runRule`’s bool via `ok`:

```go
	rule := &PlatformRule{RuleType: ruleType, Definition: definition}
	outcome, err := runRule(ctx, httpClient, rule, dr)
	ok := outcome.Unlocked
	ms := time.Since(start).Milliseconds()
```

(the rest of the function already uses `ok`).

In `services/checker/local_check.go`, change `ruleResult` and `runDefaultRulesAgainst`:

```go
type ruleResult struct {
	key     string
	outcome PlatformOutcome
}
```

and inside `runDefaultRulesAgainst`'s goroutine:

```go
		outcome, _ := runRule(ctx, client, rule, nil)
		out <- ruleResult{key: d.key, outcome: outcome}
```

and the error path `out <- ruleResult{key: d.key, ok: false}` → `out <- ruleResult{key: d.key, outcome: PlatformOutcome{}}`.

In `GetLocalUnlock` (same file), the switch `res.Netflix = r.ok` becomes `res.Netflix = r.outcome.Unlocked` for each case (Task 10 replaces this wholesale, but make it compile now).

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestRunRule_ConditionNormalizesToOutcome`
Expected: PASS. Then `go build ./services/...` succeeds.

- [ ] **Step 5: Commit**

```bash
git add services/checker/rules.go services/checker/engine.go services/checker/engine_script.go services/checker/rule_eval.go services/checker/local_check.go services/checker/engine_outcome_test.go services/checker/mockresp_test.go
git commit -m "refactor(checker): rules return PlatformOutcome instead of bool"
```

---

## Task 2: Engine extensions — `http_post`, response headers, cookie jar, object return

**Files:**
- Modify: `services/checker/engine_script.go`
- Modify: `services/checker/httputil.go` (surface headers in `httpRequestResult`)
- Test: `services/checker/engine_script_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/engine_script_test.go`:

```go
package checker

import (
	"context"
	"net/http"
	"testing"
)

func TestJSRule_ObjectReturnWithRegion(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"https://api.test/loc": {status: 200, body: "loc=US"},
	})
	def := []byte(`{"code":"var r=http_get('https://api.test/loc'); var m=r.body.match(/loc=([A-Z]{2})/); return {unlocked:true, status:'Yes', region:m[1]};"}`)
	out, err := runJSRule(context.Background(), client, "js", def, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Unlocked || out.Status != "Yes" || out.Region != "US" {
		t.Fatalf("got %+v", out)
	}
}

func TestJSRule_HTTPPostAndHeaders(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) *http.Response {
		if r.Method == "POST" && r.URL.String() == "https://api.test/dev" {
			return mockResp{status: 200, body: `{"assertion":"abc"}`, headers: map[string]string{"x-region": "JP"}}.toResponse(r)
		}
		return mockResp{status: 404}.toResponse(r)
	})}
	def := []byte(`{"code":"var r=http_post('https://api.test/dev',{headers:{'authorization':'Bearer x'},body:'{}'}); return {unlocked:r.status===200, status:r.headers['x-region']||'', region:r.headers['x-region']||''};"}`)
	out, err := runJSRule(context.Background(), client, "js", def, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Unlocked || out.Region != "JP" {
		t.Fatalf("got %+v", out)
	}
}

func TestJSRule_BareBoolStillWorks(t *testing.T) {
	client := mockClient(map[string]mockResp{"https://api.test/x": {status: 200, body: "ok"}})
	def := []byte(`{"code":"var r=http_get('https://api.test/x'); return r.status===200;"}`)
	out, _ := runJSRule(context.Background(), client, "js", def, nil)
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestJSRule_`
Expected: FAIL — `http_post` undefined; `r.headers` undefined; object return ignored.

- [ ] **Step 3: Implement engine extensions**

In `services/checker/httputil.go`, add headers to the result struct and populate it:

```go
type httpRequestResult struct {
	Status   int
	Body     string
	FinalURL string
	Headers  map[string]string
	Err      error
}
```

In `trackedHTTPRequest`, set `Headers: flattenHeaders(resp.Header)` in the returned struct (it already computes `flattenHeaders(resp.Header)` for the debug recorder — reuse it):

```go
	flat := flattenHeaders(resp.Header)
	if dr != nil {
		dr.HTTPResp(resp.StatusCode, flat, string(rawBody), durationMs, resp.Request.URL.String())
	}
	return httpRequestResult{
		Status:   resp.StatusCode,
		Body:     string(rawBody),
		FinalURL: resp.Request.URL.String(),
		Headers:  lowerKeys(flat),
	}
```

Add helper at the bottom of `httputil.go`:

```go
// lowerKeys returns a copy of h with lower-cased keys (rules look up headers case-insensitively).
func lowerKeys(h map[string]string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[strings.ToLower(k)] = v
	}
	return out
}
```

In `services/checker/engine_script.go`:

1. Extend the result struct returned to scripts:

```go
type httpGetResult struct {
	Status   int               `json:"status"`
	Body     string            `json:"body"`
	FinalURL string            `json:"final_url"`
	Headers  map[string]string `json:"headers"`
}
```

2. Give the VM a cookie-jar-backed client for the duration of the rule. At the top of `runJSRule`, after creating `vm`, wrap the incoming client with a jar (do not mutate the caller's client):

```go
	jar, _ := cookiejar.New(nil)
	ruleClient := &http.Client{Transport: client.Transport, Timeout: client.Timeout, Jar: jar}
```

and pass `ruleClient` into `injectHTTP(...)` below. (Add `"net/http/cookiejar"` to imports.)

3. Replace `injectHTTPGet` with `injectHTTP` that registers both `http_get` and `http_post`/`http_request`:

```go
func injectHTTP(ctx context.Context, vm *goja.Runtime, client *http.Client, dr *DebugRecorder) error {
	do := func(method, url string, headers map[string]string, body string) goja.Value {
		res := trackedHTTPRequest(ctx, client, method, url, headers, []byte(body), dr)
		if res.Err != nil {
			panic(vm.ToValue(res.Err.Error()))
		}
		return vm.ToValue(httpGetResult{
			Status: res.Status, Body: res.Body, FinalURL: res.FinalURL, Headers: res.Headers,
		})
	}
	parseOpts := func(call goja.FunctionCall) (map[string]string, string) {
		headers := map[string]string{}
		body := ""
		if len(call.Arguments) > 1 {
			if opts, ok := call.Arguments[1].Export().(map[string]interface{}); ok {
				if h, ok := opts["headers"].(map[string]interface{}); ok {
					for k, v := range h {
						headers[k] = fmt.Sprintf("%v", v)
					}
				}
				if b, ok := opts["body"]; ok && b != nil {
					body = fmt.Sprintf("%v", b)
				}
			}
		}
		return headers, body
	}
	get := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http_get requires a URL"))
		}
		h, _ := parseOpts(call)
		return do("GET", call.Arguments[0].String(), h, "")
	}
	post := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http_post requires a URL"))
		}
		h, b := parseOpts(call)
		return do("POST", call.Arguments[0].String(), h, b)
	}
	request := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http_request requires an options object"))
		}
		opts, _ := call.Arguments[0].Export().(map[string]interface{})
		method, _ := opts["method"].(string)
		if method == "" {
			method = "GET"
		}
		url, _ := opts["url"].(string)
		headers := map[string]string{}
		if h, ok := opts["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				headers[k] = fmt.Sprintf("%v", v)
			}
		}
		body := ""
		if b, ok := opts["body"]; ok && b != nil {
			body = fmt.Sprintf("%v", b)
		}
		return do(method, url, headers, body)
	}
	if err := vm.Set("http_get", get); err != nil {
		return err
	}
	if err := vm.Set("http_post", post); err != nil {
		return err
	}
	return vm.Set("http_request", request)
}
```

Update the call site in `runJSRule`: `injectHTTPGet(ctx, vm, client, dr)` → `injectHTTP(ctx, vm, ruleClient, dr)`.

4. Object-or-bool return. Replace the final block of `runJSRule`:

```go
	exported := val.Export()
	if obj, ok := exported.(map[string]interface{}); ok {
		out := PlatformOutcome{}
		if u, ok := obj["unlocked"].(bool); ok {
			out.Unlocked = u
		}
		if s, ok := obj["status"].(string); ok {
			out.Status = s
		}
		if r, ok := obj["region"].(string); ok {
			out.Region = r
		}
		if out.Status == "" {
			out.Status = boolOutcome(out.Unlocked).Status
		}
		if dr != nil {
			dr.Variable("return_value", out)
		}
		return out, nil
	}
	result := val.ToBoolean()
	if dr != nil {
		dr.Variable("return_value", result)
	}
	return boolOutcome(result), nil
```

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestJSRule_`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add services/checker/engine_script.go services/checker/httputil.go services/checker/engine_script_test.go
git commit -m "feat(checker): JS engine gains http_post, response headers, cookie jar, object return"
```

---

## Task 3: Migration 22 — unified `platforms jsonb`

**Files:**
- Create: `services/checker/migrations/22_unify_platforms.up.sql`
- Create: `services/checker/migrations/22_unify_platforms.down.sql`

- [ ] **Step 1: Write the up migration**

`services/checker/migrations/22_unify_platforms.up.sql`:

```sql
ALTER TABLE check_results ADD COLUMN platforms jsonb NOT NULL DEFAULT '{}';

-- Backfill builtin bool columns + extra_platforms into the unified map.
UPDATE check_results SET platforms = (
  SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
  FROM (
    SELECT k AS key,
           jsonb_build_object('unlocked', v, 'status', CASE WHEN v THEN 'Yes' ELSE 'No' END, 'region', '') AS value
    FROM (VALUES
      ('netflix', netflix), ('youtube', youtube), ('youtube_premium', youtube_premium),
      ('openai', openai), ('claude', claude), ('gemini', gemini), ('grok', grok),
      ('disney', disney), ('tiktok', tiktok)
    ) AS b(k, v)
    UNION ALL
    SELECT ep.key,
           jsonb_build_object('unlocked', (ep.value)::boolean,
                              'status', CASE WHEN (ep.value)::boolean THEN 'Yes' ELSE 'No' END,
                              'region', '')
    FROM jsonb_each(COALESCE(extra_platforms, '{}'::jsonb)) AS ep
  ) merged
);

ALTER TABLE check_results
  DROP COLUMN netflix, DROP COLUMN youtube, DROP COLUMN youtube_premium,
  DROP COLUMN openai, DROP COLUMN claude, DROP COLUMN gemini, DROP COLUMN grok,
  DROP COLUMN disney, DROP COLUMN tiktok, DROP COLUMN extra_platforms;
```

- [ ] **Step 2: Write the down migration**

`services/checker/migrations/22_unify_platforms.down.sql`:

```sql
ALTER TABLE check_results
  ADD COLUMN netflix boolean NOT NULL DEFAULT false,
  ADD COLUMN youtube boolean NOT NULL DEFAULT false,
  ADD COLUMN youtube_premium boolean NOT NULL DEFAULT false,
  ADD COLUMN openai boolean NOT NULL DEFAULT false,
  ADD COLUMN claude boolean NOT NULL DEFAULT false,
  ADD COLUMN gemini boolean NOT NULL DEFAULT false,
  ADD COLUMN grok boolean NOT NULL DEFAULT false,
  ADD COLUMN disney boolean NOT NULL DEFAULT false,
  ADD COLUMN tiktok boolean NOT NULL DEFAULT false,
  ADD COLUMN extra_platforms jsonb NOT NULL DEFAULT '{}';

UPDATE check_results SET
  netflix         = COALESCE((platforms->'netflix'->>'unlocked')::boolean, false),
  youtube         = COALESCE((platforms->'youtube'->>'unlocked')::boolean, false),
  youtube_premium = COALESCE((platforms->'youtube_premium'->>'unlocked')::boolean, false),
  openai          = COALESCE((platforms->'openai'->>'unlocked')::boolean, false),
  claude          = COALESCE((platforms->'claude'->>'unlocked')::boolean, false),
  gemini          = COALESCE((platforms->'gemini'->>'unlocked')::boolean, false),
  grok            = COALESCE((platforms->'grok'->>'unlocked')::boolean, false),
  disney          = COALESCE((platforms->'disney'->>'unlocked')::boolean, false),
  tiktok          = COALESCE((platforms->'tiktok'->>'unlocked')::boolean, false);

ALTER TABLE check_results DROP COLUMN platforms;
```

- [ ] **Step 3: Apply the migration**

Run: `encore db migrate`
Expected: migration `22` applies cleanly. Verify: `encore db shell checker` → `\d check_results` shows `platforms jsonb` and no `netflix` column.

> Code does not compile against the dropped columns yet — that is fixed in Tasks 4–9. Apply the migration first so subsequent task tests run against the new schema.

- [ ] **Step 4: Commit**

```bash
git add services/checker/migrations/22_unify_platforms.up.sql services/checker/migrations/22_unify_platforms.down.sql
git commit -m "feat(checker): migration 22 — unified platforms jsonb (backfill + drop bool columns)"
```

---

## Task 4: `nodeCheckResult.Platforms` + `checkNode` + `probeLatency`

**Files:**
- Modify: `services/checker/mihomo.go`
- Test: `services/checker/mihomo_test.go` (new — `probeLatency`)

- [ ] **Step 1: Write the failing test**

Create `services/checker/mihomo_test.go`:

```go
package checker

import (
	"context"
	"net/http"
	"testing"
)

func TestProbeLatency_AliveAndMs(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"http://cp.cloudflare.com/generate_204": {status: 204},
	})
	alive, ms := probeLatency(context.Background(), client, "")
	if !alive {
		t.Fatalf("expected alive")
	}
	if ms < 0 {
		t.Fatalf("expected non-negative ms, got %d", ms)
	}
}

func TestProbeLatency_DeadOn5xx(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"http://cp.cloudflare.com/generate_204": {status: 502},
	})
	alive, _ := probeLatency(context.Background(), client, "")
	if alive {
		t.Fatalf("expected dead on 502")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestProbeLatency`
Expected: FAIL — `probeLatency` undefined.

- [ ] **Step 3: Implement**

In `services/checker/mihomo.go`:

1. Change the constant:

```go
	aliveTestURL = "http://cp.cloudflare.com/generate_204"
```

2. Replace `isAlive` and `measureLatency` with a single `probeLatency`:

```go
// probeLatency does a single GET to the connectivity URL and returns whether the
// proxy is alive plus the round-trip latency in ms. Matches clash-verge-rev's
// single-request delay test. ms is 0 when not alive.
func probeLatency(ctx context.Context, client *http.Client, testURL string) (alive bool, ms int) {
	url := testURL
	if url == "" {
		url = aliveTestURL
	}
	start := time.Now()
	resp, err := get(ctx, client, url)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false, 0
	}
	return true, int(time.Since(start).Milliseconds())
}
```

3. In `nodeCheckResult`, replace the 9 bool fields + `ExtraPlatforms` with:

```go
	Platforms map[string]PlatformOutcome
```

(keep `NodeID, NodeName, Alive, LatencyMs, SpeedKbps, UploadSpeedKbps, IP, Country, TrafficBytes, Debug`).

4. In `checkNode`, replace the alive/latency block:

```go
	alive, latency := probeLatency(ctx, pc.Client, latencyTestURL)
	if !alive {
		if opts.Debug && result.Debug != nil {
			result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
				Platform: "connectivity", Result: false,
				Steps: []DebugStep{{Type: "variable", Description: "alive = false", Details: toRawMessage(map[string]any{"name": "alive", "value": false})}},
			})
		}
		return result
	}
	result.Alive = true
	result.LatencyMs = latency
```

(keep the debug trace block that follows; it references `result.LatencyMs`.)

5. Replace the media-apps result mapping (the whole `extra := ...` block and all `if hasApp(...)` assignments, lines ~344-390) with:

```go
		outcomes := runUserRulesWithDebug(ctx, mediaClient, rules, ruleRecorders)
		result.Platforms = make(map[string]PlatformOutcome, len(outcomes))
		for k, v := range outcomes {
			if opts.Debug && result.Debug != nil {
				if rd, ok := ruleRecorders[k]; ok {
					result.Debug.Traces = append(result.Debug.Traces, DebugTrace{Platform: k, Result: v.Unlocked, Steps: rd.Steps})
				}
			}
			result.Platforms[k] = v
		}
```

6. Add a cookie jar to `mediaClient` so multi-step cookie rules (bahamut) work across the per-node check. Change its construction:

```go
		jar, _ := cookiejar.New(nil)
		mediaClient := &http.Client{
			Transport: pc.Transport,
			Timeout:   8 * time.Second,
			Jar:       jar,
		}
```

Add `"net/http/cookiejar"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestProbeLatency`
Expected: PASS. `go build ./services/checker/` still fails (jobstore/checker not yet updated) — that's expected; Tasks 5–6 fix it.

- [ ] **Step 5: Commit**

```bash
git add services/checker/mihomo.go services/checker/mihomo_test.go
git commit -m "feat(checker): probeLatency (single Cloudflare probe) + nodeCheckResult.Platforms map + cookie jar"
```

---

## Task 5: `jobstore.insertResult` writes `platforms`

**Files:**
- Modify: `services/checker/jobstore.go`

- [ ] **Step 1: Update insertResult**

Replace the body of `insertResult`:

```go
func (s *jobStore) insertResult(ctx context.Context, jobID, nodeID string, proxy map[string]any, res nodeCheckResult) error {
	nodeType, _ := proxy["type"].(string)
	nodeConfigJSON, _ := json.Marshal(proxy)
	platformsJSON, _ := json.Marshal(res.Platforms)
	if len(platformsJSON) == 0 || string(platformsJSON) == "null" {
		platformsJSON = []byte("{}")
	}
	_, err := db.Exec(ctx, `
		INSERT INTO check_results
		  (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, upload_speed_kbps, country, ip,
		   platforms, traffic_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, uuid.New().String(), jobID, nodeID, res.NodeName, nodeType, nodeConfigJSON, time.Now(),
		res.Alive, res.LatencyMs, res.SpeedKbps, res.UploadSpeedKbps, res.Country, res.IP,
		platformsJSON, res.TrafficBytes,
	)
	return err
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./services/checker/`
Expected: still fails only in `checker.go` / `export_data.go` / `local_check.go` / `jobsummary.go` (fixed next). `jobstore.go` itself compiles.

- [ ] **Step 3: Commit**

```bash
git add services/checker/jobstore.go
git commit -m "feat(checker): insertResult writes platforms jsonb"
```

---

## Task 6: `NodeResult.Platforms` + `GetResults` + summary + options

**Files:**
- Modify: `services/checker/checker.go`

- [ ] **Step 1: Update types and queries**

1. `PlatformUnlockSummary` → a map alias for dynamic platforms:

```go
// PlatformUnlockSummary maps each platform key to how many nodes unlocked it.
type PlatformUnlockSummary map[string]int
```

2. `NodeResult`: replace the 9 bool fields + `ExtraPlatforms` with:

```go
	Platforms map[string]PlatformOutcome `json:"platforms"`
```

3. `defaultCheckOptions` MediaApps → full 15-key roster:

```go
func defaultCheckOptions() CheckOptions {
	return CheckOptions{
		SpeedTest: true,
		MediaApps: []string{
			"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
			"claude", "gemini", "grok", "disney", "tiktok",
			"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video",
		},
	}
}
```

4. In `GetResults`, change the CTE SELECT line that lists `cr.netflix, ... cr.extra_platforms` to:

```go
				       cr.platforms, cr.traffic_bytes
```

(replace `cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
				       cr.extra_platforms, cr.traffic_bytes`).

5. Change the scan loop:

```go
	var results []NodeResult
	for rows.Next() {
		var r NodeResult
		var platformsJSON []byte
		if err := rows.Scan(
			&r.NodeID, &r.NodeName, &r.NodeType, &r.Enabled,
			&r.Server, &r.Port, &r.Config,
			&r.Alive, &r.LatencyMs, &r.SpeedKbps, &r.UploadSpeedKbps, &r.Country, &r.IP,
			&platformsJSON, &r.TrafficBytes,
		); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &r.Platforms)
		}
		if r.Platforms == nil {
			r.Platforms = map[string]PlatformOutcome{}
		}
		results = append(results, r)
	}
```

- [ ] **Step 2: Verify compile of checker.go**

Run: `go build ./services/checker/`
Expected: now only `export_data.go`, `local_check.go`, `jobsummary.go` remain broken.

- [ ] **Step 3: Commit**

```bash
git add services/checker/checker.go
git commit -m "feat(checker): NodeResult.Platforms map + GetResults reads platforms jsonb + 15-key default"
```

---

## Task 7: `jobsummary.loadPlatformCounts` from jsonb

**Files:**
- Modify: `services/checker/jobsummary.go`
- Test: `services/checker/jobsummary_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/jobsummary_test.go`:

```go
package checker

import "testing"

func TestPlatformUnlockSummary_IsMap(t *testing.T) {
	var s PlatformUnlockSummary = PlatformUnlockSummary{}
	s["netflix"] = 3
	if s["netflix"] != 3 {
		t.Fatalf("map assignment failed")
	}
}
```

(The SQL aggregation itself is integration-level; this guards the type. The query is verified end-to-end in Task 15's manual check.)

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestPlatformUnlockSummary_IsMap`
Expected: FAIL to compile until Task 6's type change is present (it is) — so this passes once `loadPlatformCounts` compiles. Proceed to implement.

- [ ] **Step 3: Implement**

Replace `loadPlatformCounts`:

```go
func loadPlatformCounts(ctx context.Context, jobID string, p *PlatformUnlockSummary) {
	if *p == nil {
		*p = PlatformUnlockSummary{}
	}
	rows, err := db.Query(ctx, `
		SELECT key, COUNT(*)
		FROM check_results cr, jsonb_each(cr.platforms) AS e(key, val)
		WHERE cr.job_id = $1 AND cr.alive = true AND (val->>'unlocked')::boolean
		GROUP BY key
	`, jobID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n int
		if rows.Scan(&key, &n) == nil {
			(*p)[key] = n
		}
	}
}
```

In `loadJobSummary`, initialize the map:

```go
	s := &JobDetailedSummary{
		JobID:     jobID,
		Platforms: PlatformUnlockSummary{},
		Countries: map[string]int{},
		TopNodes:  []TopNode{},
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestPlatformUnlockSummary_IsMap`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/jobsummary.go services/checker/jobsummary_test.go
git commit -m "feat(checker): platform unlock counts aggregated from platforms jsonb"
```

---

## Task 8: `JobDetailedSummary.Platforms` type already a map

**Files:**
- Modify: `services/checker/internal_api.go` (only if it declares the field type explicitly)

- [ ] **Step 1: Confirm field type**

`internal_api.go` line ~21 declares `Platforms PlatformUnlockSummary`. Since Task 6 changed `PlatformUnlockSummary` to a map type, no field change is needed. Verify it compiles:

Run: `go build ./services/checker/`
Expected: `internal_api.go` compiles (the alias change propagated).

- [ ] **Step 2: Commit (only if a change was needed)**

If no change: skip. Otherwise:

```bash
git add services/checker/internal_api.go
git commit -m "refactor(checker): JobDetailedSummary.Platforms is a map"
```

---

## Task 9: `export_data` tagging from `platforms` map

**Files:**
- Modify: `services/checker/export_data.go`
- Test: `services/checker/export_data_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/export_data_test.go`:

```go
package checker

import (
	"testing"

	settingssvc "subs-check-re/services/settings"
)

func TestTaggedName_FromPlatformsMap(t *testing.T) {
	cfg := settingssvc.DefaultExportTags() // country off, speed on, builtin labels
	platforms := map[string]PlatformOutcome{
		"netflix":         {Unlocked: true, Status: "Yes", Region: "US"},
		"youtube":         {Unlocked: true, Status: "Yes"},
		"youtube_premium": {Unlocked: true, Status: "Yes"},
		"spotify":         {Unlocked: true, Status: "Yes"}, // custom (not in default cfg) → defaults to enabled, label=key
	}
	got := taggedName("HK-01", "HK", platforms, 2048, cfg)
	// country off; NF present; YT premium → "YT+"; spotify appended; speed 2048kbps → 2.0MB
	want := "HK-01|NF|YT+|spotify|2.0MB"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestTaggedName_FromPlatformsMap`
Expected: FAIL — `taggedName` signature still takes `unlockFlags` + `extra`.

- [ ] **Step 3: Implement**

In `services/checker/export_data.go`:

1. Delete the `unlockFlags` struct and its `builtinUnlocked` method (lines ~32-67).

2. Replace `taggedName`:

```go
// taggedName appends country / platform / speed tags to a node name per cfg.
// Order: country, built-in platforms (cfg order), custom platforms (sorted by
// key), speed. A platform is tagged when its outcome is Unlocked.
func taggedName(name, country string, platforms map[string]PlatformOutcome, speedKbps int, cfg settingssvc.ExportTagConfig) string {
	tags := []string{}

	if cfg.ShowCountry && country != "" {
		tags = append(tags, country)
	}

	unlocked := func(key string) bool {
		o, ok := platforms[key]
		return ok && o.Unlocked
	}

	cfgByKey := map[string]settingssvc.PlatformTag{}
	for _, p := range cfg.Platforms {
		cfgByKey[p.Key] = p
	}

	for _, p := range cfg.Platforms {
		if !builtinKeys[p.Key] || !p.Enabled {
			continue
		}
		if p.Key == "youtube" {
			if unlocked("youtube_premium") {
				tags = append(tags, p.Label+"+")
			} else if unlocked("youtube") {
				tags = append(tags, p.Label)
			}
			continue
		}
		if unlocked(p.Key) {
			tags = append(tags, p.Label)
		}
	}

	keys := make([]string, 0, len(platforms))
	for k, o := range platforms {
		if o.Unlocked && !builtinKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if p, ok := cfgByKey[k]; ok {
			if !p.Enabled {
				continue
			}
			tags = append(tags, p.Label)
		} else {
			tags = append(tags, k)
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

> Note `builtinKeys` is the checker-wide map (rules.go), expanded to 15 keys in Task 13. `youtube_premium` is a builtin key but is NOT iterated as its own tag (it has no entry in `cfg.Platforms` defaults); it only modifies the `youtube` tag. Custom-key loop skips all `builtinKeys`, so `youtube_premium`/`chatgpt_ios`/`bilibili_*` etc. never double-tag.

3. In `loadJobProxies`, change the CTE: replace the platform column list `cr.netflix, ..., cr.tiktok,\n cr.country, cr.extra_platforms,` with `cr.country, cr.platforms,` and the outer SELECT list likewise (`netflix, ..., tiktok, country, extra_platforms` → `country, platforms`). Then the scan:

```go
		var (
			configJSON    []byte
			name          string
			country       string
			platformsJSON []byte
			speedKbps     int
			latencyMs     sql.NullInt64
		)
		if err := rows.Scan(&configJSON, &name, &country, &platformsJSON, &speedKbps, &latencyMs); err != nil {
			continue
		}
		if len(configJSON) == 0 {
			continue
		}
		var nodeCfg map[string]any
		if json.Unmarshal(configJSON, &nodeCfg) != nil {
			continue
		}
		var platforms map[string]PlatformOutcome
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &platforms)
		}
		tagged := taggedName(name, country, platforms, speedKbps, cfg)
```

The full CTE SELECT becomes:

```go
	query := `
		WITH r AS (
			SELECT COALESCE(n.config, cr.node_config) AS config,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       cr.country, cr.platforms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       cr.latency_ms
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			WHERE cr.job_id = $1 AND ` + aliveClause + `COALESCE(n.enabled, true) = true
		)
		SELECT config, node_name, country, platforms, speed_kbps, latency_ms
		FROM r
		ORDER BY ` + orderClause(prefs.Sort)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestTaggedName_FromPlatformsMap`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/checker/export_data.go services/checker/export_data_test.go
git commit -m "feat(checker): export tagging reads platforms map"
```

---

## Task 10: `local_check` returns `Platforms` map

**Files:**
- Modify: `services/checker/local_check.go`

- [ ] **Step 1: Update LocalUnlockResult + GetLocalUnlock**

1. Replace `LocalUnlockResult`:

```go
type LocalUnlockResult struct {
	Platforms map[string]PlatformOutcome `json:"platforms"`
	IP        string                     `json:"ip"`
	Country   string                     `json:"country"`
}
```

2. Replace the switch in `GetLocalUnlock` with:

```go
	res := LocalUnlockResult{Platforms: map[string]PlatformOutcome{}}
	for _, r := range results {
		res.Platforms[r.key] = r.outcome
	}
	res.IP, res.Country = getProxyInfo(checkCtx, client)
```

(remove the now-unused per-field `var res LocalUnlockResult` + switch block.)

- [ ] **Step 2: Verify compile**

Run: `go build ./services/checker/`
Expected: checker package compiles fully now.

- [ ] **Step 3: Commit**

```bash
git add services/checker/local_check.go
git commit -m "feat(checker): GetLocalUnlock returns platforms map"
```

---

## Task 11: notify service — map-based platforms

**Files:**
- Modify: `services/notify/notify_types.go`
- Modify: `services/notify/formatters.go`
- Modify: `services/notify/alerts.go`
- Test: `services/notify/formatters_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/notify/formatters_test.go`:

```go
package notify

import (
	"strings"
	"testing"
)

func TestFormatCheckReport_IteratesPlatformMap(t *testing.T) {
	r := &JobReport{
		SubscriptionName: "Sub",
		Available:        2, Total: 3,
		Platforms: PlatformCounts{"netflix": 2, "spotify": 1},
	}
	out := formatCheckReport(r)
	if !strings.Contains(out, "Netflix: 2") {
		t.Fatalf("missing netflix line: %s", out)
	}
	if !strings.Contains(out, "Spotify: 1") {
		t.Fatalf("missing spotify line: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/notify/ -run TestFormatCheckReport_IteratesPlatformMap`
Expected: FAIL — `PlatformCounts` is a struct, not a map.

- [ ] **Step 3: Implement**

In `services/notify/notify_types.go`:

1. `PlatformCounts` → map:

```go
// PlatformCounts maps a platform key to how many nodes unlocked it.
type PlatformCounts map[string]int
```

2. `LocalUnlockReport` → map:

```go
type LocalUnlockReport struct {
	IP        string
	Country   string
	Platforms map[string]bool
}
```

3. `fromCheckerSummary` — copy the map (the checker summary `s.Platforms` is now `map[string]int`):

```go
	r := &JobReport{
		JobID:            s.JobID,
		SubscriptionName: s.SubscriptionName,
		Available:        s.Available,
		Total:            s.Total,
		Platforms:        PlatformCounts(s.Platforms),
		AvgSpeedKbps:     s.AvgSpeedKbps,
		MaxSpeedKbps:     s.MaxSpeedKbps,
		AvgLatencyMs:     s.AvgLatencyMs,
		Countries:        s.Countries,
		TopNodes:         make([]TopNode, 0, len(s.TopNodes)),
	}
	if r.Platforms == nil {
		r.Platforms = PlatformCounts{}
	}
```

4. `fromCheckerLocalUnlock` — copy outcome map to bool map:

```go
func fromCheckerLocalUnlock(r *checkersvc.LocalUnlockResult) *LocalUnlockReport {
	out := &LocalUnlockReport{IP: r.IP, Country: r.Country, Platforms: map[string]bool{}}
	for k, v := range r.Platforms {
		out.Platforms[k] = v.Unlocked
	}
	return out
}
```

In `services/notify/formatters.go`:

1. Expand `platformNames` with the 6 new keys and add an ordered key list:

```go
var platformNames = map[string]string{
	"netflix":         "Netflix",
	"youtube":         "YouTube",
	"youtube_premium": "YouTube Premium",
	"openai":          "ChatGPT Web",
	"chatgpt_ios":     "ChatGPT iOS",
	"claude":          "Claude",
	"gemini":          "Gemini",
	"grok":            "Grok",
	"disney":          "Disney+",
	"tiktok":          "TikTok",
	"bilibili_cn":     "哔哩哔哩大陆",
	"bilibili_hkmctw": "哔哩哔哩港澳台",
	"bahamut":         "巴哈姆特动画疯",
	"spotify":         "Spotify",
	"prime_video":     "Prime Video",
}

// platformOrder controls display order in reports; keys not listed render last, sorted.
var platformOrder = []string{
	"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
	"claude", "gemini", "grok", "disney", "tiktok",
	"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video",
}
```

2. Replace `platformEntries` + its use in `formatCheckReport`. Delete `platformEntries`/`platformEntry` and rewrite the unlock section of `formatCheckReport`:

```go
	var unlocked []string
	for _, key := range orderedPlatformKeys(map[string]bool(nil), s.Platforms) {
		if n := s.Platforms[key]; n > 0 {
			unlocked = append(unlocked, fmt.Sprintf("  %s: %d", platformDisplayName(key), n))
		}
	}
	if len(unlocked) > 0 {
		b.WriteString("\n🔓 <b>Platform unlocks:</b>\n")
		for _, line := range unlocked {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
```

3. Rewrite `formatUnlockReport`'s platform list similarly:

```go
	b.WriteByte('\n')
	for _, key := range orderedPlatformKeys(r.Platforms, nil) {
		status := "❌"
		if r.Platforms[key] {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", platformDisplayName(key), status))
	}
```

4. Add the ordering helper:

```go
// orderedPlatformKeys returns keys in platformOrder first, then any extra keys
// present in either map, sorted. Pass nil for the map you don't have.
func orderedPlatformKeys(boolM map[string]bool, intM map[string]int) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range platformOrder {
		if _, ok := boolM[k]; ok {
			out = append(out, k)
			seen[k] = true
			continue
		}
		if _, ok := intM[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var extra []string
	for k := range boolM {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	for k := range intM {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}
```

In `services/notify/alerts.go`, replace the hardcoded `current` map in `checkPlatformAlerts`:

```go
	current := map[string]bool{}
	for key, count := range summary.Platforms {
		current[key] = count > 0
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/notify/ -run TestFormatCheckReport_IteratesPlatformMap`
Expected: PASS. Then `go build ./services/notify/`.

- [ ] **Step 5: Commit**

```bash
git add services/notify/notify_types.go services/notify/formatters.go services/notify/alerts.go services/notify/formatters_test.go
git commit -m "feat(notify): platform counts/reports/alerts are map-driven (15 platforms)"
```

---

## Task 12: settings — default export tags for new platforms

**Files:**
- Modify: `services/settings/settings.go`

- [ ] **Step 1: Add new platform tags**

Append to `defaultExportTags()`'s `Platforms` slice (after `tiktok`):

```go
			{Key: "spotify", Label: "SP", Enabled: true},
			{Key: "prime_video", Label: "PV", Enabled: true},
			{Key: "bahamut", Label: "BH", Enabled: true},
			{Key: "bilibili_cn", Label: "B站", Enabled: true},
			{Key: "bilibili_hkmctw", Label: "B站港", Enabled: true},
```

> `chatgpt_ios`, `youtube_premium` deliberately get NO standalone export tag (matches the existing convention where `youtube_premium` is folded into `youtube`). `chatgpt_ios` is display-only.

- [ ] **Step 2: Verify**

Run: `go build ./services/settings/`
Expected: compiles. (`mergeExportTags` auto-treats these as built-ins via `defaultExportTags()`.)

- [ ] **Step 3: Commit**

```bash
git add services/settings/settings.go
git commit -m "feat(settings): default export tags for spotify/prime/bahamut/bilibili"
```

---

## Task 13: Rewrite the 15 default rules to verge logic

**Files:**
- Modify: `services/checker/rules.go` (`builtinKeys` → 15 keys)
- Modify: `services/checker/rules_defaults.go` (`defaultRules`)
- Test: `services/checker/rules_defaults_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `services/checker/rules_defaults_test.go`. It runs each default rule through the engine against a mock client and asserts the outcome. Helper runs a default rule by key:

```go
package checker

import (
	"context"
	"encoding/json"
	"testing"
)

func evalDefault(t *testing.T, key string, byURL map[string]mockResp) PlatformOutcome {
	t.Helper()
	var dr defaultRule
	found := false
	for _, d := range defaultRules {
		if d.key == key {
			dr = d
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no default rule for key %q", key)
	}
	defJSON, _ := json.Marshal(dr.def)
	rule := &PlatformRule{RuleType: dr.ruleType, Key: dr.key, Enabled: true, Definition: defJSON}
	out, err := runRule(context.Background(), mockClient(byURL), rule, nil)
	if err != nil {
		t.Fatalf("%s error: %v", key, err)
	}
	return out
}

func TestDefaultRule_ClaudeBlocklist(t *testing.T) {
	// loc=US → unlocked
	out := evalDefault(t, "claude", map[string]mockResp{
		"https://claude.ai/cdn-cgi/trace": {status: 200, body: "fl=AAA\nloc=US\n"},
	})
	if !out.Unlocked || out.Region != "US" {
		t.Fatalf("US: got %+v", out)
	}
	// loc=CN → blocked
	out = evalDefault(t, "claude", map[string]mockResp{
		"https://claude.ai/cdn-cgi/trace": {status: 200, body: "loc=CN\n"},
	})
	if out.Unlocked || out.Status != "No" || out.Region != "CN" {
		t.Fatalf("CN: got %+v", out)
	}
}

func TestDefaultRule_BilibiliCN(t *testing.T) {
	out := evalDefault(t, "bilibili_cn", map[string]mockResp{
		"https://api.bilibili.com/pgc/player/web/playurl?avid=82846771&qn=0&type=&otype=json&ep_id=307247&fourk=1&fnver=0&fnval=16&module=bangumi": {status: 200, body: `{"code":0}`},
	})
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v", out)
	}
}

func TestDefaultRule_SpotifyBlocked(t *testing.T) {
	out := evalDefault(t, "spotify", map[string]mockResp{
		"https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json": {status: 403, body: ""},
	})
	if out.Unlocked || out.Status != "No" {
		t.Fatalf("got %+v", out)
	}
}

func TestDefaultRules_AllFifteenPresent(t *testing.T) {
	want := []string{"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
		"claude", "gemini", "grok", "disney", "tiktok",
		"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video"}
	have := map[string]bool{}
	for _, d := range defaultRules {
		have[d.key] = true
	}
	for _, k := range want {
		if !have[k] {
			t.Fatalf("missing default rule %q", k)
		}
	}
	if len(defaultRules) != len(want) {
		t.Fatalf("expected %d default rules, got %d", len(want), len(defaultRules))
	}
}
```

The file contains only: imports (`context`, `encoding/json`, `testing`), `evalDefault`, and the four `Test...` functions. It reuses `mockClient`/`mockResp` from Task 1's test files.

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestDefaultRule`
Expected: FAIL — new keys not present; rules still old logic.

- [ ] **Step 3: Expand `builtinKeys` (rules.go)**

```go
var builtinKeys = map[string]bool{
	"netflix": true, "youtube": true, "youtube_premium": true, "openai": true, "chatgpt_ios": true,
	"claude": true, "gemini": true, "grok": true, "disney": true, "tiktok": true,
	"bilibili_cn": true, "bilibili_hkmctw": true, "bahamut": true, "spotify": true, "prime_video": true,
}
```

- [ ] **Step 4: Rewrite `defaultRules` (rules_defaults.go)**

Replace the entire `defaultRules` slice with the 15 entries below. Each `def` is `ScriptDef{Code: ...}` with `ruleType: "js"`. (Icons reuse the existing `simple-icons:*` scheme; new ones use a reasonable slug.)

```go
var defaultRules = []defaultRule{
	{name: "Netflix", key: "netflix", icon: "simple-icons:netflix", ruleType: "js", sortOrder: 0, def: ScriptDef{Code: `
var cdn = http_get("https://api.fast.com/netflix/speedtest/v2?https=true&token=YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm&urlCount=5");
if (cdn.status === 403) return {unlocked:false,status:"No (IP Banned)",region:""};
try { var d = JSON.parse(cdn.body); if (d.targets && d.targets.length && d.targets[0].location && d.targets[0].location.country) return {unlocked:true,status:"Yes",region:d.targets[0].location.country}; } catch(e){}
var ua = {"User-Agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36","Accept-Language":"en-US,en;q=0.9"};
var r1 = http_get("https://www.netflix.com/title/81280792",{headers:ua});
var r2 = http_get("https://www.netflix.com/title/70143836",{headers:ua});
if (r1.status===404 && r2.status===404) return {unlocked:false,status:"Originals Only",region:""};
if (r1.status===403 || r2.status===403) return {unlocked:false,status:"No",region:""};
if (r1.status===200||r1.status===301||r2.status===200||r2.status===301) return {unlocked:true,status:"Yes",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "YouTube", key: "youtube", icon: "simple-icons:youtube", ruleType: "js", sortOrder: 1, def: ScriptDef{Code: `
var r = http_get("https://www.youtube.com/");
if (r.status !== 200) return {unlocked:false,status:"Failed",region:""};
var b = (r.body||"").toLowerCase();
if (b.indexOf("not available in your country")!==-1 || b.indexOf("unavailable in your region")!==-1) return {unlocked:false,status:"No",region:""};
if (b.indexOf("youtube")!==-1) return {unlocked:true,status:"Yes",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "YouTube Premium", key: "youtube_premium", icon: "simple-icons:youtube", ruleType: "js", sortOrder: 2, def: ScriptDef{Code: `
var r = http_get("https://www.youtube.com/premium?hl=en");
var body = r.body||""; var b = body.toLowerCase(); var region="";
var pats = [/id=["']country-code["'][^>]*>\s*([A-Za-z]{2,3})\s*</, /"GL"\s*:\s*"([A-Za-z]{2})"/, /"countryCode"\s*:\s*"([A-Za-z]{2})"/, /"country_code"\s*:\s*"([A-Za-z]{2})"/];
for (var i=0;i<pats.length;i++){ var m=body.match(pats[i]); if(m){region=m[1].toUpperCase();break;} }
if (b.indexOf("youtube premium is not available in your country")!==-1 || b.indexOf("premium is not available in your country")!==-1 || b.indexOf("premium is not available in your region")!==-1) return {unlocked:false,status:"No",region:region};
if (r.status>=200 && r.status<300 && (b.indexOf("youtube premium")!==-1 || b.indexOf("ad-free")!==-1 || b.indexOf('"browseid":"spunlimited"')!==-1)) return {unlocked:true,status:"Yes",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "ChatGPT Web", key: "openai", icon: "simple-icons:openai", ruleType: "js", sortOrder: 3, def: ScriptDef{Code: `
var region=""; var t = http_get("https://chat.openai.com/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(m) region=m[2];
var r = http_get("https://api.openai.com/compliance/cookie_requirements");
if ((r.body||"").toLowerCase().indexOf("unsupported_country")!==-1) return {unlocked:false,status:"Unsupported Country",region:region};
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "ChatGPT iOS", key: "chatgpt_ios", icon: "simple-icons:openai", ruleType: "js", sortOrder: 4, def: ScriptDef{Code: `
var region=""; var t = http_get("https://chat.openai.com/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(m) region=m[2];
var r = http_get("https://ios.chat.openai.com/"); var b=(r.body||"").toLowerCase();
if (b.indexOf("you may be connected to a disallowed isp")!==-1) return {unlocked:false,status:"Disallowed ISP",region:region};
if (b.indexOf("request is not allowed. please try again later.")!==-1) return {unlocked:true,status:"Yes",region:region};
if (b.indexOf("sorry, you have been blocked")!==-1) return {unlocked:false,status:"Blocked",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "Claude", key: "claude", icon: "simple-icons:anthropic", ruleType: "js", sortOrder: 5, def: ScriptDef{Code: `
var t = http_get("https://claude.ai/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/);
if (!m) return {unlocked:false,status:"Failed",region:""};
var code=m[2]; var blocked=["AF","BY","CN","CU","HK","IR","KP","MO","RU","SY"];
var ok = blocked.indexOf(code)===-1;
return {unlocked:ok,status:ok?"Yes":"No",region:code};
`}},
	{name: "Gemini", key: "gemini", icon: "simple-icons:googlegemini", ruleType: "js", sortOrder: 6, def: ScriptDef{Code: `
var r = http_get("https://gemini.google.com"); var body=r.body||""; var marker=",2,1,200,\""; var idx=body.indexOf(marker); var code="";
if (idx!==-1) code=body.substr(idx+marker.length,3);
if (!/^[A-Z]{3}$/.test(code)) return {unlocked:false,status:"Failed",region:""};
var blocked=["CHN","RUS","BLR","CUB","IRN","PRK","SYR","HKG","MAC"]; var ok=blocked.indexOf(code)===-1;
return {unlocked:ok,status:ok?"Yes":"No",region:code};
`}},
	{name: "Grok", key: "grok", icon: "simple-icons:x", ruleType: "js", sortOrder: 7, def: ScriptDef{Code: `
var r = http_get("https://api.x.ai/v1/models"); var ok=r.status===401||r.status===200;
return {unlocked:ok,status:ok?"Yes":"No",region:""};
`}},
	{name: "Disney+", key: "disney", icon: "simple-icons:disneyplus", ruleType: "js", sortOrder: 8, def: ScriptDef{Code: `
var AUTH="Bearer ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84";
function mainRegion(){ var r=http_get("https://www.disneyplus.com/"); var m=(r.body||"").match(/region"\s*:\s*"([^"]+)"/); return m?m[1]:""; }
var dev=http_post("https://disney.api.edge.bamgrid.com/devices",{headers:{"authorization":AUTH,"content-type":"application/json; charset=UTF-8"},body:JSON.stringify({deviceFamily:"browser",applicationRuntime:"chrome",deviceProfile:"windows",attributes:{}})});
if (dev.status===403) return {unlocked:false,status:"No (IP Banned)",region:""};
var am=(dev.body||"").match(/"assertion"\s*:\s*"([^"]+)"/); if(!am) return {unlocked:false,status:"Failed",region:""};
var form="grant_type=urn:ietf:params:oauth:grant-type:token-exchange&latitude=0&longitude=0&platform=browser&subject_token="+encodeURIComponent(am[1])+"&subject_token_type=urn:bamtech:params:oauth:token-type:device";
var tok=http_post("https://disney.api.edge.bamgrid.com/token",{headers:{"authorization":AUTH,"content-type":"application/x-www-form-urlencoded"},body:form});
if ((tok.body||"").indexOf("forbidden-location")!==-1 || (tok.body||"").indexOf("403 ERROR")!==-1) return {unlocked:false,status:"No (IP Banned)",region:""};
var rt=""; try{ rt=JSON.parse(tok.body).refresh_token||""; }catch(e){ var rm=(tok.body||"").match(/"refresh_token"\s*:\s*"([^"]+)"/); rt=rm?rm[1]:""; }
if (!rt){ var reg=mainRegion(); if(reg) return {unlocked:true,status:"Yes",region:reg}; return {unlocked:false,status:"Failed",region:""}; }
var gql=JSON.stringify({query:"mutation refreshToken($input: RefreshTokenInput!) { refreshToken(refreshToken: $input) { activeSession { sessionId } } }",variables:{input:{refreshToken:rt}}});
var g=http_post("https://disney.api.edge.bamgrid.com/graph/v1/device/graphql",{headers:{"authorization":AUTH,"content-type":"application/json"},body:gql});
var prev=http_get("https://disneyplus.com"); var unavailable=prev.final_url.indexOf("preview")!==-1||prev.final_url.indexOf("unavailable")!==-1;
if (!g.body || g.status>=400){ var reg2=mainRegion(); if(reg2) return {unlocked:true,status:"Yes",region:reg2}; return {unlocked:false,status:"Failed",region:""}; }
var cm=g.body.match(/"countryCode"\s*:\s*"([^"]+)"/); var sm=g.body.match(/"inSupportedLocation"\s*:\s*(false|true)/);
if (!cm){ var reg3=mainRegion(); if(reg3) return {unlocked:true,status:"Yes",region:reg3}; return {unlocked:false,status:"No",region:""}; }
var region=cm[1];
if (region==="JP") return {unlocked:true,status:"Yes",region:region};
if (unavailable) return {unlocked:false,status:"No",region:region};
if (sm && sm[1]==="false") return {unlocked:false,status:"Soon",region:region};
if (sm && sm[1]==="true") return {unlocked:true,status:"Yes",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "TikTok", key: "tiktok", icon: "simple-icons:tiktok", ruleType: "js", sortOrder: 9, def: ScriptDef{Code: `
function st(s,b){ if(s===403||s===451) return "No"; if(!(s>=200&&s<300)) return "Failed"; var t=b.toLowerCase(); if(t.indexOf("access denied")!==-1||t.indexOf("not available in your region")!==-1||t.indexOf("tiktok is not available")!==-1) return "No"; return "Yes"; }
function rg(b){ var m=b.match(/"region"\s*:\s*"([a-zA-Z-]+)"/); if(m) return m[1].split("-")[0].toUpperCase(); return ""; }
var r=http_get("https://www.tiktok.com/cdn-cgi/trace"); var region=""; var lm=(r.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(lm) region=lm[2];
var status=st(r.status, r.body||"");
if (region==="" || status==="Failed"){ var r2=http_get("https://www.tiktok.com/"); var s2=st(r2.status,r2.body||""); var g2=rg(r2.body||""); if(status!=="No") status=s2; if(region==="") region=g2; }
var ok=status==="Yes";
return {unlocked:ok,status:status,region:region};
`}},
	{name: "哔哩哔哩大陆", key: "bilibili_cn", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 10, def: ScriptDef{Code: `
var r=http_get("https://api.bilibili.com/pgc/player/web/playurl?avid=82846771&qn=0&type=&otype=json&ep_id=307247&fourk=1&fnver=0&fnval=16&module=bangumi");
var code=null; try{ code=JSON.parse(r.body).code; }catch(e){ return {unlocked:false,status:"Failed",region:""}; }
if (code===0) return {unlocked:true,status:"Yes",region:""};
if (code===-10403) return {unlocked:false,status:"No",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "哔哩哔哩港澳台", key: "bilibili_hkmctw", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 11, def: ScriptDef{Code: `
var r=http_get("https://api.bilibili.com/pgc/player/web/playurl?avid=18281381&cid=29892777&qn=0&type=&otype=json&ep_id=183799&fourk=1&fnver=0&fnval=16&module=bangumi");
var code=null; try{ code=JSON.parse(r.body).code; }catch(e){ return {unlocked:false,status:"Failed",region:""}; }
if (code===0) return {unlocked:true,status:"Yes",region:""};
if (code===-10403) return {unlocked:false,status:"No",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "巴哈姆特动画疯", key: "bahamut", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 12, def: ScriptDef{Code: `
var d=http_get("https://ani.gamer.com.tw/ajax/getdeviceid.php"); var dm=(d.body||"").match(/"deviceid"\s*:\s*"([^"]+)"/);
if (!dm) return {unlocked:false,status:"Failed",region:""};
var t=http_get("https://ani.gamer.com.tw/ajax/token.php?adID=89422&sn=37783&device="+dm[1]);
if ((t.body||"").indexOf("animeSn")===-1) return {unlocked:false,status:"No",region:""};
var region=""; var h=http_get("https://ani.gamer.com.tw/"); var rm=(h.body||"").match(/data-geo="([^"]+)"/); if(rm) region=rm[1];
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "Spotify", key: "spotify", icon: "simple-icons:spotify", ruleType: "js", sortOrder: 13, def: ScriptDef{Code: `
var r=http_get("https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json"); var region="";
try{ var path=(r.final_url||"").replace(/^https?:\/\/[^/]+/,""); var segs=path.split("/"); for(var i=0;i<segs.length;i++){ if(segs[i]&&segs[i]!=="api"){ region=segs[i].split("-")[0].toUpperCase(); break; } } }catch(e){}
if (region===""){ var m=(r.body||"").match(/"countryCode":"([^"]+)"/); if(m) region=m[1].toUpperCase(); }
if (r.status===403||r.status===451) return {unlocked:false,status:"No",region:region};
if (!(r.status>=200&&r.status<300)) return {unlocked:false,status:"Failed",region:region};
if ((r.body||"").toLowerCase().indexOf("not available in your country")!==-1) return {unlocked:false,status:"No",region:region};
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "Prime Video", key: "prime_video", icon: "simple-icons:primevideo", ruleType: "js", sortOrder: 14, def: ScriptDef{Code: `
var r=http_get("https://www.primevideo.com"); var body=r.body||"";
var blocked=body.indexOf("isServiceRestricted")!==-1; var m=body.match(/"currentTerritory":"([^"]+)"/);
if (blocked) return {unlocked:false,status:"No",region:""};
if (m) return {unlocked:true,status:"Yes",region:m[1]};
return {unlocked:false,status:"Failed",region:""};
`}},
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `encore test ./services/checker/ -run TestDefaultRule`
Run: `encore test ./services/checker/ -run TestDefaultRules_AllFifteenPresent`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/checker/rules.go services/checker/rules_defaults.go services/checker/rules_defaults_test.go
git commit -m "feat(checker): 15 verge-parity default rules with region/status output"
```

---

## Task 14: Default-rule sync (existing users get new logic + platforms)

**Files:**
- Modify: `services/checker/rules_defaults.go` (add `syncDefaultRules`)
- Modify: `services/checker/rules.go` (`ListRules` calls sync)
- Test: `services/checker/rules_sync_test.go` (new — pure helper test)

- [ ] **Step 1: Write the failing test**

Create `services/checker/rules_sync_test.go`:

```go
package checker

import "testing"

func TestDefaultRuleByKey_ReturnsSeed(t *testing.T) {
	d, ok := defaultRuleByKey("netflix")
	if !ok || d.name != "Netflix" {
		t.Fatalf("expected Netflix seed, got %+v ok=%v", d, ok)
	}
	if _, ok := defaultRuleByKey("nope"); ok {
		t.Fatalf("unexpected match for unknown key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `encore test ./services/checker/ -run TestDefaultRuleByKey_ReturnsSeed`
Expected: FAIL — `defaultRuleByKey` undefined.

- [ ] **Step 3: Implement sync**

In `services/checker/rules_defaults.go`, add:

```go
// defaultRuleByKey returns the seed rule for a key.
func defaultRuleByKey(key string) (defaultRule, bool) {
	for _, d := range defaultRules {
		if d.key == key {
			return d, true
		}
	}
	return defaultRule{}, false
}

// syncDefaultRules upserts the built-in (is_default=true) rules for a user so
// existing users receive new platforms and updated detection logic. User-authored
// rules (is_default=false) are never touched. NOTE: this overwrites manual edits
// to built-in rules — to persist edits, clone to a custom key (is_default=false).
func syncDefaultRules(ctx context.Context, userID string) error {
	now := time.Now()
	for _, dr := range defaultRules {
		defJSON, _ := json.Marshal(dr.def)
		if _, err := db.Exec(ctx, `
			INSERT INTO platform_rules (id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,true,$6,$7,true,$8,$9,$9)
			ON CONFLICT (user_id, key) DO UPDATE SET
			  name=EXCLUDED.name, icon=EXCLUDED.icon, rule_type=EXCLUDED.rule_type,
			  definition=EXCLUDED.definition, sort_order=EXCLUDED.sort_order, updated_at=EXCLUDED.updated_at
			WHERE platform_rules.is_default = true
		`, uuid.New().String(), userID, dr.name, dr.key, dr.icon, dr.ruleType, defJSON, dr.sortOrder, now); err != nil {
			return err
		}
	}
	return nil
}
```

> The `WHERE platform_rules.is_default = true` guard on the UPDATE means a user who deleted a built-in and re-created it as a custom rule (`is_default=false`) under the same key keeps their version; the upsert's INSERT path only fires when the key is absent.

In `services/checker/rules.go`, change `ListRules` so sync runs every call (cheap upsert), seeding when empty:

```go
func ListRules(ctx context.Context) (*ListRulesResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	userID := claims.UserID

	if err := syncDefaultRules(ctx, userID); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to sync default rules").Err()
	}
	rules, err := loadUserRules(ctx, userID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to load rules").Err()
	}
	return &ListRulesResponse{Rules: rules}, nil
}
```

Keep `seedDefaultRules` (used elsewhere / harmless) but it is no longer called from `ListRules`.

- [ ] **Step 4: Run test to verify it passes**

Run: `encore test ./services/checker/ -run TestDefaultRuleByKey_ReturnsSeed`
Expected: PASS. Then `go build ./services/...` (all services compile).

- [ ] **Step 5: Commit**

```bash
git add services/checker/rules_defaults.go services/checker/rules.go services/checker/rules_sync_test.go
git commit -m "feat(checker): sync default rules on ListRules so existing users upgrade"
```

---

## Task 15: Regenerate client + full verification

**Files:**
- Modify: `frontend/src/lib/client.gen.ts` (generated; flat monorepo — NOT `frontend/apps/web/...` despite the stale path in CLAUDE.md)

- [ ] **Step 1: Full backend test pass**

Run: `encore test ./services/checker/... ./services/notify/... ./services/settings/...`
Expected: PASS (no `-race`).

- [ ] **Step 2: Regenerate the typed client**

Run: `encore gen client subs-check-uqti --lang=typescript --output=./frontend/src/lib/client.gen.ts`
Expected: `NodeResult` now has `platforms: { [key: string]: PlatformOutcome }`, `LocalUnlockResult.platforms`, and the 9 bool fields + `extra_platforms` are gone. The frontend will not type-check until its plan runs — that's expected.

- [ ] **Step 3: Manual end-to-end check on dev DB**

Run: `encore run` (separate terminal), then trigger a check via the app or `curl`/MCP, and verify in `encore db shell checker`:

```sql
SELECT node_name, alive, latency_ms, platforms FROM check_results ORDER BY checked_at DESC LIMIT 3;
```

Expected: `platforms` populated with `{"netflix":{"unlocked":...,"status":"...","region":"..."},...}`; `latency_ms` filled via the Cloudflare probe; pre-migration rows still show their backfilled unlock state (region "").

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/client.gen.ts
git commit -m "chore: regenerate typed client for platforms map"
```

---

## Self-Review Notes

- **Spec coverage:** Engine (T1,T2) · unified jsonb + migration (T3) · latency (T4) · write path (T5) · read path (T6) · summary (T7,T8) · export (T9) · local probe (T10) · notify (T11) · settings tags (T12) · 15 verge rules (T13) · default sync (T14) · client (T15). All spec sections mapped.
- **Type consistency:** `PlatformOutcome` (T1) used identically in T2/T4/T6/T9/T10/T13. `PlatformUnlockSummary`=`map[string]int` (T6) consumed in T7/T8/T11. `taggedName(name,country,map,speed,cfg)` (T9) — single signature.
- **No leftover refs:** all `cr.netflix`/`ExtraPlatforms`/`unlockFlags`/`isAlive`/`measureLatency`/`platformEntries` removed by the task that introduces their replacement.
- **Known gap:** rule logic is hermetically tested via `mockClient` for representative platforms (claude/bilibili/spotify) + engine contract; full per-branch coverage of disney/bahamut is left to manual T15 verification (documented in spec risks).
