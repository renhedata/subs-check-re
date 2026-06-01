package checker

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Maximum body bytes captured for the debug trace. Larger responses are
// truncated (with a marker appended) so the UI doesn't try to render
// hundreds of MB into the trace JSON.
const maxDebugBodyBytes = 1024 * 1024 // 1 MB

// httpRequestResult is the canonical result of a tracked HTTP request inside a rule engine.
type httpRequestResult struct {
	Status   int
	Body     string
	FinalURL string
	Err      error
}

// trackedHTTPGet performs an HTTP GET, records request + response into the DebugRecorder
// (full headers, full body, duration), and returns the result.
func trackedHTTPGet(ctx context.Context, client *http.Client, url string, headers map[string]string, dr *DebugRecorder) httpRequestResult {
	return trackedHTTPRequest(ctx, client, "GET", url, headers, nil, dr)
}

// trackedHTTPRequest is the general form. body is the request body (nil for GET/HEAD).
func trackedHTTPRequest(ctx context.Context, client *http.Client, method, url string, headers map[string]string, body []byte, dr *DebugRecorder) httpRequestResult {
	if dr != nil {
		dr.HTTPReq(method, url, headers, string(body))
	}

	start := time.Now()
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return httpRequestResult{Err: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return httpRequestResult{Err: err}
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxDebugBodyBytes+1))
	if err != nil {
		if dr != nil {
			dr.Error(err)
		}
		return httpRequestResult{Err: err}
	}
	truncated := len(rawBody) > maxDebugBodyBytes
	if truncated {
		rawBody = append(rawBody[:maxDebugBodyBytes], []byte("\n\n[...truncated — body exceeded 1 MB debug cap...]")...)
	}
	durationMs := time.Since(start).Milliseconds()

	if dr != nil {
		dr.HTTPResp(resp.StatusCode, flattenHeaders(resp.Header), string(rawBody), durationMs, resp.Request.URL.String())
	}

	return httpRequestResult{
		Status:   resp.StatusCode,
		Body:     string(rawBody),
		FinalURL: resp.Request.URL.String(),
	}
}

// flattenHeaders converts http.Header (multi-value) to a flat map suitable for JSON
// transport to the frontend. Multiple values are joined with ", ".
func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) == 0 {
			continue
		}
		out[k] = strings.Join(v, ", ")
	}
	return out
}
