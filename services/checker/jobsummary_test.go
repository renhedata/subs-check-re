package checker

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPlatformUnlockSummary_IsMap(t *testing.T) {
	var s PlatformUnlockSummary = PlatformUnlockSummary{}
	s["netflix"] = 3
	if s["netflix"] != 3 {
		t.Fatalf("map assignment failed")
	}
}

func TestLoadJobSummaryInheritsAfterAliveOnly(t *testing.T) {
	userID := "sum-user-" + uuid.New().String()
	subID := "sum-sub-" + uuid.New().String()
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Older full check: speed 5000, HK, netflix unlocked.
	seedFullResult(t, jobA, subID, userID, "N1", 2, true, 50, 5000, "HK",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}})
	// Newer alive-only check: fresh alive/latency, nothing else measured.
	seedFullResult(t, jobB, subID, userID, "N1", 0, true, 30, 0, "",
		map[string]PlatformOutcome{})

	s, err := loadJobSummary(context.Background(), jobB)
	if err != nil {
		t.Fatalf("loadJobSummary: %v", err)
	}
	if s.Platforms["netflix"] != 1 {
		t.Errorf("netflix unlock count must inherit, got %d", s.Platforms["netflix"])
	}
	if s.AvgSpeedKbps != 5000 || s.MaxSpeedKbps != 5000 {
		t.Errorf("speed stats must inherit, got avg=%d max=%d", s.AvgSpeedKbps, s.MaxSpeedKbps)
	}
	if len(s.TopNodes) != 1 || s.TopNodes[0].SpeedKbps != 5000 {
		t.Errorf("top nodes must use inherited speed, got %+v", s.TopNodes)
	}
	if s.Countries["HK"] != 1 {
		t.Errorf("country breakdown must inherit, got %+v", s.Countries)
	}
}
