package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// evaluateRuleForNode runs a single rule definition against an HTTP client (optionally
// routed through a user-owned proxy node) and returns the test result with debug trace.
// Used by both the TestRule API endpoint and any in-process rule evaluation
// (CLI tools, batch validators, etc.).
func evaluateRuleForNode(ctx context.Context, userID, ruleType string, definition json.RawMessage, nodeID string) (*TestRuleResult, error) {
	httpClient, nodeName, cleanup := openTestClient(ctx, userID, nodeID)
	if cleanup != nil {
		defer cleanup()
	}

	start := time.Now()
	dr := &DebugRecorder{}

	if ruleType == "condition" {
		return runConditionTest(ctx, httpClient, ruleType, definition, dr, start, nodeName), nil
	}

	rule := &PlatformRule{RuleType: ruleType, Definition: definition}
	ok, err := runRule(ctx, httpClient, rule, dr)
	ms := time.Since(start).Milliseconds()

	trace := &DebugTrace{Platform: ruleType, Result: ok, Steps: dr.Steps}

	// Script engines (JS/TS/Tengo/Lua) may make multiple http_get calls; surface the
	// LAST response as the top-level Body/StatusCode/FinalURL/ResponseHeaders so the
	// Body and Rendered tabs in the UI show something useful. Trace still has every step.
	statusCode, finalURL, body, respHeaders := extractConditionArtifacts(dr.Steps)

	if err != nil {
		return &TestRuleResult{
			OK: false, Error: err.Error(), DurationMs: ms,
			NodeName:        nodeName,
			Trace:           trace,
			StatusCode:      statusCode,
			FinalURL:        finalURL,
			Body:            body,
			ResponseHeaders: respHeaders,
		}, nil
	}
	return &TestRuleResult{
		OK: ok, DurationMs: ms,
		NodeName:        nodeName,
		Trace:           trace,
		StatusCode:      statusCode,
		FinalURL:        finalURL,
		Body:            body,
		ResponseHeaders: respHeaders,
	}, nil
}

// openTestClient returns an http.Client routed through the given user-owned node, or
// a plain default client if no nodeID is provided / the node is not accessible.
// The returned cleanup closes the underlying proxy client (nil if no proxy was opened).
func openTestClient(ctx context.Context, userID, nodeID string) (*http.Client, string, func()) {
	if nodeID == "" {
		return &http.Client{Timeout: 15 * time.Second}, "", nil
	}

	var name string
	var configJSON []byte
	err := db.QueryRow(ctx, `
		SELECT n.name, n.config FROM nodes n
		WHERE n.id = $1
		  AND n.subscription_id IN (
		    SELECT DISTINCT subscription_id FROM check_jobs WHERE user_id = $2
		  )
	`, nodeID, userID).Scan(&name, &configJSON)
	if err != nil || len(configJSON) == 0 {
		return &http.Client{Timeout: 15 * time.Second}, "", nil
	}

	var mapping map[string]any
	if err := json.Unmarshal(configJSON, &mapping); err != nil {
		return &http.Client{Timeout: 15 * time.Second}, "", nil
	}
	pc := newProxyClient(mapping)
	if pc == nil {
		return &http.Client{Timeout: 15 * time.Second}, "", nil
	}
	return pc.Client, name, func() { pc.close() }
}

func runConditionTest(ctx context.Context, client *http.Client, ruleType string, def json.RawMessage, dr *DebugRecorder, start time.Time, nodeName string) *TestRuleResult {
	ok, err := runConditionRule(ctx, client, def, dr)
	ms := time.Since(start).Milliseconds()
	trace := &DebugTrace{Platform: ruleType, Result: ok, Steps: dr.Steps}

	if err != nil {
		trace.Result = false
		return &TestRuleResult{OK: false, Error: err.Error(), DurationMs: ms, NodeName: nodeName, Trace: trace}
	}

	statusCode, finalURL, body, respHeaders := extractConditionArtifacts(dr.Steps)
	return &TestRuleResult{
		OK:              ok,
		StatusCode:      statusCode,
		FinalURL:        finalURL,
		Body:            body,
		ResponseHeaders: respHeaders,
		NodeName:        nodeName,
		DurationMs:      ms,
		Trace:           trace,
	}
}

// extractConditionArtifacts pulls the last HTTP response status/url/body/headers out of debug
// steps so the test UI can render them as a structured response panel. The full body lives in
// the response step's "body" field (no truncation).
func extractConditionArtifacts(steps []DebugStep) (statusCode int, finalURL, body string, respHeaders map[string]string) {
	for _, step := range steps {
		if step.Type == "http_response" && len(step.Details) > 0 {
			var details map[string]any
			if json.Unmarshal(step.Details, &details) != nil {
				continue
			}
			if v, ok := details["status_code"].(float64); ok {
				statusCode = int(v)
			}
			if v, ok := details["body"].(string); ok {
				body = v
			}
			if v, ok := details["final_url"].(string); ok && v != "" {
				finalURL = v
			}
			if h, ok := details["headers"].(map[string]any); ok {
				respHeaders = make(map[string]string, len(h))
				for k, val := range h {
					if sv, ok := val.(string); ok {
						respHeaders[k] = sv
					}
				}
			}
		}
	}
	return
}

