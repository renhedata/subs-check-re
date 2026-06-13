package checker

import (
	"context"
	"net/http"
	"testing"
)

// roundTripFunc lets tests serve canned responses for any URL a rule requests.
type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

// mockClient returns an *http.Client whose responses are keyed by exact request URL.
// Unmatched URLs return 404 with an empty body.
func mockClient(byURL map[string]mockResp) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) *http.Response {
		m, ok := byURL[r.URL.String()]
		if !ok {
			return &http.Response{StatusCode: 404, Body: http.NoBody, Request: r, Header: http.Header{}}
		}
		return m.toResponse(r)
	})}
}

func TestRunRule_ConditionNormalizesToOutcome(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"https://example.com/": {status: 200, body: "hello world"},
	})
	rule := &PlatformRule{
		RuleType:   "condition",
		Key:        "demo",
		Enabled:    true,
		Definition: []byte(`{"url":"https://example.com/","status_code":200,"body_contains":["hello"]}`),
	}
	out, err := runRule(context.Background(), client, rule, nil)
	if err != nil {
		t.Fatalf("runRule error: %v", err)
	}
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v, want Unlocked=true Status=Yes", out)
	}
}
