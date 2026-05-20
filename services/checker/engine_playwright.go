package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var playwrightServiceURL = os.Getenv("PLAYWRIGHT_URL")

// PlaywrightDef defines a playwright rule.
type PlaywrightDef struct {
	URL     string `json:"url"`
	Script  string `json:"script"`
	Timeout int    `json:"timeout,omitempty"`
}

// playwrightExecuteRequest is the request body for the Playwright service.
type playwrightExecuteRequest struct {
	Script     string       `json:"script"`
	Proxy      *proxyConfig `json:"proxy,omitempty"`
	URL        string       `json:"url,omitempty"`
	Timeout    int          `json:"timeout,omitempty"`
	Screenshot bool         `json:"screenshot,omitempty"`
}

// proxyConfig holds proxy settings for the Playwright service.
type proxyConfig struct {
	Server   string `json:"server"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// playwrightExecuteResponse is the response from the Playwright service.
type playwrightExecuteResponse struct {
	OK         bool     `json:"ok"`
	Result     bool     `json:"result"`
	FinalURL   string   `json:"final_url,omitempty"`
	Title      string   `json:"title,omitempty"`
	Logs       []string `json:"logs"`
	Screenshot string   `json:"screenshot,omitempty"`
	Error      string   `json:"error,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

func runPlaywrightRule(ctx context.Context, client *http.Client, rule *PlatformRule, dr *DebugRecorder) (bool, error) {
	if dr != nil {
		dr.PlaywrightScript("playwright rule execution")
	}

	var def PlaywrightDef
	if err := json.Unmarshal(rule.Definition, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	if playwrightServiceURL == "" {
		err := fmt.Errorf("PLAYWRIGHT_URL not configured")
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	reqBody := playwrightExecuteRequest{
		Script:  def.Script,
		URL:     def.URL,
		Timeout: def.Timeout,
	}

	if reqBody.Timeout == 0 {
		reqBody.Timeout = 30000
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	if dr != nil {
		dr.Log(fmt.Sprintf("Sending request to Playwright service: %s", playwrightServiceURL))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", playwrightServiceURL+"/execute", bytes.NewReader(reqJSON))
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: time.Duration(reqBody.Timeout+5000) * time.Millisecond}
	resp, err := httpClient.Do(req)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	var result playwrightExecuteResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	if dr != nil {
		dr.PlaywrightResult(result.Result, result.Logs)
		if result.FinalURL != "" {
			dr.Variable("final_url", result.FinalURL)
		}
		if result.Title != "" {
			dr.Variable("page_title", result.Title)
		}
	}

	if !result.OK {
		return false, fmt.Errorf("playwright execution failed: %s", result.Error)
	}

	return result.Result, nil
}
