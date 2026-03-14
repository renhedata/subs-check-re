// services/checker/platform.go
package checker

import (
	"io"
	"net/http"
	"strings"
)

// checkNetflix returns true if the proxy can access non-originals Netflix content.
func checkNetflix(client *http.Client) (bool, error) {
	for _, titleID := range []string{"81280792", "70143836"} {
		req, err := http.NewRequest("GET", "https://www.netflix.com/title/"+titleID, nil)
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
		if !strings.Contains(string(body), "Oh no!") {
			return true, nil
		}
	}
	return false, nil
}

// checkYouTube returns "YES" if YouTube Premium is available, else "".
func checkYouTube(client *http.Client) (string, error) {
	req, err := http.NewRequest("GET", "https://www.youtube.com/premium", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	resp.Body.Close()

	bodyStr := string(body)
	if strings.Contains(bodyStr, "Premium is not available in your country") {
		return "", nil
	}
	if strings.Contains(bodyStr, "ad-free") || strings.Contains(bodyStr, "YouTube Premium") {
		return "YES", nil
	}
	return "", nil
}

// checkOpenAI returns true if OpenAI API is reachable.
func checkOpenAI(client *http.Client) (bool, error) {
	resp, err := client.Get("https://api.openai.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 401, nil
}

// checkClaude returns true if Anthropic Claude API is reachable.
func checkClaude(client *http.Client) (bool, error) {
	resp, err := client.Get("https://api.anthropic.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 404, nil
}

// checkGemini returns true if Google Gemini API is reachable.
func checkGemini(client *http.Client) (bool, error) {
	resp, err := client.Get("https://generativelanguage.googleapis.com/")
	if err != nil {
		return false, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 400, nil
}

// checkDisney returns true if Disney+ is accessible in the proxy's region.
func checkDisney(client *http.Client) (bool, error) {
	resp, err := client.Get("https://www.disneyplus.com/")
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

// checkTikTok returns "YES" if TikTok is accessible, else "".
func checkTikTok(client *http.Client) (string, error) {
	resp, err := client.Get("https://www.tiktok.com/")
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	resp.Body.Close()
	if strings.Contains(string(body), "tiktok") && resp.StatusCode == 200 {
		return "YES", nil
	}
	return "", nil
}
