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

func TestGetSubscriptionByID(t *testing.T) {
	ctx := withAuth()
	sub, err := Create(ctx, &CreateParams{Name: "internal-lookup", URL: "http://example.test/sub"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := GetSubscriptionByID(context.Background(), &GetByIDParams{ID: sub.ID})
	if err != nil {
		t.Fatalf("GetSubscriptionByID: %v", err)
	}
	if got.URL != "http://example.test/sub" || got.UserID == "" {
		t.Errorf("unexpected subscription: %+v", got)
	}

	if _, err := GetSubscriptionByID(context.Background(), &GetByIDParams{ID: "missing-id"}); err == nil {
		t.Error("expected NotFound for missing subscription")
	}
}

func TestGetSubscriptionNamesEmpty(t *testing.T) {
	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "nonexistent-user",
		IDs:    []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Names == nil {
		t.Error("expected non-nil Names map")
	}
	if len(resp.Names) != 0 {
		t.Errorf("expected empty names map, got %v", resp.Names)
	}
}

func TestGetSubscriptionNamesResolvesNames(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{
		Name: "MyProxy",
		URL:  "https://example.com/sub-names.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "test-user-id",
		IDs:    []string{created.ID},
	})
	if err != nil {
		t.Fatalf("GetSubscriptionNames failed: %v", err)
	}
	if resp.Names[created.ID] != "MyProxy" {
		t.Errorf("expected name %q, got %q", "MyProxy", resp.Names[created.ID])
	}
}

func TestGetSubscriptionNamesWrongUser(t *testing.T) {
	ctx := withAuth()
	created, err := Create(ctx, &CreateParams{
		Name: "SomeProxy",
		URL:  "https://example.com/sub-wrong.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	resp, err := GetSubscriptionNames(context.Background(), &GetSubscriptionNamesParams{
		UserID: "different-user-id",
		IDs:    []string{created.ID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, found := resp.Names[created.ID]; found {
		t.Error("expected no name returned for wrong user")
	}
}

func subCtx(userID string) context.Context {
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	return context.Background()
}

func TestNormalizeExportSort(t *testing.T) {
	cases := map[string]string{
		"speed_desc": "speed_desc", "latency_asc": "latency_asc",
		"": "speed_desc", "bogus": "speed_desc",
	}
	for in, want := range cases {
		if got := normalizeExportSort(in); got != want {
			t.Errorf("normalizeExportSort(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCreateDefaultsAndUpdateExportSettings(t *testing.T) {
	ctx := subCtx("exp-user-1")

	// Create with no export settings -> defaults.
	created, err := Create(ctx, &CreateParams{Name: "S", URL: "https://e.com/s"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ExportIncludeDead != false || created.ExportSort != "speed_desc" {
		t.Errorf("create defaults wrong: %+v", created)
	}

	// Update both fields; bogus sort normalizes.
	inc := true
	sort := "latency_asc"
	updated, err := Update(ctx, created.ID, &UpdateParams{
		ExportIncludeDead: &inc,
		ExportSort:        &sort,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.ExportIncludeDead || updated.ExportSort != "latency_asc" {
		t.Errorf("update wrong: %+v", updated)
	}

	// List reflects the update.
	resp, err := List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, s := range resp.Subscriptions {
		if s.ID == created.ID {
			found = true
			if !s.ExportIncludeDead || s.ExportSort != "latency_asc" {
				t.Errorf("list row wrong: %+v", s)
			}
		}
	}
	if !found {
		t.Error("created subscription not in list")
	}

	// Updating only the name leaves export settings intact (pointer COALESCE).
	name := "S2"
	again, err := Update(ctx, created.ID, &UpdateParams{Name: &name})
	if err != nil {
		t.Fatalf("update2: %v", err)
	}
	if !again.ExportIncludeDead || again.ExportSort != "latency_asc" {
		t.Errorf("partial update clobbered export settings: %+v", again)
	}
}
