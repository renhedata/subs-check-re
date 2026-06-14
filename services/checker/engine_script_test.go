package checker

import (
	"context"
	"net/http"
	"testing"
)

func TestJSRule_ObjectReturnWithRegion(t *testing.T) {
	client := mockClient(map[string]mockResp{
		"https://api.test/loc": {status: 200, body: "loc=US"},
	})
	def := []byte(`{"code":"var r=http_get('https://api.test/loc'); var m=r.body.match(/loc=([A-Z]{2})/); return {unlocked:true, status:'Yes', region:m[1]};"}`)
	out, err := runJSRule(context.Background(), client, "js", def, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Unlocked || out.Status != "Yes" || out.Region != "US" {
		t.Fatalf("got %+v", out)
	}
}

func TestJSRule_HTTPPostAndHeaders(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) *http.Response {
		if r.Method == "POST" && r.URL.String() == "https://api.test/dev" {
			return mockResp{status: 200, body: `{"assertion":"abc"}`, headers: map[string]string{"x-region": "JP"}}.toResponse(r)
		}
		return mockResp{status: 404}.toResponse(r)
	})}
	def := []byte(`{"code":"var r=http_post('https://api.test/dev',{headers:{'authorization':'Bearer x'},body:'{}'}); return {unlocked:r.status===200, status:r.headers['x-region']||'', region:r.headers['x-region']||''};"}`)
	out, err := runJSRule(context.Background(), client, "js", def, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Unlocked || out.Region != "JP" {
		t.Fatalf("got %+v", out)
	}
}

func TestJSRule_BareBoolStillWorks(t *testing.T) {
	client := mockClient(map[string]mockResp{"https://api.test/x": {status: 200, body: "ok"}})
	def := []byte(`{"code":"var r=http_get('https://api.test/x'); return r.status===200;"}`)
	out, _ := runJSRule(context.Background(), client, "js", def, nil)
	if !out.Unlocked || out.Status != "Yes" {
		t.Fatalf("got %+v", out)
	}
}
