// services/checker/jobstore_test.go
package checker

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestReplaceNodesTransactionalAndPreservesDisabled(t *testing.T) {
	ctx := context.Background()
	subID := "store-sub-" + uuid.New().String()
	store := &jobStore{}

	// Seed: node-a disabled, node-b enabled.
	first := []map[string]any{
		{"name": "node-a", "type": "ss", "server": "1.1.1.1", "port": 1},
		{"name": "node-b", "type": "ss", "server": "2.2.2.2", "port": 2},
	}
	if _, err := store.replaceNodes(ctx, subID, first); err != nil {
		t.Fatalf("seed replace: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE nodes SET enabled=false WHERE subscription_id=$1 AND name='node-a'`, subID); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// Replace with node-a (carried over) + node-c (new); node-b dropped.
	second := []map[string]any{
		{"name": "node-a", "type": "ss", "server": "1.1.1.1", "port": 1},
		{"name": "node-c", "type": "ss", "server": "3.3.3.3", "port": 3},
	}
	ids, err := store.replaceNodes(ctx, subID, second)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 node IDs, got %d", len(ids))
	}

	rows, err := db.Query(ctx, `SELECT name, enabled FROM nodes WHERE subscription_id=$1 ORDER BY name`, subID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		var enabled bool
		if err := rows.Scan(&name, &enabled); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = enabled
	}
	if len(got) != 2 {
		t.Fatalf("want 2 nodes, got %v", got)
	}
	if got["node-a"] != false {
		t.Error("node-a should remain disabled after replacement")
	}
	if got["node-c"] != true {
		t.Error("node-c should be enabled")
	}
}

func TestLoadConfigMissingJob(t *testing.T) {
	store := &jobStore{}
	if _, err := store.loadConfig(context.Background(), "no-such-job"); err == nil {
		t.Error("expected error for missing job")
	}
}
