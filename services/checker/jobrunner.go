// services/checker/jobrunner.go
package checker

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"encore.dev/rlog"

	settingssvc "subs-check-re/services/settings"
)

const (
	checkConcurrency    = 20
	nodeTimeout         = 90 * time.Second
	defaultSpeedTestURL = "https://speed.cloudflare.com/__down?bytes=204800"
	jobTimeout          = 4 * time.Hour
)

// checkFunc is the per-node check seam; production wires checkNode (mihomo),
// tests inject fakes so the runner is testable without real proxies.
type checkFunc func(ctx context.Context, nodeID string, mapping map[string]any, speedTestURL, uploadTestURL, latencyTestURL string, opts CheckOptions, rules []*PlatformRule) nodeCheckResult

// jobRunner executes one check job end to end: load config, fetch the
// subscription (with retry), replace nodes transactionally, fan out node
// checks, persist results, and drive the job state machine
// (running → completed | failed).
type jobRunner struct {
	store   *jobStore
	fetcher SubscriptionFetcher
	bus     JobBus
	check   checkFunc
}

// runJob keeps the original entry-point signature used by the trigger
// endpoints. It binds the package-level seams at call time so tests that swap
// defaultFetcher / defaultJobBus keep working.
func runJob(parentCtx context.Context, jobID, subscriptionID, userID string) {
	r := &jobRunner{
		store:   defaultJobStore,
		fetcher: defaultFetcher,
		bus:     defaultJobBus,
		check:   checkNode,
	}
	r.run(parentCtx, jobID, subscriptionID, userID)
}

func (r *jobRunner) run(parentCtx context.Context, jobID, subscriptionID, userID string) {
	ctx, cancel := context.WithTimeout(parentCtx, jobTimeout)
	defer cancel()
	r.bus.StoreCancel(jobID, cancel)
	defer r.bus.RemoveCancel(jobID)

	// State transitions use context.Background(): they must land even when the
	// job context is canceled (that's exactly when we record 'failed').
	fail := func(reason string, err error) {
		rlog.Error("check job failed", "job_id", jobID, "subscription_id", subscriptionID, "reason", reason, "err", err)
		if ferr := r.store.markFailed(context.Background(), jobID); ferr != nil {
			rlog.Error("failed to mark job failed", "job_id", jobID, "err", ferr)
		}
		r.bus.Close(jobID)
	}

	if err := r.store.markRunning(context.Background(), jobID); err != nil {
		fail("mark running", err)
		return
	}

	cfg, err := r.store.loadConfig(context.Background(), jobID)
	if err != nil {
		fail("load config", err)
		return
	}

	proxies, err := fetchWithRetry(ctx, r.fetcher, cfg.SubURL)
	if err != nil {
		fail("fetch subscription", err)
		return
	}

	total := len(proxies)
	if err := r.store.setTotal(context.Background(), jobID, total); err != nil {
		rlog.Error("failed to persist job total", "job_id", jobID, "err", err)
	}

	nodeIDs, err := r.store.replaceNodes(context.Background(), subscriptionID, proxies)
	if err != nil {
		fail("replace nodes", err)
		return
	}

	// Best-effort lookups: a failure degrades to built-in rules / default URLs.
	userRules, err := loadUserRules(ctx, userID)
	if err != nil {
		rlog.Warn("failed to load user platform rules; using built-ins", "job_id", jobID, "err", err)
	}
	// Only evaluate rules the caller selected for this run; unselected
	// platforms inherit their last-known result at read time.
	userRules = filterRulesBySelection(userRules, cfg.Options.MediaApps)
	uploadTestURL := ""
	if userCfg, err := settingssvc.GetSpeedTestURLForUser(ctx, userID); err == nil {
		uploadTestURL = userCfg.UploadTestURL
	} else {
		rlog.Warn("failed to load user settings; deriving upload URL from speed test URL", "job_id", jobID, "err", err)
	}

	// Snapshot prior results once so each node's live event can inherit the
	// dimensions this run skips (matching GetResults). Best-effort: an empty
	// baseline just means live events show measured-only values.
	baseline, err := r.store.loadInheritanceBaseline(ctx, subscriptionID)
	if err != nil {
		rlog.Warn("failed to load inheritance baseline; live stream shows measured-only values", "job_id", jobID, "err", err)
		baseline = inheritanceBaseline{}
	}

	type task struct {
		nodeID string
		proxy  map[string]any
	}
	taskCh := make(chan task, checkConcurrency)

	var processedCount atomic.Int64
	var totalTrafficBytes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < checkConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				res := r.checkOne(ctx, jobID, t.nodeID, t.proxy, cfg, uploadTestURL, userRules)
				r.recordResult(jobID, t.nodeID, t.proxy, res, total, &processedCount, &totalTrafficBytes, cfg.Options, baseline)
			}
		}()
	}

	for i, p := range proxies {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case taskCh <- task{nodeID: nodeIDs[i], proxy: p}:
		}
	}
	close(taskCh)
	wg.Wait()

	if ctx.Err() != nil {
		fail("canceled or timed out", ctx.Err())
		return
	}

	available, err := r.store.countAvailable(context.Background(), jobID)
	if err != nil {
		rlog.Error("failed to count available nodes", "job_id", jobID, "err", err)
	}
	if err := r.store.markCompleted(context.Background(), jobID, available, totalTrafficBytes.Load()); err != nil {
		rlog.Error("failed to mark job completed", "job_id", jobID, "err", err)
	}

	// Publish completion event — notify service fetches details on demand via GetJobDetailedSummary
	JobCompletedTopic.Publish(context.Background(), &JobCompletedEvent{
		JobID:          jobID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Available:      available,
		Total:          total,
	})

	r.bus.Close(jobID)
}

// checkOne runs a single node check, converting panics from the proxy stack
// (mihomo) into a dead-node result instead of silently killing the worker and
// losing the node's result row.
func (r *jobRunner) checkOne(ctx context.Context, jobID, nodeID string, proxy map[string]any, cfg *jobConfig, uploadTestURL string, rules []*PlatformRule) (res nodeCheckResult) {
	defer func() {
		if rec := recover(); rec != nil {
			name, _ := proxy["name"].(string)
			rlog.Error("node check panicked", "job_id", jobID, "node", name, "panic", rec, "stack", string(debug.Stack()))
			res = nodeCheckResult{NodeID: nodeID, NodeName: name}
		}
	}()
	nodeCtx, cancel := context.WithTimeout(ctx, nodeTimeout)
	defer cancel()
	return r.check(nodeCtx, nodeID, proxy, cfg.SpeedTestURL, uploadTestURL, cfg.LatencyTestURL, cfg.Options, rules)
}

// recordResult persists one node's outcome and emits progress. Persistence
// failures are logged, not fatal: one lost row should not abort the job.
func (r *jobRunner) recordResult(jobID, nodeID string, proxy map[string]any, res nodeCheckResult, total int, processed, traffic *atomic.Int64, opts CheckOptions, baseline inheritanceBaseline) {
	if err := r.store.insertResult(context.Background(), jobID, nodeID, proxy, res); err != nil {
		rlog.Error("failed to persist node result", "job_id", jobID, "node", res.NodeName, "err", err)
	}
	traffic.Add(res.TrafficBytes)
	n := processed.Add(1)
	if err := r.store.setProgress(context.Background(), jobID, int(n)); err != nil {
		rlog.Error("failed to persist progress", "job_id", jobID, "err", err)
	}
	r.bus.Publish(jobID, buildProgressUpdate(int(n), total, nodeID, proxy, res, opts, baseline))
}

// buildProgressUpdate assembles one SSE event as a full, inheritance-applied
// NodeResult: measured dimensions win; unmeasured speed/upload/country fall back
// to the node's baseline; platforms merge per key (a freshly tested platform
// overrides, an untested one keeps its prior value). This keeps the live stream
// identical to GetResults.
func buildProgressUpdate(progress, total int, nodeID string, proxy map[string]any, res nodeCheckResult, opts CheckOptions, baseline inheritanceBaseline) progressUpdate {
	base := baseline[nodeKeyForProxy(proxy)]

	speed := res.SpeedKbps
	if speed == 0 {
		speed = base.speedKbps
	}
	upload := res.UploadSpeedKbps
	if upload == 0 {
		upload = base.uploadSpeedKbps
	}
	country := res.Country
	if country == "" {
		country = base.country
	}

	nodeType, _ := proxy["type"].(string)
	server, _ := proxy["server"].(string)
	configJSON, _ := json.Marshal(proxy)

	pu := progressUpdate{
		Progress:        progress,
		Total:           total,
		NodeID:          nodeID,
		NodeName:        res.NodeName,
		NodeType:        nodeType,
		Enabled:         true,
		Alive:           res.Alive,
		LatencyMs:       res.LatencyMs,
		SpeedKbps:       speed,
		UploadSpeedKbps: upload,
		Country:         country,
		IP:              res.IP,
		Server:          server,
		Port:            proxyPort(proxy),
		Config:          string(configJSON),
		Platforms:       mergePlatforms(base.platforms, res.Platforms),
		TrafficBytes:    res.TrafficBytes,
	}
	if opts.Debug && res.Debug != nil {
		pu.Debug = res.Debug
	}
	return pu
}

// nodeKeyForProxy mirrors nodeIdentityKey (SQL): a node's stable identity is
// server:port from its config, falling back to the display name.
func nodeKeyForProxy(proxy map[string]any) string {
	server, _ := proxy["server"].(string)
	if server == "" {
		name, _ := proxy["name"].(string)
		return name
	}
	port := ""
	if _, ok := proxy["port"]; ok {
		port = strconv.Itoa(proxyPort(proxy))
	}
	return server + ":" + port
}

// proxyPort extracts the port from a proxy map, tolerating the numeric types a
// JSON/YAML decode may yield.
func proxyPort(proxy map[string]any) int {
	switch v := proxy["port"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

// mergePlatforms overlays freshly measured platform outcomes onto the inherited
// baseline. Returns a new map; never mutates its inputs.
func mergePlatforms(base, fresh map[string]PlatformOutcome) map[string]PlatformOutcome {
	merged := make(map[string]PlatformOutcome, len(base)+len(fresh))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range fresh {
		merged[k] = v
	}
	return merged
}
