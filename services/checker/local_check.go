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
	Netflix        bool   `json:"netflix"`
	YouTube        bool   `json:"youtube"`
	YouTubePremium bool   `json:"youtube_premium"`
	OpenAI         bool   `json:"openai"`
	Claude         bool   `json:"claude"`
	Gemini         bool   `json:"gemini"`
	Grok           bool   `json:"grok"`
	Disney         bool   `json:"disney"`
	TikTok         bool   `json:"tiktok"`
	IP             string `json:"ip"`
	Country        string `json:"country"`
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

	var res LocalUnlockResult
	for _, r := range results {
		switch r.key {
		case "netflix":
			res.Netflix = r.ok
		case "youtube":
			res.YouTube = r.ok
		case "youtube_premium":
			res.YouTubePremium = r.ok
		case "openai":
			res.OpenAI = r.ok
		case "claude":
			res.Claude = r.ok
		case "gemini":
			res.Gemini = r.ok
		case "grok":
			res.Grok = r.ok
		case "disney":
			res.Disney = r.ok
		case "tiktok":
			res.TikTok = r.ok
		}
	}

	res.IP, res.Country = getProxyInfo(checkCtx, client)

	if err := checkCtx.Err(); err != nil {
		return nil, errs.B().Code(errs.DeadlineExceeded).Msg("check timed out").Err()
	}
	return &res, nil
}

type ruleResult struct {
	key string
	ok  bool
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
				out <- ruleResult{key: d.key, ok: false}
				return
			}
			rule := &PlatformRule{
				RuleType:   d.ruleType,
				Definition: defJSON,
			}
			ok, _ := runRule(ctx, client, rule, nil)
			out <- ruleResult{key: d.key, ok: ok}
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
