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

func TestCheckNode_AliveNodeRunsAllSelectedTests(t *testing.T) {
	rp, rs, ru, rg, rr := probeLatencyFn, measureSpeedFn, measureUploadFn, getProxyInfoFn, runRuleFn
	defer func() {
		probeLatencyFn, measureSpeedFn, measureUploadFn, getProxyInfoFn, runRuleFn = rp, rs, ru, rg, rr
	}()
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return true, 25 }
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int { return 1200 }
	measureUploadFn = func(context.Context, http.RoundTripper, string, string) int { return 600 }
	getProxyInfoFn = func(context.Context, *http.Client) (string, string) { return "1.2.3.4", "US" }
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		return PlatformOutcome{Unlocked: true, Status: "Yes"}, nil
	}

	mapping := map[string]any{
		"name": "t", "type": "ss", "server": "127.0.0.1", "port": 1,
		"cipher": "aes-128-gcm", "password": "x",
	}
	opts := CheckOptions{SpeedTest: true, UploadSpeedTest: true, MediaApps: []string{"netflix", "disney"}}
	rules := []*PlatformRule{{Key: "netflix", Enabled: true}, {Key: "disney", Enabled: true}}

	res := checkNode(context.Background(), "node-1", mapping, "", "", "", opts, rules, func(string) {})

	if !res.Alive {
		t.Fatal("alive node must be marked alive")
	}
	if res.SpeedKbps != 1200 {
		t.Errorf("want speed 1200, got %d", res.SpeedKbps)
	}
	if res.UploadSpeedKbps != 600 {
		t.Errorf("want upload 600, got %d", res.UploadSpeedKbps)
	}
	if len(res.Platforms) != 2 || !res.Platforms["netflix"].Unlocked || !res.Platforms["disney"].Unlocked {
		t.Errorf("alive node must have every selected platform tested, got %+v", res.Platforms)
	}
}

func TestCheckNode_DeadNodeSkipsSubTests(t *testing.T) {
	rp, rs, rr, rb := probeLatencyFn, measureSpeedFn, runRuleFn, aliveProbeBackoff
	defer func() { probeLatencyFn, measureSpeedFn, runRuleFn, aliveProbeBackoff = rp, rs, rr, rb }()
	aliveProbeBackoff = time.Millisecond
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return false, 0 }
	speedCalls := 0
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int { speedCalls++; return 0 }
	ruleCalls := 0
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		ruleCalls++
		return PlatformOutcome{}, nil
	}

	mapping := map[string]any{
		"name": "t", "type": "ss", "server": "127.0.0.1", "port": 1,
		"cipher": "aes-128-gcm", "password": "x",
	}
	opts := CheckOptions{SpeedTest: true, MediaApps: []string{"netflix"}}
	rules := []*PlatformRule{{Key: "netflix", Enabled: true}}

	res := checkNode(context.Background(), "node-1", mapping, "", "", "", opts, rules, func(string) {})
	if res.Alive {
		t.Fatal("want dead")
	}
	if speedCalls != 0 || ruleCalls != 0 {
		t.Errorf("dead node must skip sub-tests; speedCalls=%d ruleCalls=%d", speedCalls, ruleCalls)
	}
}
