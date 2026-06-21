package checker

import (
	"context"
	"net/http"
	"testing"
)

func TestCheckNodeEmitsPhasesInOrder(t *testing.T) {
	origNPC := newProxyClientFn
	newProxyClientFn = func(map[string]any) *proxyClient {
		return &proxyClient{Client: &http.Client{Transport: http.DefaultTransport}}
	}
	defer func() { newProxyClientFn = origNPC }()

	origProbe := probeLatencyFn
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return true, 12 }
	defer func() { probeLatencyFn = origProbe }()

	origSpeed := measureSpeedFn
	measureSpeedFn = func(context.Context, http.RoundTripper, string) int { return 100 }
	defer func() { measureSpeedFn = origSpeed }()

	origUpload := measureUploadFn
	measureUploadFn = func(context.Context, http.RoundTripper, string, string) int { return 50 }
	defer func() { measureUploadFn = origUpload }()

	origInfo := getProxyInfoFn
	getProxyInfoFn = func(context.Context, *http.Client) (string, string) { return "1.2.3.4", "US" }
	defer func() { getProxyInfoFn = origInfo }()

	var phases []string
	emit := func(p string) { phases = append(phases, p) }

	opts := CheckOptions{SpeedTest: true, UploadSpeedTest: true, MediaApps: []string{"netflix"}}
	checkNode(context.Background(), "n1", map[string]any{"name": "x"}, "", "", "", opts, nil, emit)

	want := []string{phaseLatency, phaseSpeed, phaseUpload, phaseRegion, phaseStreaming}
	if len(phases) != len(want) {
		t.Fatalf("phases = %v, want %v", phases, want)
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Fatalf("phase[%d] = %q, want %q (full %v)", i, phases[i], want[i], phases)
		}
	}
}

func TestCheckNodeDeadEmitsOnlyLatency(t *testing.T) {
	origNPC := newProxyClientFn
	newProxyClientFn = func(map[string]any) *proxyClient {
		return &proxyClient{Client: &http.Client{Transport: http.DefaultTransport}}
	}
	defer func() { newProxyClientFn = origNPC }()

	origProbe := probeLatencyFn
	probeLatencyFn = func(context.Context, *http.Client, string) (bool, int) { return false, 0 }
	defer func() { probeLatencyFn = origProbe }()

	origBackoff := aliveProbeBackoff
	aliveProbeBackoff = 0 // don't sleep between the 3 dead-probe attempts
	defer func() { aliveProbeBackoff = origBackoff }()

	var phases []string
	emit := func(p string) { phases = append(phases, p) }

	opts := CheckOptions{SpeedTest: true, MediaApps: []string{"netflix"}}
	checkNode(context.Background(), "n1", map[string]any{"name": "x"}, "", "", "", opts, nil, emit)

	if len(phases) != 1 || phases[0] != phaseLatency {
		t.Fatalf("phases = %v, want [%s]", phases, phaseLatency)
	}
}
