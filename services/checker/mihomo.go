// services/checker/mihomo.go
package checker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
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
func isAlive(ctx context.Context, client *http.Client) bool {
	resp, err := get(ctx, client, aliveTestURL)
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
func measureLatency(ctx context.Context, client *http.Client) int {
	start := time.Now()
	resp, err := get(ctx, client, aliveTestURL)
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

// nodeCheckResult holds the outcome of checking a single node.
type nodeCheckResult struct {
	NodeID         string
	NodeName       string
	Alive          bool
	LatencyMs      int
	SpeedKbps      int
	IP             string
	Country        string
	Netflix        bool
	YouTube        bool
	YouTubePremium bool
	OpenAI         bool
	Claude         bool
	Gemini         bool
	Grok           bool
	Disney         bool
	TikTok         bool
	TrafficBytes   int64
}

// checkNode runs all checks for a single proxy mapping and returns the result.
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL string, opts CheckOptions) nodeCheckResult {
	name, _ := mapping["name"].(string)
	result := nodeCheckResult{NodeID: nodeID, NodeName: name}

	pc := newProxyClient(mapping)
	if pc == nil {
		return result
	}
	defer pc.close()

	if !isAlive(ctx, pc.Client) {
		return result
	}
	result.Alive = true
	result.LatencyMs = measureLatency(ctx, pc.Client)
	if opts.SpeedTest {
		result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)
	}

	if len(opts.MediaApps) > 0 {
		mediaClient := &http.Client{
			Transport: pc.Transport,
			Timeout:   8 * time.Second,
		}
		result.IP, result.Country = getProxyInfo(ctx, mediaClient)
		if hasApp(opts, "netflix") {
			result.Netflix, _ = checkNetflix(ctx, mediaClient)
		}
		if hasApp(opts, "youtube") {
			result.YouTube, _ = checkYouTube(ctx, mediaClient)
			result.YouTubePremium, _ = checkYouTubePremium(ctx, mediaClient)
		}
		if hasApp(opts, "openai") {
			result.OpenAI, _ = checkOpenAI(ctx, mediaClient)
		}
		if hasApp(opts, "claude") {
			result.Claude, _ = checkClaude(ctx, mediaClient)
		}
		if hasApp(opts, "gemini") {
			result.Gemini, _ = checkGemini(ctx, mediaClient)
		}
		if hasApp(opts, "grok") {
			result.Grok, _ = checkGrok(ctx, mediaClient)
		}
		if hasApp(opts, "disney") {
			result.Disney, _ = checkDisney(ctx, mediaClient)
		}
		if hasApp(opts, "tiktok") {
			result.TikTok, _ = checkTikTok(ctx, mediaClient)
		}
	}

	if pc.counter != nil {
		result.TrafficBytes = atomic.LoadInt64(&pc.counter.bytes)
	}

	return result
}
