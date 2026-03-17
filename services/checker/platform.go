// services/checker/platform.go
package checker

import (
	"context"
	"io"
	"net/http"
	"strings"
)

// checkNetflix returns true if the proxy can access non-originals Netflix content.
func checkNetflix(ctx context.Context, client *http.Client) (bool, error) {
	for _, titleID := range []string{"81280792", "70143836"} {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://www.netflix.com/title/"+titleID, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if !strings.Contains(string(body), "Not Available") {
			return true, nil
		}
	}
	return false, nil
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
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	resp.Body.Close()
	bodyStr := string(body)
	notAvail := strings.Contains(bodyStr, "not available in your region") ||
		strings.Contains(bodyStr, "unavailable in your region")
	return !notAvail && resp.StatusCode == 200, nil
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

// checkTikTok returns "YES" if TikTok is accessible, else "".
func checkTikTok(ctx context.Context, client *http.Client) (bool, error) {
	resp, err := get(ctx, client, "https://www.tiktok.com/")
	if err != nil {
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	resp.Body.Close()
	if strings.Contains(string(body), "tiktok") && resp.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}
