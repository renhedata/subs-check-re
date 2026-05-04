package checker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type defaultRule struct {
	name      string
	key       string
	ruleType  string
	sortOrder int
	def       any
}

var defaultRules = []defaultRule{
	{
		name:      "Netflix",
		key:       "netflix",
		ruleType:  "js",
		sortOrder: 0,
		def: ScriptDef{Code: `
var ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0";
var hdrs = {"User-Agent": ua, "Accept-Language": "en-US,en;q=0.9"};
var ids = ["81280792", "70143836"];
for (var i = 0; i < ids.length; i++) {
  var r = http_get("https://www.netflix.com/title/" + ids[i], {headers: hdrs});
  if (r.body.indexOf("Oh no!") === -1) { return true; }
}
return false;
`},
	},
	{
		name:      "YouTube",
		key:       "youtube",
		ruleType:  "condition",
		sortOrder: 1,
		def: ConditionDef{
			URL:             "https://www.youtube.com/",
			StatusCode:      200,
			BodyContains:    []string{"youtube"},
			BodyNotContains: []string{"not available in your country", "unavailable in your region"},
		},
	},
	{
		name:      "YouTube Premium",
		key:       "youtube_premium",
		ruleType:  "condition",
		sortOrder: 2,
		def: ConditionDef{
			URL:             "https://www.youtube.com/premium",
			BodyContainsAny: []string{"ad-free", "YouTube Premium"},
			BodyNotContains: []string{"Premium is not available in your country"},
		},
	},
	{
		name:      "OpenAI",
		key:       "openai",
		ruleType:  "js",
		sortOrder: 3,
		def: ScriptDef{Code: `
var r = http_get("https://api.openai.com/v1/models");
return r.status === 401 || r.status === 200;
`},
	},
	{
		name:      "Claude",
		key:       "claude",
		ruleType:  "js",
		sortOrder: 4,
		def: ScriptDef{Code: `
var r = http_get("https://claude.ai/", {headers: {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"}});
if (r.final_url.indexOf("app-unavailable-in-region") !== -1) return false;
if (r.final_url.indexOf("claude.ai") !== -1 && r.status === 200) return true;
return r.body.indexOf("claude") !== -1 || r.body.indexOf("anthropic") !== -1;
`},
	},
	{
		name:      "Gemini",
		key:       "gemini",
		ruleType:  "condition",
		sortOrder: 5,
		def: ConditionDef{
			URL:          "https://gemini.google.com/",
			StatusCode:   200,
			BodyContains: []string{"Meet Gemini"},
		},
	},
	{
		name:      "Grok",
		key:       "grok",
		ruleType:  "js",
		sortOrder: 6,
		def: ScriptDef{Code: `
var r = http_get("https://api.x.ai/v1/models");
return r.status === 401 || r.status === 200;
`},
	},
	{
		name:      "Disney+",
		key:       "disney",
		ruleType:  "condition",
		sortOrder: 7,
		def: ConditionDef{
			URL:        "https://www.disneyplus.com/",
			StatusCode: 200,
		},
	},
	{
		name:      "TikTok",
		key:       "tiktok",
		ruleType:  "js",
		sortOrder: 8,
		def: ScriptDef{Code: `
var r = http_get("https://www.tiktok.com/", {headers: {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0"}});
if (r.status !== 200) return false;
if (r.final_url.indexOf("comingsoon") !== -1 || r.final_url.indexOf("not-available") !== -1) return false;
if (r.body.indexOf("not available in your region") !== -1 || r.body.indexOf("TikTok is not available") !== -1) return false;
if (/"region"\s*:\s*"[A-Z]{2}"/.test(r.body)) return true;
return r.body.indexOf("ttwstatic.com") !== -1 || r.body.indexOf("tiktokcdn.com") !== -1 || r.body.indexOf("bytedance") !== -1;
`},
	},
}

// seedDefaultRules inserts the 9 built-in platform rules for a new user.
func seedDefaultRules(ctx context.Context, userID string) error {
	now := time.Now()
	for _, dr := range defaultRules {
		defJSON, _ := json.Marshal(dr.def)
		if _, err := db.Exec(ctx, `
			INSERT INTO platform_rules (id, user_id, name, key, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,true,$5,$6,true,$7,$8,$8)
			ON CONFLICT (user_id, key) DO NOTHING
		`, uuid.New().String(), userID, dr.name, dr.key, dr.ruleType, defJSON, dr.sortOrder, now); err != nil {
			return err
		}
	}
	return nil
}
