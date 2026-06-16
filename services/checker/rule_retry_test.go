package checker

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRunUserRules_NoRetryOnDefinitiveLocked(t *testing.T) {
	rr := runRuleFn
	defer func() { runRuleFn = rr }()
	calls := 0
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		calls++
		return PlatformOutcome{Unlocked: false, Status: "No"}, nil // definitive locked
	}
	out := runUserRules(context.Background(), nil, []*PlatformRule{{Key: "netflix", Enabled: true}})
	if out["netflix"].Unlocked {
		t.Error("want locked")
	}
	if calls != 1 {
		t.Errorf("definitive locked must not retry; got %d calls", calls)
	}
}

func TestRunUserRules_RetriesTransientThenUnlocked(t *testing.T) {
	rr, rb := runRuleFn, mediaRuleBackoff
	defer func() { runRuleFn, mediaRuleBackoff = rr, rb }()
	mediaRuleBackoff = time.Millisecond
	calls := 0
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		calls++
		if calls == 1 {
			return PlatformOutcome{}, errors.New("connection reset")
		}
		return PlatformOutcome{Unlocked: true, Status: "Yes"}, nil
	}
	out := runUserRules(context.Background(), nil, []*PlatformRule{{Key: "netflix", Enabled: true}})
	if !out["netflix"].Unlocked {
		t.Error("want unlocked after transient retry")
	}
	if calls != 2 {
		t.Errorf("want 2 calls, got %d", calls)
	}
}

func TestRunUserRules_TransientCapped(t *testing.T) {
	rr, rb := runRuleFn, mediaRuleBackoff
	defer func() { runRuleFn, mediaRuleBackoff = rr, rb }()
	mediaRuleBackoff = time.Millisecond
	calls := 0
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		calls++
		return PlatformOutcome{}, errors.New("timeout")
	}
	runUserRules(context.Background(), nil, []*PlatformRule{{Key: "netflix", Enabled: true}})
	if calls != mediaRuleAttempts {
		t.Errorf("want %d calls (capped), got %d", mediaRuleAttempts, calls)
	}
}

func TestRunUserRules_DisabledRuleSkipped(t *testing.T) {
	rr := runRuleFn
	defer func() { runRuleFn = rr }()
	calls := 0
	runRuleFn = func(context.Context, *http.Client, *PlatformRule, *DebugRecorder) (PlatformOutcome, error) {
		calls++
		return PlatformOutcome{}, nil
	}
	out := runUserRules(context.Background(), nil, []*PlatformRule{{Key: "netflix", Enabled: false}})
	if calls != 0 {
		t.Errorf("disabled rule must not run; got %d calls", calls)
	}
	if _, ok := out["netflix"]; ok {
		t.Error("disabled rule must not appear in output")
	}
}
