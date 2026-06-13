// services/scheduler/scheduler_test.go
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestListEmpty(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService failed: %v", err)
	}
	ctx := withAuth()
	resp, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if resp.Jobs == nil {
		t.Error("expected non-nil jobs slice")
	}
}

func TestRunScheduledCheckCleansDeletedSubscription(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService: %v", err)
	}
	ctx := context.Background()
	subID := "gone-sub-id"
	if _, err := db.Exec(ctx, `
		INSERT INTO scheduled_jobs (id, subscription_id, user_id, cron_expr, created_at)
		VALUES ('gone-job-id', $1, 'test-user-id', '0 3 * * *', NOW())
		ON CONFLICT (subscription_id) DO NOTHING
	`, subID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc.runScheduledCheck(subID, defaultCheckOptions())

	var count int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM scheduled_jobs WHERE subscription_id=$1`, subID).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Error("expected stale scheduled_jobs row to be deleted when subscription is gone")
	}
}

func TestCronEntriesMapConcurrentAccess(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		id := fmt.Sprintf("race-sub-%d", i%5)
		go func() {
			defer wg.Done()
			svc.registerCron(id, "0 3 * * *", defaultCheckOptions())
		}()
		go func() {
			defer wg.Done()
			svc.removeCron(id)
		}()
	}
	wg.Wait()
}

func TestCreateInvalidCron(t *testing.T) {
	svc, _ := initService()
	ctx := withAuth()
	_, err := svc.Create(ctx, &CreateParams{
		SubscriptionID: "some-id",
		CronExpr:       "not-a-cron",
	})
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestSetEnabledTogglesRowAndCronEntry(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService: %v", err)
	}
	ctx := withAuth()
	subID := "toggle-sub-" + fmt.Sprint(time.Now().UnixNano())
	jobID := "toggle-job-" + fmt.Sprint(time.Now().UnixNano())
	if _, err := db.Exec(context.Background(), `
		INSERT INTO scheduled_jobs (id, subscription_id, user_id, cron_expr, enabled, created_at)
		VALUES ($1, $2, 'test-user-id', '0 3 * * *', true, NOW())
		ON CONFLICT (subscription_id) DO NOTHING
	`, jobID, subID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc.registerCron(subID, "0 3 * * *", defaultCheckOptions())

	if _, err := svc.SetEnabled(ctx, jobID, &SetEnabledParams{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	var enabled bool
	if err := db.QueryRow(context.Background(),
		`SELECT enabled FROM scheduled_jobs WHERE id=$1`, jobID).Scan(&enabled); err != nil {
		t.Fatalf("query: %v", err)
	}
	if enabled {
		t.Error("row should be disabled")
	}
	svc.mu.Lock()
	_, registered := svc.entries[subID]
	svc.mu.Unlock()
	if registered {
		t.Error("cron entry should be removed on disable")
	}

	if _, err := svc.SetEnabled(ctx, jobID, &SetEnabledParams{Enabled: true}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := db.QueryRow(context.Background(),
		`SELECT enabled FROM scheduled_jobs WHERE id=$1`, jobID).Scan(&enabled); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !enabled {
		t.Error("row should be enabled")
	}
	svc.mu.Lock()
	_, registered = svc.entries[subID]
	svc.mu.Unlock()
	if !registered {
		t.Error("cron entry should be registered on enable")
	}
}

func TestSetEnabledRejectsForeignJob(t *testing.T) {
	svc, _ := initService()
	ctx := withAuth()
	if _, err := svc.SetEnabled(ctx, "no-such-job-id", &SetEnabledParams{Enabled: false}); err == nil {
		t.Error("expected NotFound for unknown/foreign job id")
	}
}
