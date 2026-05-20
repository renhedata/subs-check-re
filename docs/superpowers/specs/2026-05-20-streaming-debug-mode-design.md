# Streaming Unlock Check Debug Mode

Add a debug mode to the streaming/media unlock testing interface that lets users see every intermediate variable during platform detection, like setting breakpoints in a debugger.

## Motivation

Currently, platform unlock checks return only a boolean (true/false) per platform. When a check unexpectedly fails, users have no visibility into why — which HTTP request failed, what status code was returned, or which condition evaluation caused the failure. A debug mode makes the entire detection chain transparent.

## Architecture

```
Check (with debug:true)  →  SSE stream
  ├── progressUpdate (existing, unchanged)
  └── progressUpdate.debug (new, optional field)
       └── NodeDebug → DebugTrace[] → DebugStep[]
```

- Debug data lives only in memory (SSE stream), not persisted to DB
- Debug mode is opt-in per check run via a toggle in the UI
- The existing SSE stream carries debug data as an optional field; non-debug runs are unaffected

## Data Structures

### Backend (Go)

```go
type DebugStep struct {
    Type        string         `json:"type"`
    // "http_request" | "http_response" | "variable" | "condition" | "log" | "error"
    Description string         `json:"description"`
    Details     map[string]any `json:"details"`
}

type DebugTrace struct {
    Platform string      `json:"platform"`
    Result   bool        `json:"result"`
    Steps    []DebugStep `json:"steps"`
}

type NodeDebug struct {
    NodeID   string       `json:"node_id"`
    NodeName string       `json:"node_name"`
    Traces   []DebugTrace `json:"traces"`
}
```

### SSE Protocol

The existing `progressUpdate` struct gains an optional `Debug` field:

```go
type progressUpdate struct {
    Progress        int        `json:"progress"`
    Total           int        `json:"total"`
    NodeName        string     `json:"node_name,omitempty"`
    Alive           bool       `json:"alive"`
    LatencyMs       int        `json:"latency_ms,omitempty"`
    SpeedKbps       int        `json:"speed_kbps,omitempty"`
    UploadSpeedKbps int        `json:"upload_speed_kbps,omitempty"`
    Debug           *NodeDebug `json:"debug,omitempty"`
}
```

### Frontend (TypeScript)

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
```

## Backend Changes

### 1. `CheckOptions` — add `Debug` field

```go
type CheckOptions struct {
    SpeedTest       bool     `json:"speed_test"`
    UploadSpeedTest bool     `json:"upload_speed_test"`
    MediaApps       []string `json:"media_apps"`
    Debug           bool     `json:"debug"` // NEW
}
```

### 2. Platform check functions — collect debug traces

Each built-in check function (in `platform.go`) gets a debug collector parameter. Instead of returning just `bool`, they optionally populate a `*DebugTrace` with HTTP request/response details and condition evaluations.

Signature change pattern:
```go
// Before
func checkNetflix(client *http.Client) bool

// After
func checkNetflix(client *http.Client, trace *DebugTrace) bool
```

When `trace` is nil (non-debug mode), everything runs as before with zero overhead.

### 3. Rule engines — collect debug traces

Condition engine: evaluate each sub-condition and log the intermediate result.

Script engines (JS/TS/Tengo/Lua): inject a logger proxy that captures `console.log` / `tprint` output. The return value is always captured.

### 4. `nodeCheckResult` — add `Traces []DebugTrace`

The internal result struct carries debug traces alongside the boolean results.

### 5. SSE broadcast — include debug data

When a node check completes in debug mode, the collected `NodeDebug` is attached to the `progressUpdate` event.

## Frontend Changes

### 1. Check trigger — add Debug toggle

A switch/checkbox above the "Start Check" button:

```tsx
<CheckForm>
  <DebugToggle />  // NEW
  <StartButton />
</CheckForm>
```

### 2. SSEProgress type — add `debug` field

```typescript
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
  debug?: NodeDebug  // NEW
}
```

### 3. DebugPanel component

A new collapsible card rendered below the progress panel when a debug check is active. Shows a tree structure:

```
▶ Node-01 (3 platforms)
  ▶ netflix ✓
    ├ [HTTP] GET https://www.netflix.com/title/81280792
    ├ [HTTP] 200 OK
    ├ [COND] body does not contain "Oh no!" → true
    └ result = true
  ▶ youtube ✓
    └ ...
```

Each DebugStep type renders differently:
- `http_request` — URL, method, headers (collapsible key-value table)
- `http_response` — status code badge, response headers, body snippet (collapsible)
- `variable` — monospace `name = value`
- `condition` — expression with ✓/✗ result indicator
- `log` — gray monospace console output
- `error` — red error message

**Component tree:**
```
DebugPanel (Collapsible + ScrollArea)
  ├── DebugNodeEntry (Collapsible, per node)
  │   ├── DebugPlatformEntry (Collapsible, per platform)
  │   │   ├── DebugStepView
  │   │   └── ...
  │   └── ...
  └── ...
```

Uses shadcn/ui components: `Collapsible`, `Badge`, `ScrollArea`, `Separator`.

## Edge Cases

- **Large response bodies**: body_snippet truncated to 2KB
- **Many nodes**: Debug Panel uses virtual scroll (ScrollArea) to avoid rendering thousands of DOM nodes
- **Script errors**: captured as `error` type step with the error message
- **No debug data**: panel shows "No debug data collected" empty state
- **Page refresh**: debug state is not persisted (no DB), naturally clears on refresh

## Non-Goals

- Persisting debug data to PostgreSQL (memory-only, SSE-delivered)
- Step-through debugging (pause/resume/continue) — this design is read-only trace, not interactive breakpoints
- Modifying rules from the debug panel
