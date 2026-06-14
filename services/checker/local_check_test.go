package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func TestGetLocalUnlock_OnlyEnabledRules(t *testing.T) {
	userID := "lu-user-" + uuid.New().String()
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Enabled condition rule -> should be probed and report unlocked.
	mkRule := func(key string, enabled bool) {
		def := []byte(`{"url":"` + srv.URL + `","status_code":200}`)
		if _, err := db.Exec(ctx, `
			INSERT INTO platform_rules (id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'',$5,'condition',$6,false,0,NOW(),NOW())
		`, uuid.New().String(), userID, key, key, enabled, def); err != nil {
			t.Fatalf("seed rule %s: %v", key, err)
		}
	}
	mkRule("alpha_on", true)
	mkRule("beta_off", false)

	res, err := GetLocalUnlock(ctx)
	if err != nil {
		t.Fatalf("GetLocalUnlock: %v", err)
	}
	if got, ok := res.Platforms["alpha_on"]; !ok || !got.Unlocked {
		t.Fatalf("alpha_on should be present and unlocked: %+v", res.Platforms)
	}
	if _, ok := res.Platforms["beta_off"]; ok {
		t.Fatalf("beta_off (disabled) must NOT be probed: %+v", res.Platforms)
	}
}
