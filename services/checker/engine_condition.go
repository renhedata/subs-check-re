package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// runConditionRule evaluates an HTTP condition rule. The full request/response
// (headers + body + duration + final URL) is recorded into dr for the debug UI.
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

	res := trackedHTTPRequest(ctx, client, method, def.URL, def.Headers, nil, dr)
	if res.Err != nil {
		return false, res.Err
	}

	ok := true

	if def.StatusCode != 0 {
		matched := res.Status == def.StatusCode
		if !matched {
			ok = false
		}
		if dr != nil {
			dr.Condition(fmt.Sprintf("status_code == %d", def.StatusCode), matched)
		}
	}

	for _, s := range def.BodyContains {
		matched := strings.Contains(res.Body, s)
		if !matched {
			ok = false
		}
		if dr != nil {
			dr.Condition(fmt.Sprintf("body contains %q", s), matched)
		}
	}

	if len(def.BodyContainsAny) > 0 {
		found := false
		for _, s := range def.BodyContainsAny {
			if strings.Contains(res.Body, s) {
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
		present := strings.Contains(res.Body, s)
		if present {
			ok = false
		}
		if dr != nil {
			dr.Condition(fmt.Sprintf("body does not contain %q", s), !present)
		}
	}

	if def.FinalURLContains != "" {
		matched := strings.Contains(res.FinalURL, def.FinalURLContains)
		if !matched {
			ok = false
		}
		if dr != nil {
			dr.Condition(fmt.Sprintf("final_url contains %q", def.FinalURLContains), matched)
		}
	}

	if def.FinalURLNotContains != "" {
		present := strings.Contains(res.FinalURL, def.FinalURLNotContains)
		if present {
			ok = false
		}
		if dr != nil {
			dr.Condition(fmt.Sprintf("final_url does not contain %q", def.FinalURLNotContains), !present)
		}
	}

	if dr != nil {
		dr.Variable("return_value", ok)
	}
	return ok, nil
}
