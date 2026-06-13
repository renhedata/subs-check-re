package checker

import (
	"context"
	"fmt"
	"net/http"
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

// runUserRules runs all enabled rules against the provided HTTP client.
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]PlatformOutcome {
	return runUserRulesWithDebug(ctx, client, rules, nil)
}

// runUserRulesWithDebug runs all enabled rules and optionally collects per-rule debug traces.
func runUserRulesWithDebug(ctx context.Context, client *http.Client, rules []*PlatformRule, results map[string]*DebugRecorder) map[string]PlatformOutcome {
	out := make(map[string]PlatformOutcome, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		var dr *DebugRecorder
		if results != nil {
			dr = &DebugRecorder{}
			results[rule.Key] = dr
		}
		outcome, _ := runRule(ctx, client, rule, dr)
		out[rule.Key] = outcome
	}
	return out
}
