package notify

import (
	"strings"
	"testing"
)

func TestFormatCheckReport_IteratesPlatformMap(t *testing.T) {
	r := &JobReport{
		SubscriptionName: "Sub",
		Available:        2, Total: 3,
		Platforms: PlatformCounts{"netflix": 2, "spotify": 1},
	}
	out := formatCheckReport(r)
	if !strings.Contains(out, "Netflix: 2") {
		t.Fatalf("missing netflix line: %s", out)
	}
	if !strings.Contains(out, "Spotify: 1") {
		t.Fatalf("missing spotify line: %s", out)
	}
}
