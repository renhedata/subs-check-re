// services/checker/platform.go
package checker

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// checkNetflix returns true if the proxy can access non-originals Netflix content.
// Tests two titles: LEGO Ninjago (81280792) and Breaking Bad (70143836).
// Both returning "Oh no!" means Originals Only (blocked); either accessible means unlocked.
func checkNetflix(ctx context.Context, client *http.Client) (bool, error) {
	for _, titleID := range []string{"81280792", "70143836"} {
		body, err := fetchNetflixTitle(ctx, client, titleID)
		if err != nil {
			return false, err
		}
		if !strings.Contains(body, "Oh no!") {
			return true, nil
		}
	}
	return false, nil
}

func fetchNetflixTitle(ctx context.Context, client *http.Client, titleID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.netflix.com/title/"+titleID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Microsoft Edge";v="135", "Not-A.Brand";v="8", "Chromium";v="135"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// checkYouTube returns true if basic YouTube is accessible.
func checkYouTube(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.youtube.com/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, nil
	}
	bodyStr := string(body)
	blocked := strings.Contains(bodyStr, "not available in your country") ||
		strings.Contains(bodyStr, "unavailable in your region")
	return !blocked && strings.Contains(bodyStr, "youtube"), nil
}

// checkYouTubePremium returns true if YouTube Premium is available in the proxy's region.
func checkYouTubePremium(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.youtube.com/premium", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	bodyStr := string(body)
	if strings.Contains(bodyStr, "Premium is not available in your country") {
		return false, nil
	}
	return strings.Contains(bodyStr, "ad-free") || strings.Contains(bodyStr, "YouTube Premium"), nil
}

// checkOpenAI returns true if OpenAI API is reachable.
// /v1/models returns 401 (unauthorized) when accessible but no key is provided,
// which is the expected signal that the region is not blocked.
func checkOpenAI(ctx context.Context, client *http.Client) (bool, error) {
	resp, err := get(ctx, client, "https://api.openai.com/v1/models")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 401 || resp.StatusCode == 200, nil
}

// checkClaude returns true if claude.ai is accessible in the proxy's region.
// Follows redirects — if the final URL contains "app-unavailable-in-region" the region is blocked.
func checkClaude(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://claude.ai/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "app-unavailable-in-region") {
		return false, nil
	}
	if strings.Contains(finalURL, "claude.ai") && resp.StatusCode == 200 {
		return true, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return false, err
	}
	bodyStr := string(body)
	return strings.Contains(bodyStr, "claude") || strings.Contains(bodyStr, "anthropic"), nil
}

// checkGemini returns true if Gemini is accessible in the proxy's region.
// Checks for a known token present only when the region is supported.
func checkGemini(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://gemini.google.com/", nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	bodyStr := string(body)

	hasMeet := strings.Contains(bodyStr, "Meet Gemini")

	return hasMeet, nil
}

// checkDisney returns true if Disney+ is accessible in the proxy's region.
func checkDisney(ctx context.Context, client *http.Client) (bool, error) {
	resp, err := get(ctx, client, "https://www.disneyplus.com/")
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return resp.StatusCode == 200, nil
}

// checkGrok returns true if xAI Grok API is reachable.
// /v1/models returns 401 (unauthorized) when accessible without a key.
func checkGrok(ctx context.Context, client *http.Client) (bool, error) {
	resp, err := get(ctx, client, "https://api.x.ai/v1/models")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 401 || resp.StatusCode == 200, nil
}

// checkTikTok returns true if TikTok is accessible in the proxy's region.
// Detection layers:
//  1. Blocked regions (mainland China, India) redirect to /comingsoon — check final URL.
//  2. Real page: match "region":"XX" in embedded SIGI_STATE JSON.
//  3. Slardar WAF challenge page (ttwstatic.com) — still means the region is accessible,
//     as blocked regions redirect before the WAF challenge is served.
func checkTikTok(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.tiktok.com/", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	// Blocked regions redirect to /comingsoon before serving any content.
	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "comingsoon") || strings.Contains(finalURL, "not-available") {
		return false, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return false, err
	}
	bodyStr := string(body)

	// Explicit block signals in body.
	if strings.Contains(bodyStr, "not available in your region") ||
		strings.Contains(bodyStr, "TikTok is not available") {
		return false, nil
	}

	// Primary: region code in SIGI_STATE embedded JSON.
	re := regexp.MustCompile(`"region"\s*:\s*"([A-Z]{2})"`)
	if re.Match(body) {
		return true, nil
	}

	// Fallback: ByteDance CDN domains confirm we reached TikTok's servers.
	// ttwstatic.com appears in the Slardar WAF challenge page — getting challenged
	// means the region is accessible (blocked regions redirect before reaching WAF).
	return strings.Contains(bodyStr, "ttwstatic.com") ||
		strings.Contains(bodyStr, "tiktokcdn.com") ||
		strings.Contains(bodyStr, "bytedance"), nil
}
