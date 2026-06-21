package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// loadConfig must surface node_ids when present, and leave it empty when NULL.
func TestLoadConfigReadsNodeIDs(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()

	// Job with an explicit node subset.
	jobWithIDs := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at, node_ids)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4,$5::jsonb)
	`, jobWithIDs, subID, uuid.New().String(), time.Now(), `["a","b"]`); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	cfg, err := defaultJobStore.loadConfig(ctx, jobWithIDs)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.NodeIDs) != 2 || cfg.NodeIDs[0] != "a" || cfg.NodeIDs[1] != "b" {
		t.Fatalf("expected [a b], got %v", cfg.NodeIDs)
	}

	// Job with no subset (NULL) → empty. Use a different subID to avoid the
	// one-active-job-per-subscription unique index.
	subID2 := uuid.New().String()
	jobAll := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4)
	`, jobAll, subID2, uuid.New().String(), time.Now()); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	cfgAll, err := defaultJobStore.loadConfig(ctx, jobAll)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfgAll.NodeIDs) != 0 {
		t.Fatalf("expected no node_ids, got %v", cfgAll.NodeIDs)
	}
}
