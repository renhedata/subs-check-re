package checker

import "testing"

func TestPlatformUnlockSummary_IsMap(t *testing.T) {
	var s PlatformUnlockSummary = PlatformUnlockSummary{}
	s["netflix"] = 3
	if s["netflix"] != 3 {
		t.Fatalf("map assignment failed")
	}
}
