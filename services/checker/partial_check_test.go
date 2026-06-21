package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// noFetchOnPartialFetcher fails the test if the runner attempts a fetch. A
// partial check (node_ids set) must test existing nodes only and never re-fetch.
type noFetchOnPartialFetcher struct{ t *testing.T }

func (f *noFetchOnPartialFetcher) Fetch(_ context.Context, _ string) ([]map[string]any, error) {
	f.t.Fatal("fetcher must not be called on a partial check")
	return nil, nil
}

// A partial check writes results only for the selected nodes and sets the job
// total to the subset size; non-selected nodes get no new row.
func TestPartialCheckScopesToSubset(t *testing.T) {
	ctx := context.Background()
	subID := uuid.New().String()
	userID := uuid.New().String()

	// Two persisted nodes.
	ids, err := defaultJobStore.replaceNodes(ctx, subID, []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 1111},
		{"name": "B", "type": "ss", "server": "2.2.2.2", "port": 2222},
	})
	if err != nil {
		t.Fatalf("replaceNodes: %v", err)
	}

	// Job selecting only node A.
	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at, node_ids)
		VALUES ($1,$2,$3,'http://x','{}','queued',$4,$5::jsonb)
	`, jobID, subID, userID, time.Now(), `["`+ids[0]+`"]`); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &noFetchOnPartialFetcher{t},
		bus:     newInProcessJobBus(),
		check: func(_ context.Context, nodeID string, _ map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule) nodeCheckResult {
			return nodeCheckResult{NodeID: nodeID, NodeName: "A", Alive: true, LatencyMs: 10}
		},
	}
	r.run(ctx, jobID, subID, userID)

	// Exactly one result row (node A); job total == 1.
	var rows int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM check_results WHERE job_id=$1`, jobID).Scan(&rows); err != nil {
		t.Fatalf("count results: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 result row, got %d", rows)
	}
	var total int
	if err := db.QueryRow(ctx, `SELECT total FROM check_jobs WHERE id=$1`, jobID).Scan(&total); err != nil {
		t.Fatalf("read total: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
}
