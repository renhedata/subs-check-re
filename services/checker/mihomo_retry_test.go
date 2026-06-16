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
