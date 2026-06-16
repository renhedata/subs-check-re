package checker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// seedResultWithConfig inserts a check_result whose node_config carries a stable
// server:port identity, while node_name may differ between jobs (real
// subscriptions embed live traffic/expiry counters in the name).
func seedResultWithConfig(t *testing.T, jobID, subID, userID, nodeName, server string, port, ageHours int,
	alive bool, latency, speed int, country string, platforms map[string]PlatformOutcome) {
	t.Helper()
	jobExists := 0
	db.QueryRow(context.Background(), `SELECT COUNT(*) FROM check_jobs WHERE id=$1`, jobID).Scan(&jobExists)
	if jobExists == 0 {
		if _, err := db.Exec(context.Background(), `
			INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
			VALUES ($1,$2,$3,'completed',1,1,NOW(),NOW())
		`, jobID, subID, userID); err != nil {
			t.Fatalf("seed job: %v", err)
		}
	}
	pj, _ := json.Marshal(platforms)
	if string(pj) == "null" {
		pj = []byte("{}")
	}
	cfg := map[string]any{"name": nodeName, "type": "ss", "server": server, "port": port}
	cfgJSON, _ := json.Marshal(cfg)
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,$4,'ss',$5, NOW() - make_interval(hours => $6::int), $7,$8,$9,$10,'', $11)
	`, uuid.New().String(), jobID, uuid.New().String(), nodeName, cfgJSON, ageHours, alive, latency, speed, country, pj); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

// Reproduces the prod bug: an alive-only run after a full check shows empty
// streaming results because the node's name changed (traffic counter) even
// though it is the same endpoint. Inheritance must key on a stable identity.
func TestGetResultsInheritsAcrossRenamedNode(t *testing.T) {
	userID := "rename-user-" + uuid.New().String()
	subID := "rename-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Full check: name carries a live traffic counter; netflix unlocked, speed + country measured.
	seedResultWithConfig(t, jobA, subID, userID, "HK-01 |流量:50G", "1.1.1.1", 443, 2,
		true, 50, 5000, "HK", map[string]PlatformOutcome{"netflix": {Unlocked: true}})
	// Alive-only run minutes later: SAME endpoint, name's counter changed, no platforms/speed/country.
	seedResultWithConfig(t, jobB, subID, userID, "HK-01 |流量:48G", "1.1.1.1", 443, 0,
		true, 30, 0, "", map[string]PlatformOutcome{})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobB})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(resp.Results))
	}
	r := resp.Results[0]
	if !r.Platforms["netflix"].Unlocked {
		t.Errorf("netflix must inherit across a renamed node (same server:port), got %+v", r.Platforms)
	}
	if r.SpeedKbps != 5000 {
		t.Errorf("speed must inherit across renamed node, got %d", r.SpeedKbps)
	}
	if r.Country != "HK" {
		t.Errorf("country must inherit across renamed node, got %q", r.Country)
	}
}

// Per-key inheritance must survive a rename: a freshly re-tested platform uses
// the new value, an untested one inherits its last value — keyed by endpoint.
func TestGetResultsPerKeyInheritanceAcrossRenamedNode(t *testing.T) {
	userID := "rk-user-" + uuid.New().String()
	subID := "rk-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Older full check: netflix + youtube unlocked.
	seedResultWithConfig(t, jobA, subID, userID, "JP-02 |到期:12-31", "2.2.2.2", 8443, 2,
		true, 100, 1000, "JP",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}, "youtube": {Unlocked: true}})
	// Newer check: same endpoint, name changed, re-tested netflix only (now locked).
	seedResultWithConfig(t, jobB, subID, userID, "JP-02 |到期:01-31", "2.2.2.2", 8443, 0,
		true, 100, 1000, "JP",
		map[string]PlatformOutcome{"netflix": {Unlocked: false}})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobB})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.Platforms["netflix"].Unlocked {
		t.Error("netflix must reflect the fresh (locked) result")
	}
	if !r.Platforms["youtube"].Unlocked {
		t.Error("youtube must inherit the older unlocked result across the rename")
	}
}

// Two different endpoints that happen to share a display name must NOT
// cross-inherit — keying on server:port keeps them distinct.
func TestGetResultsDoesNotCrossInheritDifferentEndpoints(t *testing.T) {
	userID := "xinh-user-" + uuid.New().String()
	subID := "xinh-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Endpoint A named "Premium": netflix unlocked.
	seedResultWithConfig(t, jobA, subID, userID, "Premium", "1.1.1.1", 443, 2,
		true, 100, 1000, "US", map[string]PlatformOutcome{"netflix": {Unlocked: true}})
	// A DIFFERENT endpoint, also named "Premium", alive-only (no platforms).
	seedResultWithConfig(t, jobB, subID, userID, "Premium", "9.9.9.9", 443, 0,
		true, 30, 0, "", map[string]PlatformOutcome{})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobB})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.Platforms["netflix"].Unlocked {
		t.Error("different endpoint sharing a name must not inherit netflix")
	}
}
