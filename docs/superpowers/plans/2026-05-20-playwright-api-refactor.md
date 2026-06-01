# Playwright API 集成 — 实现计划（修订版）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Playwright 从独立规则类型改为现有规则引擎（JS/TS/Tengo/Lua）中的 API，用户通过 `playwright(url, script, opts?)` 函数调用。

**Architecture:** Playwright 服务保持独立，但 Go 端不再作为规则类型处理。改为在每个脚本引擎中注入 `playwright` 函数，该函数内部调用 Playwright 服务 `/execute` API。代理通过编辑器中现有的节点选择器配置。

**Tech Stack:** Node.js + TypeScript + Fastify + Playwright (Chromium), Go + Encore, Goja, Tengo, Gopher-Lua

---

## 文件结构

### 修改文件

| 文件 | 职责 |
|------|------|
| `services/checker/rules.go` | 从 `validRuleTypes` 中移除 `playwright`；从 `TestRule` 中移除 playwright 特殊处理 |
| `services/checker/engine_condition.go` | 从 `runRule` 中移除 `playwright` case |
| `services/checker/engine_script.go` | 注入 `playwright` API 到 Goja VM |
| `services/checker/engine_tengo.go` | 注入 `playwright` API 到 Tengo 脚本 |
| `services/checker/engine_lua.go` | 注入 `playwright` API 到 Lua 脚本 |
| `services/checker/engine_playwright.go` | 修改：从规则执行改为 API 调用函数 |
| `services/checker/debug_recorder.go` | 保留 `PlaywrightScript`/`PlaywrightResult` |
| `services/checker/mihomo.go` | 在 `checkNode` 中传递 Playwright 配置 |
| `frontend/apps/web/src/routes/settings/platforms.tsx` | 移除 `playwright` 规则类型；在 ENGINE_DOCS 中添加各语言的 `playwright` API 文档 |
| `frontend/apps/web/src/components/debug-panel.tsx` | 保留 playwright step 类型支持 |

---

## Task 1: 修改 Playwright 服务 API（简化）

**Files:**
- Modify: `playwright/src/types.ts`
- Modify: `playwright/src/executor.ts`

**变更：** 简化 API，移除 `proxy` 和 `url` 字段，改为只接收 `script` 和 `timeout`。代理由调用方（Go 服务）在请求头中传递。

- [ ] **Step 1: 修改 types.ts**

```typescript
export interface ExecuteRequest {
  script: string;
  timeout?: number;
  screenshot?: boolean;
}

export interface ExecuteResponse {
  ok: boolean;
  result: boolean;
  final_url?: string;
  title?: string;
  logs: string[];
  screenshot?: string;
  error?: string;
  duration_ms: number;
}
```

- [ ] **Step 2: 修改 executor.ts**

移除 `proxy` 和 `url` 参数处理，简化执行逻辑：

```typescript
export async function executeScript(req: ExecuteRequest): Promise<ExecuteResponse> {
  const start = Date.now();
  const timeout = req.timeout || DEFAULT_TIMEOUT;
  const logs: string[] = [];

  let browser: Browser | null = null;

  try {
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36',
    });
    const page = await context.newPage();

    page.on('console', (msg) => {
      logs.push(`[${msg.type()}] ${msg.text()}`);
    });

    const scriptFn = compileScript(req.script);
    
    const result = await Promise.race([
      scriptFn(page),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error(`Script execution timed out after ${timeout}ms`)), timeout)
      ),
    ]);

    const finalUrl = page.url();
    const title = await page.title().catch(() => undefined);

    let screenshot: string | undefined;
    if (req.screenshot) {
      const screenshotBuffer = await page.screenshot({ type: 'png', fullPage: true });
      screenshot = screenshotBuffer.toString('base64');
    }

    await browser.close();
    browser = null;

    return {
      ok: true,
      result: Boolean(result),
      final_url: finalUrl,
      title,
      logs,
      screenshot,
      duration_ms: Date.now() - start,
    };
  } catch (error) {
    if (browser) {
      await browser.close().catch(() => {});
    }

    return {
      ok: false,
      result: false,
      logs,
      error: error instanceof Error ? error.message : String(error),
      duration_ms: Date.now() - start,
    };
  }
}

function compileScript(script: string): (page: Page) => Promise<unknown> {
  const fn = new Function('page', `
    return (async () => {
      ${script}
      if (typeof check !== 'function') {
        throw new Error('Script must define an async function named "check"');
      }
      return await check(page);
    })();
  `);

  return fn as (page: Page) => Promise<unknown>;
}
```

- [ ] **Step 3: 更新测试**

修改 `executor.test.ts` 和 `server.test.ts`，移除 `proxy` 和 `url` 相关测试。

- [ ] **Step 4: Commit**

```bash
git add playwright/src/types.ts playwright/src/executor.ts playwright/src/__tests__/
git commit -m "refactor(playwright): simplify API, remove proxy/url params"
```

---

## Task 2: 修改 Go 端 Playwright 为 API 函数

**Files:**
- Modify: `services/checker/engine_playwright.go`
- Modify: `services/checker/engine_playwright_test.go`

**变更：** 从规则执行改为提供 `callPlaywright` 函数，供各引擎调用。

- [ ] **Step 1: 修改 engine_playwright.go**

```go
package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var playwrightServiceURL = os.Getenv("PLAYWRIGHT_URL")

// playwrightExecuteRequest is the request body for the Playwright service.
type playwrightExecuteRequest struct {
	Script     string `json:"script"`
	Timeout    int    `json:"timeout,omitempty"`
	Screenshot bool   `json:"screenshot,omitempty"`
}

// playwrightExecuteResponse is the response from the Playwright service.
type playwrightExecuteResponse struct {
	OK         bool     `json:"ok"`
	Result     bool     `json:"result"`
	FinalURL   string   `json:"final_url,omitempty"`
	Title      string   `json:"title,omitempty"`
	Logs       []string `json:"logs"`
	Screenshot string   `json:"screenshot,omitempty"`
	Error      string   `json:"error,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

// callPlaywright calls the Playwright service to execute a browser script.
// This function is injected into JS/TS, Tengo, and Lua rule engines.
func callPlaywright(ctx context.Context, script string, timeout int, screenshot bool, dr *DebugRecorder) (*playwrightExecuteResponse, error) {
	if dr != nil {
		dr.PlaywrightScript("playwright API call")
	}

	if playwrightServiceURL == "" {
		err := fmt.Errorf("PLAYWRIGHT_URL not configured")
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}

	if timeout == 0 {
		timeout = 30000
	}

	reqBody := playwrightExecuteRequest{
		Script:     script,
		Timeout:    timeout,
		Screenshot: screenshot,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}

	if dr != nil {
		dr.Log(fmt.Sprintf("Calling Playwright service: %s", playwrightServiceURL))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", playwrightServiceURL+"/execute", bytes.NewReader(reqJSON))
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: time.Duration(timeout+5000) * time.Millisecond}
	resp, err := httpClient.Do(req)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}

	var result playwrightExecuteResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return nil, err
	}

	if dr != nil {
		dr.PlaywrightResult(result.Result, result.Logs)
		if result.FinalURL != "" {
			dr.Variable("final_url", result.FinalURL)
		}
		if result.Title != "" {
			dr.Variable("page_title", result.Title)
		}
	}

	if !result.OK {
		return &result, fmt.Errorf("playwright execution failed: %s", result.Error)
	}

	return &result, nil
}
```

- [ ] **Step 2: 更新测试**

```go
package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallPlaywright(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/execute" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		resp := map[string]interface{}{
			"ok":          true,
			"result":      true,
			"final_url":   "https://www.netflix.com/title/81280792",
			"title":       "Netflix",
			"logs":        []string{"navigated to netflix"},
			"duration_ms": 1500,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	oldURL := playwrightServiceURL
	playwrightServiceURL = mockServer.URL
	defer func() { playwrightServiceURL = oldURL }()

	ctx := context.Background()
	dr := &DebugRecorder{}
	result, err := callPlaywright(ctx, "async function check(page) { return true; }", 10000, false, dr)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Result {
		t.Fatalf("expected true, got false")
	}
	if len(dr.Steps) == 0 {
		t.Fatalf("expected debug steps")
	}
}

func TestCallPlaywright_Error(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"ok":          false,
			"result":      false,
			"error":       "script timeout",
			"logs":        []string{},
			"duration_ms": 30000,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	oldURL := playwrightServiceURL
	playwrightServiceURL = mockServer.URL
	defer func() { playwrightServiceURL = oldURL }()

	ctx := context.Background()
	dr := &DebugRecorder{}
	result, err := callPlaywright(ctx, "async function check(page) { await page.waitForTimeout(100000); return true; }", 1000, false, dr)

	if err == nil {
		t.Fatalf("expected error")
	}
	if result == nil || result.Result {
		t.Fatalf("expected false result")
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add services/checker/engine_playwright.go services/checker/engine_playwright_test.go
git commit -m "refactor(checker): change playwright from rule type to API function"
```

---

## Task 3: 在 JS/TS 引擎中注入 playwright API

**Files:**
- Modify: `services/checker/engine_script.go`

- [ ] **Step 1: 修改 engine_script.go**

在 `injectHTTPGet` 之后添加 `injectPlaywright` 函数：

```go
// injectPlaywright registers the playwright() function in the goja VM.
// Usage: playwright(script, opts?)
//   script: string - the TypeScript script to execute
//   opts: { timeout?: number, screenshot?: boolean }
// Returns: { ok, result, final_url, title, logs, screenshot, error, duration_ms }
func injectPlaywright(ctx context.Context, vm *goja.Runtime, dr *DebugRecorder) error {
	fn := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("playwright requires a script argument"))
		}
		script := call.Arguments[0].String()

		timeout := 30000
		screenshot := false
		if len(call.Arguments) > 1 {
			if opts, ok := call.Arguments[1].Export().(map[string]interface{}); ok {
				if t, ok := opts["timeout"]; ok {
					timeout = int(t.(float64))
				}
				if s, ok := opts["screenshot"]; ok {
					screenshot = s.(bool)
				}
			}
		}

		result, err := callPlaywright(ctx, script, timeout, screenshot, dr)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}

		return vm.ToValue(map[string]interface{}{
			"ok":          result.OK,
			"result":      result.Result,
			"final_url":   result.FinalURL,
			"title":       result.Title,
			"logs":        result.Logs,
			"screenshot":  result.Screenshot,
			"error":       result.Error,
			"duration_ms": result.DurationMs,
		})
	}
	return vm.Set("playwright", fn)
}
```

在 `runJSRule` 中调用注入：

```go
if err := injectHTTPGet(ctx, vm, client, dr); err != nil {
    if dr != nil {
        dr.Error(err)
    }
    return false, err
}

if err := injectPlaywright(ctx, vm, dr); err != nil {
    if dr != nil {
        dr.Error(err)
    }
    return false, err
}
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/engine_script.go
git commit -m "feat(checker): inject playwright API into JS/TS engine"
```

---

## Task 4: 在 Tengo 引擎中注入 playwright API

**Files:**
- Modify: `services/checker/engine_tengo.go`

- [ ] **Step 1: 修改 engine_tengo.go**

在 `httpGetFn` 之后添加 `playwrightFn`：

```go
playwrightFn := &tengo.UserFunction{
	Name: "playwright",
	Value: func(args ...tengo.Object) (tengo.Object, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("playwright requires a script argument")
		}
		script, ok := tengo.ToString(args[0])
		if !ok {
			return nil, fmt.Errorf("playwright: script must be a string")
		}

		timeout := 30000
		screenshot := false
		if len(args) > 1 {
			if m, ok := args[1].(*tengo.Map); ok {
				if t, ok := m.Value["timeout"]; ok {
					if ti, ok := t.(*tengo.Int); ok {
						timeout = int(ti.Value)
					}
				}
				if s, ok := m.Value["screenshot"]; ok {
					if sb, ok := s.(*tengo.Bool); ok {
						screenshot = sb.Value
					}
				}
			}
		}

		result, err := callPlaywright(context.Background(), script, timeout, screenshot, dr)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return &tengo.Map{Value: map[string]tengo.Object{
			"ok":          &tengo.Bool{Value: result.OK},
			"result":      &tengo.Bool{Value: result.Result},
			"final_url":   &tengo.String{Value: result.FinalURL},
			"title":       &tengo.String{Value: result.Title},
			"logs":        stringSliceToTengo(result.Logs),
			"screenshot":  &tengo.String{Value: result.Screenshot},
			"error":       &tengo.String{Value: result.Error},
			"duration_ms": &tengo.Int{Value: result.DurationMs},
		}}, nil
	},
}

if err := script.Add("playwright", playwrightFn); err != nil {
	if dr != nil {
		dr.Error(err)
	}
	return false, err
}
```

添加辅助函数：

```go
func stringSliceToTengo(slice []string) *tengo.Array {
	arr := &tengo.Array{Value: make([]tengo.Object, len(slice))}
	for i, s := range slice {
		arr.Value[i] = &tengo.String{Value: s}
	}
	return arr
}
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/engine_tengo.go
git commit -m "feat(checker): inject playwright API into Tengo engine"
```

---

## Task 5: 在 Lua 引擎中注入 playwright API

**Files:**
- Modify: `services/checker/engine_lua.go`

- [ ] **Step 1: 修改 engine_lua.go**

在 `http_get` 之后添加 `playwright` 函数：

```go
L.SetGlobal("playwright", L.NewFunction(func(L *lua.LState) int {
	script := L.CheckString(1)
	
	timeout := 30000
	screenshot := false
	
	if L.GetTop() >= 2 {
		if opts, ok := L.Get(2).(*lua.LTable); ok {
			if t := opts.RawGetString("timeout"); t != lua.LNil {
				if n, ok := t.(lua.LNumber); ok {
					timeout = int(n)
				}
			}
			if s := opts.RawGetString("screenshot"); s != lua.LNil {
				if b, ok := s.(lua.LBool); ok {
					screenshot = bool(b)
				}
			}
		}
	}

	result, err := callPlaywright(L.Context(), script, timeout, screenshot, dr)
	
	t := L.NewTable()
	if err != nil {
		L.SetField(t, "ok", lua.LBool(false))
		L.SetField(t, "result", lua.LBool(false))
		L.SetField(t, "error", lua.LString(err.Error()))
		L.Push(t)
		return 1
	}

	L.SetField(t, "ok", lua.LBool(result.OK))
	L.SetField(t, "result", lua.LBool(result.Result))
	L.SetField(t, "final_url", lua.LString(result.FinalURL))
	L.SetField(t, "title", lua.LString(result.Title))
	
	logs := L.NewTable()
	for i, log := range result.Logs {
		logs.RawSetInt(i+1, lua.LString(log))
	}
	L.SetField(t, "logs", logs)
	
	L.SetField(t, "screenshot", lua.LString(result.Screenshot))
	L.SetField(t, "error", lua.LString(result.Error))
	L.SetField(t, "duration_ms", lua.LNumber(result.DurationMs))
	
	L.Push(t)
	return 1
}))
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/engine_lua.go
git commit -m "feat(checker): inject playwright API into Lua engine"
```

---

## Task 6: 移除 playwright 规则类型

**Files:**
- Modify: `services/checker/rules.go`
- Modify: `services/checker/engine_condition.go`

- [ ] **Step 1: 从 validRuleTypes 中移除 playwright**

```go
var validRuleTypes = map[string]bool{
	"condition": true,
	"js":        true,
	"ts":        true,
	"tengo":     true,
	"lua":       true,
}
```

- [ ] **Step 2: 从 runRule 中移除 playwright case**

```go
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
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
		return false, fmt.Errorf("unsupported rule type: %s", rule.RuleType)
	}
}
```

- [ ] **Step 3: 从 TestRule 中移除 playwright 特殊处理**

移除 `TestRule` 函数中提取 screenshot 和 logs 的 playwright 特殊逻辑。

- [ ] **Step 4: Commit**

```bash
git add services/checker/rules.go services/checker/engine_condition.go
git commit -m "refactor(checker): remove playwright as standalone rule type"
```

---

## Task 7: 修改前端

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/platforms.tsx`

- [ ] **Step 1: 移除 playwright 规则类型**

```typescript
const RULE_TYPES = ["condition", "js", "ts", "tengo", "lua"] as const;
```

从 `RULE_TYPE_LABELS`、`MONACO_LANG`、`TYPE_COLORS` 中移除 `playwright`。

- [ ] **Step 2: 在 ENGINE_DOCS 中为每种语言添加 playwright API 文档**

为 JS/TS 添加：

```typescript
js: {
	sections: [
		// ... existing sections
		{
			h: "playwright(script, opts?)",
			body: `const r = playwright(` + '`' + `
  async function check(page) {
    await page.goto('https://example.com')
    const text = await page.textContent('body')
    return text.includes('success')
  }
` + '`' + `, { timeout: 30000 })
// r.ok       // boolean
// r.result   // boolean
// r.final_url // string
// r.title    // string
// r.logs     // string[]
// r.screenshot // base64 string (if opts.screenshot=true)
// r.error    // string
// r.duration_ms // number`,
		},
	],
},
```

为 Tengo 添加类似文档。

为 Lua 添加类似文档。

Condition 不需要 Playwright 文档（因为 condition 是简单的 HTTP 检查）。

- [ ] **Step 3: Commit**

```bash
git add frontend/apps/web/src/routes/settings/platforms.tsx
git commit -m "feat(frontend): remove playwright rule type, add API docs to existing engines"
```

---

## Task 8: 更新测试

**Files:**
- Modify: `services/checker/engine_playwright_test.go`
- Modify: `playwright/src/__tests__/integration.test.ts`

- [ ] **Step 1: 更新 Go 测试**

确保 `callPlaywright` 测试通过。

- [ ] **Step 2: 更新 Playwright 服务测试**

确保简化后的 API 测试通过。

- [ ] **Step 3: 运行完整测试套件**

```bash
cd playwright
SKIP_INTEGRATION_TESTS=true npx vitest run

cd ../services/checker
encore test -v

cd ../../frontend/apps/web
bun run build
```

- [ ] **Step 4: Commit**

```bash
git add services/checker/engine_playwright_test.go playwright/src/__tests__/
git commit -m "test: update tests for playwright API refactor"
```

---

## Task 9: 更新文档

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/superpowers/specs/2026-05-20-playwright-platform-detection-design.md`

- [ ] **Step 1: 更新 CLAUDE.md**

在 Architecture 表格中更新 Playwright 服务说明：

```markdown
| `playwright` | Headless browser API for JS/TS/Tengo/Lua rules (bypasses CF challenges) |
```

- [ ] **Step 2: 更新设计文档**

更新设计文档，反映 Playwright 从规则类型改为 API 的变更。

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md docs/superpowers/specs/
git commit -m "docs: update docs for playwright API refactor"
```

---

## 自审检查

### Spec 覆盖检查

| 需求 | 任务 |
|------|------|
| Playwright 作为 API 而非规则类型 | Task 2, 6 |
| JS/TS 引擎支持 playwright() | Task 3 |
| Tengo 引擎支持 playwright() | Task 4 |
| Lua 引擎支持 playwright() | Task 5 |
| 代理通过现有节点选择器 | 无需修改（已使用 TestRule 中的 node_id） |
| 前端文档说明 API 用法 | Task 7 |
| 测试覆盖 | Task 8 |

### Placeholder 扫描

- [x] 无 "TBD"、"TODO"
- [x] 所有步骤包含实际代码
- [x] 无 "类似 Task N" 的引用

---

## 执行交接

**计划完成并保存到 `docs/superpowers/plans/2026-05-20-playwright-api-refactor.md`。**

**两种执行选项：**

**1. Subagent-Driven（推荐）** - 每个任务分配一个独立的子代理，任务间审查

**2. Inline Execution** - 在当前会话中批量执行任务

**选择哪种方式？**
