// services/checker/export_test.go
package checker

import (
	"context"
	"fmt"
	"testing"
	"time"

	settingssvc "subs-check-re/services/settings"
)

func expUniq() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func TestTaggedNameDefaultConfigFullOrder(t *testing.T) {
	// All built-ins unlocked under the real defaults must reproduce the exact
	// legacy export name (order + labels), with premium rendering YT+.
	platforms := map[string]PlatformOutcome{
		"netflix":         {Unlocked: true, Status: "Yes"},
		"youtube":         {Unlocked: true, Status: "Yes"},
		"youtube_premium": {Unlocked: true, Status: "Yes"},
		"openai":          {Unlocked: true, Status: "Yes"},
		"claude":          {Unlocked: true, Status: "Yes"},
		"gemini":          {Unlocked: true, Status: "Yes"},
		"grok":            {Unlocked: true, Status: "Yes"},
		"disney":          {Unlocked: true, Status: "Yes"},
		"tiktok":          {Unlocked: true, Status: "Yes"},
	}
	got := taggedName("N", "HK", platforms, 1536, settingssvc.DefaultExportTags())
	if got != "N|NF|GPT|GM|CL|GK|YT+|D+|TK|1.5MB" {
		t.Errorf("default full order: got %q", got)
	}
}

func TestTaggedNamePremiumOnlyEmitsTag(t *testing.T) {
	// A node flagged premium but not basic youtube must still emit the tag
	// (the two unlock checks hit different URLs and can disagree).
	platforms := map[string]PlatformOutcome{"youtube_premium": {Unlocked: true, Status: "Yes"}}
	got := taggedName("N", "", platforms, 0, settingssvc.DefaultExportTags())
	if got != "N|YT+" {
		t.Errorf("premium-only: got %q", got)
	}
}

func legacyCfg() settingssvc.ExportTagConfig {
	return settingssvc.ExportTagConfig{
		ShowCountry: false,
		ShowSpeed:   true,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "NF", Enabled: true},
			{Key: "openai", Label: "GPT", Enabled: true},
			{Key: "youtube", Label: "YT", Enabled: true},
		},
	}
}

func TestTaggedNameLegacyDefault(t *testing.T) {
	platforms := map[string]PlatformOutcome{
		"netflix": {Unlocked: true, Status: "Yes"},
		"openai":  {Unlocked: true, Status: "Yes"},
	}
	got := taggedName("HK-01", "HK", platforms, 1536, legacyCfg())
	if got != "HK-01|NF|GPT|1.5MB" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCountryAndPremiumAndDisabled(t *testing.T) {
	cfg := legacyCfg()
	cfg.ShowCountry = true
	cfg.Platforms[1].Enabled = false // disable openai
	platforms := map[string]PlatformOutcome{
		"netflix":         {Unlocked: true, Status: "Yes"},
		"openai":          {Unlocked: true, Status: "Yes"},
		"youtube":         {Unlocked: true, Status: "Yes"},
		"youtube_premium": {Unlocked: true, Status: "Yes"},
	}
	got := taggedName("JP-1", "JP", platforms, 0, cfg)
	if got != "JP-1|JP|NF|YT+" {
		t.Errorf("got %q", got)
	}
}

func TestTaggedNameCustomLabelAndExtraSorted(t *testing.T) {
	cfg := settingssvc.ExportTagConfig{
		ShowSpeed: false,
		Platforms: []settingssvc.PlatformTag{
			{Key: "netflix", Label: "Netflix", Enabled: true},
			{Key: "spotify", Label: "Spotify", Enabled: true},
		},
	}
	platforms := map[string]PlatformOutcome{
		"netflix": {Unlocked: true, Status: "Yes"},
		"zlib":    {Unlocked: true, Status: "Yes"},
		"spotify": {Unlocked: true, Status: "Yes"},
		"off":     {Unlocked: false, Status: "No"},
	}
	got := taggedName("US", "US", platforms, 999, cfg)
	if got != "US|Netflix|Spotify|zlib" {
		t.Errorf("got %q", got)
	}
}

func seedExportNode(t *testing.T, ctx context.Context, subID, jobID, name string, alive bool, speedKbps int, latencyMs *int) {
	t.Helper()
	nodeID := "expn-" + name + "-" + jobID
	cfg := `{"type":"ss","name":"` + name + `","server":"1.1.1.1","port":1,"cipher":"aes-256-gcm","password":"x"}`
	if _, err := db.Exec(ctx, `
		INSERT INTO nodes (id, subscription_id, name, type, server, port, config, enabled)
		VALUES ($1,$2,$3,'ss','1.1.1.1',1,$4::jsonb,true)
	`, nodeID, subID, name, cfg); err != nil {
		t.Fatalf("seed node %s: %v", name, err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, alive, latency_ms, speed_kbps, upload_speed_kbps, country, ip, traffic_bytes)
		VALUES ($1,$2,$3,$4,'ss',$5::jsonb,$6,$7,$8,0,'','',0)
	`, "expr-"+name+"-"+jobID, jobID, nodeID, name, cfg, alive, latencyMs, speedKbps); err != nil {
		t.Fatalf("seed result %s: %v", name, err)
	}
}

func proxyNames(proxies []map[string]any) []string {
	out := make([]string, len(proxies))
	for i, p := range proxies {
		out[i], _ = p["name"].(string)
	}
	return out
}

func TestLoadJobProxiesIncludeDeadAndSort(t *testing.T) {
	ctx := context.Background()
	subID := "expsub-" + expUniq()
	jobID := "expjob-" + expUniq()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
		VALUES ($1,$2,'u','completed',3,2,NOW(),NOW())
	`, jobID, subID); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	lat100, lat30 := 100, 30
	seedExportNode(t, ctx, subID, jobID, "A", true, 2000, &lat100) // fast, slow latency
	seedExportNode(t, ctx, subID, jobID, "C", true, 500, &lat30)   // slow, fast latency
	seedExportNode(t, ctx, subID, jobID, "B", false, 0, nil)       // dead

	defCfg := settingssvc.ExportTagConfig{} // tags irrelevant here

	// default: exclude dead, speed desc -> [A, C]
	got, err := loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: false, Sort: "speed_desc"})
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if g := proxyNames(got); len(g) != 2 || g[0] != "A" || g[1] != "C" {
		t.Errorf("default speed_desc exclude-dead: got %v", g)
	}

	// include dead, speed desc -> [A, C, B] (dead speed 0 last)
	got, _ = loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: true, Sort: "speed_desc"})
	if g := proxyNames(got); len(g) != 3 || g[2] != "B" {
		t.Errorf("include-dead speed_desc: got %v", g)
	}

	// exclude dead, latency asc -> [C, A]
	got, _ = loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: false, Sort: "latency_asc"})
	if g := proxyNames(got); len(g) != 2 || g[0] != "C" || g[1] != "A" {
		t.Errorf("latency_asc exclude-dead: got %v", g)
	}

	// include dead, latency asc -> [C, A, B]: the dead node has NULL latency and
	// must sort LAST (NULLS LAST), not first (regression: a Go re-sort treated
	// NULL as 0 and hoisted it to the top).
	got, _ = loadJobProxies(ctx, jobID, subID, "", defCfg, exportPrefs{IncludeDead: true, Sort: "latency_asc"})
	if g := proxyNames(got); len(g) != 3 || g[0] != "C" || g[1] != "A" || g[2] != "B" {
		t.Errorf("include-dead latency_asc: got %v", g)
	}
}
