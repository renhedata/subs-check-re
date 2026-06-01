package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	subsvc "subs-check-re/services/subscription"
)

// rankedNode is an internal carrier used to sort by speed/latency
// before stripping down to []map[string]any.
type rankedNode struct {
	config    map[string]any
	speedKbps int
	latencyMs int
}

// latestUsableProxies returns the alive, enabled nodes from the latest completed job
// for the given subscription, with names tagged (NF/GPT/CL/YT+...) and sorted by speed.
func latestUsableProxies(ctx context.Context, subscriptionID, userID string) ([]map[string]any, error) {
	var jobID string
	if err := db.QueryRow(ctx, `
		SELECT id FROM check_jobs
		WHERE subscription_id=$1 AND user_id=$2 AND status='completed'
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID, userID).Scan(&jobID); err != nil {
		return nil, fmt.Errorf("no completed check found")
	}
	return loadJobProxies(ctx, jobID, subscriptionID, "")
}

// latestUsableProxiesAcrossAllSubs aggregates alive nodes from the latest completed job
// of every subscription owned by the user; each node's name is prefixed with its
// subscription name so they remain disambiguated in clients.
func latestUsableProxiesAcrossAllSubs(ctx context.Context, userID string) ([]map[string]any, error) {
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
	subIDs := []string{}
	for rows.Next() {
		var js jobSub
		if rows.Scan(&js.jobID, &js.subscriptionID) == nil {
			jobs = append(jobs, js)
			subIDs = append(subIDs, js.subscriptionID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}

	namesResp, err := subsvc.GetSubscriptionNames(ctx, &subsvc.GetSubscriptionNamesParams{
		UserID: userID, IDs: subIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("name lookup failed: %w", err)
	}

	var all []map[string]any
	for _, js := range jobs {
		subName := namesResp.Names[js.subscriptionID]
		if subName == "" {
			continue
		}
		proxies, _ := loadJobProxies(ctx, js.jobID, js.subscriptionID, subName)
		all = append(all, proxies...)
	}
	return all, nil
}

// loadJobProxies returns the alive node configs for the given job, tagged + sorted by speed.
// If subNamePrefix is non-empty, each name is prefixed with "<subName>|" for cross-sub aggregation.
func loadJobProxies(ctx context.Context, jobID, subscriptionID, subNamePrefix string) ([]map[string]any, error) {
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
			WHERE cr.job_id = $1 AND cr.alive = true AND COALESCE(n.enabled, true) = true
		)
		SELECT config, node_name, netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok,
		       speed_kbps, latency_ms
		FROM r
		ORDER BY speed_kbps DESC NULLS LAST, latency_ms ASC NULLS LAST
	`, jobID, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []rankedNode
	for rows.Next() {
		var (
			configJSON                                                                     []byte
			name                                                                           string
			netflix, youtube, youtubePremium, openai, claude, gemini, grok, disney, tiktok bool
			speedKbps, latencyMs                                                           int
		)
		if err := rows.Scan(&configJSON, &name,
			&netflix, &youtube, &youtubePremium, &openai, &claude, &gemini, &grok, &disney, &tiktok,
			&speedKbps, &latencyMs); err != nil {
			continue
		}
		if len(configJSON) == 0 {
			continue
		}
		var cfg map[string]any
		if json.Unmarshal(configJSON, &cfg) != nil {
			continue
		}
		tagged := taggedName(name, netflix, youtube, youtubePremium, openai, claude, gemini, grok, disney, tiktok, speedKbps)
		if subNamePrefix != "" {
			cfg["name"] = subNamePrefix + "|" + tagged
		} else {
			cfg["name"] = tagged
		}
		nodes = append(nodes, rankedNode{config: cfg, speedKbps: speedKbps, latencyMs: latencyMs})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].speedKbps != nodes[j].speedKbps {
			return nodes[i].speedKbps > nodes[j].speedKbps
		}
		return nodes[i].latencyMs < nodes[j].latencyMs
	})

	out := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		out[i] = n.config
	}
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

// taggedName appends platform unlock tags (NF, GPT, CL, YT+, …) and a speed tag (e.g. "10.5MB")
// to a node name. The original name is returned unchanged when no tags apply.
func taggedName(name string, netflix, youtube, youtubePremium, openai, claude, gemini, grok, disney, tiktok bool, speedKbps int) string {
	tags := []string{}
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
