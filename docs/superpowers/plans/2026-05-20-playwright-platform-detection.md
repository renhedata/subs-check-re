# Playwright 平台检测支持 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `playwright` 规则类型，让用户可以编写 TypeScript 浏览器脚本进行平台检测，独立 Playwright 服务通过代理节点访问目标网站。

**Architecture:** 独立 Node.js Playwright 服务提供 `/execute` API，Go checker 服务通过 HTTP 调用。用户在前端选择 `playwright` 规则类型并编写 TS 脚本。

**Tech Stack:** Node.js 20 + TypeScript + Fastify + Playwright (Chromium), Go + Encore

---

## 文件结构

### 新建文件

| 文件 | 职责 |
|------|------|
| `playwright/package.json` | Playwright 服务依赖 |
| `playwright/tsconfig.json` | TypeScript 配置 |
| `playwright/Dockerfile` | Docker 镜像定义 |
| `playwright/src/types.ts` | 类型定义（ExecuteRequest, ExecuteResponse） |
| `playwright/src/executor.ts` | 脚本执行引擎（编译 TS、启动浏览器、执行脚本） |
| `playwright/src/server.ts` | Fastify HTTP 服务（/execute 路由） |
| `playwright/src/index.ts` | 服务入口 |
| `playwright/src/__tests__/executor.test.ts` | 执行引擎单元测试 |
| `playwright/src/__tests__/server.test.ts` | HTTP API 集成测试 |
| `services/checker/engine_playwright.go` | Go 端 Playwright 规则执行 |
| `services/checker/engine_playwright_test.go` | Go 端 Playwright 规则测试 |

### 修改文件

| 文件 | 修改内容 |
|------|----------|
| `services/checker/rules.go` | `validRuleTypes` 新增 `playwright` |
| `services/checker/engine_condition.go` | `runRule` switch 新增 `playwright` case |
| `services/checker/debug_recorder.go` | 新增 `PlaywrightScript`、`PlaywrightResult` 方法 |
| `services/checker/mihomo.go` | 新增 `extractProxyConfig` 方法 |
| `frontend/apps/web/src/routes/settings/platforms.tsx` | 规则类型下拉框新增 `playwright`，编辑器支持 TS |
| `frontend/apps/web/src/components/debug-panel.tsx` | 新增 `playwright_script`、`playwright_result` step 渲染 |
| `deploy/docker-compose.yml` | 新增 `playwright` 服务（如果存在） |

---

## Task 1: Playwright 服务基础结构

**Files:**
- Create: `playwright/package.json`
- Create: `playwright/tsconfig.json`
- Create: `playwright/Dockerfile`
- Create: `playwright/src/types.ts`

- [ ] **Step 1: 创建 package.json**

```json
{
  "name": "subs-check-playwright",
  "version": "1.0.0",
  "description": "Playwright service for subs-check platform detection",
  "type": "module",
  "scripts": {
    "dev": "tsx src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "test": "vitest"
  },
  "dependencies": {
    "fastify": "^5.2.1",
    "playwright": "^1.51.0",
    "p-limit": "^6.2.0"
  },
  "devDependencies": {
    "@types/node": "^22.13.0",
    "tsx": "^4.19.3",
    "typescript": "^5.7.3",
    "vitest": "^3.0.5"
  }
}
```

- [ ] **Step 2: 创建 tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "esModuleInterop": true,
    "strict": true,
    "skipLibCheck": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "declaration": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] **Step 3: 创建 Dockerfile**

```dockerfile
FROM mcr.microsoft.com/playwright:v1.51.0-noble

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 3000

ENV PORT=3000
ENV MAX_CONCURRENT=5
ENV DEFAULT_TIMEOUT=30000

CMD ["npx", "tsx", "src/index.ts"]
```

- [ ] **Step 4: 创建 types.ts**

```typescript
export interface ProxyConfig {
  server: string;
  username?: string;
  password?: string;
}

export interface ExecuteRequest {
  script: string;
  proxy?: ProxyConfig;
  url?: string;
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

export interface PageContext {
  proxy?: ProxyConfig;
  url?: string;
}
```

- [ ] **Step 5: Commit**

```bash
git add playwright/
git commit -m "feat(playwright): add service foundation (package, tsconfig, dockerfile, types)"
```

---

## Task 2: Playwright 执行引擎

**Files:**
- Create: `playwright/src/executor.ts`
- Create: `playwright/src/__tests__/executor.test.ts`

- [ ] **Step 1: 编写执行引擎测试**

```typescript
import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { executeScript } from '../executor';

describe('executeScript', () => {
  it('should execute a simple script that returns true', async () => {
    const script = `
      async function check(page, context) {
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(true);
  });

  it('should execute a simple script that returns false', async () => {
    const script = `
      async function check(page, context) {
        return false;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(false);
  });

  it('should capture console logs', async () => {
    const script = `
      async function check(page, context) {
        console.log('test log');
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.logs).toContain('test log');
  });

  it('should handle script errors gracefully', async () => {
    const script = `
      async function check(page, context) {
        throw new Error('script error');
      }
    `;
    const result = await executeScript({ script, timeout: 10000 });
    expect(result.ok).toBe(false);
    expect(result.error).toContain('script error');
  });

  it('should respect timeout', async () => {
    const script = `
      async function check(page, context) {
        await page.waitForTimeout(100000);
        return true;
      }
    `;
    const result = await executeScript({ script, timeout: 1000 });
    expect(result.ok).toBe(false);
    expect(result.error).toContain('timeout');
  });

  it('should navigate to URL and check content', async () => {
    const script = `
      async function check(page, context) {
        await page.goto('https://example.com', { waitUntil: 'domcontentloaded' });
        const title = await page.title();
        return title.includes('Example');
      }
    `;
    const result = await executeScript({ script, timeout: 15000 });
    expect(result.ok).toBe(true);
    expect(result.result).toBe(true);
    expect(result.title).toContain('Example');
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd playwright
npm install
npx vitest src/__tests__/executor.test.ts --run
```

Expected: FAIL - `executeScript` not found

- [ ] **Step 3: 实现执行引擎**

```typescript
import { chromium, Browser, Page } from 'playwright';
import { ExecuteRequest, ExecuteResponse, PageContext } from './types';

const DEFAULT_TIMEOUT = 30000;
const DEFAULT_CONCURRENT = 5;

export async function executeScript(req: ExecuteRequest): Promise<ExecuteResponse> {
  const start = Date.now();
  const timeout = req.timeout || DEFAULT_TIMEOUT;
  const logs: string[] = [];

  let browser: Browser | null = null;

  try {
    // 启动浏览器
    const launchOptions: Parameters<typeof chromium.launch>[0] = {
      headless: true,
    };

    if (req.proxy) {
      launchOptions.proxy = {
        server: req.proxy.server,
        username: req.proxy.username,
        password: req.proxy.password,
      };
    }

    browser = await chromium.launch(launchOptions);

    const context = await browser.newContext({
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36',
    });

    const page = await context.newPage();

    // 捕获 console.log
    page.on('console', (msg) => {
      logs.push(`[${msg.type()}] ${msg.text()}`);
    });

    // 编译并执行用户脚本
    const scriptFn = compileScript(req.script);
    const pageContext: PageContext = {
      proxy: req.proxy,
      url: req.url,
    };

    // 带超时执行
    const result = await Promise.race([
      scriptFn(page, pageContext),
      new Promise<never>((_, reject) =
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

function compileScript(script: string): (page: Page, context: PageContext) => Promise<unknown> {
  // 包装用户脚本，提取 check 函数
  const wrapped = `
    ${script}
    if (typeof check !== 'function') {
      throw new Error('Script must define an async function named "check"');
    }
    check;
  `;

  // 使用 Function 构造器创建函数（在 Node.js 上下文中）
  // 注意：这里我们返回一个函数，该函数接收 page 和 context
  const fn = new Function('page', 'context', `
    return (async () => {
      ${script}
      if (typeof check !== 'function') {
        throw new Error('Script must define an async function named "check"');
      }
      return await check(page, context);
    })();
  `);

  return fn as (page: Page, context: PageContext) => Promise<unknown>;
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
npx vitest src/__tests__/executor.test.ts --run
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add playwright/src/executor.ts playwright/src/__tests__/executor.test.ts
git commit -m "feat(playwright): add script execution engine with tests"
```

---

## Task 3: Playwright HTTP 服务

**Files:**
- Create: `playwright/src/server.ts`
- Create: `playwright/src/index.ts`
- Create: `playwright/src/__tests__/server.test.ts`

- [ ] **Step 1: 编写 HTTP 服务测试**

```typescript
import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import Fastify from 'fastify';
import { registerRoutes } from '../server';

describe('POST /execute', () => {
  const app = Fastify();

  beforeAll(async () => {
    await registerRoutes(app);
    await app.ready();
  });

  afterAll(async () => {
    await app.close();
  });

  it('should execute a script and return result', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/execute',
      payload: {
        script: `
          async function check(page, context) {
            return true;
          }
        `,
        timeout: 10000,
      },
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.ok).toBe(true);
    expect(body.result).toBe(true);
    expect(body.duration_ms).toBeGreaterThan(0);
  });

  it('should handle invalid requests', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/execute',
      payload: {},
    });

    expect(response.statusCode).toBe(400);
  });

  it('should handle script errors', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/execute',
      payload: {
        script: `
          async function check(page, context) {
            throw new Error('test error');
          }
        `,
        timeout: 10000,
      },
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.ok).toBe(false);
    expect(body.error).toContain('test error');
  });

  it('should return screenshot when requested', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/execute',
      payload: {
        script: `
          async function check(page, context) {
            await page.goto('https://example.com', { waitUntil: 'domcontentloaded' });
            return true;
          }
        `,
        screenshot: true,
        timeout: 15000,
      },
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.ok).toBe(true);
    expect(body.screenshot).toBeDefined();
    expect(body.screenshot.length).toBeGreaterThan(0);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

```bash
npx vitest src/__tests__/server.test.ts --run
```

Expected: FAIL - `registerRoutes` not found

- [ ] **Step 3: 实现 HTTP 服务**

```typescript
import Fastify from 'fastify';
import { executeScript } from './executor';
import { ExecuteRequest } from './types';

export async function registerRoutes(app: Fastify.FastifyInstance) {
  app.post<{
    Body: ExecuteRequest;
  }>('/execute', async (request, reply) => {
    const { script, proxy, url, timeout, screenshot } = request.body;

    if (!script || typeof script !== 'string') {
      return reply.status(400).send({ error: 'script is required' });
    }

    const result = await executeScript({
      script,
      proxy,
      url,
      timeout,
      screenshot,
    });

    return reply.send(result);
  });

  app.get('/health', async () => {
    return { status: 'ok' };
  });
}
```

- [ ] **Step 4: 实现服务入口**

```typescript
import Fastify from 'fastify';
import { registerRoutes } from './server';

const app = Fastify({
  logger: true,
});

const PORT = parseInt(process.env.PORT || '3000', 10);

async function start() {
  await registerRoutes(app);

  try {
    await app.listen({ port: PORT, host: '0.0.0.0' });
    console.log(`Playwright service listening on port ${PORT}`);
  } catch (err) {
    app.log.error(err);
    process.exit(1);
  }
}

start();
```

- [ ] **Step 5: 运行测试确认通过**

```bash
npx vitest src/__tests__/server.test.ts --run
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add playwright/src/server.ts playwright/src/index.ts playwright/src/__tests__/server.test.ts
git commit -m "feat(playwright): add HTTP server with /execute endpoint and tests"
```

---

## Task 4: Go 端 Playwright 规则执行

**Files:**
- Create: `services/checker/engine_playwright.go`
- Create: `services/checker/engine_playwright_test.go`
- Modify: `services/checker/rules.go`
- Modify: `services/checker/engine_condition.go`

- [ ] **Step 1: 编写 Go 端测试**

```go
package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunPlaywrightRule(t *testing.T) {
	// 创建模拟的 Playwright 服务
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/execute" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// 返回模拟结果
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

	// 临时设置 Playwright URL
	oldURL := playwrightServiceURL
	playwrightServiceURL = mockServer.URL
	defer func() { playwrightServiceURL = oldURL }()

	ctx := context.Background()
	def := PlaywrightDef{
		URL:    "https://www.netflix.com/title/81280792",
		Script: "async function check(page, context) { return true; }",
	}
	defRaw, _ := json.Marshal(def)

	rule := &PlatformRule{
		RuleType:   "playwright",
		Definition: defRaw,
	}

	dr := &DebugRecorder{}
	result, err := runPlaywrightRule(ctx, nil, rule, dr)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Fatalf("expected true, got false")
	}

	// 验证 debug steps
	if len(dr.Steps) == 0 {
		t.Fatalf("expected debug steps")
	}
}

func TestRunPlaywrightRule_Error(t *testing.T) {
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
	def := PlaywrightDef{
		URL:    "https://www.netflix.com",
		Script: "async function check(page, context) { await page.waitForTimeout(100000); return true; }",
	}
	defRaw, _ := json.Marshal(def)

	rule := &PlatformRule{
		RuleType:   "playwright",
		Definition: defRaw,
	}

	dr := &DebugRecorder{}
	result, err := runPlaywrightRule(ctx, nil, rule, dr)

	if err == nil {
		t.Fatalf("expected error")
	}
	if result {
		t.Fatalf("expected false, got true")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/checker
go test -run TestRunPlaywrightRule -v
```

Expected: FAIL - `runPlaywrightRule` not defined

- [ ] **Step 3: 实现 Playwright 规则执行**

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

// PlaywrightDef defines a playwright rule.
type PlaywrightDef struct {
	URL     string `json:"url"`
	Script  string `json:"script"`
	Timeout int    `json:"timeout,omitempty"`
}

// playwrightExecuteRequest is the request body for the Playwright service.
type playwrightExecuteRequest struct {
	Script     string         `json:"script"`
	Proxy      *proxyConfig   `json:"proxy,omitempty"`
	URL        string         `json:"url,omitempty"`
	Timeout    int            `json:"timeout,omitempty"`
	Screenshot bool           `json:"screenshot,omitempty"`
}

// proxyConfig holds proxy settings for the Playwright service.
type proxyConfig struct {
	Server   string `json:"server"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
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

func runPlaywrightRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
	if dr != nil {
		dr.PlaywrightScript("playwright rule execution")
	}

	var def PlaywrightDef
	if err := json.Unmarshal(rule.Definition, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	if playwrightServiceURL == "" {
		err := fmt.Errorf("PLAYWRIGHT_URL not configured")
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	// Extract proxy config from client if available
	var proxy *proxyConfig
	if pc, ok := client.(*proxyClient); ok && pc.proxy != nil {
		proxy = extractProxyConfig(pc)
	}

	reqBody := playwrightExecuteRequest{
		Script:  def.Script,
		Proxy:   proxy,
		URL:     def.URL,
		Timeout: def.Timeout,
	}

	if reqBody.Timeout == 0 {
		reqBody.Timeout = 30000
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	if dr != nil {
		dr.Log(fmt.Sprintf("Sending request to Playwright service: %s", playwrightServiceURL))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", playwrightServiceURL+"/execute", bytes.NewReader(reqJSON))
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: time.Duration(reqBody.Timeout+5000) * time.Millisecond}
	resp, err := httpClient.Do(req)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	var result playwrightExecuteResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
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
		return false, fmt.Errorf("playwright execution failed: %s", result.Error)
	}

	return result.Result, nil
}

// extractProxyConfig extracts proxy configuration from a proxyClient.
func extractProxyConfig(pc *proxyClient) *proxyConfig {
	if pc == nil || pc.proxy == nil {
		return nil
	}

	// Try to extract proxy info from the mihomo proxy
	// This is a best-effort extraction
	addr := pc.proxy.Addr()
	if addr == "" {
		return nil
	}

	return &proxyConfig{
		Server: addr,
	}
}
```

- [ ] **Step 4: 修改 rules.go 添加 playwright 到 validRuleTypes**

```go
var validRuleTypes = map[string]bool{
	"condition":  true,
	"js":         true,
	"ts":         true,
	"tengo":      true,
	"lua":        true,
	"playwright": true, // 新增
}
```

- [ ] **Step 5: 修改 engine_condition.go 的 runRule 添加 playwright case**

```go
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
	switch rule.RuleType {
	case "condition":
		return runConditionRule(ctx, client, rule.Definition, dr)
	case "js", "ts":
		return runScriptRule(ctx, client, rule, dr)
	case "tengo":
		return runTengoRule(ctx, client, rule, dr)
	case "lua":
		return runLuaRule(ctx, client, rule, dr)
	case "playwright":
		return runPlaywrightRule(ctx, client, rule, dr) // 新增
	default:
		return false, fmt.Errorf("unsupported rule type: %s", rule.RuleType)
	}
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd services/checker
go test -run TestRunPlaywrightRule -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add services/checker/engine_playwright.go services/checker/engine_playwright_test.go
git add services/checker/rules.go services/checker/engine_condition.go
git commit -m "feat(checker): add playwright rule execution support"
```

---

## Task 5: DebugRecorder 扩展

**Files:**
- Modify: `services/checker/debug_recorder.go`

- [ ] **Step 1: 新增 Playwright 相关 debug 方法**

```go
func (d *DebugRecorder) PlaywrightScript(description string) {
	d.Add(DebugStep{
		Type:        "playwright_script",
		Description: description,
		Details:     toRawMessage(map[string]any{"type": "playwright_script"}),
	})
}

func (d *DebugRecorder) PlaywrightResult(result bool, logs []string) {
	d.Add(DebugStep{
		Type:        "playwright_result",
		Description: fmt.Sprintf("Playwright result: %v", result),
		Details:     toRawMessage(map[string]any{"result": result, "logs": logs}),
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add services/checker/debug_recorder.go
git commit -m "feat(debug): add playwright debug step types"
```

---

## Task 6: 前端规则编辑器支持

**Files:**
- Modify: `frontend/apps/web/src/routes/settings/platforms.tsx`
- Modify: `frontend/apps/web/src/components/debug-panel.tsx`

- [ ] **Step 1: 修改 platforms.tsx 添加 playwright 规则类型**

在 `RULE_TYPES` 和 `RULE_TYPE_LABELS` 中新增：

```typescript
const RULE_TYPES = ["condition", "js", "ts", "tengo", "lua", "playwright"] as const;

const RULE_TYPE_LABELS: Record<RuleType, string> = {
	condition: "Condition",
	js: "JavaScript",
	ts: "TypeScript",
	tengo: "Tengo",
	lua: "Lua",
	playwright: "Playwright", // 新增
};
```

在 `MONACO_LANG` 中新增：

```typescript
const MONACO_LANG: Record<RuleType, string> = {
	condition: "plaintext",
	js: "javascript",
	ts: "typescript",
	tengo: "go",
	lua: "lua",
	playwright: "typescript", // 新增
};
```

在 `TYPE_COLORS` 中新增：

```typescript
const TYPE_COLORS: Record<RuleType, string> = {
	condition: "bg-blue-500/10 text-blue-400 border-blue-500/30",
	js: "bg-yellow-500/10 text-yellow-400 border-yellow-500/30",
	ts: "bg-blue-600/10 text-blue-300 border-blue-600/30",
	tengo: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
	lua: "bg-purple-500/10 text-purple-400 border-purple-500/30",
	playwright: "bg-green-500/10 text-green-400 border-green-500/30", // 新增
};
```

- [ ] **Step 2: 添加 Playwright 文档**

在 `ENGINE_DOCS` 中新增：

```typescript
playwright: {
	sections: [
		{
			h: "Script Template",
			body: `async function check(page, context) {
  // context.proxy: { server, username?, password? }
  // context.url: string

  await page.goto('https://example.com', {
    waitUntil: 'networkidle'
  });

  const text = await page.textContent('body');
  return text.includes('success');
}`,
		},
		{
			h: "Page API",
			body: `page.goto(url, opts)     // 导航
page.waitForSelector(sel)  // 等待元素
page.waitForTimeout(ms)    // 等待时间
page.textContent(sel)      // 获取文本
page.innerText(sel)        // 获取内部文本
page.innerHTML(sel)        // 获取 HTML
page.getAttribute(sel, name) // 获取属性
page.evaluate(fn, arg)     // 执行 JS
page.screenshot(opts)      // 截图
page.url()                 // 当前 URL
page.title()               // 页面标题
page.click(sel)            // 点击
page.fill(sel, value)      // 填充输入`,
		},
		{
			h: "Return",
			body: "return true  // unlocked\nreturn false // blocked",
		},
	],
},
```

- [ ] **Step 3: 修改 ConsolePanel 显示 playwright 结果**

在 `ConsolePanel` 中新增 playwright 结果展示：

```typescript
{result.trace && result.trace.steps.length > 0 && (
  // 现有的 trace 展示
)}

{result.screenshot && (
  <div className="rounded border border-white/5">
    <div className="flex items-center justify-between border-white/5 border-b px-2 py-1">
      <span className="text-[#858585]">Screenshot</span>
    </div>
    <img
      src={`data:image/png;base64,${result.screenshot}`}
      alt="Playwright screenshot"
      className="max-h-64 w-full object-contain"
    />
  </div>
)}

{result.logs && result.logs.length > 0 && (
  <div className="rounded border border-white/5">
    <div className="border-white/5 border-b px-2 py-1 text-[#858585]">
      Logs <span className="text-[#569cd6]">{result.logs.length}</span>
    </div>
    <div className="max-h-32 overflow-auto px-2 py-1">
      {result.logs.map((log, i) => (
        <div key={i} className="text-[#858585] text-[10px]">{log}</div>
      ))}
    </div>
  </div>
)}
```

- [ ] **Step 4: 修改 debug-panel.tsx 支持 playwright step 类型**

在 `DebugStepView` 的 type badge 中新增：

```typescript
{step.type === "playwright_script"
  ? "PW"
  : step.type === "playwright_result"
    ? "PW_RES"
    : step.type === "http_request"
      ? "REQ"
      // ... 其余类型
}
```

在 badge 颜色中新增：

```typescript
step.type === "playwright_script" || step.type === "playwright_result"
  ? "var(--color-badge-info-bg)"
  : step.type === "error"
    ? // ... 其余类型
```

- [ ] **Step 5: Commit**

```bash
git add frontend/apps/web/src/routes/settings/platforms.tsx
git add frontend/apps/web/src/components/debug-panel.tsx
git commit -m "feat(frontend): add playwright rule type support in UI"
```

---

## Task 7: Docker Compose 配置

**Files:**
- Modify: `deploy/docker-compose.yml`（如果存在）
- Create: `deploy/docker-compose.example.yml`（如果不存在）

- [ ] **Step 1: 添加 playwright 服务到 docker-compose**

```yaml
version: '3.8'

services:
  subs-check:
    image: subs-check:latest
    environment:
      - PLAYWRIGHT_URL=http://playwright:3000
    depends_on:
      - playwright
    ports:
      - "8080:8080"

  playwright:
    build:
      context: ../playwright
      dockerfile: Dockerfile
    environment:
      - PORT=3000
      - MAX_CONCURRENT=5
      - DEFAULT_TIMEOUT=30000
    ports:
      - "3000:3000"
```

- [ ] **Step 2: Commit**

```bash
git add deploy/
git commit -m "feat(deploy): add playwright service to docker-compose"
```

---

## Task 8: 集成测试与验证

**Files:**
- Modify: `playwright/src/__tests__/integration.test.ts`

- [ ] **Step 1: 编写集成测试**

```typescript
import { describe, it, expect } from 'vitest';

describe('Integration: Playwright service', () => {
	it('should execute Netflix-like check script', async () => {
		const script = `
			async function check(page, context) {
				await page.goto('https://www.netflix.com/title/81280792', {
					waitUntil: 'domcontentloaded'
				});
				
				// Wait for potential CF challenge
				await page.waitForTimeout(3000);
				
				const text = await page.textContent('body');
				return !text.includes('Oh no!');
			}
		`;

		// 这里需要启动实际服务进行测试
		// 或者使用 mock
		const response = await fetch('http://localhost:3000/execute', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ script, timeout: 30000 }),
		});

		const result = await response.json();
		expect(result.ok).toBe(true);
		expect(typeof result.result).toBe('boolean');
	});
});
```

- [ ] **Step 2: 运行完整测试套件**

```bash
# Playwright 服务测试
cd playwright
npm test

# Go 服务测试
cd ../services/checker
go test -v

# 前端构建
cd ../../frontend/apps/web
npm run build
```

- [ ] **Step 3: Commit**

```bash
git add playwright/src/__tests__/integration.test.ts
git commit -m "test(playwright): add integration tests"
```

---

## Task 9: 文档更新

**Files:**
- Modify: `CLAUDE.md`（添加 Playwright 相关说明）

- [ ] **Step 1: 更新 CLAUDE.md**

在 Development Commands 部分添加：

```markdown
### Playwright 服务

```bash
cd playwright
npm install
npm run dev        # 启动 Playwright 服务（端口 3000）
npm test           # 运行测试
```

在 Architecture 表格中添加 Playwright 服务说明。

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with Playwright service docs"
```

---

## 自审检查

### Spec 覆盖检查

| Spec 要求 | 实现任务 |
|-----------|----------|
| 新增 `playwright` 规则类型 | Task 4 |
| 支持 TypeScript 脚本 | Task 1-3 |
| 独立 Playwright 服务 | Task 1-3 |
| 代理节点路由 | Task 4 (`extractProxyConfig`) |
| HTTP API (`/execute`) | Task 3 |
| 前端规则编辑器支持 | Task 6 |
| Docker 部署 | Task 7 |
| 调试信息（logs、screenshot） | Task 3, 5, 6 |
| 测试覆盖 | Task 2, 3, 4, 8 |

### Placeholder 扫描

- [x] 无 "TBD"、"TODO"
- [x] 所有步骤包含实际代码
- [x] 所有测试包含实际测试代码
- [x] 无 "类似 Task N" 的引用

### 类型一致性

- [x] `PlaywrightDef` 在 Go 和前端类型一致
- [x] `ExecuteRequest`/`ExecuteResponse` 在 TypeScript 和 Go 中字段一致
- [x] Debug step 类型 `playwright_script`/`playwright_result` 前后端一致

---

## 执行交接

**计划完成并保存到 `docs/superpowers/plans/2026-05-20-playwright-platform-detection.md`。**

**两种执行选项：**

**1. Subagent-Driven（推荐）** - 每个任务分配一个独立的子代理，任务间审查，快速迭代

**2. Inline Execution** - 在本会话中使用 executing-plans 执行任务，批量执行并设置检查点

**选择哪种方式？**
