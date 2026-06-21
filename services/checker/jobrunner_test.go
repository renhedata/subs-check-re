// services/checker/jobrunner_test.go
package checker

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func insertTestJob(t *testing.T, subID string) string {
	t.Helper()
	jobID := uuid.New().String()
	optsJSON, _ := json.Marshal(defaultCheckOptions())
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1, $2, 'test-user-id', 'http://example.test/sub', $3, 'queued', NOW())
	`, jobID, subID, optsJSON); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	return jobID
}

func runnerProxies() []map[string]any {
	return []map[string]any{
		{"name": "node-a", "type": "ss", "server": "1.1.1.1", "port": 1},
		{"name": "node-b", "type": "ss", "server": "2.2.2.2", "port": 2},
	}
}

func aliveCheck(ctx context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, _ []*PlatformRule, _ phaseEmitter) nodeCheckResult {
	name, _ := mapping["name"].(string)
	return nodeCheckResult{NodeID: nodeID, NodeName: name, Alive: true, LatencyMs: 10}
}

func jobState(t *testing.T, jobID string) (status string, available, total int) {
	t.Helper()
	if err := db.QueryRow(context.Background(),
		`SELECT status, available, total FROM check_jobs WHERE id=$1`, jobID).Scan(&status, &available, &total); err != nil {
		t.Fatalf("job state: %v", err)
	}
	return
}

func TestJobRunnerHappyPath(t *testing.T) {
	subID := "runner-sub-" + uuid.New().String()
	jobID := insertTestJob(t, subID)
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: runnerProxies()},
		bus:     newInProcessJobBus(),
		check:   aliveCheck,
	}
	r.run(context.Background(), jobID, subID, "test-user-id")

	status, available, total := jobState(t, jobID)
	if status != "completed" || available != 2 || total != 2 {
		t.Errorf("want completed/2/2, got %s/%d/%d", status, available, total)
	}
	var results int
	db.QueryRow(context.Background(), `SELECT COUNT(*) FROM check_results WHERE job_id=$1`, jobID).Scan(&results)
	if results != 2 {
		t.Errorf("want 2 result rows, got %d", results)
	}
	var nodes int
	db.QueryRow(context.Background(), `SELECT COUNT(*) FROM nodes WHERE subscription_id=$1`, subID).Scan(&nodes)
	if nodes != 2 {
		t.Errorf("want 2 nodes after replacement, got %d", nodes)
	}
}

func TestJobRunnerFetchFailureMarksJobFailed(t *testing.T) {
	subID := "runner-sub-" + uuid.New().String()
	jobID := insertTestJob(t, subID)
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{results: []error{permanent(context.DeadlineExceeded)}},
		bus:     newInProcessJobBus(),
		check:   aliveCheck,
	}
	r.run(context.Background(), jobID, subID, "test-user-id")

	status, _, _ := jobState(t, jobID)
	if status != "failed" {
		t.Errorf("want failed, got %s", status)
	}
}

func TestJobRunnerPanicInCheckRecordsDeadNode(t *testing.T) {
	subID := "runner-sub-" + uuid.New().String()
	jobID := insertTestJob(t, subID)
	panicky := func(ctx context.Context, nodeID string, mapping map[string]any, a, b, c string, o CheckOptions, ru []*PlatformRule, _ phaseEmitter) nodeCheckResult {
		if name, _ := mapping["name"].(string); name == "node-a" {
			panic("mihomo exploded")
		}
		return aliveCheck(ctx, nodeID, mapping, a, b, c, o, ru, nil)
	}
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: runnerProxies()},
		bus:     newInProcessJobBus(),
		check:   panicky,
	}
	r.run(context.Background(), jobID, subID, "test-user-id")

	status, available, _ := jobState(t, jobID)
	if status != "completed" || available != 1 {
		t.Errorf("want completed/1 alive, got %s/%d", status, available)
	}
	var deadAlive bool
	if err := db.QueryRow(context.Background(),
		`SELECT alive FROM check_results WHERE job_id=$1 AND node_name='node-a'`, jobID).Scan(&deadAlive); err != nil {
		t.Fatalf("panicked node must still have a result row: %v", err)
	}
	if deadAlive {
		t.Error("panicked node must be recorded as dead")
	}
}

func TestJobRunnerCanceledContextMarksFailed(t *testing.T) {
	subID := "runner-sub-" + uuid.New().String()
	jobID := insertTestJob(t, subID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: runnerProxies()},
		bus:     newInProcessJobBus(),
		check:   aliveCheck,
	}
	r.run(ctx, jobID, subID, "test-user-id")

	status, _, _ := jobState(t, jobID)
	if status != "failed" {
		t.Errorf("want failed for canceled context, got %s", status)
	}
}

func TestJobRunnerFiltersRulesBySelection(t *testing.T) {
	userID := "filter-user-" + uuid.New().String()
	subID := "runner-sub-" + uuid.New().String()
	for _, k := range []string{"netflix", "youtube"} {
		if _, err := db.Exec(context.Background(),
			`INSERT INTO platform_rules (id, user_id, name, key, rule_type) VALUES ($1,$2,$3,$4,'condition')`,
			uuid.New().String(), userID, k, k); err != nil {
			t.Fatalf("seed rule %s: %v", k, err)
		}
	}

	opts := CheckOptions{SpeedTest: false, MediaApps: []string{"netflix"}}
	optsJSON, _ := json.Marshal(opts)
	jobID := uuid.New().String()
	if _, err := db.Exec(context.Background(), `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, options_json, status, created_at)
		VALUES ($1,$2,$3,'http://example.test/sub',$4,'queued',NOW())
	`, jobID, subID, userID, optsJSON); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	var mu sync.Mutex
	var gotKeys []string
	recording := func(_ context.Context, nodeID string, mapping map[string]any, _, _, _ string, _ CheckOptions, rules []*PlatformRule, _ phaseEmitter) nodeCheckResult {
		mu.Lock()
		if gotKeys == nil {
			gotKeys = keysOf(rules)
			if gotKeys == nil {
				gotKeys = []string{}
			}
		}
		mu.Unlock()
		name, _ := mapping["name"].(string)
		return nodeCheckResult{NodeID: nodeID, NodeName: name, Alive: true}
	}

	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: &scriptedFetcher{out: runnerProxies()},
		bus:     newInProcessJobBus(),
		check:   recording,
	}
	r.run(context.Background(), jobID, subID, userID)

	if len(gotKeys) != 1 || gotKeys[0] != "netflix" {
		t.Errorf("runner must pass only selected rules; want [netflix], got %v", gotKeys)
	}
}
