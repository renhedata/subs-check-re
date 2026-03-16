// services/checker/export.go
package checker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	settingssvc "subs-check-re/services/settings"
	"gopkg.in/yaml.v3"
)

// Export generates a subscription link from the latest completed check results.
//
//encore:api public raw method=GET path=/export/:subscriptionID
func Export(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Extract subscriptionID from path: /export/<subscriptionID>
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	subscriptionID := parts[1]

	token := req.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}
	target := req.URL.Query().Get("target")
	if target == "" {
		target = "clash"
	}

	// Resolve token → user_id.
	userResp, err := settingssvc.GetUserIDByAPIKey(ctx, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID := userResp.UserID

	// Find latest completed job for this subscription owned by this user.
	var jobID string
	if err := db.QueryRow(ctx, `
		SELECT id FROM check_jobs
		WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID, userID).Scan(&jobID); err != nil {
		http.Error(w, "no completed check found", http.StatusNotFound)
		return
	}

	// Query alive nodes with their config.
	rows, err := db.Query(ctx, `
		SELECT n.config, COALESCE(n.name, cr.node_name),
		       cr.netflix, cr.youtube, cr.openai, cr.claude, cr.gemini, cr.disney, cr.tiktok,
		       cr.speed_kbps, cr.latency_ms
		FROM check_results cr
		LEFT JOIN nodes n ON n.id = cr.node_id
		WHERE cr.job_id=$1 AND cr.alive=true
		ORDER BY cr.speed_kbps DESC NULLS LAST, cr.latency_ms ASC NULLS LAST
	`, jobID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type nodeRow struct {
		config  map[string]any
		name    string
		netflix bool
		youtube string
		openai  bool
		claude  bool
		gemini  bool
		disney  bool
		tiktok  string
	}

	var nodes []nodeRow
	for rows.Next() {
		var nr nodeRow
		var configJSON []byte
		if err := rows.Scan(&configJSON, &nr.name,
			&nr.netflix, &nr.youtube, &nr.openai, &nr.claude, &nr.gemini, &nr.disney, &nr.tiktok,
			new(int), new(int)); err != nil {
			continue
		}
		if len(configJSON) == 0 {
			continue // skip nodes from old checks with no config
		}
		if err := json.Unmarshal(configJSON, &nr.config); err != nil {
			continue
		}
		nodes = append(nodes, nr)
	}

	// Build tagged proxy configs.
	proxies := make([]map[string]any, 0, len(nodes))
	for _, nr := range nodes {
		cfg := make(map[string]any, len(nr.config))
		for k, v := range nr.config {
			cfg[k] = v
		}
		cfg["name"] = taggedName(nr.name, nr.netflix, nr.youtube, nr.openai, nr.claude, nr.gemini, nr.disney, nr.tiktok)
		proxies = append(proxies, cfg)
	}

	switch target {
	case "base64":
		renderBase64(w, proxies)
	default:
		renderClash(w, proxies)
	}
}

func taggedName(name string, netflix bool, youtube string, openai bool, claude bool, gemini bool, disney bool, tiktok string) string {
	var tags []string
	if netflix {
		tags = append(tags, "NF")
	}
	if openai {
		tags = append(tags, "GPT")
	}
	if gemini {
		tags = append(tags, "GM")
	}
	if claude {
		tags = append(tags, "CL")
	}
	if youtube != "" {
		tags = append(tags, "YT-"+youtube)
	}
	if disney {
		tags = append(tags, "D+")
	}
	if tiktok != "" {
		tags = append(tags, "TK-"+tiktok)
	}
	if len(tags) == 0 {
		return name
	}
	return name + "|" + strings.Join(tags, "|")
}

func renderClash(w http.ResponseWriter, proxies []map[string]any) {
	data, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		http.Error(w, "yaml error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(data)
}

func renderBase64(w http.ResponseWriter, proxies []map[string]any) {
	var lines []string
	for _, p := range proxies {
		uri := proxyToURI(p)
		if uri != "" {
			lines = append(lines, uri)
		}
	}
	raw := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, encoded)
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
