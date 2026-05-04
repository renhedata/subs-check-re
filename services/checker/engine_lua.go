package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	lua "github.com/yuin/gopher-lua"
)

// runLuaRule runs a Lua script rule using gopher-lua.
// Scripts have access to http_get(url, opts?) and must return a boolean value.
// Example: return http_get("https://example.com").status == 200
func runLuaRule(ctx context.Context, client *http.Client, defRaw json.RawMessage) (bool, error) {
	var def ScriptDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		return false, err
	}

	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("http_get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		headers := map[string]string{}

		if L.GetTop() >= 2 {
			if opts, ok := L.Get(2).(*lua.LTable); ok {
				if h := opts.RawGetString("headers"); h != lua.LNil {
					if ht, ok := h.(*lua.LTable); ok {
						ht.ForEach(func(k, v lua.LValue) {
							headers[k.String()] = v.String()
						})
					}
				}
			}
		}

		pushError := func(msg string) int {
			t := L.NewTable()
			L.SetField(t, "status", lua.LNumber(0))
			L.SetField(t, "body", lua.LString(""))
			L.SetField(t, "final_url", lua.LString(""))
			L.SetField(t, "error", lua.LString(msg))
			L.Push(t)
			return 1
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return pushError(err.Error())
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			return pushError(err.Error())
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		resp.Body.Close()

		t := L.NewTable()
		L.SetField(t, "status", lua.LNumber(resp.StatusCode))
		L.SetField(t, "body", lua.LString(string(body)))
		L.SetField(t, "final_url", lua.LString(resp.Request.URL.String()))
		L.SetField(t, "error", lua.LString(""))
		L.Push(t)
		return 1
	}))

	if err := L.DoString(def.Code); err != nil {
		return false, fmt.Errorf("lua error: %w", err)
	}

	ret := L.Get(-1)
	if b, ok := ret.(lua.LBool); ok {
		return bool(b), nil
	}
	if ret == lua.LNil {
		return false, nil
	}
	return false, fmt.Errorf("lua script must return a boolean, got %s", ret.Type())
}
