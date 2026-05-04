package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
)

// runJSRule runs a JS or TS script rule using goja.
// Scripts receive an `http_get(url, opts?)` function and must return a bool value.
func runJSRule(ctx context.Context, client *http.Client, ruleType string, defRaw json.RawMessage) (bool, error) {
	var def ScriptDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		return false, err
	}

	code := def.Code
	if ruleType == "ts" {
		var err error
		code, err = transpileTS(code)
		if err != nil {
			return false, fmt.Errorf("typescript transpile error: %w", err)
		}
	}

	// Wrap so a bare `return` at top level is valid.
	wrapped := "(function() {\n" + code + "\n})()"

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	if err := injectHTTPGet(ctx, vm, client); err != nil {
		return false, err
	}

	val, err := vm.RunString(wrapped)
	if err != nil {
		return false, fmt.Errorf("script error: %w", err)
	}
	return val.ToBoolean(), nil
}

// httpGetResult is the object returned to scripts by http_get().
type httpGetResult struct {
	Status   int    `json:"status"`
	Body     string `json:"body"`
	FinalURL string `json:"final_url"`
}

// injectHTTPGet registers the http_get() function in the goja VM.
func injectHTTPGet(ctx context.Context, vm *goja.Runtime, client *http.Client) error {
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

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		resp.Body.Close()

		return vm.ToValue(httpGetResult{
			Status:   resp.StatusCode,
			Body:     string(body),
			FinalURL: resp.Request.URL.String(),
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
