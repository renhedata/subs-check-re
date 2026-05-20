package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
)

// runTengoRule runs a Tengo script rule.
// Scripts have access to an `http_get` variable (callable) and must assign their result to `output`.
// Example: output := http_get("https://example.com").status == 200
func runTengoRule(ctx context.Context, client *http.Client, defRaw json.RawMessage, dr *DebugRecorder) (bool, error) {
	var def ScriptDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	fullCode := "output := false\n" + def.Prelude + "\n" + def.Code

	script := tengo.NewScript([]byte(fullCode))
	script.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))

	if err := script.Add("output", false); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	httpGetFn := &tengo.UserFunction{
		Name: "http_get",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("http_get requires a URL argument")
			}
			url, ok := tengo.ToString(args[0])
			if !ok {
				return nil, fmt.Errorf("http_get: URL must be a string")
			}

			headers := map[string]string{}
			if len(args) > 1 {
				if m, ok := args[1].(*tengo.Map); ok {
					if h, ok := m.Value["headers"]; ok {
						if hm, ok := h.(*tengo.Map); ok {
							for k, v := range hm.Value {
								if s, ok := tengo.ToString(v); ok {
									headers[k] = s
								}
							}
						}
					}
				}
			}

			if dr != nil {
				dr.HTTPReq(url, "GET", headers)
			}

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return errorResult(err.Error()), nil
			}
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				return errorResult(err.Error()), nil
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			resp.Body.Close()

			if dr != nil {
				dr.HTTPResp(resp.StatusCode, nil, string(body))
			}

			return &tengo.Map{Value: map[string]tengo.Object{
				"status":    &tengo.Int{Value: int64(resp.StatusCode)},
				"body":      &tengo.String{Value: string(body)},
				"final_url": &tengo.String{Value: resp.Request.URL.String()},
				"error":     &tengo.String{Value: ""},
			}}, nil
		},
	}

	if err := script.Add("http_get", httpGetFn); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, err
	}

	// Inject print functions for debug logging
	if dr != nil {
		printFn := &tengo.UserFunction{
			Name: "tprint",
			Value: func(args ...tengo.Object) (tengo.Object, error) {
				var msg string
				for i, a := range args {
					if i > 0 {
						msg += " "
					}
					msg += a.String()
				}
				dr.Log(msg)
				return nil, nil
			},
		}
		_ = script.Add("tprint", printFn)
		_ = script.Add("println", printFn)
	}

	compiled, err := script.Compile()
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, fmt.Errorf("tengo compile: %w", err)
	}
	if err := compiled.RunContext(ctx); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, fmt.Errorf("tengo runtime: %w", err)
	}

	outVar := compiled.Get("output")
	if outVar == nil {
		if dr != nil {
			dr.Variable("return_value", false)
		}
		return false, nil
	}
	val, _ := outVar.Value().(bool)
	if dr != nil {
		dr.Variable("return_value", val)
	}
	return val, nil
}

func errorResult(msg string) *tengo.Map {
	return &tengo.Map{Value: map[string]tengo.Object{
		"status":    &tengo.Int{Value: 0},
		"body":      &tengo.String{Value: ""},
		"final_url": &tengo.String{Value: ""},
		"error":     &tengo.String{Value: msg},
	}}
}
