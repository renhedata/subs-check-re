package checker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	settingssvc "subs-check-re/services/settings"
)

func TestTaggedName_FromPlatformsMap(t *testing.T) {
	cfg := settingssvc.DefaultExportTags() // country off, speed on, builtin labels
	platforms := map[string]PlatformOutcome{
		"netflix":         {Unlocked: true, Status: "Yes", Region: "US"},
		"youtube":         {Unlocked: true, Status: "Yes"},
		"youtube_premium": {Unlocked: true, Status: "Yes"},
		"myplat":          {Unlocked: true, Status: "Yes"}, // genuinely custom (not in default cfg) → defaults to enabled, label=key
	}
	got := taggedName("HK-01", "HK", platforms, 2048, cfg)
	// country off; NF present; YT premium → "YT+"; custom key appended raw; speed 2048kbps → 2.0MB
	want := "HK-01|NF|YT+|myplat|2.0MB"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadJobProxies_InheritsPlatformsAfterAliveOnly(t *testing.T) {
	subID := "exp-sub-" + uuid.New().String()
	userID := "exp-user-" + uuid.New().String()
	jobA, jobB := uuid.New().String(), uuid.New().String()
	cfgRow := func(jobID string, ageHours int) {
		db.Exec(context.Background(), `
			INSERT INTO check_jobs (id, subscription_id, user_id, status, total, available, created_at, finished_at)
			VALUES ($1,$2,$3,'completed',1,1,NOW(),NOW())
		`, jobID, subID, userID)
	}
	cfgRow(jobA, 2)
	cfgRow(jobB, 0)

	nodeCfg := `{"type":"ss","server":"1.1.1.1","port":1,"name":"N1"}`
	pjA, _ := json.Marshal(map[string]PlatformOutcome{"netflix": {Unlocked: true}})
	// Older full check on N1.
	db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,'N1','ss',$4::jsonb, NOW() - interval '2 hours', true, 50, 2048, 'HK', '', $5)
	`, uuid.New().String(), jobA, uuid.New().String(), nodeCfg, pjA)
	// Newer alive-only check on N1: empty platforms, no speed/country.
	db.Exec(context.Background(), `
		INSERT INTO check_results (id, job_id, node_id, node_name, node_type, node_config, checked_at, alive, latency_ms, speed_kbps, country, ip, platforms)
		VALUES ($1,$2,$3,'N1','ss',$4::jsonb, NOW(), true, 30, 0, '', '', '{}'::jsonb)
	`, uuid.New().String(), jobB, uuid.New().String(), nodeCfg)

	cfg := settingssvc.DefaultExportTags() // netflix→"NF" enabled, speed on
	proxies, err := loadJobProxies(context.Background(), jobB, subID, "", cfg, exportPrefs{Sort: "speed_desc"})
	if err != nil {
		t.Fatalf("loadJobProxies: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("want 1 proxy, got %d", len(proxies))
	}
	name, _ := proxies[0]["name"].(string)
	if !strings.Contains(name, "NF") {
		t.Errorf("alive-only export must keep inherited Netflix tag, got %q", name)
	}
	if !strings.Contains(name, "2.0MB") {
		t.Errorf("alive-only export must keep inherited speed tag, got %q", name)
	}
}
