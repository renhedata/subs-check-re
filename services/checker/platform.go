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
func checkNetflix(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	dr.HTTPReq("https://www.netflix.com/title/81280792", "GET", nil)
	for _, titleID := range []string{"81280792", "70143836"} {
		body, err := fetchNetflixTitle(ctx, client, titleID, dr)
		if err != nil {
			dr.Error(err)
			dr.Variable("netflix_unlocked", false)
			return false
		}
		if !strings.Contains(body, "Oh no!") {
			dr.Condition("body does not contain 'Oh no!'", true)
			dr.Variable("netflix_unlocked", true)
			return true
		}
		dr.Condition("body contains 'Oh no!'", true)
	}
	dr.Variable("netflix_unlocked", false)
	return false
}

func fetchNetflixTitle(ctx context.Context, client *http.Client, titleID string, dr *DebugRecorder) (string, error) {
	url := "https://www.netflix.com/title/" + titleID
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
	dr.HTTPResp(resp.StatusCode, nil, string(body))
	return string(body), nil
}

// checkYouTube returns true if basic YouTube is accessible.
func checkYouTube(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.youtube.com/"
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		dr.Error(err)
		dr.Variable("youtube_unlocked", false)
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		dr.Error(err)
		dr.Variable("youtube_unlocked", false)
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	dr.HTTPResp(resp.StatusCode, nil, string(body))
	if resp.StatusCode != 200 {
		dr.Condition("status_code == 200", false)
		dr.Variable("youtube_unlocked", false)
		return false
	}
	bodyStr := string(body)
	blocked := strings.Contains(bodyStr, "not available in your country") ||
		strings.Contains(bodyStr, "unavailable in your region")
	dr.Condition("body does not contain region block", !blocked)
	dr.Condition("body contains 'youtube'", strings.Contains(bodyStr, "youtube"))
	result := !blocked && strings.Contains(bodyStr, "youtube")
	dr.Variable("youtube_unlocked", result)
	return result
}

// checkYouTubePremium returns true if YouTube Premium is available in the proxy's region.
func checkYouTubePremium(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.youtube.com/premium"
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		dr.Error(err)
		dr.Variable("youtube_premium_unlocked", false)
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		dr.Error(err)
		dr.Variable("youtube_premium_unlocked", false)
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()
	dr.HTTPResp(resp.StatusCode, nil, string(body))
	bodyStr := string(body)
	if strings.Contains(bodyStr, "Premium is not available in your country") {
		dr.Condition("body does not contain 'Premium is not available in your country'", false)
		dr.Variable("youtube_premium_unlocked", false)
		return false
	}
	result := strings.Contains(bodyStr, "ad-free") || strings.Contains(bodyStr, "YouTube Premium")
	dr.Condition("body contains 'ad-free' or 'YouTube Premium'", result)
	dr.Variable("youtube_premium_unlocked", result)
	return result
}

// checkOpenAI returns true if OpenAI API is reachable.
// /v1/models returns 401 (unauthorized) when accessible but no key is provided,
// which is the expected signal that the region is not blocked.
func checkOpenAI(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://api.openai.com/v1/models"
	dr.HTTPReq(url, "GET", nil)
	resp, err := get(ctx, client, url)
	if err != nil {
		dr.Error(err)
		dr.Variable("openai_unlocked", false)
		return false
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	dr.HTTPResp(resp.StatusCode, nil, "")
	result := resp.StatusCode == 401 || resp.StatusCode == 200
	dr.Condition("status_code == 401 or 200", result)
	dr.Variable("openai_unlocked", result)
	return result
}

// checkClaude returns true if claude.ai is accessible in the proxy's region.
// Follows redirects — if the final URL contains "app-unavailable-in-region" the region is blocked.
func checkClaude(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://claude.ai/"
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		dr.Error(err)
		dr.Variable("claude_unlocked", false)
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		dr.Error(err)
		dr.Variable("claude_unlocked", false)
		return false
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	dr.HTTPResp(resp.StatusCode, nil, "")
	if strings.Contains(finalURL, "app-unavailable-in-region") {
		dr.Condition("final_url does not contain 'app-unavailable-in-region'", false)
		dr.Variable("claude_unlocked", false)
		return false
	}
	if strings.Contains(finalURL, "claude.ai") && resp.StatusCode == 200 {
		dr.Condition("final_url contains 'claude.ai' and status_code == 200", true)
		dr.Variable("claude_unlocked", true)
		return true
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		dr.Error(err)
		dr.Variable("claude_unlocked", false)
		return false
	}
	bodyStr := string(body)
	result := strings.Contains(bodyStr, "claude") || strings.Contains(bodyStr, "anthropic")
	dr.Condition("body contains 'claude' or 'anthropic'", result)
	dr.Variable("claude_unlocked", result)
	return result
}

// checkGemini returns true if Gemini is accessible in the proxy's region.
// Checks for a known token present only when the region is supported.
func checkGemini(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://gemini.google.com/"
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		dr.Error(err)
		dr.Variable("gemini_unlocked", false)
		return false
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		dr.Error(err)
		dr.Variable("gemini_unlocked", false)
		return false
	}
	defer resp.Body.Close()

	dr.HTTPResp(resp.StatusCode, nil, "")
	if resp.StatusCode != 200 {
		dr.Condition("status_code == 200", false)
		dr.Variable("gemini_unlocked", false)
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		dr.Error(err)
		dr.Variable("gemini_unlocked", false)
		return false
	}

	bodyStr := string(body)
	dr.HTTPResp(resp.StatusCode, nil, bodyStr)

	hasMeet := strings.Contains(bodyStr, "Meet Gemini")
	dr.Condition("body contains 'Meet Gemini'", hasMeet)
	dr.Variable("gemini_unlocked", hasMeet)
	return hasMeet
}

// checkDisney returns true if Disney+ is accessible in the proxy's region.
func checkDisney(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.disneyplus.com/"
	dr.HTTPReq(url, "GET", nil)
	resp, err := get(ctx, client, url)
	if err != nil {
		dr.Error(err)
		dr.Variable("disney_unlocked", false)
		return false
	}
	resp.Body.Close()
	dr.HTTPResp(resp.StatusCode, nil, "")
	result := resp.StatusCode == 200
	dr.Condition("status_code == 200", result)
	dr.Variable("disney_unlocked", result)
	return result
}

// checkGrok returns true if xAI Grok API is reachable.
// /v1/models returns 401 (unauthorized) when accessible without a key.
func checkGrok(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://api.x.ai/v1/models"
	dr.HTTPReq(url, "GET", nil)
	resp, err := get(ctx, client, url)
	if err != nil {
		dr.Error(err)
		dr.Variable("grok_unlocked", false)
		return false
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	dr.HTTPResp(resp.StatusCode, nil, "")
	result := resp.StatusCode == 401 || resp.StatusCode == 200
	dr.Condition("status_code == 401 or 200", result)
	dr.Variable("grok_unlocked", result)
	return result
}

// checkTikTok returns true if TikTok is accessible in the proxy's region.
// Detection layers:
//  1. Blocked regions (mainland China, India) redirect to /comingsoon — check final URL.
//  2. Real page: match "region":"XX" in embedded SIGI_STATE JSON.
//  3. Slardar WAF challenge page (ttwstatic.com) — still means the region is accessible,
//     as blocked regions redirect before the WAF challenge is served.
func checkTikTok(ctx context.Context, client *http.Client, dr *DebugRecorder) bool {
	url := "https://www.tiktok.com/"
	dr.HTTPReq(url, "GET", nil)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		dr.Error(err)
		dr.Variable("tiktok_unlocked", false)
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0")

	resp, err := client.Do(req)
	if err != nil {
		dr.Error(err)
		dr.Variable("tiktok_unlocked", false)
		return false
	}
	defer resp.Body.Close()

	dr.HTTPResp(resp.StatusCode, nil, "")
	if resp.StatusCode != http.StatusOK {
		dr.Condition("status_code == 200", false)
		dr.Variable("tiktok_unlocked", false)
		return false
	}

	// Blocked regions redirect to /comingsoon before serving any content.
	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "comingsoon") || strings.Contains(finalURL, "not-available") {
		dr.Condition("final_url does not contain 'comingsoon' or 'not-available'", false)
		dr.Variable("tiktok_unlocked", false)
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		dr.Error(err)
		dr.Variable("tiktok_unlocked", false)
		return false
	}
	bodyStr := string(body)
	dr.HTTPResp(resp.StatusCode, nil, bodyStr)

	// Explicit block signals in body.
	if strings.Contains(bodyStr, "not available in your region") ||
		strings.Contains(bodyStr, "TikTok is not available") {
		dr.Condition("body does not contain region block", false)
		dr.Variable("tiktok_unlocked", false)
		return false
	}

	// Primary: region code in SIGI_STATE embedded JSON.
	re := regexp.MustCompile(`"region"\s*:\s*"([A-Z]{2})"`)
	if re.Match(body) {
		dr.Condition("body contains region code in SIGI_STATE", true)
		dr.Variable("tiktok_unlocked", true)
		return true
	}

	// Fallback: ByteDance CDN domains confirm we reached TikTok's servers.
	// ttwstatic.com appears in the Slardar WAF challenge page — getting challenged
	// means the region is accessible (blocked regions redirect before reaching WAF).
	result := strings.Contains(bodyStr, "ttwstatic.com") ||
		strings.Contains(bodyStr, "tiktokcdn.com") ||
		strings.Contains(bodyStr, "bytedance")
	dr.Condition("body contains ByteDance CDN domains", result)
	dr.Variable("tiktok_unlocked", result)
	return result
}
