// services/checker/export.go
package checker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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

	// Log this export request (best effort).
	ip := clientIP(req)
	db.Exec(ctx, `
		INSERT INTO export_logs (id, subscription_id, user_id, ip, requested_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New().String(), subscriptionID, userID, ip, time.Now()) //nolint:errcheck

	// Query alive nodes with their config.
	// node_config is denormalized into check_results so it survives node table replacement.
	// speed_kbps falls back to the most recent historical speed if this job skipped speed testing.
	rows, err := db.Query(ctx, `
		WITH r AS (
			SELECT COALESCE(n.config, cr.node_config) AS config,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       cr.netflix, cr.youtube, cr.youtube_premium, cr.openai, cr.claude, cr.gemini, cr.grok, cr.disney, cr.tiktok,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps
			                FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name
			                  AND cj2.subscription_id = $2
			                  AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC
			                LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       cr.latency_ms
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			WHERE cr.job_id = $1 AND cr.alive = true
		)
		SELECT config, node_name, netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok,
		       speed_kbps, latency_ms
		FROM r
		ORDER BY speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST
	`, jobID, subscriptionID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type nodeRow struct {
		config         map[string]any
		name           string
		netflix        bool
		youtube        bool
		youtubePremium bool
		openai         bool
		claude         bool
		gemini         bool
		grok           bool
		disney         bool
		tiktok         bool
		speedKbps      int
	}

	var nodes []nodeRow
	for rows.Next() {
		var nr nodeRow
		var configJSON []byte
		if err := rows.Scan(&configJSON, &nr.name,
			&nr.netflix, &nr.youtube, &nr.youtubePremium, &nr.openai, &nr.claude, &nr.gemini, &nr.grok, &nr.disney, &nr.tiktok,
			&nr.speedKbps, new(int)); err != nil {
			continue
		}
		if len(configJSON) == 0 {
			continue // skip nodes with no config (pre-migration rows)
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
		cfg["name"] = taggedName(nr.name, nr.netflix, nr.youtube, nr.youtubePremium, nr.openai, nr.claude, nr.gemini, nr.grok, nr.disney, nr.tiktok, nr.speedKbps)
		proxies = append(proxies, cfg)
	}

	switch target {
	case "base64":
		renderBase64(w, proxies)
	default:
		renderClash(w, proxies)
	}
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

func taggedName(name string, netflix bool, youtube bool, youtubePremium bool, openai bool, claude bool, gemini bool, grok bool, disney bool, tiktok bool, speedKbps int) string {
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
	if grok {
		tags = append(tags, "GK")
	}
	if youtubePremium {
		tags = append(tags, "YT+")
	} else if youtube {
		tags = append(tags, "YT")
	}
	if disney {
		tags = append(tags, "D+")
	}
	if tiktok {
		tags = append(tags, "TK")
	}
	if speedKbps > 0 {
		if speedKbps >= 1024 {
			tags = append(tags, fmt.Sprintf("%.1fMB", float64(speedKbps)/1024))
		} else {
			tags = append(tags, fmt.Sprintf("%dKB", speedKbps))
		}
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
