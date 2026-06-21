// services/checker/jobstore.go
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// jobStore owns every check_jobs / nodes / check_results write performed by a
// running job. Every method returns an error so the runner (jobrunner.go) can
// decide whether a failure is fatal for the job (state transitions, node
// replacement) or per-node (results, progress).
type jobStore struct{}

var defaultJobStore = &jobStore{}

// jobConfig is the per-job configuration snapshot stored on the check_jobs row.
type jobConfig struct {
	SubURL         string
	SpeedTestURL   string
	LatencyTestURL string
	Options        CheckOptions
}

func (s *jobStore) loadConfig(ctx context.Context, jobID string) (*jobConfig, error) {
	var cfg jobConfig
	var optsJSON []byte
	if err := db.QueryRow(ctx,
		`SELECT sub_url, COALESCE(speed_test_url, ''), COALESCE(latency_test_url, ''), COALESCE(options_json, '{}')
		 FROM check_jobs WHERE id=$1`,
		jobID).Scan(&cfg.SubURL, &cfg.SpeedTestURL, &cfg.LatencyTestURL, &optsJSON); err != nil {
		return nil, fmt.Errorf("load job config: %w", err)
	}
	if cfg.SubURL == "" {
		return nil, fmt.Errorf("job %s has no subscription URL", jobID)
	}
	if cfg.SpeedTestURL == "" {
		cfg.SpeedTestURL = defaultSpeedTestURL
	}
	if err := json.Unmarshal(optsJSON, &cfg.Options); err != nil {
		cfg.Options = defaultCheckOptions()
	}
	return &cfg, nil
}

func (s *jobStore) markRunning(ctx context.Context, jobID string) error {
	_, err := db.Exec(ctx, `UPDATE check_jobs SET status='running' WHERE id=$1`, jobID)
	return err
}

func (s *jobStore) markFailed(ctx context.Context, jobID string) error {
	_, err := db.Exec(ctx, `UPDATE check_jobs SET status='failed', finished_at=$2 WHERE id=$1`, jobID, time.Now())
	return err
}

func (s *jobStore) markCompleted(ctx context.Context, jobID string, available int, trafficBytes int64) error {
	_, err := db.Exec(ctx,
		`UPDATE check_jobs SET status='completed', finished_at=$2, available=$3, total_traffic_bytes=$4 WHERE id=$1`,
		jobID, time.Now(), available, trafficBytes)
	return err
}

func (s *jobStore) setTotal(ctx context.Context, jobID string, total int) error {
	_, err := db.Exec(ctx, `UPDATE check_jobs SET total=$2 WHERE id=$1`, jobID, total)
	return err
}

func (s *jobStore) setProgress(ctx context.Context, jobID string, progress int) error {
	_, err := db.Exec(ctx, `UPDATE check_jobs SET progress=$2 WHERE id=$1`, jobID, progress)
	return err
}

func (s *jobStore) countAvailable(ctx context.Context, jobID string) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM check_results WHERE job_id=$1 AND alive=true`, jobID).Scan(&n)
	return n, err
}

// replaceNodes swaps the subscription's node list in a single transaction so
// a crash mid-replacement can never leave the subscription with a partial or
// empty node list. Disabled flags carry over by node name.
func (s *jobStore) replaceNodes(ctx context.Context, subscriptionID string, proxies []map[string]any) ([]string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	disabled := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT name FROM nodes WHERE subscription_id=$1 AND enabled=false`, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("read disabled nodes: %w", err)
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan disabled node: %w", err)
		}
		disabled[n] = true
	}
	rows.Close()

	if _, err := tx.Exec(ctx, `DELETE FROM nodes WHERE subscription_id=$1`, subscriptionID); err != nil {
		return nil, fmt.Errorf("delete old nodes: %w", err)
	}

	nodeIDs := make([]string, len(proxies))
	for i, p := range proxies {
		id := uuid.New().String()
		nodeIDs[i] = id
		name, _ := p["name"].(string)
		ptype, _ := p["type"].(string)
		server, _ := p["server"].(string)
		port := 0
		if v, ok := p["port"].(int); ok {
			port = v
		}
		configJSON, _ := json.Marshal(p)
		if _, err := tx.Exec(ctx, `
			INSERT INTO nodes (id, subscription_id, name, type, server, port, config, enabled)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, id, subscriptionID, name, ptype, server, port, configJSON, !disabled[name]); err != nil {
			return nil, fmt.Errorf("insert node %q: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return nodeIDs, nil
}

func (s *jobStore) insertResult(ctx context.Context, jobID, nodeID string, proxy map[string]any, res nodeCheckResult) error {
	nodeType, _ := proxy["type"].(string)
	nodeConfigJSON, _ := json.Marshal(proxy)
	platformsJSON, _ := json.Marshal(res.Platforms)
	if len(platformsJSON) == 0 || string(platformsJSON) == "null" {
		platformsJSON = []byte("{}")
	}
	_, err := db.Exec(ctx, `
		INSERT INTO check_results
		  (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, upload_speed_kbps, country, ip,
		   platforms, traffic_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, uuid.New().String(), jobID, nodeID, res.NodeName, nodeType, nodeConfigJSON, time.Now(),
		res.Alive, res.LatencyMs, res.SpeedKbps, res.UploadSpeedKbps, res.Country, res.IP,
		platformsJSON, res.TrafficBytes,
	)
	return err
}

// inheritedDims is a node's last-known measured values across the
// subscription's prior jobs, used to fill dimensions a partial (e.g. alive-only)
// run did not measure. Mirrors the read-time inheritance in GetResults so the
// live SSE stream agrees with the results endpoint.
type inheritedDims struct {
	speedKbps       int
	uploadSpeedKbps int
	country         string
	platforms       map[string]PlatformOutcome
}

// inheritanceBaseline maps a stable node identity (server:port, see
// nodeIdentityKey) to its inherited dimensions.
type inheritanceBaseline map[string]inheritedDims

// loadInheritanceBaseline snapshots, per node identity, the latest non-empty
// speed / upload / country and the latest value of each platform key across all
// of the subscription's prior check results. Computed once at job start (before
// this job has written any rows), it lets recordResult emit a fully inherited
// result per node without an extra query per node.
func (s *jobStore) loadInheritanceBaseline(ctx context.Context, subscriptionID string) (inheritanceBaseline, error) {
	key := nodeIdentityKey("cr")
	rows, err := db.Query(ctx, `
		WITH hist AS (
			SELECT `+key+` AS node_key, cr.speed_kbps, cr.upload_speed_kbps,
			       cr.country, cr.platforms, cr.checked_at
			FROM check_results cr
			JOIN check_jobs cj ON cj.id = cr.job_id
			WHERE cj.subscription_id = $1
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
			FROM hist
			CROSS JOIN LATERAL jsonb_each(hist.platforms) AS kv(key, value)
			WHERE hist.platforms IS NOT NULL AND hist.platforms <> '{}'::jsonb
			ORDER BY node_key, kv.key, hist.checked_at DESC
		),
		plat AS (
			SELECT node_key, jsonb_object_agg(key, value) AS platforms
			FROM plat_kv GROUP BY node_key
		)
		SELECT k.node_key,
		       COALESCE(spd.speed_kbps, 0),
		       COALESCE(upl.upload_speed_kbps, 0),
		       COALESCE(ctry.country, ''),
		       COALESCE(plat.platforms, '{}'::jsonb)
		FROM (SELECT DISTINCT node_key FROM hist) k
		LEFT JOIN spd  ON spd.node_key  = k.node_key
		LEFT JOIN upl  ON upl.node_key  = k.node_key
		LEFT JOIN ctry ON ctry.node_key = k.node_key
		LEFT JOIN plat ON plat.node_key = k.node_key
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	baseline := inheritanceBaseline{}
	for rows.Next() {
		var nodeKey string
		var dims inheritedDims
		var platformsJSON []byte
		if err := rows.Scan(&nodeKey, &dims.speedKbps, &dims.uploadSpeedKbps, &dims.country, &platformsJSON); err != nil {
			return nil, err
		}
		if len(platformsJSON) > 0 {
			_ = json.Unmarshal(platformsJSON, &dims.platforms)
		}
		baseline[nodeKey] = dims
	}
	return baseline, rows.Err()
}
