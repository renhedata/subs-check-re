package checker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	settingssvc "subs-check-re/services/settings"
	subsvc "subs-check-re/services/subscription"
)

// exportPrefs are the per-subscription export options resolved from the
// subscription record.
type exportPrefs struct {
	IncludeDead bool
	Sort        string // "speed_desc" | "latency_asc"
}

func orderClause(s string) string {
	if s == "latency_asc" {
		return "latency_ms ASC NULLS LAST, speed_kbps DESC NULLS LAST"
	}
	return "speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST"
}

// taggedName appends country / platform / speed tags to a node name per cfg.
// Order: country, built-in platforms (cfg order), custom platforms (sorted by
// key), speed. A platform is tagged when its outcome is Unlocked.
func taggedName(name, country string, platforms map[string]PlatformOutcome, speedKbps int, cfg settingssvc.ExportTagConfig) string {
	tags := []string{}

	if cfg.ShowCountry && country != "" {
		tags = append(tags, country)
	}

	unlocked := func(key string) bool {
		o, ok := platforms[key]
		return ok && o.Unlocked
	}

	cfgByKey := map[string]settingssvc.PlatformTag{}
	for _, p := range cfg.Platforms {
		cfgByKey[p.Key] = p
	}

	for _, p := range cfg.Platforms {
		if !builtinKeys[p.Key] || !p.Enabled {
			continue
		}
		if p.Key == "youtube" {
			if unlocked("youtube_premium") {
				tags = append(tags, p.Label+"+")
			} else if unlocked("youtube") {
				tags = append(tags, p.Label)
			}
			continue
		}
		if unlocked(p.Key) {
			tags = append(tags, p.Label)
		}
	}

	keys := make([]string, 0, len(platforms))
	for k, o := range platforms {
		if o.Unlocked && !builtinKeys[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if p, ok := cfgByKey[k]; ok {
			if !p.Enabled {
				continue
			}
			tags = append(tags, p.Label)
		} else {
			tags = append(tags, k)
		}
	}

	if cfg.ShowSpeed && speedKbps > 0 {
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

// latestUsableProxies returns the alive, enabled nodes from the latest completed job
// for the given subscription, with names tagged (NF/GPT/CL/YT+...) and sorted by speed.
func latestUsableProxies(ctx context.Context, subscriptionID, userID string, cfg settingssvc.ExportTagConfig, prefs exportPrefs) ([]map[string]any, error) {
	var jobID string
	if err := db.QueryRow(ctx, `
		SELECT id FROM check_jobs
		WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID, userID).Scan(&jobID); err != nil {
		return nil, fmt.Errorf("no completed check found")
	}
	return loadJobProxies(ctx, jobID, subscriptionID, "", cfg, prefs)
}

// latestUsableProxiesAcrossAllSubs aggregates alive nodes from the latest completed job
// of every subscription owned by the user; each node's name is prefixed with its
// subscription name so they remain disambiguated in clients.
func latestUsableProxiesAcrossAllSubs(ctx context.Context, userID string, cfg settingssvc.ExportTagConfig) ([]map[string]any, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (subscription_id) id AS job_id, subscription_id
		FROM check_jobs
		WHERE user_id = $1 AND status = 'completed'
		ORDER BY subscription_id, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type jobSub struct{ jobID, subscriptionID string }
	var jobs []jobSub
	for rows.Next() {
		var js jobSub
		if rows.Scan(&js.jobID, &js.subscriptionID) == nil {
			jobs = append(jobs, js)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}

	var all []map[string]any
	for _, js := range jobs {
		sub, err := subsvc.GetSubscriptionByID(ctx, &subsvc.GetByIDParams{ID: js.subscriptionID})
		if err != nil || sub.Name == "" {
			continue
		}
		prefs := exportPrefs{IncludeDead: sub.ExportIncludeDead, Sort: sub.ExportSort}
		proxies, _ := loadJobProxies(ctx, js.jobID, js.subscriptionID, sub.Name, cfg, prefs)
		all = append(all, proxies...)
	}
	return all, nil
}

// loadJobProxies returns the node configs for the given job, tagged + sorted per prefs.
// If subNamePrefix is non-empty, each name is prefixed with "<subName>|" for cross-sub aggregation.
func loadJobProxies(ctx context.Context, jobID, subscriptionID, subNamePrefix string, cfg settingssvc.ExportTagConfig, prefs exportPrefs) ([]map[string]any, error) {
	aliveClause := "cr.alive = true AND "
	if prefs.IncludeDead {
		aliveClause = ""
	}
	query := `
		WITH latest_platform_kv AS (
			SELECT DISTINCT ON (cr2.node_name, kv.key) cr2.node_name, kv.key AS key, kv.value AS value
			FROM check_results cr2
			JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			CROSS JOIN LATERAL jsonb_each(cr2.platforms) AS kv(key, value)
			WHERE cj2.subscription_id = $2 AND cr2.platforms IS NOT NULL AND cr2.platforms <> '{}'::jsonb
			ORDER BY cr2.node_name, kv.key, cr2.checked_at DESC
		),
		merged_platforms AS (
			SELECT node_name, jsonb_object_agg(key, value) AS platforms
			FROM latest_platform_kv
			GROUP BY node_name
		),
		r AS (
			SELECT COALESCE(n.config, cr.node_config) AS config,
			       COALESCE(n.name, cr.node_name) AS node_name,
			       CASE WHEN cr.country <> '' THEN cr.country
			            ELSE COALESCE((
			                SELECT cr2.country FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.country <> ''
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), '')
			       END AS country,
			       COALESCE(mp.platforms, cr.platforms, '{}'::jsonb) AS platforms,
			       CASE WHEN cr.speed_kbps > 0 THEN cr.speed_kbps
			            ELSE COALESCE((
			                SELECT cr2.speed_kbps FROM check_results cr2
			                JOIN check_jobs cj2 ON cj2.id = cr2.job_id
			                WHERE cr2.node_name = cr.node_name AND cj2.subscription_id = $2 AND cr2.speed_kbps > 0
			                ORDER BY cr2.checked_at DESC LIMIT 1
			            ), 0)
			       END AS speed_kbps,
			       cr.latency_ms
			FROM check_results cr
			LEFT JOIN nodes n ON n.id = cr.node_id
			LEFT JOIN merged_platforms mp ON mp.node_name = cr.node_name
			WHERE cr.job_id = $1 AND ` + aliveClause + `COALESCE(n.enabled, true) = true
		)
		SELECT config, node_name, country, platforms, speed_kbps, latency_ms
		FROM r
		ORDER BY ` + orderClause(prefs.Sort)
	rows, err := db.Query(ctx, query, jobID, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var (
			configJSON    []byte
			name          string
			country       string
			platformsJSON []byte
			speedKbps     int
			latencyMs     sql.NullInt64
		)
		if err := rows.Scan(&configJSON, &name, &country, &platformsJSON, &speedKbps, &latencyMs); err != nil {
			continue
		}
		if len(configJSON) == 0 {
			continue
		}
		var nodeCfg map[string]any
		if json.Unmarshal(configJSON, &nodeCfg) != nil {
			continue
		}
		var platforms map[string]PlatformOutcome
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &platforms)
		}
		tagged := taggedName(name, country, platforms, speedKbps, cfg)
		if subNamePrefix != "" {
			nodeCfg["name"] = subNamePrefix + "|" + tagged
		} else {
			nodeCfg["name"] = tagged
		}
		out = append(out, nodeCfg)
	}

	// Order is established by the SQL ORDER BY (orderClause), which correctly
	// places dead nodes (NULL latency / 0 speed) last via NULLS LAST. A Go
	// re-sort here would treat a dead node's NULL latency as 0 and wrongly hoist
	// it to the top under latency_asc, so we keep the DB order as-is.
	return out, nil
}

// latestServerAddresses returns the distinct server hostnames/IPs of enabled nodes
// for the subscription (or all subscriptions when subscriptionID == "all").
// notFound is true when the subscription does not belong to the user.
func latestServerAddresses(ctx context.Context, subscriptionID, userID string) (servers []string, notFound bool, err error) {
	if subscriptionID == "all" {
		rows, qErr := db.Query(ctx, `
			SELECT DISTINCT n.server
			FROM nodes n
			INNER JOIN (
				SELECT DISTINCT subscription_id FROM check_jobs WHERE user_id = $1
			) owned ON owned.subscription_id = n.subscription_id
			WHERE n.server != '' AND n.enabled = true
			ORDER BY n.server
		`, userID)
		if qErr != nil {
			return nil, false, qErr
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil && s != "" {
				servers = append(servers, s)
			}
		}
		return servers, false, rows.Err()
	}

	var count int
	if scanErr := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM check_jobs WHERE subscription_id=$1 AND user_id=$2`,
		subscriptionID, userID).Scan(&count); scanErr != nil || count == 0 {
		return nil, true, nil
	}

	rows, qErr := db.Query(ctx, `
		SELECT DISTINCT server FROM nodes
		WHERE subscription_id = $1 AND server != '' AND enabled = true
		ORDER BY server
	`, subscriptionID)
	if qErr != nil {
		return nil, false, qErr
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil && s != "" {
			servers = append(servers, s)
		}
	}
	return servers, false, rows.Err()
}

// logExport records an export request (best effort — failures are silent).
func logExport(ctx context.Context, subscriptionID, userID, ip string) {
	_, _ = db.Exec(ctx, `
		INSERT INTO export_logs (id, subscription_id, user_id, ip, requested_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New().String(), subscriptionID, userID, ip, time.Now())
}
