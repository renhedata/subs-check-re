package checker

import (
	"context"
	"encoding/json"
	"testing"
)

// evalDefault looks up a seeded default rule by key and runs it through the real
// engine against a mock HTTP client keyed by request URL.
func evalDefault(t *testing.T, key string, byURL map[string]mockResp) PlatformOutcome {
	t.Helper()
	var dr defaultRule
	found := false
	for _, d := range defaultRules {
		if d.key == key {
			dr = d
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no default rule for key %q", key)
	}
	defJSON, _ := json.Marshal(dr.def)
	rule := &PlatformRule{RuleType: dr.ruleType, Key: dr.key, Enabled: true, Definition: defJSON}
	out, err := runRule(context.Background(), mockClient(byURL), rule, nil)
	if err != nil {
		t.Fatalf("%s error: %v", key, err)
	}
	return out
}

func TestDefaultRule_ClaudeBlocklist(t *testing.T) {
	// loc=US → unlocked
	out := evalDefault(t, "claude", map[string]mockResp{
		"https://claude.ai/cdn-cgi/trace": {status: 200, body: "fl=AAA\nloc=US\n"},
	})
	if !out.Unlocked || out.Region != "US" {
		t.Fatalf("US: got %+v", out)
	}
	// loc=CN → blocked
	out = evalDefault(t, "claude", map[string]mockResp{
		"https://claude.ai/cdn-cgi/trace": {status: 200, body: "loc=CN\n"},
	})
	if out.Unlocked || out.Status != "No" || out.Region != "CN" {
		t.Fatalf("CN: got %+v", out)
	}
}

func TestDefaultRule_BilibiliCN(t *testing.T) {
	out := evalDefault(t, "bilibili_cn", map[string]mockResp{
		"https://api.bilibili.com/pgc/player/web/playurl?avid=82846771&qn=0&type=&otype=json&ep_id=307247&fourk=1&fnver=0&fnval=16&module=bangumi": {status: 200, body: `{"code":0}`},
	})
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v", out)
	}
}

func TestDefaultRule_SpotifyBlocked(t *testing.T) {
	out := evalDefault(t, "spotify", map[string]mockResp{
		"https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json": {status: 403, body: ""},
	})
	if out.Unlocked || out.Status != "No" {
		t.Fatalf("got %+v", out)
	}
}

func TestDefaultRules_AllFifteenPresent(t *testing.T) {
	want := []string{"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
		"claude", "gemini", "grok", "disney", "tiktok",
		"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video"}
	have := map[string]bool{}
	for _, d := range defaultRules {
		have[d.key] = true
	}
	for _, k := range want {
		if !have[k] {
			t.Fatalf("missing default rule %q", k)
		}
	}
	if len(defaultRules) != len(want) {
		t.Fatalf("expected %d default rules, got %d", len(want), len(defaultRules))
	}
}
