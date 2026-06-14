package checker

import "testing"

func TestDefaultRuleByKey_ReturnsSeed(t *testing.T) {
	d, ok := defaultRuleByKey("netflix")
	if !ok || d.name != "Netflix" {
		t.Fatalf("expected Netflix seed, got %+v ok=%v", d, ok)
	}
	if _, ok := defaultRuleByKey("nope"); ok {
		t.Fatalf("unexpected match for unknown key")
	}
}
