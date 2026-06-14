package checker

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

func TestCustomizeAndResetBuiltinRule(t *testing.T) {
	userID := "reset-user-" + uuid.New().String()
	et.OverrideAuthInfo(auth.UID(userID), &authsvc.UserClaims{UserID: userID})
	ctx := context.Background()

	if err := syncDefaultRules(ctx, userID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// find the seeded netflix rule
	var id string
	if err := db.QueryRow(ctx,
		`SELECT id FROM platform_rules WHERE user_id=$1 AND key='netflix'`, userID).Scan(&id); err != nil {
		t.Fatalf("find netflix: %v", err)
	}

	// edit its content -> should mark customized
	edited := json.RawMessage(`{"code":"return {unlocked:true,status:\"Yes\",region:\"ZZ\"};"}`)
	if _, err := UpdateRule(ctx, id, &UpdateRuleParams{
		Name: "Netflix", Icon: "simple-icons:netflix", Enabled: true,
		RuleType: "js", Definition: edited, SortOrder: 0,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	var customized bool
	db.QueryRow(ctx, `SELECT customized FROM platform_rules WHERE id=$1`, id).Scan(&customized)
	if !customized {
		t.Fatal("editing a built-in rule's content must set customized=true")
	}

	// sync again -> must NOT overwrite the customized rule
	if err := syncDefaultRules(ctx, userID); err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	// Compare via JSON unmarshal to handle postgres JSON normalization (spaces after colons).
	var defAfterMap, editedMap map[string]any
	var defAfterRaw []byte
	db.QueryRow(ctx, `SELECT definition FROM platform_rules WHERE id=$1`, id).Scan(&defAfterRaw)
	if json.Unmarshal(defAfterRaw, &defAfterMap) != nil || json.Unmarshal(edited, &editedMap) != nil {
		t.Fatal("could not unmarshal definitions for comparison")
	}
	if !reflect.DeepEqual(defAfterMap, editedMap) {
		t.Fatalf("sync overwrote a customized rule: %s", defAfterRaw)
	}

	// reset -> back to seed + customized cleared
	if _, err := ResetRule(ctx, id); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var cust2 bool
	var def2Raw []byte
	db.QueryRow(ctx, `SELECT customized, definition FROM platform_rules WHERE id=$1`, id).Scan(&cust2, &def2Raw)
	if cust2 {
		t.Fatal("reset must clear customized")
	}
	// After reset the definition must NOT equal the edited value (it should be the seed).
	var def2Map map[string]any
	if json.Unmarshal(def2Raw, &def2Map) != nil || json.Unmarshal(edited, &editedMap) != nil {
		t.Fatal("could not unmarshal definitions after reset")
	}
	if reflect.DeepEqual(def2Map, editedMap) {
		t.Fatal("reset must restore the seed definition")
	}
}
