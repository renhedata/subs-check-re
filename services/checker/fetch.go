// services/checker/fetch.go
package checker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"encore.dev/rlog"
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
	// ProxyConfig, when set, tunnels the fetch through this mihomo proxy node
	// (for subscription URLs the server can't reach directly).
	ProxyConfig map[string]any
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
		if len(f.ProxyConfig) > 0 {
			c, closeProxy, err := proxyHTTPClient(f.ProxyConfig, 30*time.Second)
			if err != nil {
				return nil, fmt.Errorf("build proxy for fetch: %w", err)
			}
			defer closeProxy()
			client = c
		} else {
			client = newHTTPClient()
		}
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
		err := fmt.Errorf("subscription returned status %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, permanent(err)
		}
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	proxies, err := parseProxies(data)
	if err != nil {
		return nil, permanent(err)
	}
	return proxies, nil
}

// permanentError marks a fetch failure retrying cannot fix (4xx, unparseable payload).
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func permanent(err error) error { return &permanentError{err: err} }

func isPermanentFetchError(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

const fetchAttempts = 3

// fetchBackoff is the initial retry delay (doubles per attempt); var so tests can shrink it.
var fetchBackoff = 1 * time.Second

// fetchWithRetry retries transient fetch failures (network errors, 5xx) with
// exponential backoff. Permanent failures and context cancellation abort
// immediately — a temporarily unreachable subscription source should not fail
// the whole check job on the first hiccup.
func fetchWithRetry(ctx context.Context, fetcher SubscriptionFetcher, url string) ([]map[string]any, error) {
	backoff := fetchBackoff
	var lastErr error
	for attempt := 1; attempt <= fetchAttempts; attempt++ {
		proxies, err := fetcher.Fetch(ctx, url)
		if err == nil {
			return proxies, nil
		}
		lastErr = err
		if isPermanentFetchError(err) || ctx.Err() != nil {
			return nil, err
		}
		if attempt == fetchAttempts {
			break
		}
		rlog.Warn("subscription fetch failed; retrying", "attempt", attempt, "url", url, "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, lastErr
}

// defaultFetcher is the package-level fetcher used by runJob.
// Tests can swap this for a stub.
var defaultFetcher SubscriptionFetcher = &HTTPSubscriptionFetcher{}

// fetcherForProxy returns a fetcher that tunnels through the node described by
// the given JSON config, or the default (direct / env-proxy) fetcher when the
// config is empty/invalid.
func fetcherForProxy(configJSON string) SubscriptionFetcher {
	cfg := parseProxyConfig(configJSON)
	if len(cfg) == 0 {
		return defaultFetcher
	}
	return &HTTPSubscriptionFetcher{ProxyConfig: cfg}
}

// parseProxyConfig decodes a stored fetch-proxy node config (JSON) into a proxy
// map, or returns nil for an empty/invalid value (→ direct fetch).
func parseProxyConfig(s string) map[string]any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

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
