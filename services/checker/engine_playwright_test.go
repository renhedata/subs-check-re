package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
