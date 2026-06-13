// services/settings/settings_test.go
package settings

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestGetAPIKeyCreatesOnFirstCall(t *testing.T) {
	ctx := withAuth()
	resp, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if resp.APIKey == "" {
		t.Error("expected non-empty API key")
	}

	// Second call returns same key.
	resp2, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey second call: %v", err)
	}
	if resp2.APIKey != resp.APIKey {
		t.Error("expected same key on second call")
	}
}

func TestRegenerateAPIKey(t *testing.T) {
	ctx := withAuth()
	first, _ := GetAPIKey(ctx)
	second, err := RegenerateAPIKey(ctx)
	if err != nil {
		t.Fatalf("RegenerateAPIKey: %v", err)
	}
	if second.APIKey == first.APIKey {
		t.Error("expected new key after regenerate")
	}
}

func TestDefaultExportTagsMatchesLegacy(t *testing.T) {
	d := defaultExportTags()
	if !d.ShowSpeed {
		t.Error("ShowSpeed should default true")
	}
	if d.ShowCountry {
		t.Error("ShowCountry should default false (preserve current export names)")
	}
	want := map[string]string{
		"netflix": "NF", "openai": "GPT", "gemini": "GM", "claude": "CL",
		"grok": "GK", "youtube": "YT", "disney": "D+", "tiktok": "TK",
	}
	got := map[string]string{}
	for _, p := range d.Platforms {
		if !p.Enabled {
			t.Errorf("default platform %q should be enabled", p.Key)
		}
		got[p.Key] = p.Label
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("default label for %q: want %q got %q", k, v, got[k])
		}
	}
}

func TestMergeExportTagsOverridesAndKeepsCustom(t *testing.T) {
	stored := ExportTagConfig{
		ShowCountry: true,
		ShowSpeed:   false,
		Platforms: []PlatformTag{
			{Key: "netflix", Label: "Netflix", Enabled: false},
			{Key: "spotify", Label: "Spotify", Enabled: true},
		},
	}
	m := mergeExportTags(stored)
	if !m.ShowCountry || m.ShowSpeed {
		t.Errorf("scalar flags not carried: %+v", m)
	}
	byKey := map[string]PlatformTag{}
	for _, p := range m.Platforms {
		byKey[p.Key] = p
	}
	if byKey["netflix"].Label != "Netflix" || byKey["netflix"].Enabled {
		t.Errorf("netflix override lost: %+v", byKey["netflix"])
	}
	if byKey["openai"].Label != "GPT" || !byKey["openai"].Enabled {
		t.Errorf("untouched builtin openai should keep default: %+v", byKey["openai"])
	}
	if byKey["spotify"].Label != "Spotify" || !byKey["spotify"].Enabled {
		t.Errorf("custom spotify dropped: %+v", byKey["spotify"])
	}
}
