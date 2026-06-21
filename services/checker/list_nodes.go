// services/checker/list_nodes.go
package checker

import (
	"context"
	"encoding/json"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
	subsvc "subs-check-re/services/subscription"
)

// Node is a persisted node enriched with its latest-known result. Metrics are
// zero / Platforms empty / LastCheckedAt nil for a node that has never been
// checked. Inheritance mirrors GetResults: alive/latency/ip come from the most
// recent result row; speed/upload/country/platforms take the latest non-empty
// value per node identity (server:port).
type Node struct {
	NodeID          string                     `json:"node_id"`
	NodeName        string                     `json:"node_name"`
	NodeType        string                     `json:"node_type"`
	Enabled         bool                       `json:"enabled"`
	Alive           bool                       `json:"alive"`
	LatencyMs       int                        `json:"latency_ms"`
	SpeedKbps       int                        `json:"speed_kbps"`
	UploadSpeedKbps int                        `json:"upload_speed_kbps"`
	Country         string                     `json:"country"`
	IP              string                     `json:"ip"`
	Server          string                     `json:"server"`
	Port            int                        `json:"port"`
	Config          string                     `json:"config"`
	Platforms       map[string]PlatformOutcome `json:"platforms"`
	TrafficBytes    int64                      `json:"traffic_bytes"`
	LastCheckedAt   *time.Time                 `json:"last_checked_at,omitempty"`
}

// ListNodesResponse is returned by GET /subscription/:subscriptionID/nodes.
type ListNodesResponse struct {
	Nodes []Node `json:"nodes"`
}

// ListNodes returns the subscription's persisted nodes with their latest-known
// results, so the UI can display nodes immediately after fetch/import — before
// any check has run.
//
//encore:api auth method=GET path=/subscription/:subscriptionID/nodes
func ListNodes(ctx context.Context, subscriptionID string) (*ListNodesResponse, error) {
	_ = encauth.Data().(*authsvc.UserClaims)
	// Ownership: GetSubscription is user-scoped and errors if not owned.
	if _, err := subsvc.GetSubscription(ctx, subscriptionID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}

	nodes, err := defaultJobStore.listNodes(ctx, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	return &ListNodesResponse{Nodes: nodes}, nil
}

// listNodes is the read-only query behind ListNodes. It is a store method so it
// can be tested without an auth context.
func (s *jobStore) listNodes(ctx context.Context, subscriptionID string) ([]Node, error) {
	crKey := nodeIdentityKey("cr")
	// nodes-table identity must match the check_results identity (server:port
	// from the proxy config, falling back to the display name).
	nodeKey := `CASE WHEN COALESCE(n.config->>'server','') <> '' ` +
		`THEN (n.config->>'server') || ':' || COALESCE(n.config->>'port','') ` +
		`ELSE n.name END`

	rows, err := db.Query(ctx, `
		WITH node_keys AS (
			SELECT n.id, n.name, COALESCE(n.type,'') AS type, n.enabled,
			       COALESCE(n.server,'') AS server, COALESCE(n.port,0) AS port,
			       COALESCE(n.config::text,'') AS config,
			       (`+nodeKey+`) AS node_key
			FROM nodes n
			WHERE n.subscription_id = $1
		),
		hist AS (
			SELECT `+crKey+` AS node_key, cr.alive, cr.latency_ms, cr.ip,
			       cr.speed_kbps, cr.upload_speed_kbps, cr.country, cr.platforms,
			       cr.traffic_bytes, cr.checked_at
			FROM check_results cr
			JOIN check_jobs cj ON cj.id = cr.job_id
			WHERE cj.subscription_id = $1
		),
		latest AS (
			SELECT DISTINCT ON (node_key) node_key, alive, latency_ms, ip, traffic_bytes, checked_at
			FROM hist ORDER BY node_key, checked_at DESC
		),
		spd AS (
			SELECT DISTINCT ON (node_key) node_key, speed_kbps
			FROM hist WHERE speed_kbps > 0 ORDER BY node_key, checked_at DESC
		),
		upl AS (
			SELECT DISTINCT ON (node_key) node_key, upload_speed_kbps
			FROM hist WHERE upload_speed_kbps > 0 ORDER BY node_key, checked_at DESC
		),
		ctry AS (
			SELECT DISTINCT ON (node_key) node_key, country
			FROM hist WHERE country <> '' ORDER BY node_key, checked_at DESC
		),
		plat_kv AS (
			SELECT DISTINCT ON (node_key, kv.key) node_key, kv.key AS key, kv.value AS value
			FROM hist CROSS JOIN LATERAL jsonb_each(hist.platforms) AS kv(key, value)
			WHERE hist.platforms IS NOT NULL AND hist.platforms <> '{}'::jsonb
			ORDER BY node_key, kv.key, hist.checked_at DESC
		),
		plat AS (
			SELECT node_key, jsonb_object_agg(key, value) AS platforms
			FROM plat_kv GROUP BY node_key
		)
		SELECT nk.id, nk.name, nk.type, nk.enabled, nk.server, nk.port, nk.config,
		       COALESCE(latest.alive, false), COALESCE(latest.latency_ms, 0),
		       COALESCE(spd.speed_kbps, 0), COALESCE(upl.upload_speed_kbps, 0),
		       COALESCE(ctry.country, ''), COALESCE(latest.ip, ''),
		       COALESCE(plat.platforms, '{}'::jsonb),
		       COALESCE(latest.traffic_bytes, 0),
		       latest.checked_at
		FROM node_keys nk
		LEFT JOIN latest ON latest.node_key = nk.node_key
		LEFT JOIN spd  ON spd.node_key  = nk.node_key
		LEFT JOIN upl  ON upl.node_key  = nk.node_key
		LEFT JOIN ctry ON ctry.node_key = nk.node_key
		LEFT JOIN plat ON plat.node_key = nk.node_key
		ORDER BY nk.name
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Node
	for rows.Next() {
		var n Node
		var platformsJSON []byte
		var checkedAt *time.Time
		if err := rows.Scan(
			&n.NodeID, &n.NodeName, &n.NodeType, &n.Enabled, &n.Server, &n.Port, &n.Config,
			&n.Alive, &n.LatencyMs, &n.SpeedKbps, &n.UploadSpeedKbps, &n.Country, &n.IP,
			&platformsJSON, &n.TrafficBytes, &checkedAt,
		); err != nil {
			return nil, err
		}
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &n.Platforms)
		}
		if n.Platforms == nil {
			n.Platforms = map[string]PlatformOutcome{}
		}
		n.LastCheckedAt = checkedAt
		out = append(out, n)
	}
	if out == nil {
		out = []Node{}
	}
	return out, rows.Err()
}
