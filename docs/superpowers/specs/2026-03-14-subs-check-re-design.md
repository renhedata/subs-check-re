# subs-check-re 设计文档

**日期：** 2026-03-14
**状态：** 草稿

---

## 概述

`subs-check-re` 是对 [subs-check](https://github.com/beck-8/subs-check) 的重写版本。原项目是一个代理节点订阅检测工具，本项目将其升级为一个支持多用户的完整 Web 平台，后端使用 Go + Encore 框架，前端使用 React（better-t-stack）。

---

## 目标

- 支持多用户管理各自的代理订阅链接
- 自动/手动检测节点可用性、延迟、测速、流媒体解锁
- 定时任务（cron）周期性自动检测
- 多渠道通知告警
- Web 控制面板实时查看检测进度和结果

---

## 技术栈

| 层 | 技术 |
|---|---|
| 后端框架 | [Encore](https://encore.dev) (Go) |
| 代理引擎 | `github.com/metacubex/mihomo` |
| 数据库 | PostgreSQL（Encore 托管） |
| 定时任务 | `robfig/cron/v3` |
| 前端框架 | React 19 + Vite |
| 路由 | TanStack Router（文件路由） |
| 数据请求 | TanStack Query |
| UI 组件 | shadcn/ui |
| 前端工具链 | Bun + Biome + Turborepo |

---

## 项目结构

```
subs-check-re/
├── encore.app                   # Encore 项目配置
├── go.mod
├── services/
│   ├── auth/                    # 认证服务
│   │   ├── auth.go              # API：注册、登录、获取用户信息
│   │   └── migrations/          # DB migration
│   ├── subscription/            # 订阅管理服务
│   │   ├── subscription.go      # API：CRUD + 节点列表
│   │   └── migrations/
│   ├── checker/                 # 节点检测服务
│   │   ├── checker.go           # API：触发检测、SSE 进度、历史结果
│   │   ├── mihomo.go            # mihomo 代理引擎封装
│   │   ├── platform/            # 流媒体解锁检测（复用参考项目）
│   │   └── migrations/
│   ├── scheduler/               # 定时任务服务
│   │   ├── scheduler.go         # API：CRUD cron 任务
│   │   └── migrations/
│   └── notify/                  # 通知服务
│       ├── notify.go            # API：配置通知渠道
│       └── migrations/
├── frontend/                    # better-t-stack React 应用
│   ├── apps/web/
│   │   ├── src/
│   │   │   ├── routes/          # TanStack Router 页面
│   │   │   ├── components/      # 业务组件
│   │   │   └── lib/
│   │   │       ├── api.ts       # fetch 封装（带 JWT header）
│   │   │       └── auth.ts      # token 存储
│   │   └── vite.config.ts       # 代理 /api → Encore :4000
│   └── packages/ui/             # shadcn/ui 共享组件
├── .mcp.json                    # MCP 配置（encore、context7、shadcn）
└── Makefile                     # 快捷命令
```

---

## 架构与数据流

```
Browser
  └─ TanStack Query
       └─ fetch /api/*
            └─ Encore REST API (:4000)
                 ├─ auth service      → PostgreSQL (users)
                 ├─ subscription svc  → PostgreSQL (subscriptions, nodes)
                 ├─ checker service   → mihomo 引擎 → 节点检测
                 │                    → SSE 进度推送 → Browser
                 │                    → Encore PubSub → notify service
                 ├─ scheduler service → cron → checker service
                 └─ notify service    → webhook / email / telegram
```

**本地开发：**
- `encore run` 启动后端（:4000），自动管理 PostgreSQL
- `bun dev` 启动前端（:3001），Vite 代理 `/api` 到 `:4000`

---

## API 设计

### auth 服务

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /auth/register | 注册（username + password） |
| POST | /auth/login | 登录，返回 JWT |
| GET  | /auth/me | 获取当前用户（需 JWT） |

### subscription 服务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET    | /subscriptions | 列出当前用户的订阅 |
| POST   | /subscriptions | 添加订阅 |
| PUT    | /subscriptions/:id | 更新订阅 |
| DELETE | /subscriptions/:id | 删除订阅 |
| GET    | /subscriptions/:id/nodes | 节点列表 + 最新检测结果 |

### checker 服务

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /check/:subscriptionId | 触发检测任务 |
| GET  | /check/:jobId/progress | SSE 实时进度流 |
| GET  | /check/:subscriptionId/results | 历史检测结果 |

### scheduler 服务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET    | /scheduler | 列出定时任务 |
| POST   | /scheduler | 创建定时任务（cron 表达式 + 订阅 ID） |
| DELETE | /scheduler/:id | 删除任务 |

### notify 服务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET    | /notify/channels | 列出通知渠道 |
| POST   | /notify/channels | 配置渠道（webhook/email/telegram） |
| DELETE | /notify/channels/:id | 删除渠道 |

---

## 数据库 Schema

```sql
-- 用户
users (
  id            UUID PRIMARY KEY,
  username      TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ
)

-- 订阅链接
subscriptions (
  id          UUID PRIMARY KEY,
  user_id     UUID → users.id,
  name        TEXT,
  url         TEXT NOT NULL,
  enabled     BOOL DEFAULT true,
  cron_expr   TEXT,            -- 定时任务 cron 表达式，NULL 表示不定时
  created_at  TIMESTAMPTZ,
  last_run_at TIMESTAMPTZ
)

-- 节点（每次检测任务开始前全量替换，按 subscription_id 删除后重新插入）
nodes (
  id              UUID PRIMARY KEY,
  subscription_id UUID → subscriptions.id,
  name            TEXT,
  type            TEXT,        -- vmess/vless/ss/trojan/hysteria2...
  server          TEXT,
  port            INT,
  config          JSONB        -- 完整 mihomo 配置（原始 map[string]any）
)

-- 检测任务
check_jobs (
  id              UUID PRIMARY KEY,
  subscription_id UUID → subscriptions.id,
  user_id         UUID → users.id,
  status          TEXT,        -- queued / running / completed / failed
  total           INT,
  progress        INT DEFAULT 0,
  created_at      TIMESTAMPTZ,
  finished_at     TIMESTAMPTZ
)

-- 检测结果（追加，保留历史，按 job 分组）
check_results (
  id          UUID PRIMARY KEY,
  job_id      UUID → check_jobs.id,
  node_id     UUID → nodes.id,
  checked_at  TIMESTAMPTZ,
  alive       BOOL,
  latency_ms  INT,
  speed_kbps  INT,
  country     TEXT,
  ip          TEXT,
  openai      BOOL,
  netflix     BOOL,
  youtube     TEXT,
  disney      BOOL,
  claude      BOOL,
  gemini      BOOL,
  tiktok      TEXT
)

-- 通知渠道
notify_channels (
  id      UUID PRIMARY KEY,
  user_id UUID → users.id,
  name    TEXT,
  type    TEXT,               -- webhook / telegram / email
  config  JSONB,              -- 见下方各类型说明
  enabled BOOL
)
```

**notify_channels.config 结构示例：**
```json
// type=webhook
{ "url": "https://...", "method": "POST", "headers": {} }

// type=telegram
{ "bot_token": "...", "chat_id": "..." }

// type=email
{ "smtp_host": "...", "smtp_port": 465, "username": "...", "password": "...", "to": "..." }
```

**通知触发条件：** 每次检测任务完成后发送，payload 包含可用节点数量和变化情况。

---

## 前端页面

| 路由 | 页面 | 主要功能 |
|------|------|---------|
| `/login` | 登录/注册 | 表单认证，写入 JWT 到 localStorage |
| `/` | 仪表盘 | 订阅数、可用节点数、最近检测时间 |
| `/subscriptions` | 订阅列表 | 添加/删除订阅，触发检测 |
| `/subscriptions/$id` | 订阅详情 | SSE 进度条 + 节点结果表格，支持历史 job 切换 |
| `/scheduler` | 定时任务 | 查看/删除已绑定 cron 的订阅 |
| `/settings/notify` | 通知渠道 | 添加/删除 webhook/telegram/email 渠道 |

**核心组件：**
- `NodeTable` — 结果表格，支持按国家/流媒体解锁筛选排序，含分页
- `CheckProgress` — 消费 SSE 的实时进度条，断线自动重连
- `CountryFlag` — 国家旗帜标识

**TanStack Query 策略：**
- `staleTime: 30s`（节点结果），`staleTime: 0`（check job 进度）
- 错误时最多重试 2 次，展示 error toast

---

## 核心实现说明

### 节点生命周期

- **创建/更新订阅时**：不立即拉取节点，仅保存 URL
- **触发检测任务时**：
  1. 从订阅 URL 拉取内容，解析为 mihomo 节点格式
  2. 按 `subscription_id` 全量替换 `nodes` 表（DELETE + INSERT）
  3. 若 URL 不可达或内容解析失败，任务标记为 `failed`，保留上次节点数据

### 检测任务流程

1. `POST /check/:subscriptionId` → 创建 `check_jobs` 记录（status=queued），返回 `{ job_id }`
2. 异步 goroutine 开始执行：拉取节点 → 并发检测（默认并发数：20）
3. 每个节点检测完成后：更新 `jobs.progress`，推送 SSE 事件
4. 全部完成后：status=completed，通过 Encore PubSub 发布事件
5. **并发限制**：同一订阅同时只允许一个运行中的任务（status=running 时拒绝新建，返回 409）
6. **单节点超时**：15 秒

**Job 状态机：**

```
queued → running → completed
                 → failed
```

- `queued`：任务已创建，等待执行（通常极短暂）
- `running`：正在拉取节点或检测中
- `completed`：所有节点检测完成（含部分失败的节点，任务本身算完成）
- `failed`：订阅 URL 拉取失败或解析失败，节点未更新

SSE 客户端收到 `{"done":true}` 或轮询到 status=completed/failed 时停止监听。

### 节点检测（mihomo）

复用参考项目核心逻辑，以 Go 库方式使用（非子进程）：
- `mihomo/adapter` 创建代理客户端
- 并发 goroutine 池，每个 goroutine 处理一个节点
- 平台检测：Netflix、YouTube、OpenAI、Claude、Gemini、Disney、TikTok
- 速度测试：通过代理 HTTP 下载测速

### 实时进度（SSE）

1. `POST /check/:subscriptionId` 返回 `{ job_id }`
2. 前端连接 `GET /check/:jobId/progress`（SSE）
3. 推送格式：`data: {"progress":5,"total":100,"node_name":"..."}`
4. 完成时推送：`data: {"done":true,"available":82}`
5. SSE 断开重连时，前端可用 job_id 重新订阅，后端从当前 progress 继续推送

### 认证（JWT）

- Encore `encore.dev/beta/auth` 实现 JWT 中间件
- JWT claims：`{ sub: user_id, exp: now+24h }`
- 所有非 `/auth/*` 接口强制验证，多租户隔离在 service 层通过 `user_id` 过滤

### 错误响应格式

所有错误统一返回：
```json
{ "error": "human readable message", "code": "ERROR_CODE" }
```

### 定时任务（scheduler 服务）

- `subscriptions.cron_expr` 非 NULL 时该订阅参与定时检测
- 使用 `robfig/cron/v3` 管理，**服务启动时从数据库全量加载** `enabled=true AND cron_expr IS NOT NULL` 的订阅并注册 cron
- 任务触发时调用 checker 服务触发检测
- 更新/删除订阅 cron_expr 时同步更新内存中的 cron 注册（先 Remove 旧 entry，再 AddFunc 新 entry）
- 进程重启后 cron 状态从数据库恢复，不依赖内存持久化

### 分页与数据保留

- `GET /check/:subscriptionId/results` 默认返回最新一次 job 的结果，可通过 `?job_id=` 查询历史
- `check_results` 无自动清理策略（MVP 阶段保留全部历史）

---

## 非目标（本版本不包含）

- OAuth 登录
- 节点导出/订阅转换（Sub-Store 集成）
- 移动端适配
- Docker 一键部署脚本

---

## 参考

- 原项目：`/Users/ashark/tmp/subs-check`
- Encore 文档：https://encore.dev/docs/go
- better-t-stack：https://better-t-stack.dev
