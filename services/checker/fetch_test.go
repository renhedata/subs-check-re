// services/checker/fetch_test.go
package checker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scriptedFetcher struct {
	calls   int
	results []error // error per call; nil = success
	out     []map[string]any
}

func (s *scriptedFetcher) Fetch(ctx context.Context, url string) ([]map[string]any, error) {
	idx := s.calls
	s.calls++
	if idx < len(s.results) && s.results[idx] != nil {
		return nil, s.results[idx]
	}
	return s.out, nil
}

func TestFetchWithRetryTransientThenSuccess(t *testing.T) {
	restore := fetchBackoff
	fetchBackoff = time.Millisecond
	defer func() { fetchBackoff = restore }()

	f := &scriptedFetcher{
		results: []error{errors.New("connection reset"), errors.New("timeout"), nil},
		out:     []map[string]any{{"name": "n1"}},
	}
	proxies, err := fetchWithRetry(context.Background(), f, "http://example.test")
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if f.calls != 3 || len(proxies) != 1 {
		t.Errorf("want 3 calls and 1 proxy, got %d calls, %d proxies", f.calls, len(proxies))
	}
}

func TestFetchWithRetryPermanentFailsFast(t *testing.T) {
	restore := fetchBackoff
	fetchBackoff = time.Millisecond
	defer func() { fetchBackoff = restore }()

	f := &scriptedFetcher{results: []error{permanent(errors.New("status 404"))}}
	if _, err := fetchWithRetry(context.Background(), f, "http://example.test"); err == nil {
		t.Fatal("expected error")
	}
	if f.calls != 1 {
		t.Errorf("permanent error must not retry; got %d calls", f.calls)
	}
}

func TestFetchWithRetryRespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &scriptedFetcher{results: []error{errors.New("transient")}}
	if _, err := fetchWithRetry(ctx, f, "http://example.test"); err == nil {
		t.Fatal("expected error with canceled context")
	}
	if f.calls > 1 {
		t.Errorf("canceled context must not retry; got %d calls", f.calls)
	}
}
