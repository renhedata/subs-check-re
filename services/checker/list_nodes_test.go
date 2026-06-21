package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// insertJob creates a minimal completed job row so insertResult's FK is satisfied.
func insertJob(t *testing.T, ctx context.Context, subID, userID string) string {
	t.Helper()
	id := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://x','{}','completed',$4)
	`, id, subID, userID, time.Now()); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	return id
}

// A node that has never been checked shows blank metrics and a nil last_checked_at.
func TestListNodesNeverChecked(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	if _, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
	}); err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}

	nodes, err := defaultJobStore.listNodes(ctx, subID)
	if err != nil {
		t.Fatalf("listNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.Alive || n.LatencyMs != 0 || n.SpeedKbps != 0 || n.LastCheckedAt != nil {
		t.Fatalf("unchecked node should be blank: %+v", n)
	}
}

// listNodes shows latest-per-node values; checking one node does not change another.
func TestListNodesLatestAndIsolation(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	userID := uuid.New().String()
	ids, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
		{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222},
	})
	if err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}
	proxyA := map[string]any{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111}
	proxyB := map[string]any{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222}

	// Job 1: both nodes measured fully.
	job1 := insertJob(t, ctx, subID, userID)
	if err := defaultJobStore.insertResult(ctx, job1, ids[0], proxyA, nodeCheckResult{
		NodeName: "A", Alive: true, LatencyMs: 30, SpeedKbps: 500, Country: "US",
		Platforms: map[string]PlatformOutcome{"netflix": {Unlocked: true}},
	}); err != nil {
		t.Fatalf("insert A: %v", err)
	}
	if err := defaultJobStore.insertResult(ctx, job1, ids[1], proxyB, nodeCheckResult{
		NodeName: "B", Alive: true, LatencyMs: 80, SpeedKbps: 200, Country: "JP",
	}); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	// Job 2: alive-only re-check of A (speed 0, no platforms, no country).
	job2 := insertJob(t, ctx, subID, userID)
	if err := defaultJobStore.insertResult(ctx, job2, ids[0], proxyA, nodeCheckResult{
		NodeName: "A", Alive: true, LatencyMs: 25,
	}); err != nil {
		t.Fatalf("insert A2: %v", err)
	}

	nodes, err := defaultJobStore.listNodes(ctx, subID)
	if err != nil {
		t.Fatalf("listNodes: %v", err)
	}
	byName := map[string]Node{}
	for _, n := range nodes {
		byName[n.NodeName] = n
	}
	a, b := byName["A"], byName["B"]

	// A: latency from latest (25), speed/country/platforms inherited from job1.
	if a.LatencyMs != 25 || a.SpeedKbps != 500 || a.Country != "US" {
		t.Fatalf("A latest+inherited wrong: %+v", a)
	}
	if o, ok := a.Platforms["netflix"]; !ok || !o.Unlocked {
		t.Fatalf("A should inherit netflix unlock: %+v", a.Platforms)
	}
	if a.LastCheckedAt == nil {
		t.Fatalf("A should have a last_checked_at")
	}
	// B: untouched by job2 — still job1's values.
	if b.LatencyMs != 80 || b.SpeedKbps != 200 || b.Country != "JP" {
		t.Fatalf("B should be unchanged: %+v", b)
	}
}
