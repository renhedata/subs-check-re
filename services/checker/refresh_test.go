package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Refresh fetches the URL and replaces the node list (not appends).
func TestRefreshNodesReplacesExisting(t *testing.T) {
	subID := "refresh-sub-" + uuid.New().String()
	// One pre-existing node, as if previously imported.
	if _, err := defaultJobStore.replaceNodes(context.Background(), subID,
		[]map[string]any{{"name": "old", "type": "ss", "server": "9.9.9.9", "port": 1}}); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	f := &scriptedFetcher{out: runnerProxies()} // two nodes
	count, err := refreshNodes(context.Background(), f, subID, "http://example.test/sub")
	if err != nil {
		t.Fatalf("refreshNodes: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 nodes after refresh, got %d", count)
	}

	var n int
	if err := db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM nodes WHERE subscription_id=$1`, subID).Scan(&n); err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if n != 2 {
		t.Errorf("refresh must replace, not append; want 2 persisted nodes, got %d", n)
	}
}

// A dry-run test surfaces the fetch error instead of touching the node list.
func TestDryRunFetchReportsError(t *testing.T) {
	f := &scriptedFetcher{results: []error{permanent(errors.New("provider unreachable"))}}
	if _, err := dryRunFetch(context.Background(), f, "http://example.test/sub"); err == nil {
		t.Error("dryRunFetch must return the provider error")
	}
}

// A successful dry-run reports the node count without persisting anything.
func TestDryRunFetchSuccess(t *testing.T) {
	subID := "dryrun-sub-" + uuid.New().String()
	f := &scriptedFetcher{out: runnerProxies()}
	count, err := dryRunFetch(context.Background(), f, "http://example.test/sub")
	if err != nil {
		t.Fatalf("dryRunFetch: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 nodes reported, got %d", count)
	}

	var n int
	db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM nodes WHERE subscription_id=$1`, subID).Scan(&n)
	if n != 0 {
		t.Errorf("dry-run must not persist nodes, got %d", n)
	}
}
