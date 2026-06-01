// services/checker/fetch.go
package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/metacubex/mihomo/common/convert"
	"gopkg.in/yaml.v3"
)

// SubscriptionFetcher resolves a subscription URL to a list of proxy maps.
// The default adapter ([HTTPSubscriptionFetcher]) performs an HTTP GET; tests
// inject in-memory adapters by overriding [defaultFetcher].
type SubscriptionFetcher interface {
	Fetch(ctx context.Context, url string) ([]map[string]any, error)
}

// HTTPSubscriptionFetcher fetches via HTTP GET. Respects HTTP_PROXY/HTTPS_PROXY
// so region-restricted URLs work through a local proxy.
type HTTPSubscriptionFetcher struct {
	Client *http.Client
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func (f *HTTPSubscriptionFetcher) Fetch(ctx context.Context, url string) ([]map[string]any, error) {
	client := f.Client
	if client == nil {
		client = newHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "ClashMeta/1.19")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseProxies(data)
}

// defaultFetcher is the package-level fetcher used by runJob.
// Tests can swap this for a stub.
var defaultFetcher SubscriptionFetcher = &HTTPSubscriptionFetcher{}

// parseProxies tries Clash YAML first, then V2Ray format.
func parseProxies(data []byte) ([]map[string]any, error) {
	var clash struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &clash); err == nil && clash.Proxies != nil {
		return clash.Proxies, nil
	}

	proxyList, err := convert.ConvertsV2Ray(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse as Clash YAML or V2Ray format: %w", err)
	}
	return proxyList, nil
}
