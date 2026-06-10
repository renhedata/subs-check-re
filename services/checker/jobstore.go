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
	extraJSON, _ := json.Marshal(res.ExtraPlatforms)
	_, err := db.Exec(ctx, `
		INSERT INTO check_results
		  (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, upload_speed_kbps, country, ip,
		   netflix, youtube, youtube_premium, openai, claude, gemini, grok, disney, tiktok, traffic_bytes, extra_platforms)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
	`, uuid.New().String(), jobID, nodeID, res.NodeName, nodeType, nodeConfigJSON, time.Now(),
		res.Alive, res.LatencyMs, res.SpeedKbps, res.UploadSpeedKbps, res.Country, res.IP,
		res.Netflix, res.YouTube, res.YouTubePremium, res.OpenAI,
		res.Claude, res.Gemini, res.Grok, res.Disney, res.TikTok,
		res.TrafficBytes, extraJSON,
	)
	return err
}
