package checker

import (
	"testing"
	"time"
)

func ssNodeConfig() map[string]any {
	return map[string]any{
		"name": "via", "type": "ss", "server": "1.2.3.4", "port": 8388,
		"cipher": "aes-128-gcm", "password": "secret",
	}
}

// With no proxy config a fetch stays direct (the shared default fetcher).
func TestFetcherForProxyDirectWhenEmpty(t *testing.T) {
	if fetcherForProxy("") != defaultFetcher {
		t.Error("empty config must use the default fetcher")
	}
	if fetcherForProxy("   ") != defaultFetcher {
		t.Error("blank config must use the default fetcher")
	}
	if fetcherForProxy("not json") != defaultFetcher {
		t.Error("invalid config must fall back to the default fetcher")
	}
}

// A valid JSON config produces a fetcher that tunnels through that node.
func TestFetcherForProxyBuildsProxyFetcher(t *testing.T) {
	cfgJSON := `{"name":"via","type":"ss","server":"1.2.3.4","port":8388,"cipher":"aes-128-gcm","password":"secret"}`
	f := fetcherForProxy(cfgJSON)
	hf, ok := f.(*HTTPSubscriptionFetcher)
	if !ok || hf.ProxyConfig == nil {
		t.Fatalf("expected an HTTPSubscriptionFetcher with ProxyConfig, got %T", f)
	}
}

// The proxy HTTP client builds from a valid node config (no network needed).
func TestProxyHTTPClientBuilds(t *testing.T) {
	c, closer, err := proxyHTTPClient(ssNodeConfig(), 5*time.Second)
	if err != nil {
		t.Fatalf("proxyHTTPClient: %v", err)
	}
	defer closer()
	if c == nil {
		t.Fatal("expected a client")
	}
}
