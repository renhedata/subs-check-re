package checker

import (
	"context"
	"fmt"
	"net/http"
)

// runRule dispatches a PlatformRule to the correct engine based on rule_type.
func runRule(ctx context.Context, client *http.Client, rule *PlatformRule) (bool, error) {
	switch rule.RuleType {
	case "condition":
		return runConditionRule(ctx, client, rule.Definition)
	case "js", "ts":
		return runJSRule(ctx, client, rule.RuleType, rule.Definition)
	case "tengo":
		return runTengoRule(ctx, client, rule.Definition)
	case "lua":
		return runLuaRule(ctx, client, rule.Definition)
	default:
		return false, fmt.Errorf("unknown rule_type: %s", rule.RuleType)
	}
}

// runUserRules runs all enabled rules against the provided HTTP client.
// Returns a map of rule key → bool result.
func runUserRules(ctx context.Context, client *http.Client, rules []*PlatformRule) map[string]bool {
	results := make(map[string]bool, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		ok, _ := runRule(ctx, client, rule)
		results[rule.Key] = ok
	}
	return results
}
