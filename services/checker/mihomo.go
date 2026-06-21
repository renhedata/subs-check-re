// services/checker/mihomo.go
package checker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

const (
	proxyTimeout = 10 * time.Second
	aliveTestURL = "http://cp.cloudflare.com/generate_204"
	ipLookupURL  = "http://ip-api.com/json/?fields=query,countryCode"
)

// Test seams: production points at the real network probes; tests override
// these to drive retry logic without real I/O.
var probeLatencyFn = probeLatency

const aliveProbeAttempts = 3

// aliveProbeBackoff is the pause between failed alive probes; var so tests shrink it.
var aliveProbeBackoff = 300 * time.Millisecond

// probeLatencyWithRetry retries the single-shot alive probe up to
// aliveProbeAttempts times, returning on the first success. A genuinely alive
// node that drops one probe (a transient blip, or a handshake that just missed
// the client timeout) is no longer misrecorded as dead. Context cancellation
// aborts immediately.
func probeLatencyWithRetry(ctx context.Context, client *http.Client, testURL string) (bool, int) {
	for attempt := 1; attempt <= aliveProbeAttempts; attempt++ {
		if alive, ms := probeLatencyFn(ctx, client, testURL); alive {
			return true, ms
		}
		if attempt == aliveProbeAttempts || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			return false, 0
		case <-time.After(aliveProbeBackoff):
		}
	}
	return false, 0
}

var (
	measureSpeedFn  = measureSpeed
	measureUploadFn = measureUploadSpeed
	getProxyInfoFn  = getProxyInfo
)

const speedTestAttempts = 2

// measureSpeedWithRetry retries the download test once when it returns 0 (a
// transient download failure), bounded by speedTestAttempts. A node that is
// alive but genuinely slow returns 0 after the retries are exhausted.
func measureSpeedWithRetry(ctx context.Context, transport http.RoundTripper, speedTestURL string) int {
	var kbps int
	for attempt := 1; attempt <= speedTestAttempts; attempt++ {
		kbps = measureSpeedFn(ctx, transport, speedTestURL)
		if kbps > 0 || ctx.Err() != nil {
			break
		}
	}
	return kbps
}

// measureUploadWithRetry mirrors measureSpeedWithRetry for the upload test.
func measureUploadWithRetry(ctx context.Context, transport http.RoundTripper, speedTestURL, uploadTestURL string) int {
	var kbps int
	for attempt := 1; attempt <= speedTestAttempts; attempt++ {
		kbps = measureUploadFn(ctx, transport, speedTestURL, uploadTestURL)
		if kbps > 0 || ctx.Err() != nil {
			break
		}
	}
	return kbps
}

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

// proxyTransport builds an http.Transport that dials through the given mihomo
// proxy. Returns the proxy so the caller can Close() it.
func proxyTransport(mapping map[string]any) (*http.Transport, constant.Proxy, error) {
	proxy, err := adapter.ParseProxy(mapping)
	if err != nil {
		return nil, nil, err
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
	return transport, proxy, nil
}

// newProxyClient creates an HTTP client that routes through the given proxy map.
// Returns nil if the proxy config is invalid.
func newProxyClient(mapping map[string]any) *proxyClient {
	transport, proxy, err := proxyTransport(mapping)
	if err != nil {
		return nil
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

// proxyHTTPClient builds an http.Client that tunnels through the given proxy
// config, used to fetch a subscription URL the server can't reach directly. The
// returned closer releases the proxy. TLS verification is relaxed to match the
// direct-fetch client.
func proxyHTTPClient(mapping map[string]any, timeout time.Duration) (*http.Client, func(), error) {
	transport, proxy, err := proxyTransport(mapping)
	if err != nil {
		return nil, nil, err
	}
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Timeout: timeout, Transport: transport}
	closer := func() {
		client.CloseIdleConnections()
		proxy.Close()
	}
	return client, closer, nil
}

// get performs a GET request using the given context, honoring cancellation.
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// probeLatency does a single GET to the connectivity URL and returns whether the
// proxy is alive plus the round-trip latency in ms. Matches clash-verge-rev's
// single-request delay test. ms is 0 when not alive.
func probeLatency(ctx context.Context, client *http.Client, testURL string) (alive bool, ms int) {
	url := testURL
	if url == "" {
		url = aliveTestURL
	}
	start := time.Now()
	resp, err := get(ctx, client, url)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false, 0
	}
	return true, int(time.Since(start).Milliseconds())
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
	Platforms       map[string]PlatformOutcome
	TrafficBytes    int64
	Debug           *NodeDebug
}

// checkNode runs all checks for a single proxy mapping and returns the result.
// Every enabled rule is evaluated and its outcome stored in result.Platforms,
// keyed by rule key.
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

	alive, latency := probeLatencyWithRetry(ctx, pc.Client, latencyTestURL)
	if !alive {
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
	result.LatencyMs = latency
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
		result.SpeedKbps = measureSpeedWithRetry(ctx, pc.Client.Transport, speedTestURL)
	}
	if opts.UploadSpeedTest {
		result.UploadSpeedKbps = measureUploadWithRetry(ctx, pc.Client.Transport, speedTestURL, uploadTestURL)
	}

	if len(opts.MediaApps) > 0 {
		jar, _ := cookiejar.New(nil)
		mediaClient := &http.Client{
			Transport: pc.Transport,
			Timeout:   8 * time.Second,
			Jar:       jar,
		}
		result.IP, result.Country = getProxyInfoFn(ctx, mediaClient)

		var ruleRecorders map[string]*DebugRecorder
		if opts.Debug {
			ruleRecorders = make(map[string]*DebugRecorder)
		}
		outcomes := runUserRulesWithDebug(ctx, mediaClient, rules, ruleRecorders)
		result.Platforms = make(map[string]PlatformOutcome, len(outcomes))
		for k, v := range outcomes {
			if opts.Debug && result.Debug != nil {
				if rd, ok := ruleRecorders[k]; ok {
					result.Debug.Traces = append(result.Debug.Traces, DebugTrace{Platform: k, Result: v.Unlocked, Steps: rd.Steps})
				}
			}
			result.Platforms[k] = v
		}
	}

	if pc.counter != nil {
		result.TrafficBytes = atomic.LoadInt64(&pc.counter.bytes)
	}

	return result
}
