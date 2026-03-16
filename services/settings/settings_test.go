// services/settings/settings_test.go
package settings

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

func TestGetAPIKeyCreatesOnFirstCall(t *testing.T) {
	ctx := withAuth()
	resp, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if resp.APIKey == "" {
		t.Error("expected non-empty API key")
	}

	// Second call returns same key.
	resp2, err := GetAPIKey(ctx)
	if err != nil {
		t.Fatalf("GetAPIKey second call: %v", err)
	}
	if resp2.APIKey != resp.APIKey {
		t.Error("expected same key on second call")
	}
}

func TestRegenerateAPIKey(t *testing.T) {
	ctx := withAuth()
	first, _ := GetAPIKey(ctx)
	second, err := RegenerateAPIKey(ctx)
	if err != nil {
		t.Fatalf("RegenerateAPIKey: %v", err)
	}
	if second.APIKey == first.APIKey {
		t.Error("expected new key after regenerate")
	}
}
