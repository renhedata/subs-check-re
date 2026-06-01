// services/checker/mihomo.go
package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

const (
	proxyTimeout = 10 * time.Second
	aliveTestURL = "http://www.gstatic.com/generate_204"
	ipLookupURL  = "http://ip-api.com/json/?fields=query,countryCode"
)

// countingTransport wraps an http.RoundTripper and counts response body bytes read.
type countingTransport struct {
	base  http.RoundTripper
	bytes int64
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	resp.Body = &countingReader{ReadCloser: resp.Body, n: &t.bytes}
	return resp, nil
}

type countingReader struct {
	io.ReadCloser
	n *int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	atomic.AddInt64(r.n, int64(n))
	return n, err
}

// proxyClient wraps an HTTP client and its underlying mihomo proxy.
type proxyClient struct {
	*http.Client
	proxy   constant.Proxy
	counter *countingTransport
}

// close releases proxy resources.
func (pc *proxyClient) close() {
	if pc.Client != nil {
		pc.Client.CloseIdleConnections()
	}
	if pc.proxy != nil {
		pc.proxy.Close()
	}
}

// newProxyClient creates an HTTP client that routes through the given proxy map.
// Returns nil if the proxy config is invalid.
func newProxyClient(mapping map[string]any) *proxyClient {
	proxy, err := adapter.ParseProxy(mapping)
	if err != nil {
		return nil
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			port, err := strconv.ParseUint(portStr, 10, 16)
			if err != nil {
				return nil, err
			}
			return proxy.DialContext(ctx, &constant.Metadata{
				Host:    host,
				DstPort: uint16(port),
			})
		},
		DisableKeepAlives: true,
	}

	ct := &countingTransport{base: transport}
	return &proxyClient{
		Client: &http.Client{
			Timeout:   proxyTimeout,
			Transport: ct,
		},
		proxy:   proxy,
		counter: ct,
	}
}

// get performs a GET request using the given context, honoring cancellation.
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// isAlive returns true if the proxy can reach the connectivity test URL.
func isAlive(ctx context.Context, client *http.Client, testURL string) bool {
	url := testURL
	if url == "" {
		url = aliveTestURL
	}
	resp, err := get(ctx, client, url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// getProxyInfo retrieves the external IP and country code via the proxy.
func getProxyInfo(ctx context.Context, client *http.Client) (ip, country string) {
	resp, err := get(ctx, client, ipLookupURL)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", ""
	}

	var result struct {
		Query       string `json:"query"`
		CountryCode string `json:"countryCode"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ""
	}
	return result.Query, result.CountryCode
}

// measureLatency measures round-trip latency in milliseconds.
func measureLatency(ctx context.Context, client *http.Client, testURL string) int {
	url := testURL
	if url == "" {
		url = aliveTestURL
	}
	start := time.Now()
	resp, err := get(ctx, client, url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0
	}
	return int(time.Since(start).Milliseconds())
}

const speedTestTimeout = 30 * time.Second

// measureSpeed downloads a fixed-size file and returns throughput in KB/s.
func measureSpeed(ctx context.Context, transport http.RoundTripper, speedTestURL string) int {
	return measureSpeedWithTimeout(ctx, transport, speedTestURL, speedTestTimeout)
}

func measureSpeedWithTimeout(ctx context.Context, transport http.RoundTripper, speedTestURL string, timeout time.Duration) int {
	client := &http.Client{Timeout: timeout, Transport: transport}
	resp, err := get(ctx, client, speedTestURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	startDownload := time.Now()
	n, _ := io.Copy(io.Discard, resp.Body)
	if n == 0 {
		return 0
	}
	elapsed := time.Since(startDownload).Seconds()
	if elapsed == 0 {
		return 0
	}
	return int(float64(n) / 1024 / elapsed)
}

const uploadTestSize = 1 * 1024 * 1024 // 1 MB

// uploadURLFrom derives an upload endpoint from the configured speed-test URL.
// For Cloudflare /__down URLs it returns the /__up sibling on the same host.
// For anything else it falls back to the Cloudflare default.
func uploadURLFrom(speedTestURL string) string {
	u, err := url.Parse(speedTestURL)
	if err == nil && u.Host != "" {
		u.RawQuery = ""
		u.Path = "/__up"
		return u.String()
	}
	return "https://speed.cloudflare.com/__up"
}

// measureUploadSpeed sends a fixed-size payload through the proxy and returns
// throughput in KB/s. It measures only the body-send window to avoid counting
// TCP/TLS setup time. uploadTestURL overrides the derived URL when non-empty.
func measureUploadSpeed(ctx context.Context, transport http.RoundTripper, speedTestURL, uploadTestURL string) int {
	uploadURL := uploadTestURL
	if uploadURL == "" {
		uploadURL = uploadURLFrom(speedTestURL)
	}
	payload := make([]byte, uploadTestSize)

	// pr/pw pipe lets us start the timer exactly when body bytes start flowing,
	// not when the TCP connection is being established.
	pr, pw := io.Pipe()
	var start time.Time
	var sent int64

	go func() {
		start = time.Now()
		n, _ := io.Copy(pw, bytes.NewReader(payload))
		sent = n
		pw.Close()
	}()

	client := &http.Client{Timeout: speedTestTimeout, Transport: transport}
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, pr)
	if err != nil {
		pr.CloseWithError(err)
		return 0
	}
	req.ContentLength = int64(len(payload))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	elapsed := time.Since(start).Seconds()
	if err != nil || elapsed == 0 || sent == 0 {
		return 0
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return int(float64(sent) / 1024 / elapsed)
}

// nodeCheckResult holds the outcome of checking a single node.
type nodeCheckResult struct {
	NodeID          string
	NodeName        string
	Alive           bool
	LatencyMs       int
	SpeedKbps       int
	UploadSpeedKbps int
	IP              string
	Country         string
	Netflix         bool
	YouTube         bool
	YouTubePremium  bool
	OpenAI          bool
	Claude          bool
	Gemini          bool
	Grok            bool
	Disney          bool
	TikTok          bool
	TrafficBytes    int64
	ExtraPlatforms  map[string]bool
	Debug           *NodeDebug
}

// checkNode runs all checks for a single proxy mapping and returns the result.
// User rules override the corresponding built-in checks; rules with non-builtin
// keys are stored in ExtraPlatforms.
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL, uploadTestURL, latencyTestURL string, opts CheckOptions, rules []*PlatformRule) nodeCheckResult {
	name, _ := mapping["name"].(string)
	result := nodeCheckResult{NodeID: nodeID, NodeName: name}

	if opts.Debug {
		result.Debug = &NodeDebug{NodeID: nodeID, NodeName: name}
	}

	pc := newProxyClient(mapping)
	if pc == nil {
		if opts.Debug && result.Debug != nil {
			result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
				Platform: "connectivity",
				Result:   false,
				Steps:    []DebugStep{{Type: "error", Description: "failed to create proxy client", Details: toRawMessage(map[string]any{"error": "invalid proxy config"})}},
			})
		}
		return result
	}
	defer pc.close()

	if !isAlive(ctx, pc.Client, latencyTestURL) {
		if opts.Debug && result.Debug != nil {
			result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
				Platform: "connectivity",
				Result:   false,
				Steps:    []DebugStep{{Type: "variable", Description: "alive = false", Details: toRawMessage(map[string]any{"name": "alive", "value": false})}},
			})
		}
		return result
	}
	result.Alive = true
	result.LatencyMs = measureLatency(ctx, pc.Client, latencyTestURL)
	if opts.Debug && result.Debug != nil {
		result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
			Platform: "connectivity",
			Result:   true,
			Steps: []DebugStep{
				{Type: "variable", Description: "alive = true", Details: toRawMessage(map[string]any{"name": "alive", "value": true})},
				{Type: "variable", Description: "latency_ms", Details: toRawMessage(map[string]any{"name": "latency_ms", "value": result.LatencyMs})},
			},
		})
	}
	if opts.SpeedTest {
		result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)
	}
	if opts.UploadSpeedTest {
		result.UploadSpeedKbps = measureUploadSpeed(ctx, pc.Client.Transport, speedTestURL, uploadTestURL)
	}

	if len(opts.MediaApps) > 0 {
		mediaClient := &http.Client{
			Transport: pc.Transport,
			Timeout:   8 * time.Second,
		}
		result.IP, result.Country = getProxyInfo(ctx, mediaClient)

		var ruleRecorders map[string]*DebugRecorder
		if opts.Debug {
			ruleRecorders = make(map[string]*DebugRecorder)
		}
		ruleResults := runUserRulesWithDebug(ctx, mediaClient, rules, ruleRecorders)

		extra := make(map[string]bool)
		for k, v := range ruleResults {
			if opts.Debug && result.Debug != nil {
				if rd, ok := ruleRecorders[k]; ok {
					result.Debug.Traces = append(result.Debug.Traces, DebugTrace{
						Platform: k, Result: v, Steps: rd.Steps,
					})
				}
			}
			if !builtinKeys[k] {
				extra[k] = v
			}
		}
		if len(extra) > 0 {
			result.ExtraPlatforms = extra
		}

		// Surface builtin platform results from rule engine output. If a user disabled
		// or deleted the rule for a key, that platform simply isn't checked — there's
		// no Go fallback. Default rules are seeded into platform_rules on first ListRules.
		if hasApp(opts, "netflix") {
			result.Netflix = ruleResults["netflix"]
		}
		if hasApp(opts, "youtube") {
			result.YouTube = ruleResults["youtube"]
			result.YouTubePremium = ruleResults["youtube_premium"]
		}
		if hasApp(opts, "openai") {
			result.OpenAI = ruleResults["openai"]
		}
		if hasApp(opts, "claude") {
			result.Claude = ruleResults["claude"]
		}
		if hasApp(opts, "gemini") {
			result.Gemini = ruleResults["gemini"]
		}
		if hasApp(opts, "grok") {
			result.Grok = ruleResults["grok"]
		}
		if hasApp(opts, "disney") {
			result.Disney = ruleResults["disney"]
		}
		if hasApp(opts, "tiktok") {
			result.TikTok = ruleResults["tiktok"]
		}
	}

	if pc.counter != nil {
		result.TrafficBytes = atomic.LoadInt64(&pc.counter.bytes)
	}

	return result
}
