package checker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// runConditionRule evaluates an HTTP condition rule.
func runConditionRule(ctx context.Context, client *http.Client, defRaw json.RawMessage) (bool, error) {
	var def ConditionDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		return false, err
	}

	method := def.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, def.URL, nil)
	if err != nil {
		return false, err
	}
	for k, v := range def.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	resp.Body.Close()

	bodyStr := string(body)
	finalURL := resp.Request.URL.String()

	if def.StatusCode != 0 && resp.StatusCode != def.StatusCode {
		return false, nil
	}
	for _, s := range def.BodyContains {
		if !strings.Contains(bodyStr, s) {
			return false, nil
		}
	}
	if len(def.BodyContainsAny) > 0 {
		found := false
		for _, s := range def.BodyContainsAny {
			if strings.Contains(bodyStr, s) {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	for _, s := range def.BodyNotContains {
		if strings.Contains(bodyStr, s) {
			return false, nil
		}
	}
	if def.FinalURLContains != "" && !strings.Contains(finalURL, def.FinalURLContains) {
		return false, nil
	}
	if def.FinalURLNotContains != "" && strings.Contains(finalURL, def.FinalURLNotContains) {
		return false, nil
	}
	return true, nil
}
