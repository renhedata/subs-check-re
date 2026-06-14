// services/checker/local_check.go
package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"encore.dev/beta/errs"
)

// LocalUnlockResult holds platform accessibility from the server's own network.
type LocalUnlockResult struct {
	Platforms map[string]PlatformOutcome `json:"platforms"`
	IP        string                     `json:"ip"`
	Country   string                     `json:"country"`
}

// GetLocalUnlock checks which streaming/AI platforms are accessible from the server's own network.
// It runs the seeded default rules (see rules_defaults.go) against a plain HTTP client — no proxy.
//
//encore:api auth method=GET path=/network-unlock
func GetLocalUnlock(ctx context.Context) (*LocalUnlockResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	results := runDefaultRulesAgainst(checkCtx, client)

	res := LocalUnlockResult{Platforms: map[string]PlatformOutcome{}}
	for _, r := range results {
		res.Platforms[r.key] = r.outcome
	}
	res.IP, res.Country = getProxyInfo(checkCtx, client)

	if err := checkCtx.Err(); err != nil {
		return nil, errs.B().Code(errs.DeadlineExceeded).Msg("check timed out").Err()
	}
	return &res, nil
}

type ruleResult struct {
	key     string
	outcome PlatformOutcome
}

// runDefaultRulesAgainst evaluates the seeded default rules against the given HTTP client.
// Runs all rules concurrently and aggregates results.
func runDefaultRulesAgainst(ctx context.Context, client *http.Client) []ruleResult {
	out := make(chan ruleResult, len(defaultRules))
	var wg sync.WaitGroup
	for _, dr := range defaultRules {
		wg.Add(1)
		go func(d defaultRule) {
			defer wg.Done()
			defer func() { _ = recover() }()
			defJSON, err := json.Marshal(d.def)
			if err != nil {
				out <- ruleResult{key: d.key, outcome: PlatformOutcome{}}
				return
			}
			rule := &PlatformRule{
				RuleType:   d.ruleType,
				Definition: defJSON,
			}
			outcome, _ := runRule(ctx, client, rule, nil)
			out <- ruleResult{key: d.key, outcome: outcome}
		}(dr)
	}
	wg.Wait()
	close(out)

	results := make([]ruleResult, 0, len(defaultRules))
	for r := range out {
		results = append(results, r)
	}
	return results
}
