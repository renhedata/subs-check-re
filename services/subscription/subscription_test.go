// services/subscription/subscription_test.go
package subscription

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

func TestCreateAndListSubscription(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{
		Name: "Test Sub",
		URL:  "https://example.com/sub.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	list, err := List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list.Subscriptions) == 0 {
		t.Error("expected at least one subscription")
	}
}

func TestDeleteSubscription(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{URL: "https://example.com/sub2.yaml"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	resp, err := Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !resp.OK {
		t.Error("expected OK=true")
	}
}
