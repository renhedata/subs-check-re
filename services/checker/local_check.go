// services/checker/local_check.go
package checker

import (
	"context"
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
//
//encore:api auth method=GET path=/network-unlock
func GetLocalUnlock(ctx context.Context) (*LocalUnlockResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var (
		mu  sync.Mutex
		res LocalUnlockResult
		wg  sync.WaitGroup
	)

	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { recover() }()
			fn()
		}()
	}

	run(func() {
		v, _ := checkNetflix(checkCtx, client)
		mu.Lock()
		res.Netflix = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkYouTube(checkCtx, client)
		mu.Lock()
		res.YouTube = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkYouTubePremium(checkCtx, client)
		mu.Lock()
		res.YouTubePremium = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkOpenAI(checkCtx, client)
		mu.Lock()
		res.OpenAI = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkClaude(checkCtx, client)
		mu.Lock()
		res.Claude = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkGemini(checkCtx, client)
		mu.Lock()
		res.Gemini = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkGrok(checkCtx, client)
		mu.Lock()
		res.Grok = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkDisney(checkCtx, client)
		mu.Lock()
		res.Disney = v
		mu.Unlock()
	})
	run(func() {
		v, _ := checkTikTok(checkCtx, client)
		mu.Lock()
		res.TikTok = v
		mu.Unlock()
	})
	run(func() {
		ip, country := getProxyInfo(checkCtx, client)
		mu.Lock()
		res.IP = ip
		res.Country = country
		mu.Unlock()
	})

	wg.Wait()

	if err := checkCtx.Err(); err != nil {
		return nil, errs.B().Code(errs.DeadlineExceeded).Msg("check timed out").Err()
	}

	return &res, nil
}
