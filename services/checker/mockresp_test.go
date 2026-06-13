package checker

import (
	"io"
	"net/http"
	"strings"
)

type mockResp struct {
	status  int
	body    string
	headers map[string]string
	// finalURL, when set, becomes resp.Request.URL so rules see a post-redirect URL.
	finalURL string
}

func (m mockResp) toResponse(r *http.Request) *http.Response {
	h := http.Header{}
	for k, v := range m.headers {
		h.Set(k, v)
	}
	req := r
	if m.finalURL != "" {
		u := *r
		parsed := r.URL
		if p, err := parsed.Parse(m.finalURL); err == nil {
			u.URL = p
		}
		req = &u
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     h,
		Request:    req,
	}
}
