package checker

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// runRule dispatches a PlatformRule to the correct engine and normalizes the result.
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (PlatformOutcome, error) {
	if dr != nil {
		dr.Variable("rule_name", rule.Name)
		dr.Variable("rule_type", rule.RuleType)
		dr.Variable("rule_key", rule.Key)
	}
	switch rule.RuleType {
	case "condition":
		ok, err := runConditionRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	case "js", "ts":
		return runJSRule(ctx, client, rule.RuleType, rule.Definition, dr)
	case "tengo":
		ok, err := runTengoRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	case "lua":
		ok, err := runLuaRule(ctx, client, rule.Definition, dr)
		return boolOutcome(ok), err
	default:
		err := fmt.Errorf("unknown rule_type: %s", rule.RuleType)
		if dr != nil {
			dr.Error(err)
		}
		return PlatformOutcome{}, err
	}
}

// runRuleFn is a seam over runRule so tests can drive the media retry loop
// without a real engine or network.
var runRuleFn = runRule

const mediaRuleAttempts = 2

// mediaRuleBackoff is the pause between transient platform-check retries; var so tests shrink it.
var mediaRuleBackoff = 200 * time.Millisecond

// runUserRules runs all enabled rules against the provided HTTP client.
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]PlatformOutcome {
	return runUserRulesWithDebug(ctx, client, rules, nil)
}

// runUserRulesWithDebug runs all enabled rules and optionally collects per-rule debug traces.
// A platform check that returns a transient error (could not reach the platform)
// is retried up to mediaRuleAttempts; a definitive answer (err == nil, locked or
// unlocked) is never retried. A fresh DebugRecorder is used per attempt so the
// surviving trace reflects only the final try.
func runUserRulesWithDebug(ctx context.Context, client *http.Client, rules []*PlatformRule, results map[string]*DebugRecorder) map[string]PlatformOutcome {
	out := make(map[string]PlatformOutcome, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		var outcome PlatformOutcome
		var dr *DebugRecorder
		for attempt := 1; attempt <= mediaRuleAttempts; attempt++ {
			if results != nil {
				dr = &DebugRecorder{}
			}
			var err error
			outcome, err = runRuleFn(ctx, client, rule, dr)
			if err == nil || attempt == mediaRuleAttempts || ctx.Err() != nil {
				break
			}
			select {
			case <-ctx.Done():
			case <-time.After(mediaRuleBackoff):
			}
		}
		if results != nil {
			results[rule.Key] = dr
		}
		out[rule.Key] = outcome
	}
	return out
}

// filterRulesBySelection returns only the rules whose Key is in selected.
// An empty/nil selection yields no rules — an alive-only check tests no
// platforms, so every platform inherits its last-known value at read time.
func filterRulesBySelection(rules []*PlatformRule, selected []string) []*PlatformRule {
	if len(selected) == 0 {
		return nil
	}
	want := make(map[string]bool, len(selected))
	for _, k := range selected {
		want[k] = true
	}
	out := make([]*PlatformRule, 0, len(rules))
	for _, r := range rules {
		if want[r.Key] {
			out = append(out, r)
		}
	}
	return out
}
