package checker

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// A node re-checked alive-only must stream a FULL result live: freshly measured
// platforms win, untested platforms + unmeasured dims (country/speed) inherit
// the node's last-known values (keyed by server:port). The live SSE event must
// match what GetResults returns for the same job — no drift between the two
// inheritance paths.
func TestLiveProgressEmitsInheritedResult(t *testing.T) {
	userID := "test-user-id"
	subID := "live-inherit-sub-" + uuid.New().String()
	histJob := uuid.New().String()

	// Prior full check on endpoint 1.1.1.1:443: netflix + youtube unlocked,
	// country + speed measured. Name carries a live traffic counter.
	seedResultWithConfig(t, histJob, subID, userID, "HK-01 |流量:50G", "1.1.1.1", 443, 2,
		true, 50, 5000, "HK",
		map[string]PlatformOutcome{
			"netflix": {Unlocked: true, Status: "Yes"},
			"youtube": {Unlocked: true, Status: "Yes"},
		})

	// Alive-only re-check: SAME endpoint, renamed, only netflix re-measured
	// (now locked), no country/speed.
	jobID := insertTestJob(t, subID)
	proxy := map[string]any{"name": "HK-01 |流量:48G", "type": "ss", "server": "1.1.1.1", "port": 443}
	check := func(_ context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule, _ phaseEmitter) nodeCheckResult {
		name, _ := mapping["name"].(string)
		return nodeCheckResult{
			NodeID: nodeID, NodeName: name, Alive: true, LatencyMs: 20,
			Platforms: map[string]PlatformOutcome{"netflix": {Unlocked: false, Status: "No"}},
		}
	}

	bus := newInProcessJobBus()
	sub := bus.Subscribe(jobID)
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: []map[string]any{proxy}},
		bus:     bus,
		check:   check,
	}
	r.run(context.Background(), jobID, subID, userID)

	// run() closes the channel on completion; drain it for the per-node event.
	var got *progressUpdate
	for ev := range sub {
		ev := ev
		if ev.NodeName == "HK-01 |流量:48G" {
			got = &ev
		}
	}
	if got == nil {
		t.Fatal("no per-node progress event was published")
	}

	if got.Platforms["netflix"].Unlocked {
		t.Errorf("netflix must reflect the fresh (locked) result, got %+v", got.Platforms["netflix"])
	}
	if !got.Platforms["youtube"].Unlocked {
		t.Errorf("youtube must inherit the prior unlocked result, got %+v", got.Platforms["youtube"])
	}
	if got.Country != "HK" {
		t.Errorf("country must inherit, got %q", got.Country)
	}
	if got.SpeedKbps != 5000 {
		t.Errorf("speed must inherit, got %d", got.SpeedKbps)
	}
	if got.NodeID == "" || got.Server != "1.1.1.1" || got.Port != 443 || got.NodeType != "ss" {
		t.Errorf("live event must carry node identity, got id=%q server=%q port=%d type=%q",
			got.NodeID, got.Server, got.Port, got.NodeType)
	}

	// Cross-check: the live event must agree with GetResults for the same job.
	resp, err := GetResults(resultsCtx(userID), subID, &GetResultsParams{JobID: jobID})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(resp.Results))
	}
	rr := resp.Results[0]
	if rr.Platforms["netflix"].Unlocked != got.Platforms["netflix"].Unlocked ||
		rr.Platforms["youtube"].Unlocked != got.Platforms["youtube"].Unlocked ||
		rr.Country != got.Country || rr.SpeedKbps != got.SpeedKbps {
		t.Errorf("live event must match GetResults: live{nf=%v yt=%v c=%q s=%d} results{nf=%v yt=%v c=%q s=%d}",
			got.Platforms["netflix"].Unlocked, got.Platforms["youtube"].Unlocked, got.Country, got.SpeedKbps,
			rr.Platforms["netflix"].Unlocked, rr.Platforms["youtube"].Unlocked, rr.Country, rr.SpeedKbps)
	}
}
