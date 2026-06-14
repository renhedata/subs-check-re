package checker

import (
	"context"
	"testing"
)

func TestProbeLatency_AliveAndMs(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"http://cp.cloudflare.com/generate_204": {status: 204},
	})
	alive, ms := probeLatency(context.Background(), client, "")
	if !alive {
		t.Fatalf("expected alive")
	}
	if ms < 0 {
		t.Fatalf("expected non-negative ms, got %d", ms)
	}
}

func TestProbeLatency_DeadOn5xx(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"http://cp.cloudflare.com/generate_204": {status: 502},
	})
	alive, _ := probeLatency(context.Background(), client, "")
	if alive {
		t.Fatalf("expected dead on 502")
	}
}
