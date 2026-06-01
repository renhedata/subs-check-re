package checker

import (
	"context"
	"fmt"
	"net/http"
)

// runRule dispatches a PlatformRule to the correct engine based on rule_type.
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
	if dr != nil {
		dr.Variable("rule_name", rule.Name)
		dr.Variable("rule_type", rule.RuleType)
		dr.Variable("rule_key", rule.Key)
	}
	switch rule.RuleType {
	case "condition":
		return runConditionRule(ctx, client, rule.Definition, dr)
	case "js", "ts":
		return runJSRule(ctx, client, rule.RuleType, rule.Definition, dr)
	case "tengo":
		return runTengoRule(ctx, client, rule.Definition, dr)
	case "lua":
		return runLuaRule(ctx, client, rule.Definition, dr)
	default:
		err := fmt.Errorf("unknown rule_type: %s", rule.RuleType)
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
}

// runUserRules runs all enabled rules against the provided HTTP client.
// Returns a map of rule key → bool result.
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]bool {
	return runUserRulesWithDebug(ctx, client, rules, nil)
}

// runUserRulesWithDebug runs all enabled rules and optionally collects per-rule debug traces.
func runUserRulesWithDebug(ctx context.Context, client *http.Client, rules []*PlatformRule, results map[string]*DebugRecorder) map[string]bool {
	out := make(map[string]bool, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		var dr *DebugRecorder
		if results != nil {
			dr = &DebugRecorder{}
			results[rule.Key] = dr
		}
		ok, _ := runRule(ctx, client, rule, dr)
		out[rule.Key] = ok
	}
	return out
}
