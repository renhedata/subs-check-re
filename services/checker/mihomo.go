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
	proxyTimeout = 15 * time.Second
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

// isAlive returns true if the proxy can reach the connectivity test URL.
func isAlive(client *http.Client) bool {
	resp, err := client.Get(aliveTestURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 302
}

// getProxyInfo retrieves the external IP and country code via the proxy.
func getProxyInfo(client *http.Client) (ip, country string) {
	resp, err := client.Get(ipLookupURL)
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
func measureLatency(client *http.Client) int {
	start := time.Now()
	resp, err := client.Get(aliveTestURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 302 {
		return 0
	}
	return int(time.Since(start).Milliseconds())
}

// nodeCheckResult holds the outcome of checking a single node.
type nodeCheckResult struct {
	NodeID    string
	NodeName  string
	Alive     bool
	LatencyMs int
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
func checkNode(nodeID string, mapping map[string]any) nodeCheckResult {
	name, _ := mapping["name"].(string)
	result := nodeCheckResult{NodeID: nodeID, NodeName: name}

	pc := newProxyClient(mapping)
	if pc == nil {
		return result
	}
	defer pc.close()

	if !isAlive(pc.Client) {
		return result
	}
	result.Alive = true
	result.LatencyMs = measureLatency(pc.Client)

	// Reuse same transport with shorter timeout for media checks
	mediaClient := &http.Client{
		Transport: pc.Transport,
		Timeout:   10 * time.Second,
	}

	result.IP, result.Country = getProxyInfo(mediaClient)
	result.Netflix, _ = checkNetflix(mediaClient)
	result.YouTube, _ = checkYouTube(mediaClient)
	result.OpenAI, _ = checkOpenAI(mediaClient)
	result.Claude, _ = checkClaude(mediaClient)
	result.Gemini, _ = checkGemini(mediaClient)
	result.Disney, _ = checkDisney(mediaClient)
	result.TikTok, _ = checkTikTok(mediaClient)

	return result
}
