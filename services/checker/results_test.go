// services/checker/results_test.go
package checker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func resultsCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func seedFullResult(t *testing.T, jobID, subID, userID, nodeName string, ageHours int,
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
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,$4,'ss', NOW() - make_interval(hours => $5::int), $6,$7,$8,$9,'', $10)
	`, uuid.New().String(), jobID, uuid.New().String(), nodeName, ageHours, alive, latency, speed, country, pj); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

func TestGetResultsInheritsUnmeasuredDimensions(t *testing.T) {
	userID := "inh-user-" + uuid.New().String()
	subID := "inh-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobB := uuid.New().String(), uuid.New().String()

	// Older full check: speed 5000, HK, netflix + youtube unlocked.
	seedFullResult(t, jobA, subID, userID, "N1", 2, true, 50, 5000, "HK",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}, "youtube": {Unlocked: true}})
	// Newer alive-only check: fresh alive/latency, nothing else.
	seedFullResult(t, jobB, subID, userID, "N1", 0, true, 30, 0, "",
		map[string]PlatformOutcome{})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobB})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.LatencyMs != 30 {
		t.Errorf("latency must be fresh from alive-only run, got %d", r.LatencyMs)
	}
	if r.SpeedKbps != 5000 {
		t.Errorf("speed must inherit, got %d", r.SpeedKbps)
	}
	if r.Country != "HK" {
		t.Errorf("country must inherit, got %q", r.Country)
	}
	if !r.Platforms["netflix"].Unlocked || !r.Platforms["youtube"].Unlocked {
		t.Errorf("platforms must inherit, got %+v", r.Platforms)
	}
}

func TestGetResultsPlatformInheritanceIsPerKey(t *testing.T) {
	userID := "perkey-user-" + uuid.New().String()
	subID := "perkey-sub-" + uuid.New().String()
	ctx := resultsCtx(userID)
	jobA, jobC := uuid.New().String(), uuid.New().String()

	// Older: netflix + youtube both unlocked.
	seedFullResult(t, jobA, subID, userID, "N1", 2, true, 100, 1000, "US",
		map[string]PlatformOutcome{"netflix": {Unlocked: true}, "youtube": {Unlocked: true}})
	// Newer: tested netflix only, now locked. youtube NOT tested.
	seedFullResult(t, jobC, subID, userID, "N1", 1, true, 100, 1000, "US",
		map[string]PlatformOutcome{"netflix": {Unlocked: false}})

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobC})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	r := resp.Results[0]
	if r.Platforms["netflix"].Unlocked {
		t.Error("netflix must reflect the fresh (locked) result")
	}
	if !r.Platforms["youtube"].Unlocked {
		t.Error("youtube must inherit the older unlocked result (per-key)")
	}
}

func TestGetResultsReturnsServerPortConfig(t *testing.T) {
	userID := "res-user-" + uuid.New().String()
	subID := "res-sub-" + uuid.New().String()
	jobID := uuid.New().String()
	nodeID := uuid.New().String()
	ctx := resultsCtx(userID)

	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1,$2,$3,'completed',1,1,$4,$4)
	`, jobID, subID, userID, time.Now()); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO nodes (id, subscription_id, name, type, server, port, config, enabled)
		VALUES ($1,$2,'N1','vmess','example.com',443,'{"type":"vmess","server":"example.com","port":443}'::jsonb,true)
	`, nodeID, subID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, alive, latency_ms, country, ip)
		VALUES ($1,$2,$3,'N1','vmess',true,42,'HK','1.2.3.4')
	`, uuid.New().String(), jobID, nodeID); err != nil {
		t.Fatalf("seed result: %v", err)
	}

	resp, err := GetResults(ctx, subID, &GetResultsParams{JobID: jobID})
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Server != "example.com" {
		t.Errorf("server: want example.com got %q", r.Server)
	}
	if r.Port != 443 {
		t.Errorf("port: want 443 got %d", r.Port)
	}
	if r.Config == "" || r.Config[0] != '{' {
		t.Errorf("config: want JSON object string, got %q", r.Config)
	}
	if r.NodeName != "N1" || r.LatencyMs != 42 || r.Country != "HK" {
		t.Errorf("existing fields wrong: %+v", r)
	}
}
