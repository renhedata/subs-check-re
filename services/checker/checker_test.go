// services/checker/checker_test.go
package checker

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestTriggerCheckMissingSubscription(t *testing.T) {
	ctx := withAuth()
	_, err := TriggerCheck(ctx, "nonexistent-sub-id", nil)
	if err == nil {
		t.Error("expected error for missing subscription")
	}
}

func TestGetResultsNoJobs(t *testing.T) {
	ctx := withAuth()
	_, err := GetResults(ctx, "nonexistent-sub-id", nil)
	if err == nil {
		t.Error("expected error when no jobs exist")
	}
}

func TestListJobsEmpty(t *testing.T) {
	ctx := withAuth()
	resp, err := ListJobs(ctx, "nonexistent-sub-id", &ListJobsParams{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestGetResultsWithJobIDNotFound(t *testing.T) {
	ctx := withAuth()
	_, err := GetResults(ctx, "nonexistent-sub-id", &GetResultsParams{JobID: "nonexistent-job"})
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestParseProxies(t *testing.T) {
	// Clash YAML format
	yaml := []byte(`
proxies:
  - name: "test-node"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: chacha20-ietf-poly1305
    password: "testpass"
`)
	proxies, err := parseProxies(yaml)
	if err != nil {
		t.Fatalf("parseProxies failed: %v", err)
	}
	if len(proxies) != 1 {
		t.Errorf("expected 1 proxy, got %d", len(proxies))
	}
	if proxies[0]["name"] != "test-node" {
		t.Errorf("expected name 'test-node', got %v", proxies[0]["name"])
	}
}

func TestDefaultCheckOptionsHasAllPlatforms(t *testing.T) {
	opts := defaultCheckOptions()
	if !opts.SpeedTest {
		t.Error("expected SpeedTest=true by default")
	}
	wantPlatforms := []string{"openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"}
	for _, p := range wantPlatforms {
		found := false
		for _, m := range opts.MediaApps {
			if m == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected platform %q in default MediaApps", p)
		}
	}
}

func TestCountingTransport(t *testing.T) {
	body := "hello world" // 11 bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	ct := &countingTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: ct}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()

	got := atomic.LoadInt64(&ct.bytes)
	if got != int64(len(body)) {
		t.Errorf("want %d bytes, got %d", len(body), got)
	}
}

func TestMeasureSpeedPartialDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 512))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(60 * time.Second)
	}))
	defer srv.Close()

	ctx := context.Background()
	speed := measureSpeedWithTimeout(ctx, http.DefaultTransport, srv.URL, 300*time.Millisecond)
	if speed == 0 {
		t.Error("expected non-zero speed for partial download")
	}
}
