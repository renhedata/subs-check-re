package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	lua "github.com/yuin/gopher-lua"
)

// runLuaRule runs a Lua script rule using gopher-lua.
// Scripts have access to http_get(url, opts?) and must return a boolean value.
// Example: return http_get("https://example.com").status == 200
func runLuaRule(ctx context.Context, client *http.Client, defRaw json.RawMessage, dr *DebugRecorder) (bool, error) {
	var def ScriptDef
	if err := json.Unmarshal(defRaw, &def); err != nil {
		if dr != nil {
			dr.Error(err)
		}
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

		res := trackedHTTPGet(ctx, client, url, headers, dr)
		if res.Err != nil {
			return pushError(res.Err.Error())
		}

		t := L.NewTable()
		L.SetField(t, "status", lua.LNumber(res.Status))
		L.SetField(t, "body", lua.LString(res.Body))
		L.SetField(t, "final_url", lua.LString(res.FinalURL))
		L.SetField(t, "error", lua.LString(""))
		L.Push(t)
		return 1
	}))

	// Inject print function for debug logging
	if dr != nil {
		L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
			var msg string
			top := L.GetTop()
			for i := 1; i <= top; i++ {
				if i > 1 {
					msg += "\t"
				}
				msg += L.Get(i).String()
			}
			dr.Log(msg)
			return 0
		}))
	}

	if def.Prelude != "" {
		if err := L.DoString(def.Prelude); err != nil {
			if dr != nil {
				dr.Error(err)
			}
			return false, fmt.Errorf("lua prelude error: %w", err)
		}
	}
	if err := L.DoString(def.Code); err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return false, fmt.Errorf("lua error: %w", err)
	}

	ret := L.Get(-1)
	if b, ok := ret.(lua.LBool); ok {
		result := bool(b)
		if dr != nil {
			dr.Variable("return_value", result)
		}
		return result, nil
	}
	if ret == lua.LNil {
		if dr != nil {
			dr.Variable("return_value", false)
		}
		return false, nil
	}
	err := fmt.Errorf("lua script must return a boolean, got %s", ret.Type())
	if dr != nil {
		dr.Error(err)
	}
	return false, err
}

