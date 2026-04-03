// services/checker/fetch.go
package checker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/metacubex/mihomo/common/convert"
	"gopkg.in/yaml.v3"
)

// fetchProxies fetches a subscription URL and returns parsed proxy maps.
// Supports Clash YAML format and V2Ray/base64 format.
func fetchProxies(url string) ([]map[string]any, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
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

// parseProxies tries Clash YAML first, then V2Ray format.
func parseProxies(data []byte) ([]map[string]any, error) {
	// Try Clash YAML
	var clash struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &clash); err == nil && clash.Proxies != nil {
		return clash.Proxies, nil
	}

	// Try V2Ray/base64 format
	proxyList, err := convert.ConvertsV2Ray(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse as Clash YAML or V2Ray format: %w", err)
	}
	return proxyList, nil
}
