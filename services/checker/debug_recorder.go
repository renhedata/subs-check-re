package checker

import (
	"encoding/json"
	"fmt"
	"sync"
)

type DebugRecorder struct {
	mu    sync.Mutex
	Steps []DebugStep
}

func (d *DebugRecorder) Add(s DebugStep) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.Steps = append(d.Steps, s)
	d.mu.Unlock()
}

func toRawMessage(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// HTTPReq records an outgoing HTTP request. body is the request body string
// (empty for GET/HEAD); pass an empty string when there is no body.
func (d *DebugRecorder) HTTPReq(method, url string, headers map[string]string, body string) {
	d.Add(DebugStep{
		Type:        "http_request",
		Description: method + " " + url,
		Details: toRawMessage(map[string]any{
			"method":  method,
			"url":     url,
			"headers": headers,
			"body":    body,
		}),
	})
}

// HTTPResp records a response with the FULL body (no truncation). The console
// panel displays / lets users search this. durationMs is round-trip latency in ms.
// finalURL is the URL after redirects.
func (d *DebugRecorder) HTTPResp(code int, headers map[string]string, body string, durationMs int64, finalURL string) {
	d.Add(DebugStep{
		Type:        "http_response",
		Description: fmt.Sprintf("HTTP %d  ·  %d ms  ·  %s", code, durationMs, humanBytes(len(body))),
		Details: toRawMessage(map[string]any{
			"status_code": code,
			"headers":     headers,
			"body":        body,
			"duration_ms": durationMs,
			"final_url":   finalURL,
			"size_bytes":  len(body),
		}),
	})
}

func humanBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func (d *DebugRecorder) Variable(name string, value any) {
	d.Add(DebugStep{
		Type:        "variable",
		Description: name + " = " + fmt.Sprintf("%v", value),
		Details:     toRawMessage(map[string]any{"name": name, "value": value}),
	})
}

func (d *DebugRecorder) Condition(expression string, matched bool) {
	d.Add(DebugStep{
		Type:        "condition",
		Description: expression + " → " + fmt.Sprintf("%v", matched),
		Details:     toRawMessage(map[string]any{"expression": expression, "matched": matched}),
	})
}

func (d *DebugRecorder) Log(msg string) {
	d.Add(DebugStep{
		Type:        "log",
		Description: msg,
		Details:     toRawMessage(map[string]any{"output": msg}),
	})
}

func (d *DebugRecorder) Error(err error) {
	d.Add(DebugStep{
		Type:        "error",
		Description: err.Error(),
		Details:     toRawMessage(map[string]any{"error": err.Error()}),
	})
}
