// services/notify/notify_test.go
package notify

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

func TestCreateAndListChannel(t *testing.T) {
	ctx := withAuth()
	ch, err := CreateChannel(ctx, &CreateChannelParams{
		Name: "My Webhook",
		Type: "webhook",
	})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if ch.ID == "" {
		t.Error("expected non-empty ID")
	}

	list, err := ListChannels(ctx)
	if err != nil {
		t.Fatalf("ListChannels failed: %v", err)
	}
	if len(list.Channels) == 0 {
		t.Error("expected at least one channel")
	}
}

func TestCreateInvalidType(t *testing.T) {
	ctx := withAuth()
	_, err := CreateChannel(ctx, &CreateChannelParams{
		Name: "Bad",
		Type: "email",
	})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}
