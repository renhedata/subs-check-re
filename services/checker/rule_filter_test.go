package checker

import "testing"

func keysOf(rules []*PlatformRule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.Key
	}
	return out
}

func TestFilterRulesBySelection(t *testing.T) {
	rules := []*PlatformRule{{Key: "netflix"}, {Key: "youtube"}, {Key: "disney"}}

	got := filterRulesBySelection(rules, []string{"netflix", "disney", "nope"})
	if len(got) != 2 || got[0].Key != "netflix" || got[1].Key != "disney" {
		t.Fatalf("want [netflix disney], got %v", keysOf(got))
	}
	if filterRulesBySelection(rules, nil) != nil {
		t.Error("nil selection must yield no rules")
	}
	if len(filterRulesBySelection(rules, []string{})) != 0 {
		t.Error("empty selection must yield no rules")
	}
}
