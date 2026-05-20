# Streaming Debug Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a debug mode to the streaming/media unlock check interface that captures every intermediate variable during platform detection (HTTP request/response, condition evaluations, rule engine variables) and displays them in a collapsible debug panel.

**Architecture:** Debug mode is opt-in via a toggle in check options. When enabled, each node's platform checks collect `DebugStep` traces that are sent alongside existing SSE progress events. The frontend accumulates these into a tree-structured Debug Panel.

**Tech Stack:** Go (Encore), React 19 + TypeScript, shadcn/ui, SSE

---

### Task 1: Backend debug data types + CheckOptions

**Files:**
- Modify: `services/checker/checker.go:152-156` (CheckOptions)
- Modify: `services/checker/checker.go:193-201` (progressUpdate)
- Create: `services/checker/debug.go` (debug types)

- [ ] **Step 1: Create `services/checker/debug.go` with debug types**

```go
package checker

// DebugStep captures one step in a platform check trace.
type DebugStep struct {
	Type        string         `json:"type"`        // "http_request" | "http_response" | "variable" | "condition" | "log" | "error"
	Description string         `json:"description"` // human-readable summary
	Details     map[string]any `json:"details"`     // free-form key-value pairs
}

// DebugTrace is the full trace for one platform check on one node.
type DebugTrace struct {
	Platform string      `json:"platform"`
	Result   bool        `json:"result"`
	Steps    []DebugStep `json:"steps"`
}

// NodeDebug holds debug traces for all platforms checked on one node.
type NodeDebug struct {
	NodeID   string       `json:"node_id"`
	NodeName string       `json:"node_name"`
	Traces   []DebugTrace `json:"traces"`
}
```

- [ ] **Step 2: Add `Debug` field to `CheckOptions`**

```go
// CheckOptions controls which tests are run per node.
type CheckOptions struct {
	SpeedTest       bool     `json:"speed_test"`
	UploadSpeedTest bool     `json:"upload_speed_test"`
	MediaApps       []string `json:"media_apps"`
	Debug           bool     `json:"debug"` // NEW
}
```

- [ ] **Step 3: Add `Debug` field to `progressUpdate`**

```go
type progressUpdate struct {
	Progress        int        `json:"progress"`
	Total           int        `json:"total"`
	NodeName        string     `json:"node_name,omitempty"`
	Alive           bool       `json:"alive"`
	LatencyMs       int        `json:"latency_ms,omitempty"`
	SpeedKbps       int        `json:"speed_kbps,omitempty"`
	UploadSpeedKbps int        `json:"upload_speed_kbps,omitempty"`
	Debug           *NodeDebug `json:"debug,omitempty"` // NEW
}
```

- [ ] **Step 4: Commit**

```bash
git add services/checker/debug.go services/checker/checker.go
git commit -m "feat: add debug data types and CheckOptions.Debug flag"
```

---

### Task 2: DebugRecorder helper

**Files:**
- Create: `services/checker/debug_recorder.go`

- [ ] **Step 1: Create `services/checker/debug_recorder.go`**

```go
package checker

import "sync"

// DebugRecorder collects debug steps for one platform check.
// Nil-safe: all methods are no-ops when called on nil receiver.
type DebugRecorder struct {
	mu     sync.Mutex
	Steps  []DebugStep
}

func (d *DebugRecorder) Add(s DebugStep) {
	if d == nil { return }
	d.mu.Lock()
	d.Steps = append(d.Steps, s)
	d.mu.Unlock()
}

func (d *DebugRecorder) HTTPReq(url, method string, headers map[string]string) {
	d.Add(DebugStep{
		Type:        "http_request",
		Description: method + " " + url,
		Details: map[string]any{"url": url, "method": method, "headers": headers},
	})
}

func (d *DebugRecorder) HTTPResp(code int, headers map[string]string, body string) {
	snippet := body
	if len(snippet) > 2000 {
		snippet = snippet[:2000]
	}
	d.Add(DebugStep{
		Type:        "http_response",
		Description: "HTTP " + strconv.Itoa(code),
		Details: map[string]any{"status_code": code, "headers": headers, "body_snippet": snippet},
	})
}

func (d *DebugRecorder) Variable(name string, value any) {
	d.Add(DebugStep{
		Type:        "variable",
		Description: name + " = " + fmt.Sprintf("%v", value),
		Details:     map[string]any{"name": name, "value": value},
	})
}

func (d *DebugRecorder) Condition(expression string, matched bool) {
	d.Add(DebugStep{
		Type:        "condition",
		Description: expression + " → " + fmt.Sprintf("%v", matched),
		Details:     map[string]any{"expression": expression, "matched": matched},
	})
}

func (d *DebugRecorder) Log(msg string) {
	d.Add(DebugStep{
		Type:        "log",
		Description: msg,
		Details:     map[string]any{"output": msg},
	})
}

func (d *DebugRecorder) Error(err error) {
	d.Add(DebugStep{
		Type:        "error",
		Description: err.Error(),
		Details:     map[string]any{"error": err.Error()},
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/debug_recorder.go
git commit -m "feat: add DebugRecorder helper for collecting trace steps"
```

---

### Task 3: Instrument built-in platform checks

**Files:**
- Modify: `services/checker/platform.go`

Each built-in check function gets a `*DebugRecorder` parameter. When non-nil, HTTP request/response details and condition results are recorded.

- [ ] **Step 1: Modify `checkNetflix` (lines 15-26)**

```go
func checkNetflix(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	if dr != nil {
		dr.Variable("method", "fetch Netflix title pages")
	}
	ok1 := fetchNetflixTitle(ctx, client, "81280792", dr)
	ok2 := fetchNetflixTitle(ctx, client, "70143836", dr)
	result := ok1 || ok2
	if dr != nil {
		dr.Variable("title_81280792_accessible", ok1)
		dr.Variable("title_70143836_accessible", ok2)
		dr.Variable("netflix_unlocked", result)
	}
	return result
}
```

- [ ] **Step 2: Modify `fetchNetflixTitle` (lines 28-55)**

```go
func fetchNetflixTitle(ctx context.Context, client *http.Client, titleID string, dr *DebugRecorder) bool {
	url := "https://www.netflix.com/title/" + titleID
	if dr != nil {
		dr.HTTPReq(url, "GET", nil)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, body)
	}
	unlocked := !strings.Contains(body, "Oh no!")
	if dr != nil {
		dr.Condition("body does not contain 'Oh no!'", unlocked)
	}
	return unlocked
}
```

- [ ] **Step 3: Modify `checkYouTube` (lines 58-78)**

```go
func checkYouTube(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.youtube.com/"
	if dr != nil {
		dr.HTTPReq(url, "GET", nil)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	// Use a transport that doesn't follow redirects so we can inspect the chain
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	if resp.StatusCode != 200 {
		if dr != nil { dr.Variable("youtube_unlocked", false); dr.Condition("status == 200", false) }
		return false
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	snippet := body
	if len(snippet) > 2000 { snippet = snippet[:2000] }
	if dr != nil {
		dr.Variable("body_snippet", snippet)
		dr.Condition("status == 200", true)
	}
	blocked := strings.Contains(body, "https://www.youtube.com/geolocation_block") ||
		strings.Contains(body, '"country"') && !strings.Contains(body, `"US"`)
	if dr != nil { dr.Condition("not geo-blocked", !blocked) }
	unlocked := !blocked
	if dr != nil { dr.Variable("youtube_unlocked", unlocked) }
	return unlocked
}
```

- [ ] **Step 4: Modify remaining check functions — `checkYouTubePremium`, `checkOpenAI`, `checkClaude`, `checkGemini`, `checkDisney`, `checkGrok`, `checkTikTok`**

Each follows the same pattern:
1. Add `dr *DebugRecorder` parameter
2. Before HTTP call: `dr.HTTPReq(url, "GET", nil)`
3. After HTTP call: capture response headers + truncated body via `dr.HTTPResp(statusCode, headers, body)`
4. Before returning: `dr.Variable("unlocked", result)`

**Key instrumentation for each:**

`checkYouTubePremium`:
```go
func checkYouTubePremium(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.youtube.com/premium"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	snippet := body
	if len(snippet) > 2000 { snippet = snippet[:2000] }
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, snippet)
	}
	hasPremium := strings.Contains(body, "ad-free") || strings.Contains(body, "YouTube Premium")
	blocked := strings.Contains(body, "Premium is not available")
	if dr != nil {
		dr.Condition("body contains 'ad-free' or 'YouTube Premium'", hasPremium)
		dr.Condition("body does NOT contain 'Premium is not available'", !blocked)
	}
	result := hasPremium && !blocked
	if dr != nil { dr.Variable("youtube_premium", result) }
	return result
}
```

`checkOpenAI`:
```go
func checkOpenAI(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://api.openai.com/v1/models"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	result := resp.StatusCode == 401 || resp.StatusCode == 200
	if dr != nil {
		dr.Condition("status == 401 (unauthorized, reachable) or status == 200", result)
		dr.Variable("status_code", resp.StatusCode)
		dr.Variable("openai_unlocked", result)
	}
	return result
}
```

`checkClaude`:
```go
func checkClaude(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://claude.ai/"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	client2 := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client2.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	redirectURL := ""
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectURL = resp.Header.Get("Location")
	}
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	if strings.Contains(redirectURL, "app-unavailable-in-region") {
		if dr != nil { dr.Variable("claude_unlocked", false); dr.Condition("redirect does NOT contain 'app-unavailable-in-region'", false) }
		return false
	}
	if dr != nil { dr.Condition("redirect does NOT contain 'app-unavailable-in-region'", true) }
	if resp.StatusCode == 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)
		snippet := body
		if len(snippet) > 2000 { snippet = snippet[:2000] }
		available := strings.Contains(body, "claude") || strings.Contains(body, "Anthropic")
		if dr != nil {
			dr.Variable("body_snippet", snippet)
			dr.Condition("body contains 'claude' or 'Anthropic'", available)
		}
		if dr != nil { dr.Variable("claude_unlocked", available) }
		return available
	}
	if dr != nil { dr.Variable("claude_unlocked", true) }
	return true
}
```

`checkGemini`:
```go
func checkGemini(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://gemini.google.com/"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	snippet := body
	if len(snippet) > 2000 { snippet = snippet[:2000] }
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, snippet)
	}
	result := strings.Contains(body, "Meet Gemini")
	if dr != nil {
		dr.Condition("body contains 'Meet Gemini'", result)
		dr.Variable("gemini_unlocked", result)
	}
	return result
}
```

`checkDisney`:
```go
func checkDisney(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.disneyplus.com/"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	result := resp.StatusCode == 200
	if dr != nil {
		dr.Condition("status == 200", result)
		dr.Variable("disney_unlocked", result)
	}
	return result
}
```

`checkGrok`:
```go
func checkGrok(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://api.x.ai/v1/models"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	result := resp.StatusCode == 401 || resp.StatusCode == 200
	if dr != nil {
		dr.Condition("status == 401 or 200", result)
		dr.Variable("status_code", resp.StatusCode)
		dr.Variable("grok_unlocked", result)
	}
	return result
}
```

`checkTikTok`:
```go
func checkTikTok(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.tiktok.com/"
	if dr != nil { dr.HTTPReq(url, "GET", nil) }
	client2 := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client2.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false
	}
	defer resp.Body.Close()
	redirectURL := ""
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectURL = resp.Header.Get("Location")
	}
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, "")
	}
	if strings.Contains(redirectURL, "comingsoon") || strings.Contains(redirectURL, "not-available") {
		if dr != nil { dr.Variable("tiktok_unlocked", false); dr.Condition("redirect not to comingsoon/not-available", false) }
		return false
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	snippet := body
	if len(snippet) > 2000 { snippet = snippet[:2000] }
	available := strings.Contains(body, `"region"`) || strings.Contains(body, "p16-tiktokcdn")
	if dr != nil {
		dr.Variable("body_snippet", snippet)
		dr.Condition("body has region JSON or CDN domain", available)
		dr.Variable("tiktok_unlocked", available)
	}
	return available
}
```

Note: The original functions returned `(bool, error)`. The new signatures return `bool` and use the recorder for error capture. The `checkNetflix` helper `fetchNetflixTitle` and the YouTube/TikTok helper logic are inlined above.

- [ ] **Step 5: Update `local_check.go` to pass nil DebugRecorder to the new signatures**

Each call like `v, _ := checkNetflix(checkCtx, client)` becomes `v := checkNetflix(checkCtx, client, nil)` (error return removed since it's now captured internally via DebugRecorder):

```go
// services/checker/local_check.go
run(func() {
    v := checkNetflix(checkCtx, client, nil)
    mu.Lock()
    res.Netflix = v
    mu.Unlock()
})
run(func() {
    v := checkYouTube(checkCtx, client, nil)
    mu.Lock()
    res.YouTube = v
    mu.Unlock()
})
// ... same pattern (remove _, err) for youtube_premium, openai, claude, gemini, grok, disney, tiktok
```

- [ ] **Step 7: Commit**

```bash
git add services/checker/platform.go services/checker/local_check.go
git commit -m "feat: instrument built-in platform checks with DebugRecorder"
```

---

### Task 4: Instrument rule engines

**Files:**
- Modify: `services/checker/engine.go`
- Modify: `services/checker/engine_condition.go`

- [ ] **Step 1: Modify `engine.go` — add `runRuleWithTrace` that accepts a DebugRecorder**

Change `runRule` to accept `dr *DebugRecorder`:
```go
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
	if dr != nil {
		dr.Variable("rule_name", rule.Name)
		dr.Variable("rule_type", rule.RuleType)
		dr.Variable("rule_key", rule.Key)
	}
	switch rule.RuleType {
	case "condition":
		return runConditionRule(ctx, client, rule.Definition, dr)
	case "js", "ts":
		return runJSRule(ctx, client, rule.RuleType, rule.Definition, dr)
	case "tengo":
		return runTengoRule(ctx, client, rule.Definition, dr)
	case "lua":
		return runLuaRule(ctx, client, rule.Definition, dr)
	default:
		err := fmt.Errorf("unknown rule_type: %s", rule.RuleType)
		if dr != nil { dr.Error(err) }
		return false, err
	}
}

// runUserRules runs all enabled rules, returns map[rule.Key]bool
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]bool {
	return runUserRulesWithDebug(ctx, client, rules, nil)
}

func runUserRulesWithDebug(ctx context.Context, client *http.Client, rules []*PlatformRule, results map[string]*DebugRecorder) map[string]bool {
	out := make(map[string]bool, len(rules))
	for _, rule := range rules {
		if !rule.Enabled { continue }
		var dr *DebugRecorder
		if results != nil {
			dr = &DebugRecorder{}
			results[rule.Key] = dr
		}
		ok, _ := runRule(ctx, client, rule, dr)
		out[rule.Key] = ok
	}
	return out
}
```

- [ ] **Step 2: Modify `engine_condition.go` — instrument condition evaluations**

```go
func runConditionRule(ctx context.Context, client *http.Client, rawDef json.RawMessage, dr *DebugRecorder) (bool, error) {
	var cond ConditionDef
	if err := json.Unmarshal(rawDef, &cond); err != nil {
		return false, fmt.Errorf("invalid condition definition: %w", err)
	}

	if dr != nil { dr.Variable("url", cond.URL); dr.Variable("method", cond.Method) }

	url := cond.URL
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	if dr != nil { dr.HTTPReq(url, cond.Method, cond.Headers) }

	req, err := http.NewRequestWithContext(ctx, cond.Method, url, nil)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false, err
	}
	for k, v := range cond.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	snippet := body
	if len(snippet) > 2000 { snippet = snippet[:2000] }
	if dr != nil {
		headers := map[string]string{}
		for k := range resp.Header { headers[k] = resp.Header.Get(k) }
		dr.HTTPResp(resp.StatusCode, headers, snippet)
	}

	finalURL := resp.Request.URL.String()

	// Evaluate all conditions, recording each
	allMatch := true

	if cond.StatusCode != 0 {
		match := resp.StatusCode == cond.StatusCode
		if dr != nil { dr.Condition(fmt.Sprintf("status_code == %d", cond.StatusCode), match) }
		if !match { allMatch = false }
	}

	if cond.BodyContains != "" {
		match := strings.Contains(body, cond.BodyContains)
		if dr != nil { dr.Condition(fmt.Sprintf("body contains %q", cond.BodyContains), match) }
		if !match { allMatch = false }
	}

	if len(cond.BodyContainsAny) > 0 {
		match := false
		for _, s := range cond.BodyContainsAny {
			if strings.Contains(body, s) { match = true; break }
		}
		if dr != nil { dr.Condition(fmt.Sprintf("body contains any of %v", cond.BodyContainsAny), match) }
		if !match { allMatch = false }
	}

	if cond.BodyNotContains != "" {
		match := !strings.Contains(body, cond.BodyNotContains)
		if dr != nil { dr.Condition(fmt.Sprintf("body does NOT contain %q", cond.BodyNotContains), match) }
		if !match { allMatch = false }
	}

	if cond.FinalURLContains != "" {
		match := strings.Contains(finalURL, cond.FinalURLContains)
		if dr != nil { dr.Condition(fmt.Sprintf("final_url contains %q", cond.FinalURLContains), match) }
		if !match { allMatch = false }
	}

	if cond.FinalURLNotContains != "" {
		match := !strings.Contains(finalURL, cond.FinalURLNotContains)
		if dr != nil { dr.Condition(fmt.Sprintf("final_url does NOT contain %q", cond.FinalURLNotContains), match) }
		if !match { allMatch = false }
	}

	if dr != nil { dr.Variable("unlock_result", allMatch) }
	return allMatch, nil
}
```

- [ ] **Step 3: Modify script engines (`engine_script.go`, `engine_tengo.go`, `engine_lua.go`) to accept DebugRecorder**

Add `dr *DebugRecorder` parameter to each `runJSRule`, `runTengoRule`, `runLuaRule`. In each engine, capture `console.log`/`tprint` output into `dr.Log()` and the return value into `dr.Variable()`.

For `engine_script.go` (goja JS/TS), add console.log capture:
```go
func runJSRule(ctx context.Context, client *http.Client, ruleType string, rawDef json.RawMessage, dr *DebugRecorder) (bool, error) {
	var def ScriptDef
	if err := json.Unmarshal(rawDef, &def); err != nil {
		return false, fmt.Errorf("invalid script definition: %w", err)
	}

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	// Console.log capture
	consoleObj := vm.NewObject()
	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		msg := strings.Join(parts, " ")
		if dr != nil { dr.Log(msg) }
		return goja.Undefined()
	})
	vm.Set("console", consoleObj)

	// ... rest of existing setup (injectHTTPGet, etc.) ...
	
	val, err := vm.RunString(def.Prelude + "\n" + def.Code)
	if err != nil {
		if dr != nil { dr.Error(err) }
		return false, err
	}
	result := val.ToBoolean()
	if dr != nil { dr.Variable("return_value", result) }
	return result, nil
}
```

Inject `dr` tracking into `injectHTTPGet` (lines 56-98):
```go
func injectHTTPGet(vm *goja.Runtime, client *http.Client, dr *DebugRecorder) {
	httpGet := func(url string, opts goja.Value) map[string]any {
		if dr != nil { dr.HTTPReq(url, "GET", nil) }
		// ... existing implementation ...
		resp, err := client.Get(url)
		if err != nil {
			if dr != nil { dr.Error(err) }
			return map[string]any{"error": err.Error()}
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)
		if dr != nil {
			headers := map[string]string{}
			for k := range resp.Header { headers[k] = resp.Header.Get(k) }
			snippet := body
			if len(snippet) > 2000 { snippet = snippet[:2000] }
			dr.HTTPResp(resp.StatusCode, headers, snippet)
		}
		// ... return existing map ...
	}
	vm.Set("http_get", httpGet)
}
```

Apply the same pattern to `engine_tengo.go` (capture `tprint`/`println` output) and `engine_lua.go` (capture `print` output).

- [ ] **Step 4: Commit**

```bash
git add services/checker/engine.go services/checker/engine_condition.go services/checker/engine_script.go services/checker/engine_tengo.go services/checker/engine_lua.go
git commit -m "feat: instrument rule engines with DebugRecorder"
```

---

### Task 5: Modify `checkNode` to collect and attach debug data

**Files:**
- Modify: `services/checker/mihomo.go` (nodeCheckResult + checkNode)

- [ ] **Step 1: Add `Debug *NodeDebug` to `nodeCheckResult`**

```go
type nodeCheckResult struct {
	// ... existing fields unchanged ...
	ExtraPlatforms  map[string]bool  `json:"extra_platforms"`
	Debug           *NodeDebug       `json:"debug,omitempty"` // NEW
}
```

- [ ] **Step 2: Modify `checkNode` to collect debug traces per platform**

```go
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL, uploadTestURL, latencyTestURL string, opts CheckOptions, rules []*PlatformRule) nodeCheckResult {
	name, _ := mapping["name"].(string)
	result := nodeCheckResult{NodeID: nodeID, NodeName: name}

	if opts.Debug {
		result.Debug = &NodeDebug{NodeID: nodeID, NodeName: name}
	}

	pc := newProxyClient(mapping)
	if pc == nil {
		if opts.Debug && result.Debug != nil {
			result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
				Platform: "connectivity",
				Result:   false,
				Steps:    []DebugStep{{Type: "error", Description: "failed to create proxy client", Details: map[string]any{"error": "invalid proxy config"}}},
			})
		}
		return result
	}
	defer pc.close()

	aliveDr := &DebugRecorder{}
	if !isAlive(ctx, pc.Client, latencyTestURL) {
		if opts.Debug && result.Debug != nil {
			result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
				Platform: "connectivity",
				Result:   false,
				Steps:    []DebugStep{{Type: "variable", Description: "alive = false", Details: map[string]any{"name": "alive", "value": false}}},
			})
		}
		return result
	}
	result.Alive = true
	result.LatencyMs = measureLatency(ctx, pc.Client, latencyTestURL)
	if opts.Debug && result.Debug != nil {
		result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
			Platform: "connectivity",
			Result:   true,
			Steps: []DebugStep{
				{Type: "variable", Description: "alive = true", Details: map[string]any{"name": "alive", "value": true}},
				{Type: "variable", Description: "latency_ms", Details: map[string]any{"name": "latency_ms", "value": result.LatencyMs}},
			},
		})
	}
	// ... speed test ...

	if len(opts.MediaApps) > 0 {
		mediaClient := &http.Client{
			Transport: pc.Transport,
			Timeout:   8 * time.Second,
		}
		result.IP, result.Country = getProxyInfo(ctx, mediaClient)

		// Run user rules with per-rule debug recorders
		var ruleRecorders map[string]*DebugRecorder
		if opts.Debug {
			ruleRecorders = make(map[string]*DebugRecorder)
		}
		ruleResults := runUserRulesWithDebug(ctx, mediaClient, rules, ruleRecorders)

		extra := make(map[string]bool)
		for k, v := range ruleResults {
			if !builtinKeys[k] {
				extra[k] = v
			}
		}
		if len(extra) > 0 {
			result.ExtraPlatforms = extra
		}

		resolve := func(key string, fallback func(*DebugRecorder) bool) bool {
			// Check if a user-defined rule provided the result for this key
			if v, ok := ruleResults[key]; ok {
				if opts.Debug && result.Debug != nil {
					steps := []DebugStep{}
					if rd, ok := ruleRecorders[key]; ok {
						steps = rd.Steps
					}
					result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
						Platform: key, Result: v, Steps: steps,
					})
				}
				return v
			}
			if v, ok := extra[key]; ok {
				if opts.Debug && result.Debug != nil {
					steps := []DebugStep{}
					if rd, ok := ruleRecorders[key]; ok {
						steps = rd.Steps
					}
					result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
						Platform: key, Result: v, Steps: steps,
					})
				}
				return v
			}
			// Fall back to built-in check function
			var dr *DebugRecorder
			if opts.Debug {
				dr = &DebugRecorder{}
			}
			v := fallback(dr)
			if opts.Debug && result.Debug != nil {
				result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
					Platform: key, Result: v, Steps: dr.Steps,
				})
			}
			return v
		}

		if hasApp(opts, "netflix") {
			result.Netflix = resolve("netflix", func(dr *DebugRecorder) bool { return checkNetflix(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "youtube") {
			result.YouTube = resolve("youtube", func(dr *DebugRecorder) bool { return checkYouTube(ctx, mediaClient, dr) })
			result.YouTubePremium = resolve("youtube_premium", func(dr *DebugRecorder) bool { return checkYouTubePremium(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "openai") {
			result.OpenAI = resolve("openai", func(dr *DebugRecorder) bool { return checkOpenAI(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "claude") {
			result.Claude = resolve("claude", func(dr *DebugRecorder) bool { return checkClaude(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "gemini") {
			result.Gemini = resolve("gemini", func(dr *DebugRecorder) bool { return checkGemini(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "grok") {
			result.Grok = resolve("grok", func(dr *DebugRecorder) bool { return checkGrok(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "disney") {
			result.Disney = resolve("disney", func(dr *DebugRecorder) bool { return checkDisney(ctx, mediaClient, dr) })
		}
		if hasApp(opts, "tiktok") {
			result.TikTok = resolve("tiktok", func(dr *DebugRecorder) bool { return checkTikTok(ctx, mediaClient, dr) })
		}
	}
	// ... traffic bytes ...
	return result
}
```

- [ ] **Step 3: Commit**

```bash
git add services/checker/mihomo.go
git commit -m "feat: collect debug traces in checkNode"
```

---

### Task 6: Modify SSE broadcast to include debug data

**Files:**
- Modify: `services/checker/checker.go` (broadcastProgress call in runJob)

- [ ] **Step 1: In `runJob` (line 804), include debug data in broadcast**

```go
n := processedCount.Add(1)
db.Exec(context.Background(), `UPDATE check_jobs SET progress=$2 WHERE id=$1`, jobID, n)
pu := progressUpdate{
    Progress:        int(n),
    Total:           total,
    NodeName:        res.NodeName,
    Alive:           res.Alive,
    LatencyMs:       res.LatencyMs,
    SpeedKbps:       res.SpeedKbps,
    UploadSpeedKbps: res.UploadSpeedKbps,
}
if opts.Debug && res.Debug != nil {
    pu.Debug = res.Debug
}
broadcastProgress(jobID, pu)
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/checker.go
git commit -m "feat: include debug data in SSE progress events"
```

---

### Task 7: Wire debug option through TriggerCheck flow

**Files:**
- Modify: `services/checker/checker.go` (TriggerParams + TriggerCheck)

- [ ] **Step 1: Add `Debug` field to `TriggerParams`**

```go
type TriggerParams struct {
	SpeedTest       *bool    `json:"speed_test"`
	UploadSpeedTest *bool    `json:"upload_speed_test"`
	MediaApps       []string `json:"media_apps"`
	Debug           *bool    `json:"debug"` // NEW
}
```

- [ ] **Step 2: Apply debug option in `applyOptionDefaults` or `TriggerCheck`**

In `TriggerCheck`, after merging params into options:
```go
if params.Debug != nil {
    opts.Debug = *params.Debug
}
```

- [ ] **Step 3: Commit**

```bash
git add services/checker/checker.go
git commit -m "feat: wire debug option through TriggerParams"
```

---

### Task 8: Regenerate frontend TypeScript types

**Files:**
- Modify: `frontend/apps/web/src/lib/client.gen.ts` (run `encore gen client`)

- [ ] **Step 1: Regenerate the TypeScript client**

```bash
cd /Users/ashark/Code/subs-check-re
encore gen client subs-check-uqti --lang=typescript --output=./frontend/apps/web/src/lib/client.gen.ts
```

Verify the generated `CheckOptions` and `TriggerParams` now include `"debug": boolean`.

- [ ] **Step 2: Commit**

```bash
git add frontend/apps/web/src/lib/client.gen.ts
git commit -m "chore: regenerate TypeScript client with debug option"
```

---

### Task 9: Frontend — SSEProgress type and debug state

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx`

- [ ] **Step 1: Update `SSEProgress` interface**

```typescript
interface DebugStep {
  type: "http_request" | "http_response" | "variable" | "condition" | "log" | "error"
  description: string
  details: Record<string, unknown>
}

interface DebugTrace {
  platform: string
  result: boolean
  steps: DebugStep[]
}

interface NodeDebug {
  node_id: string
  node_name: string
  traces: DebugTrace[]
}

interface SSEProgress {
  progress?: number
  total?: number
  node_name?: string
  alive?: boolean
  latency_ms?: number
  speed_kbps?: number
  upload_speed_kbps?: number
  done?: boolean
  status?: string
  debug?: NodeDebug
}
```

- [ ] **Step 2: Add debug state to the component**

```typescript
const [debugData, setDebugData] = useState<NodeDebug[]>([]);
```

- [ ] **Step 3: Clear debug state when job changes**

```typescript
useEffect(() => {
  setLogEntries([])
  setProgress(null)
  setDebugData([])  // NEW
}, [jobId])
```

- [ ] **Step 4: Accumulate debug data from SSE events**

```typescript
es.onmessage = (e) => {
  const data: SSEProgress = JSON.parse(e.data)
  setProgress(data)
  if (data.debug) {
    setDebugData((prev) => [...prev, data.debug!])
  }
  if (data.node_name) {
    setLogEntries((prev) => [...prev, data])
  }
  if (data.done) {
    es.close()
    setSelectedJobId(null)
    resultsQuery.refetch()
    qc.invalidateQueries({ queryKey: ["jobs", id] })
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/$id.tsx
git commit -m "feat: add SSEProgress debug types and state accumulation"
```

---

### Task 10: Frontend — DebugPanel component

**Files:**
- Create: `frontend/apps/web/src/components/debug-panel.tsx`

- [ ] **Step 1: Create `debug-panel.tsx`**

```typescript
import { ChevronDown, ChevronRight } from "lucide-react"
import { useState } from "react"
import type { DebugStep, DebugTrace, NodeDebug } from "@/routes/subscriptions/$id"

function DebugStepView({ step }: { step: DebugStep }) {
  const [open, setOpen] = useState(false)

  return (
    <div className="border-l-2 border-border pl-3 py-1 text-[11px] font-mono">
      <div className="flex items-center gap-2">
        {/* Type badge */}
        <span
          className="rounded px-1 py-0.5 text-[10px] font-medium uppercase"
          style={{
            background:
              step.type === "error" ? "var(--color-badge-danger-bg)" :
              step.type === "http_request" || step.type === "http_response" ? "var(--color-badge-info-bg)" :
              step.type === "variable" ? "var(--color-badge-success-bg)" :
              step.type === "condition" ? "var(--color-badge-warning-bg)" :
              "transparent",
            color:
              step.type === "error" ? "var(--destructive)" :
              step.type === "http_request" || step.type === "http_response" ? "var(--color-badge-info)" :
              step.type === "variable" ? "var(--color-badge-success)" :
              step.type === "condition" ? "var(--color-warning)" :
              "var(--muted-foreground)",
          }}
        >
          {step.type === "http_request" ? "REQ" :
           step.type === "http_response" ? "RES" :
           step.type === "variable" ? "VAR" :
           step.type === "condition" ? "IF" :
           step.type === "log" ? "LOG" :
           step.type === "error" ? "ERR" : step.type}
        </span>
        <span style={{ color: "var(--color-code)" }}>{step.description}</span>
        {Object.keys(step.details).length > 0 && (
          <button
            type="button"
            onClick={() => setOpen(!open)}
            className="ml-auto flex-shrink-0 p-0.5 rounded hover:bg-white/5"
          >
            {open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
          </button>
        )}
      </div>
      {open && Object.keys(step.details).length > 0 && (
        <div className="mt-1 ml-4 space-y-0.5 text-[10px]" style={{ color: "var(--color-dimmed)" }}>
          {Object.entries(step.details).map(([k, v]) => (
            <div key={k} className="flex gap-2">
              <span className="flex-shrink-0">{k}:</span>
              <span className="break-all" style={{ color: "var(--color-code)" }}>
                {typeof v === "string" ? v : JSON.stringify(v)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function DebugPlatformEntry({ trace }: { trace: DebugTrace }) {
  const [open, setOpen] = useState(false)

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-2 py-1 text-left text-xs hover:bg-white/5 rounded transition-colors"
      >
        {open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
        <span className="font-medium">{trace.platform}</span>
        <span
          className="ml-auto rounded px-1.5 py-0.5 text-[10px] font-medium"
          style={{
            background: trace.result ? "var(--color-badge-success-bg)" : "var(--color-badge-danger-bg)",
            color: trace.result ? "var(--color-badge-success)" : "var(--color-badge-danger)",
          }}
        >
          {trace.result ? "✓ UNLOCKED" : "✗ BLOCKED"}
        </span>
      </button>
      {open && (
        <div className="ml-3 space-y-0.5">
          {trace.steps.map((step, i) => (
            <DebugStepView key={i} step={step} />
          ))}
        </div>
      )}
    </div>
  )
}

function DebugNodeEntry({ node }: { node: NodeDebug }) {
  const [open, setOpen] = useState(false)

  return (
    <div className="rounded border border-border">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium hover:bg-white/5 transition-colors"
      >
        {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <span style={{ color: "var(--color-code)" }}>{node.node_name}</span>
        <span className="text-muted-foreground">({node.traces.length} platforms)</span>
      </button>
      {open && (
        <div className="border-t border-border pb-1 pt-1">
          {node.traces.map((trace) => (
            <DebugPlatformEntry key={trace.platform} trace={trace} />
          ))}
        </div>
      )}
    </div>
  )
}

export function DebugPanel({ data }: { data: NodeDebug[] }) {
  const [open, setOpen] = useState(true)

  if (data.length === 0) return null

  return (
    <div className="rounded-lg border border-border bg-card">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm font-medium transition-colors hover:bg-white/[0.02]"
      >
        {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        <span>Debug</span>
        <span className="text-muted-foreground text-xs font-normal">
          ({data.length} nodes, {data.reduce((n, d) => n + d.traces.length, 0)} traces)
        </span>
      </button>
      {open && (
        <div className="space-y-1.5 border-t border-border p-2 text-xs max-h-[600px] overflow-y-auto" style={{ scrollbarWidth: "thin" }}>
          {data.map((node) => (
            <DebugNodeEntry key={node.node_id || node.node_name} node={node} />
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/apps/web/src/components/debug-panel.tsx
git commit -m "feat: add DebugPanel component"
```

---

### Task 11: Frontend — Debug toggle in check options + render DebugPanel

**Files:**
- Modify: `frontend/apps/web/src/routes/subscriptions/index.tsx` (add debug toggle)
- Modify: `frontend/apps/web/src/routes/subscriptions/$id.tsx` (render DebugPanel)

- [ ] **Step 1: Add debug toggle state and wire into handleCheck on subscriptions list page**

In `SubRow` (subscriptions/index.tsx), add:
```typescript
const [debugMode, setDebugMode] = useState(false);
```

Update `handleCheck`:
```typescript
function handleCheck() {
  triggerMut.mutate({
    id: sub.id,
    opts: {
      speed_test: speedTest,
      upload_speed_test: uploadSpeedTest,
      media_apps: mediaApps,
      debug: debugMode,  // NEW
    },
  });
  setShowOpts(false);
}
```

Update the `triggerMut` type signature:
```typescript
triggerMut: {
  mutate: (args: {
    id: string;
    opts: { speed_test: boolean; upload_speed_test: boolean; media_apps: string[]; debug: boolean };
  }) => void;
  isPending: boolean;
};
```

Add the checkbox in the options panel (after the upload test checkbox, around line 397):
```tsx
<label className="flex cursor-pointer select-none items-center gap-2">
  <Checkbox
    checked={debugMode}
    onCheckedChange={(v) => setDebugMode(v === true)}
  />
  <span className="text-xs text-muted-foreground">🔧 Debug mode</span>
</label>
```

- [ ] **Step 2: Render DebugPanel in subscription detail page (`$id.tsx`)**

Import the component:
```typescript
import { DebugPanel } from "@/components/debug-panel";
```

After the progress section (after the closing `}` of the progress panel at line 374), add:
```tsx
{debugData.length > 0 && (
  <DebugPanel data={debugData} />
)}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/subscriptions/index.tsx frontend/apps/web/src/routes/subscriptions/$id.tsx
git commit -m "feat: add debug toggle to check options and render DebugPanel"
```

---

### Task 12: Verify build

**Files:**
- Root: `go.mod`

- [ ] **Step 1: Check backend compiles**

```bash
cd /Users/ashark/Code/subs-check-re
go build ./...
```

Expected: no errors.

- [ ] **Step 2: Check frontend compiles**

```bash
cd /Users/ashark/Code/subs-check-re/frontend
bun check-types
```

Expected: no type errors.

- [ ] **Step 3: Run linter**

```bash
cd /Users/ashark/Code/subs-check-re/frontend
bun check
```

Expected: no lint errors.
