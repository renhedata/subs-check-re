// services/scheduler/scheduler_test.go
package scheduler

import (
	"context"
	"testing"

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
