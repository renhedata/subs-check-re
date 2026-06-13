// services/checker/export_test.go
package checker

import (
	"testing"

	settingssvc "subs-check-re/services/settings"
)

func TestTaggedNameDefaultConfigFullOrder(t *testing.T) {
	// All built-ins unlocked under the real defaults must reproduce the exact
	// legacy export name (order + labels), with premium rendering YT+.
	flags := unlockFlags{
		Netflix: true, YouTube: true, YouTubePremium: true, OpenAI: true,
		Claude: true, Gemini: true, Grok: true, Disney: true, TikTok: true,
	}
	got := taggedName("N", "HK", flags, nil, 1536, settingssvc.DefaultExportTags())
	if got != "N|NF|GPT|GM|CL|GK|YT+|D+|TK|1.5MB" {
		t.Errorf("default full order: got %q", got)
	}
}

func TestTaggedNamePremiumOnlyEmitsTag(t *testing.T) {
	// A node flagged premium but not basic youtube must still emit the tag
	// (the two unlock checks hit different URLs and can disagree).
	flags := unlockFlags{YouTubePremium: true}
	got := taggedName("N", "", flags, nil, 0, settingssvc.DefaultExportTags())
	if got != "N|YT+" {
		t.Errorf("premium-only: got %q", got)
	}
}

func legacyCfg() settingssvc.ExportTagConfig {
	return settingssvc.ExportTagConfig{
		ShowCountry: false,
		ShowSpeed:   true,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "NF", Enabled: true},
			{Key: "openai", Label: "GPT", Enabled: true},
			{Key: "youtube", Label: "YT", Enabled: true},
		},
	}
}

func TestTaggedNameLegacyDefault(t *testing.T) {
	flags := unlockFlags{Netflix: true, OpenAI: true}
	got := taggedName("HK-01", "HK", flags, nil, 1536, legacyCfg())
	if got != "HK-01|NF|GPT|1.5MB" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCountryAndPremiumAndDisabled(t *testing.T) {
	cfg := legacyCfg()
	cfg.ShowCountry = true
	cfg.Platforms[1].Enabled = false // disable openai
	flags := unlockFlags{Netflix: true, OpenAI: true, YouTube: true, YouTubePremium: true}
	got := taggedName("JP-1", "JP", flags, nil, 0, cfg)
	if got != "JP-1|JP|NF|YT+" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCustomLabelAndExtraSorted(t *testing.T) {
	cfg := settingssvc.ExportTagConfig{
		ShowSpeed: false,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "Netflix", Enabled: true},
			{Key: "spotify", Label: "Spotify", Enabled: true},
		},
	}
	flags := unlockFlags{Netflix: true}
	extra := map[string]bool{"zlib": true, "spotify": true, "off": false}
	got := taggedName("US", "US", flags, extra, 999, cfg)
	if got != "US|Netflix|Spotify|zlib" {
		t.Errorf("got %q", got)
	}
}
