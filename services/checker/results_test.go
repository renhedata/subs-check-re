// services/checker/results_test.go
package checker

import (
	"context"
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
