package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// runConditionRule evaluates an HTTP condition rule.
func runConditionRule(ctx context.Context, client *http.Client, defRaw json.RawMessage, dr *DebugRecorder) (bool, error) {
	var def ConditionDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	method := def.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, def.URL, nil)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	for k, v := range def.Headers {
		req.Header.Set(k, v)
	}

	if dr != nil {
		dr.HTTPReq(def.URL, method, def.Headers)
	}

	resp, err := client.Do(req)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	resp.Body.Close()

	bodyStr := string(body)
	finalURL := resp.Request.URL.String()

	if dr != nil {
		hdrs := make(map[string]string, len(resp.Header))
		for k, vv := range resp.Header {
			if len(vv) > 0 {
				hdrs[k] = vv[0]
			}
		}
		dr.HTTPResp(resp.StatusCode, hdrs, bodyStr)
	}

	ok := true
	if def.StatusCode != 0 && resp.StatusCode != def.StatusCode {
		ok = false
		if dr != nil {
			dr.Condition(fmt.Sprintf("status_code == %d", def.StatusCode), false)
		}
	} else if def.StatusCode != 0 && dr != nil {
		dr.Condition(fmt.Sprintf("status_code == %d", def.StatusCode), true)
	}
	for _, s := range def.BodyContains {
		if !strings.Contains(bodyStr, s) {
			ok = false
			if dr != nil {
				dr.Condition(fmt.Sprintf("body contains %q", s), false)
			}
		} else if dr != nil {
			dr.Condition(fmt.Sprintf("body contains %q", s), true)
		}
	}
	if len(def.BodyContainsAny) > 0 {
		found := false
		for _, s := range def.BodyContainsAny {
			if strings.Contains(bodyStr, s) {
				found = true
				if dr != nil {
					dr.Condition(fmt.Sprintf("body contains any %q", s), true)
				}
				break
			}
		}
		if !found {
			ok = false
			if dr != nil {
				dr.Condition("body contains any of specified strings", false)
			}
		}
	}
	for _, s := range def.BodyNotContains {
		if strings.Contains(bodyStr, s) {
			ok = false
			if dr != nil {
				dr.Condition(fmt.Sprintf("body does not contain %q", s), false)
			}
		} else if dr != nil {
			dr.Condition(fmt.Sprintf("body does not contain %q", s), true)
		}
	}
	if def.FinalURLContains != "" && !strings.Contains(finalURL, def.FinalURLContains) {
		ok = false
		if dr != nil {
			dr.Condition(fmt.Sprintf("final_url contains %q", def.FinalURLContains), false)
		}
	} else if def.FinalURLContains != "" && dr != nil {
		dr.Condition(fmt.Sprintf("final_url contains %q", def.FinalURLContains), true)
	}
	if def.FinalURLNotContains != "" && strings.Contains(finalURL, def.FinalURLNotContains) {
		ok = false
		if dr != nil {
			dr.Condition(fmt.Sprintf("final_url does not contain %q", def.FinalURLNotContains), false)
		}
	} else if def.FinalURLNotContains != "" && dr != nil {
		dr.Condition(fmt.Sprintf("final_url does not contain %q", def.FinalURLNotContains), true)
	}
	if dr != nil {
		dr.Variable("return_value", ok)
	}
	return ok, nil
}

type conditionDebug struct {
	ok              bool
	statusCode      int
	finalURL        string
	body            string
	responseHeaders map[string]string
}

// testConditionVerbose is like runConditionRule but also returns HTTP response details for the test UI.
func testConditionVerbose(ctx context.Context, client *http.Client, defRaw json.RawMessage) (*conditionDebug, error) {
	var def ConditionDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		return nil, err
	}

	method := def.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, def.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range def.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	resp.Body.Close()

	bodyStr := string(body)
	finalURL := resp.Request.URL.String()

	// Collect response headers (canonicalized, one value per key).
	hdrs := make(map[string]string, len(resp.Header))
	for k, vv := range resp.Header {
		if len(vv) > 0 {
			hdrs[k] = vv[0]
		}
	}

	ok := true
	if def.StatusCode != 0 && resp.StatusCode != def.StatusCode {
		ok = false
	}
	for _, s := range def.BodyContains {
		if !strings.Contains(bodyStr, s) {
			ok = false
		}
	}
	if ok && len(def.BodyContainsAny) > 0 {
		found := false
		for _, s := range def.BodyContainsAny {
			if strings.Contains(bodyStr, s) {
				found = true
				break
			}
		}
		if !found {
			ok = false
		}
	}
	for _, s := range def.BodyNotContains {
		if strings.Contains(bodyStr, s) {
			ok = false
		}
	}
	if def.FinalURLContains != "" && !strings.Contains(finalURL, def.FinalURLContains) {
		ok = false
	}
	if def.FinalURLNotContains != "" && strings.Contains(finalURL, def.FinalURLNotContains) {
		ok = false
	}

	return &conditionDebug{
		ok:              ok,
		statusCode:      resp.StatusCode,
		finalURL:        finalURL,
		body:            bodyStr,
		responseHeaders: hdrs,
	}, nil
}
