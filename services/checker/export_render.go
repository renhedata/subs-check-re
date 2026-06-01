package checker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// renderClash writes a Clash YAML document containing the proxies list.
func renderClash(w io.Writer, proxies []map[string]any) error {
	data, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// renderBase64 writes the proxies encoded as a single base64-encoded line list,
// matching the V2Ray-style subscription format.
func renderBase64(w io.Writer, proxies []map[string]any) error {
	lines := []string{}
	for _, p := range proxies {
		if uri := proxyToURI(p); uri != "" {
			lines = append(lines, uri)
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	_, err := fmt.Fprint(w, encoded)
	return err
}

// renderRouterOS writes a RouterOS .rsc script that rebuilds the firewall
// address-list for the given server addresses.
func renderRouterOS(w io.Writer, servers []string, listName string) error {
	if _, err := fmt.Fprintf(w, "/ip firewall address-list remove [find where list=%s]\n", listName); err != nil {
		return err
	}
	for _, s := range servers {
		if _, err := fmt.Fprintf(w, "/ip firewall address-list add list=%s address=%s\n", listName, s); err != nil {
			return err
		}
	}
	return nil
}

// proxyToURI converts a mihomo proxy config map to a URI string.
// Supports ss, trojan, vless, vmess. Returns "" for unknown types.
func proxyToURI(p map[string]any) string {
	ptype, _ := p["type"].(string)
	name, _ := p["name"].(string)
	server, _ := p["server"].(string)
	port := fmt.Sprint(p["port"])

	switch ptype {
	case "ss":
		cipher, _ := p["cipher"].(string)
		password, _ := p["password"].(string)
		userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
		return fmt.Sprintf("ss://%s@%s:%s#%s", userinfo, server, port, urlEncode(name))
	case "trojan":
		password, _ := p["password"].(string)
		return fmt.Sprintf("trojan://%s@%s:%s#%s", password, server, port, urlEncode(name))
	case "vless":
		uuid, _ := p["uuid"].(string)
		network, _ := p["network"].(string)
		tls, _ := p["tls"].(bool)
		params := ""
		if network != "" {
			params += "type=" + network
		}
		if tls {
			if params != "" {
				params += "&"
			}
			params += "security=tls"
		}
		if params != "" {
			params = "?" + params
		}
		return fmt.Sprintf("vless://%s@%s:%s%s#%s", uuid, server, port, params, urlEncode(name))
	case "vmess":
		uuid, _ := p["uuid"].(string)
		network, _ := p["network"].(string)
		aid := 0
		if v, ok := p["alterId"].(int); ok {
			aid = v
		}
		vmessObj := map[string]any{
			"v": "2", "ps": name, "add": server, "port": port,
			"id": uuid, "aid": aid, "net": network, "type": "none",
			"host": "", "path": "", "tls": "",
		}
		if tls, ok := p["tls"].(bool); ok && tls {
			vmessObj["tls"] = "tls"
		}
		vmessJSON, _ := json.Marshal(vmessObj)
		return "vmess://" + base64.StdEncoding.EncodeToString(vmessJSON)
	}
	return ""
}

func urlEncode(s string) string {
	return strings.NewReplacer(" ", "%20", "#", "%23", "&", "%26").Replace(s)
}

// clientIP extracts the real client IP from the request.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, err := net.SplitHostPort(strings.SplitN(xff, ",", 2)[0]); err == nil {
			return ip
		}
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
