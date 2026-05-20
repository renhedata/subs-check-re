package checker

import (
	"fmt"
	"strconv"
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

func (d *DebugRecorder) HTTPReq(url, method string, headers map[string]string) {
	d.Add(DebugStep{
		Type:        "http_request",
		Description: method + " " + url,
		Details:     map[string]any{"url": url, "method": method, "headers": headers},
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
		Details:     map[string]any{"status_code": code, "headers": headers, "body_snippet": snippet},
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
