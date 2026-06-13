package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

// runJSRule runs a JS or TS script rule using goja.
// Scripts receive an `http_get(url, opts?)` function and must return a bool value.
func runJSRule(ctx context.Context, client *http.Client, ruleType string, defRaw json.RawMessage, dr *DebugRecorder) (PlatformOutcome, error) {
	var def ScriptDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return PlatformOutcome{}, err
	}

	code := def.Prelude + "\n" + def.Code
	if ruleType == "ts" {
		var err error
		code, err = transpileTS(code)
		if err != nil {
			if dr != nil {
				dr.Error(err)
			}
			return PlatformOutcome{}, fmt.Errorf("typescript transpile error: %w", err)
		}
	}

	// Wrap so a bare `return` at top level is valid.
	wrapped := "(function() {\n" + code + "\n})()"

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	if err := injectHTTPGet(ctx, vm, client, dr); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return PlatformOutcome{}, err
	}

	// Inject console.log
	if dr != nil {
		_ = vm.Set("console", map[string]any{
			"log": func(args ...goja.Value) {
				var msg string
				for i, a := range args {
					if i > 0 {
						msg += " "
					}
					msg += a.String()
				}
				dr.Log(msg)
			},
		})
	}

	val, err := vm.RunString(wrapped)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return PlatformOutcome{}, fmt.Errorf("script error: %w", err)
	}
	result := val.ToBoolean()
	if dr != nil {
		dr.Variable("return_value", result)
	}
	return boolOutcome(result), nil
}

// httpGetResult is the object returned to scripts by http_get().
type httpGetResult struct {
	Status   int    `json:"status"`
	Body     string `json:"body"`
	FinalURL string `json:"final_url"`
}

// injectHTTPGet registers the http_get() function in the goja VM.
func injectHTTPGet(ctx context.Context, vm *goja.Runtime, client *http.Client, dr *DebugRecorder) error {
	fn := func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.ToValue("http_get requires a URL argument"))
		}
		url := call.Arguments[0].String()

		headers := map[string]string{}
		if len(call.Arguments) > 1 {
			if opts, ok := call.Arguments[1].Export().(map[string]interface{}); ok {
				if h, ok := opts["headers"]; ok {
					if hm, ok := h.(map[string]interface{}); ok {
						for k, v := range hm {
							headers[k] = fmt.Sprintf("%v", v)
						}
					}
				}
			}
		}

		res := trackedHTTPGet(ctx, client, url, headers, dr)
		if res.Err != nil {
			panic(vm.ToValue(res.Err.Error()))
		}
		return vm.ToValue(httpGetResult{
			Status:   res.Status,
			Body:     res.Body,
			FinalURL: res.FinalURL,
		})
	}
	return vm.Set("http_get", fn)
}

// transpileTS converts TypeScript to JavaScript using esbuild.
func transpileTS(code string) (string, error) {
	result := api.Transform(code, api.TransformOptions{
		Loader: api.LoaderTS,
		Target: api.ES2015,
	})
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("%s", result.Errors[0].Text)
	}
	return string(result.Code), nil
}
