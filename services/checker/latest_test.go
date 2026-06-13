// services/checker/latest_test.go
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

func latestTestCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func seedJob(t *testing.T, ctx context.Context, subID, userID, status string, available, total int, createdAt time.Time) string {
	t.Helper()
	id := uuid.New().String()
	finished := createdAt.Add(2 * time.Minute)
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, subID, userID, status, total, available, createdAt, finished); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	return id
}

func seedResult(t *testing.T, ctx context.Context, jobID string, alive bool, latencyMs int) {
	t.Helper()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, alive, latency_ms)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New().String(), jobID, uuid.New().String(), alive, latencyMs); err != nil {
		t.Fatalf("seed result: %v", err)
	}
}

func TestLatestJobsPicksNewestPerSubscriptionAndAveragesAliveLatency(t *testing.T) {
	userID := "latest-user-" + uuid.New().String()
	ctx := latestTestCtx(userID)
	subA := "latest-sub-a-" + uuid.New().String()
	subB := "latest-sub-b-" + uuid.New().String()

	base := time.Now().Add(-2 * time.Hour)
	seedJob(t, ctx, subA, userID, "completed", 1, 5, base) // older — must lose
	newest := seedJob(t, ctx, subA, userID, "completed", 2, 6, base.Add(30*time.Minute))
	jobB := seedJob(t, ctx, subB, userID, "failed", 0, 4, base)
	// Another user's job must not leak.
	seedJob(t, ctx, "latest-sub-other", "other-user", "completed", 9, 9, base)

	seedResult(t, ctx, newest, true, 40)
	seedResult(t, ctx, newest, true, 44)
	seedResult(t, ctx, newest, false, 999) // dead node — excluded from avg

	resp, err := LatestJobs(ctx)
	if err != nil {
		t.Fatalf("LatestJobs: %v", err)
	}

	a, ok := resp.Jobs[subA]
	if !ok {
		t.Fatalf("missing summary for subA; got %v", resp.Jobs)
	}
	if a.ID != newest {
		t.Errorf("subA should pick newest job: want %s got %s", newest, a.ID)
	}
	if a.Available != 2 || a.Total != 6 {
		t.Errorf("subA counters: want 2/6 got %d/%d", a.Available, a.Total)
	}
	if a.AvgLatencyMs != 42 {
		t.Errorf("subA avg latency: want 42 got %d", a.AvgLatencyMs)
	}
	if a.FinishedAt == nil {
		t.Error("subA finished_at should be set")
	}

	b, ok := resp.Jobs[subB]
	if !ok {
		t.Fatalf("missing summary for subB")
	}
	if b.ID != jobB || b.Status != "failed" {
		t.Errorf("subB: want failed job %s, got %+v", jobB, b)
	}
	if b.AvgLatencyMs != 0 {
		t.Errorf("subB avg latency with no results: want 0 got %d", b.AvgLatencyMs)
	}

	if _, leaked := resp.Jobs["latest-sub-other"]; leaked {
		t.Error("other user's subscription leaked into response")
	}
}
