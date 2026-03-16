// services/checker/mihomo.go
package checker

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
)

const (
	proxyTimeout = 10 * time.Second
	aliveTestURL = "http://www.gstatic.com/generate_204"
	ipLookupURL  = "http://ip-api.com/json/?fields=query,countryCode"
)

// proxyClient wraps an HTTP client and its underlying mihomo proxy.
type proxyClient struct {
	*http.Client
	proxy constant.Proxy
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

	return &proxyClient{
		Client: &http.Client{
			Timeout:   proxyTimeout,
			Transport: transport,
		},
		proxy: proxy,
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
	return resp.StatusCode >= 200 && resp.StatusCode < 302
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
	if resp.StatusCode < 200 || resp.StatusCode >= 302 {
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
	NodeID    string
	NodeName  string
	Alive     bool
	LatencyMs int
	SpeedKbps int
	IP        string
	Country   string
	Netflix   bool
	YouTube   string
	OpenAI    bool
	Claude    bool
	Gemini    bool
	Disney    bool
	TikTok    string
}

// checkNode runs all checks for a single proxy mapping and returns the result.
func checkNode(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL string) nodeCheckResult {
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
	result.SpeedKbps = measureSpeed(ctx, pc.Client.Transport, speedTestURL)

	// Reuse same transport with shorter timeout for media checks
	mediaClient := &http.Client{
		Transport: pc.Transport,
		Timeout:   8 * time.Second,
	}

	result.IP, result.Country = getProxyInfo(ctx, mediaClient)
	result.Netflix, _ = checkNetflix(ctx, mediaClient)
	result.YouTube, _ = checkYouTube(ctx, mediaClient)
	result.OpenAI, _ = checkOpenAI(ctx, mediaClient)
	result.Claude, _ = checkClaude(ctx, mediaClient)
	result.Gemini, _ = checkGemini(ctx, mediaClient)
	result.Disney, _ = checkDisney(ctx, mediaClient)
	result.TikTok, _ = checkTikTok(ctx, mediaClient)

	return result
}
