package checker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// failIfCalledFetcher records whether Fetch was invoked. A check that tests the
// subscription's existing nodes must never reach the network.
type failIfCalledFetcher struct{ called atomic.Bool }

func (f *failIfCalledFetcher) Fetch(_ context.Context, _ string) ([]map[string]any, error) {
	f.called.Store(true)
	return nil, errors.New("fetch must not run when nodes already exist")
}

// A check tests the subscription's persisted nodes as-is and never re-fetches
// the subscription URL. Re-fetching on every check is exactly what made an
// unreachable provider break checking of otherwise-good nodes.
func TestJobRunnerChecksExistingNodesWithoutFetch(t *testing.T) {
	subID := "existing-nodes-sub-" + uuid.New().String()
	// Persisted nodes, as if a prior refresh or manual import populated them.
	if _, err := defaultJobStore.replaceNodes(context.Background(), subID, runnerProxies()); err != nil {
		t.Fatalf("seed nodes: %v", err)
	}

	jobID := insertTestJob(t, subID)
	f := &failIfCalledFetcher{}
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: f,
		bus:     newInProcessJobBus(),
		check:   aliveCheck,
	}
	r.run(context.Background(), jobID, subID, "test-user-id")

	if f.called.Load() {
		t.Error("a check must not fetch the subscription when nodes already exist")
	}
	status, available, total := jobState(t, jobID)
	if status != "completed" || total != 2 || available != 2 {
		t.Errorf("want completed/2/2 from existing nodes, got %s/%d/%d", status, available, total)
	}
}

// Manual import parses pasted content and becomes the subscription's node list.
func TestImportNodesPopulatesNodes(t *testing.T) {
	subID := "import-sub-" + uuid.New().String()
	content := "proxies:\n" +
		"  - {name: imp-a, type: ss, server: 1.2.3.4, port: 8388, cipher: aes-128-gcm, password: x}\n" +
		"  - {name: imp-b, type: ss, server: 5.6.7.8, port: 8389, cipher: aes-128-gcm, password: y}\n"

	count, err := importNodes(context.Background(), subID, content)
	if err != nil {
		t.Fatalf("importNodes: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2 imported nodes, got %d", count)
	}

	var n int
	if err := db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM nodes WHERE subscription_id=$1`, subID).Scan(&n); err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 persisted nodes, got %d", n)
	}
}

// Empty / unparseable content is rejected rather than wiping the node list.
func TestImportNodesRejectsEmptyContent(t *testing.T) {
	subID := "import-empty-sub-" + uuid.New().String()
	if _, err := importNodes(context.Background(), subID, "not a subscription"); err == nil {
		t.Error("importing garbage content must return an error")
	}
}
