package checker

import (
	"testing"

	settingssvc "subs-check-re/services/settings"
)

func TestTaggedName_FromPlatformsMap(t *testing.T) {
	cfg := settingssvc.DefaultExportTags() // country off, speed on, builtin labels
	platforms := map[string]PlatformOutcome{
		"netflix":         {Unlocked: true, Status: "Yes", Region: "US"},
		"youtube":         {Unlocked: true, Status: "Yes"},
		"youtube_premium": {Unlocked: true, Status: "Yes"},
		"spotify":         {Unlocked: true, Status: "Yes"}, // custom (not in default cfg) → defaults to enabled, label=key
	}
	got := taggedName("HK-01", "HK", platforms, 2048, cfg)
	// country off; NF present; YT premium → "YT+"; spotify appended; speed 2048kbps → 2.0MB
	want := "HK-01|NF|YT+|spotify|2.0MB"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
