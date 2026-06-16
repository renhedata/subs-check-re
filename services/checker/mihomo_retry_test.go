package checker

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestProbeLatencyWithRetry_RecoversAfterTransientFailures(t *testing.T) {
	rp, rb := probeLatencyFn, aliveProbeBackoff
	defer func() { probeLatencyFn, aliveProbeBackoff = rp, rb }()
	aliveProbeBackoff = time.Millisecond

	calls := 0
	results := []bool{false, false, true}
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) {
		i := calls
		calls++
		return results[i], 42
	}

	alive, ms := probeLatencyWithRetry(context.Background(), nil, "")
	if !alive {
		t.Fatal("expected alive after retries")
	}
	if ms != 42 {
		t.Errorf("want ms 42, got %d", ms)
	}
	if calls != 3 {
		t.Errorf("want 3 attempts, got %d", calls)
	}
}

func TestProbeLatencyWithRetry_SuccessFirstTryNoRetry(t *testing.T) {
	rp := probeLatencyFn
	defer func() { probeLatencyFn = rp }()
	calls := 0
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) {
		calls++
		return true, 10
	}
	alive, _ := probeLatencyWithRetry(context.Background(), nil, "")
	if !alive || calls != 1 {
		t.Errorf("want alive in 1 call, got alive=%v calls=%d", alive, calls)
	}
}

func TestProbeLatencyWithRetry_AllFailMarksDead(t *testing.T) {
	rp, rb := probeLatencyFn, aliveProbeBackoff
	defer func() { probeLatencyFn, aliveProbeBackoff = rp, rb }()
	aliveProbeBackoff = time.Millisecond
	calls := 0
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) {
		calls++
		return false, 0
	}
	alive, _ := probeLatencyWithRetry(context.Background(), nil, "")
	if alive {
		t.Error("want dead")
	}
	if calls != aliveProbeAttempts {
		t.Errorf("want %d attempts, got %d", aliveProbeAttempts, calls)
	}
}

func TestProbeLatencyWithRetry_CanceledContextStopsEarly(t *testing.T) {
	rp := probeLatencyFn
	defer func() { probeLatencyFn = rp }()
	calls := 0
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) {
		calls++
		return false, 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	alive, _ := probeLatencyWithRetry(ctx, nil, "")
	if alive {
		t.Error("want dead on canceled ctx")
	}
	if calls > 1 {
		t.Errorf("canceled ctx must not retry; got %d calls", calls)
	}
}

func TestMeasureSpeedWithRetry_RetriesOnZero(t *testing.T) {
	rs := measureSpeedFn
	defer func() { measureSpeedFn = rs }()
	calls := 0
	vals := []int{0, 1500}
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int {
		i := calls
		calls++
		return vals[i]
	}
	got := measureSpeedWithRetry(context.Background(), nil, "")
	if got != 1500 || calls != 2 {
		t.Errorf("want 1500 in 2 calls, got %d in %d", got, calls)
	}
}

func TestMeasureSpeedWithRetry_NonZeroFirstNoRetry(t *testing.T) {
	rs := measureSpeedFn
	defer func() { measureSpeedFn = rs }()
	calls := 0
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int {
		calls++
		return 900
	}
	got := measureSpeedWithRetry(context.Background(), nil, "")
	if got != 900 || calls != 1 {
		t.Errorf("want 900 in 1 call, got %d in %d", got, calls)
	}
}

func TestMeasureSpeedWithRetry_AllZeroCapped(t *testing.T) {
	rs := measureSpeedFn
	defer func() { measureSpeedFn = rs }()
	calls := 0
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int {
		calls++
		return 0
	}
	got := measureSpeedWithRetry(context.Background(), nil, "")
	if got != 0 || calls != speedTestAttempts {
		t.Errorf("want 0 in %d calls, got %d in %d", speedTestAttempts, got, calls)
	}
}

func TestMeasureUploadWithRetry_RetriesOnZero(t *testing.T) {
	ru := measureUploadFn
	defer func() { measureUploadFn = ru }()
	calls := 0
	vals := []int{0, 800}
	measureUploadFn = func(context.Context, http.RoundTripper, string, string) int {
		i := calls
		calls++
		return vals[i]
	}
	got := measureUploadWithRetry(context.Background(), nil, "", "")
	if got != 800 || calls != 2 {
		t.Errorf("want 800 in 2 calls, got %d in %d", got, calls)
	}
}
