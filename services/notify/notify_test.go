// services/notify/notify_test.go
package notify

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/robfig/cron/v3"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func testSvc() *Service {
	return &Service{
		cron:    cron.New(),
		entries: make(map[string]cron.EntryID),
	}
}

func TestCreateAndListChannel(t *testing.T) {
	ctx := withAuth()
	s := testSvc()
	ch, err := s.CreateChannel(ctx, &CreateChannelParams{
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

func TestUpdateChannel(t *testing.T) {
	ctx := withAuth()
	s := testSvc()
	ch, err := s.CreateChannel(ctx, &CreateChannelParams{
		Name: "Before",
		Type: "webhook",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newName := "After"
	enabled := false
	updated, err := s.UpdateChannel(ctx, ch.ID, &UpdateChannelParams{
		Name:    &newName,
		Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "After" {
		t.Errorf("expected name 'After', got %q", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected enabled=false after update")
	}
}

func TestCreateInvalidType(t *testing.T) {
	ctx := withAuth()
	s := testSvc()
	_, err := s.CreateChannel(ctx, &CreateChannelParams{
		Name: "Bad",
		Type: "email",
	})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}
