# Playwright 平台检测支持 — 设计文档（修订版）

**日期**: 2026-05-20
**状态**: 已批准
**修订**: 2026-05-21 — Playwright 从独立规则类型改为现有规则引擎中的 API

---

## 背景

部分流媒体平台（如 Netflix、Disney+ 等）使用 Cloudflare 人机验证，传统的 HTTP 请求检测方式无法通过验证。需要引入 Playwright（headless 浏览器）来模拟真实用户访问，自动通过 CF 验证后再进行解锁状态检测。

## 目标

1. 在现有规则引擎（JS/TS/Tengo/Lua）中注入 `playwright(script, opts?)` API，让用户可以在任何脚本中调用浏览器检测
2. 支持 TypeScript 编写 Playwright 脚本
3. 流量通过被检测的代理节点路由（通过现有节点选择器）
4. 独立 Playwright 服务部署，Go 服务通过 HTTP API 调用

## 非目标

- 不自动判断何时使用 Playwright（由用户在脚本中显式调用）
- Playwright 服务不需要认证（内部网络调用）
- 不替代现有规则类型（condition/js/ts/tengo/lua），而是在这些引擎中新增 API

## 架构

### 服务架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Docker Network                        │
│  ┌──────────────┐      HTTP API      ┌──────────────────┐  │
│  │   Go App     │ ─────────────────> │  Playwright Svc  │  │
│  │  (Encore)    │  POST /execute     │   (Node.js/TS)   │  │
│  └──────────────┘                    └──────────────────┘  │
│                                             │               │
│                                             ▼               │
│                                       ┌─────────────┐      │
│                                       │   Browser   │──┐   │
│                                       │  (Chromium) │  │   │
│                                       └─────────────┘  │   │
│                                                        ▼   │
│                                                 目标网站    │
│                                              (via proxy)   │
└─────────────────────────────────────────────────────────────┘
```

### 数据流

1. 用户在 `/settings/platforms` 创建规则，选择 JS/TS/Tengo/Lua 类型
2. 在脚本中调用 `playwright(script, opts?)` 函数，传入 TypeScript 浏览器脚本
3. 检测时，Go checker 服务将脚本发送到 Playwright 服务 `/execute`
4. Playwright 服务启动浏览器，执行脚本
5. 脚本返回 `boolean` 表示解锁状态
6. Playwright 服务返回结果 + 调试信息（logs、final_url、screenshot 等）

## Playwright 服务

### 技术栈

- **Runtime**: Node.js 20 + TypeScript
- **Framework**: Fastify（轻量、性能好）
- **Browser**: Playwright + Chromium
- **Build**: `tsx` 直接运行 TS，无需编译步骤

### HTTP API

#### POST /execute

请求体：
```typescript
interface ExecuteRequest {
  /** 用户编写的 TypeScript 脚本 */
  script: string;
  /** 超时时间（毫秒），默认 30000 */
  timeout?: number;
  /** 是否返回截图（调试用） */
  screenshot?: boolean;
}
```

响应体：
```typescript
interface ExecuteResponse {
  /** 脚本是否成功执行 */
  ok: boolean;
  /** 脚本返回值（boolean） */
  result: boolean;
  /** 最终 URL */
  final_url?: string;
  /** 页面标题 */
  title?: string;
  /** 执行日志 */
  logs: string[];
  /** 截图（base64，调试用） */
  screenshot?: string;
  /** 错误信息 */
  error?: string;
  /** 执行耗时（毫秒） */
  duration_ms: number;
}
```

### 脚本执行环境

用户脚本是一个 TypeScript 异步函数，接收一个参数：

```typescript
// 用户脚本签名
async function check(page: Page): Promise<boolean> {
  // 用户实现
}
```

#### 提供的 API

用户脚本可以使用 Playwright 的完整 `Page` API：

| API | 说明 |
|-----|------|
| `page.goto(url, options)` | 导航到 URL |
| `page.waitForSelector(selector, options)` | 等待元素出现 |
| `page.waitForTimeout(ms)` | 等待指定时间 |
| `page.textContent(selector)` | 获取元素文本 |
| `page.innerText(selector)` | 获取元素内部文本 |
| `page.innerHTML(selector)` | 获取元素 HTML |
| `page.getAttribute(selector, name)` | 获取元素属性 |
| `page.evaluate(fn, arg)` | 在页面上下文中执行 JS |
| `page.screenshot(options)` | 截图 |
| `page.url()` | 当前 URL |
| `page.title()` | 页面标题 |
| `page.click(selector)` | 点击元素 |
| `page.fill(selector, value)` | 填充输入框 |
| `page.selectOption(selector, values)` | 选择下拉框 |
| `page.check(selector)` | 勾选复选框 |
| `page.uncheck(selector)` | 取消勾选 |
| `page.isVisible(selector)` | 元素是否可见 |
| `page.isHidden(selector)` | 元素是否隐藏 |
| `page.content()` | 页面完整 HTML |

### 执行流程

```
收到 /execute 请求
        ↓
编译用户脚本（Function 构造器）
        ↓
启动浏览器（chromium.launch）
        ↓
创建新 context
        ↓
创建新 page
        ↓
注入日志捕获器（console.log → logs 数组）
        ↓
执行用户脚本（page）
        ↓
收集结果（return value、final_url、title、logs、screenshot）
        ↓
关闭 browser
        ↓
返回 JSON 响应
```

### 安全与资源控制

1. **超时控制**：默认 30 秒，通过 `Promise.race` 实现
2. **并发限制**：使用 `p-limit` 限制同时运行的浏览器实例数（默认 5）
3. **资源清理**：无论成功或失败，都确保关闭 browser
4. **脚本隔离**：每个请求独立的 browser 实例，无共享状态
5. **无文件系统访问**：不暴露 fs、path 等 Node.js API 给用户脚本

### 项目结构

```
playwright/
├── Dockerfile                 # 基于 mcr.microsoft.com/playwright
├── package.json
├── tsconfig.json
└── src/
    ├── index.ts              # 服务入口，启动 Fastify
    ├── server.ts             # HTTP 路由定义
    ├── executor.ts           # 脚本执行引擎
    └── types.ts              # 类型定义
```

### Dockerfile

```dockerfile
FROM mcr.microsoft.com/playwright:v1.51.0-noble

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 3000

CMD ["npx", "tsx", "src/index.ts"]
```

## Go 服务集成

### 配置

环境变量：
```bash
PLAYWRIGHT_URL=http://playwright:3000  # Playwright 服务地址
```

### Playwright API 函数

`callPlaywright` 是一个内部函数，被注入到各脚本引擎中：

```go
// callPlaywright calls the Playwright service to execute a browser script.
func callPlaywright(ctx context.Context, script string, timeout int, screenshot bool, dr *DebugRecorder) (*playwrightExecuteResponse, error) {
    // 1. 检查 PLAYWRIGHT_URL 是否配置
    // 2. 构造请求体（script, timeout, screenshot）
    // 3. POST 到 Playwright 服务 /execute
    // 4. 记录 debug steps
    // 5. 返回结果
}
```

### 引擎注入

#### JS/TS 引擎（Goja）

```go
func injectPlaywright(ctx context.Context, vm *goja.Runtime, dr *DebugRecorder) error {
    fn := func(call goja.FunctionCall) goja.Value {
        script := call.Arguments[0].String()
        // 解析 opts（timeout, screenshot）
        result, err := callPlaywright(ctx, script, timeout, screenshot, dr)
        // 返回 { ok, result, final_url, title, logs, screenshot, error, duration_ms }
    }
    return vm.Set("playwright", fn)
}
```

#### Tengo 引擎

```go
playwrightFn := &tengo.UserFunction{
    Name: "playwright",
    Value: func(args ...tengo.Object) (tengo.Object, error) {
        script, _ := tengo.ToString(args[0])
        // 解析 opts
        result, err := callPlaywright(ctx, script, timeout, screenshot, dr)
        // 返回 tengo.Map
    },
}
script.Add("playwright", playwrightFn)
```

#### Lua 引擎

```go
L.SetGlobal("playwright", L.NewFunction(func(L *lua.LState) int {
    script := L.CheckString(1)
    // 解析 opts
    result, err := callPlaywright(ctx, script, timeout, screenshot, dr)
    // 返回 lua table
}))
```

### DebugRecorder 扩展

保留 playwright 相关的 debug step 类型：

```go
func (d *DebugRecorder) PlaywrightScript(description string) {
    d.Add(DebugStep{
        Type:        "playwright_script",
        Description: description,
        Details:     toRawMessage(map[string]any{"script": description}),
    })
}

func (d *DebugRecorder) PlaywrightResult(result bool, logs []string, screenshot string) {
    d.Add(DebugStep{
        Type:        "playwright_result",
        Description: fmt.Sprintf("playwright result = %v", result),
        Details:     toRawMessage(map[string]any{"result": result, "logs": logs, "screenshot": screenshot}),
    })
}
```

## 前端集成

### 规则编辑器

在 `/settings/platforms` 页面：

1. **规则类型选择**：只保留 condition/js/ts/tengo/lua（移除独立的 playwright 选项）
2. **编辑器**：Monaco Editor，根据类型使用对应语法高亮
3. **API 文档面板**：在每种脚本引擎的文档中新增 `playwright()` API 说明
4. **调试功能**：
   - 测试规则时，如果脚本调用了 `playwright()` 并返回 screenshot，显示截图
   - 显示执行日志（logs）

### 前端类型更新

```typescript
// TestRuleResult 保留 screenshot 和 logs 字段
export interface TestRuleResult {
  // ... 现有字段
  screenshot?: string;  // base64 截图（调试用）
  logs?: string[];      // 执行日志
}
```

### 示例脚本

#### JavaScript 中使用 Playwright

```javascript
// 先进行 HTTP 检测
const r = http_get("https://www.netflix.com/title/81280792")
if (r.status === 200 && !r.body.includes("Oh no!")) {
  return true
}

// HTTP 检测失败，尝试浏览器检测
const p = playwright(`
  async function check(page) {
    await page.goto('https://www.netflix.com/title/81280792', {
      waitUntil: 'networkidle'
    })
    await page.waitForTimeout(3000)
    const text = await page.textContent('body')
    return !text.includes('Oh no!')
  }
`, { timeout: 30000 })

return p.result
```

#### TypeScript 中使用 Playwright

```typescript
const r = http_get("https://www.netflix.com/title/81280792")
if (r.status === 200 && !r.body.includes("Oh no!")) {
  return true
}

const p = playwright(`
  async function check(page) {
    await page.goto('https://www.netflix.com/title/81280792')
    await page.waitForTimeout(3000)
    const text = await page.textContent('body')
    return !text.includes('Oh no!')
  }
`, { timeout: 30000, screenshot: true })

return p.result
```

#### Tengo 中使用 Playwright

```go
r := http_get("https://www.netflix.com/title/81280792")
if r.status == 200 {
    output = true
} else {
    p := playwright(`
      async function check(page) {
        await page.goto('https://www.netflix.com/title/81280792')
        await page.waitForTimeout(3000)
        const text = await page.textContent('body')
        return !text.includes('Oh no!')
      }
    `, { timeout: 30000 })
    output = p.result
}
```

#### Lua 中使用 Playwright

```lua
local r = http_get("https://www.netflix.com/title/81280792")
if r.status == 200 and not r.body:find("Oh no!") then
    return true
end

local p = playwright([[
  async function check(page) {
    await page.goto('https://www.netflix.com/title/81280792')
    await page.waitForTimeout(3000)
    const text = await page.textContent('body')
    return !text.includes('Oh no!')
  }
]], { timeout = 30000 })

return p.result
```

## 数据库迁移

无需数据库变更。Playwright 作为 API 在现有规则引擎中使用，不新增规则类型。

## 部署

### Docker Compose 示例

```yaml
version: '3.8'

services:
  subs-check:
    image: subs-check:latest
    environment:
      - PLAYWRIGHT_URL=http://playwright:3000
    depends_on:
      - playwright

  playwright:
    build: ./playwright
    environment:
      - PORT=3000
      - MAX_CONCURRENT=5
      - DEFAULT_TIMEOUT=30000
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PLAYWRIGHT_URL` | Playwright 服务地址 | `http://playwright:3000` |
| `PORT` | Playwright 服务端口 | `3000` |
| `MAX_CONCURRENT` | 最大并发浏览器实例 | `5` |
| `DEFAULT_TIMEOUT` | 默认脚本超时（毫秒） | `30000` |

## 测试策略

1. **单元测试**：测试脚本编译、执行、超时、错误处理
2. **集成测试**：测试与 Go 服务的 HTTP API 集成
3. **端到端测试**：测试完整流程（创建规则 → 检测 → 查看结果）

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| Playwright 服务崩溃 | 健康检查 + 自动重启；Go 服务优雅降级（返回错误） |
| 浏览器内存泄漏 | 确保每个请求后关闭 browser；设置内存限制 |
| 脚本执行时间过长 | 强制超时（30s）；限制并发数 |
| 用户脚本恶意操作 | 不暴露敏感 API（fs、network）；隔离执行环境 |
| Docker 镜像过大 | 使用官方精简镜像；只安装 Chromium |

## 后续优化

1. **浏览器复用**：使用 browser pool 减少启动开销
2. **缓存**：对相同 URL + 代理的组合缓存结果
3. **截图存储**：支持将截图上传到对象存储（S3/MinIO）
4. **更多浏览器**：支持 Firefox、WebKit
5. **代理路由**：将 Clash 代理配置转换为 Playwright 可用的 HTTP 代理格式

## 决策记录

- **ADR-1**: 选择独立服务而非 Go 绑定
  - 原因：Playwright 原生是 Node.js，独立服务更稳定；Go 绑定（playwright-go）需要下载浏览器二进制，Docker 镜像更大
- **ADR-2**: Playwright 作为 API 而非独立规则类型
  - 原因：用户可以在同一规则中混合 HTTP 检测和浏览器检测；更灵活，避免规则类型爆炸
- **ADR-3**: 支持 TypeScript
  - 原因：提供更好的开发体验；类型提示减少错误
- **ADR-4**: Playwright 服务无认证
  - 原因：内部网络调用，通过 Docker 网络隔离；简化部署
- **ADR-5**: 代理通过现有节点选择器配置
  - 原因：复用现有 UI，无需在脚本中配置代理；所有规则统一使用 testNodeId 选择代理
