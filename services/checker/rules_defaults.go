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
	icon      string
	ruleType  string
	sortOrder int
	def       any
}

var defaultRules = []defaultRule{
	{name: "Netflix", key: "netflix", icon: "simple-icons:netflix", ruleType: "js", sortOrder: 0, def: ScriptDef{Code: `
var cdn = http_get("https://api.fast.com/netflix/speedtest/v2?https=true&token=YXNkZmFzZGxmbnNkYWZoYXNkZmhrYWxm&urlCount=5");
if (cdn.status === 403) return {unlocked:false,status:"No (IP Banned)",region:""};
try { var d = JSON.parse(cdn.body); if (d.targets && d.targets.length && d.targets[0].location && d.targets[0].location.country) return {unlocked:true,status:"Yes",region:d.targets[0].location.country}; } catch(e){}
var ua = {"User-Agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36","Accept-Language":"en-US,en;q=0.9"};
var r1 = http_get("https://www.netflix.com/title/81280792",{headers:ua});
var r2 = http_get("https://www.netflix.com/title/70143836",{headers:ua});
if (r1.status===404 && r2.status===404) return {unlocked:false,status:"Originals Only",region:""};
if (r1.status===403 || r2.status===403) return {unlocked:false,status:"No",region:""};
if (r1.status===200||r1.status===301||r2.status===200||r2.status===301) return {unlocked:true,status:"Yes",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "YouTube", key: "youtube", icon: "simple-icons:youtube", ruleType: "js", sortOrder: 1, def: ScriptDef{Code: `
var r = http_get("https://www.youtube.com/");
if (r.status !== 200) return {unlocked:false,status:"Failed",region:""};
var b = (r.body||"").toLowerCase();
if (b.indexOf("not available in your country")!==-1 || b.indexOf("unavailable in your region")!==-1) return {unlocked:false,status:"No",region:""};
if (b.indexOf("youtube")!==-1) return {unlocked:true,status:"Yes",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "YouTube Premium", key: "youtube_premium", icon: "simple-icons:youtube", ruleType: "js", sortOrder: 2, def: ScriptDef{Code: `
var r = http_get("https://www.youtube.com/premium?hl=en");
var body = r.body||""; var b = body.toLowerCase(); var region="";
var pats = [/id=["']country-code["'][^>]*>\s*([A-Za-z]{2,3})\s*</, /"GL"\s*:\s*"([A-Za-z]{2})"/, /"countryCode"\s*:\s*"([A-Za-z]{2})"/, /"country_code"\s*:\s*"([A-Za-z]{2})"/];
for (var i=0;i<pats.length;i++){ var m=body.match(pats[i]); if(m){region=m[1].toUpperCase();break;} }
if (b.indexOf("youtube premium is not available in your country")!==-1 || b.indexOf("premium is not available in your country")!==-1 || b.indexOf("premium is not available in your region")!==-1) return {unlocked:false,status:"No",region:region};
if (r.status>=200 && r.status<300 && (b.indexOf("youtube premium")!==-1 || b.indexOf("ad-free")!==-1 || b.indexOf('"browseid":"spunlimited"')!==-1)) return {unlocked:true,status:"Yes",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "ChatGPT Web", key: "openai", icon: "simple-icons:openai", ruleType: "js", sortOrder: 3, def: ScriptDef{Code: `
var region=""; var t = http_get("https://chat.openai.com/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(m) region=m[2];
var r = http_get("https://api.openai.com/compliance/cookie_requirements");
if ((r.body||"").toLowerCase().indexOf("unsupported_country")!==-1) return {unlocked:false,status:"Unsupported Country",region:region};
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "ChatGPT iOS", key: "chatgpt_ios", icon: "simple-icons:openai", ruleType: "js", sortOrder: 4, def: ScriptDef{Code: `
var region=""; var t = http_get("https://chat.openai.com/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(m) region=m[2];
var r = http_get("https://ios.chat.openai.com/"); var b=(r.body||"").toLowerCase();
if (b.indexOf("you may be connected to a disallowed isp")!==-1) return {unlocked:false,status:"Disallowed ISP",region:region};
if (b.indexOf("request is not allowed. please try again later.")!==-1) return {unlocked:true,status:"Yes",region:region};
if (b.indexOf("sorry, you have been blocked")!==-1) return {unlocked:false,status:"Blocked",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "Claude", key: "claude", icon: "simple-icons:anthropic", ruleType: "js", sortOrder: 5, def: ScriptDef{Code: `
var t = http_get("https://claude.ai/cdn-cgi/trace"); var m=(t.body||"").match(/(^|\n)loc=([A-Z]{2})/);
if (!m) return {unlocked:false,status:"Failed",region:""};
var code=m[2]; var blocked=["AF","BY","CN","CU","HK","IR","KP","MO","RU","SY"];
var ok = blocked.indexOf(code)===-1;
return {unlocked:ok,status:ok?"Yes":"No",region:code};
`}},
	{name: "Gemini", key: "gemini", icon: "simple-icons:googlegemini", ruleType: "js", sortOrder: 6, def: ScriptDef{Code: `
var r = http_get("https://gemini.google.com"); var body=r.body||""; var marker=",2,1,200,\""; var idx=body.indexOf(marker); var code="";
if (idx!==-1) code=body.substr(idx+marker.length,3);
if (!/^[A-Z]{3}$/.test(code)) return {unlocked:false,status:"Failed",region:""};
var blocked=["CHN","RUS","BLR","CUB","IRN","PRK","SYR","HKG","MAC"]; var ok=blocked.indexOf(code)===-1;
return {unlocked:ok,status:ok?"Yes":"No",region:code};
`}},
	{name: "Grok", key: "grok", icon: "simple-icons:x", ruleType: "js", sortOrder: 7, def: ScriptDef{Code: `
var r = http_get("https://api.x.ai/v1/models"); var ok=r.status===401||r.status===200;
return {unlocked:ok,status:ok?"Yes":"No",region:""};
`}},
	{name: "Disney+", key: "disney", icon: "simple-icons:disneyplus", ruleType: "js", sortOrder: 8, def: ScriptDef{Code: `
var AUTH="Bearer ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84";
function mainRegion(){ var r=http_get("https://www.disneyplus.com/"); var m=(r.body||"").match(/region"\s*:\s*"([^"]+)"/); return m?m[1]:""; }
var dev=http_post("https://disney.api.edge.bamgrid.com/devices",{headers:{"authorization":AUTH,"content-type":"application/json; charset=UTF-8"},body:JSON.stringify({deviceFamily:"browser",applicationRuntime:"chrome",deviceProfile:"windows",attributes:{}})});
if (dev.status===403) return {unlocked:false,status:"No (IP Banned)",region:""};
var am=(dev.body||"").match(/"assertion"\s*:\s*"([^"]+)"/); if(!am) return {unlocked:false,status:"Failed",region:""};
var form="grant_type=urn:ietf:params:oauth:grant-type:token-exchange&latitude=0&longitude=0&platform=browser&subject_token="+encodeURIComponent(am[1])+"&subject_token_type=urn:bamtech:params:oauth:token-type:device";
var tok=http_post("https://disney.api.edge.bamgrid.com/token",{headers:{"authorization":AUTH,"content-type":"application/x-www-form-urlencoded"},body:form});
if ((tok.body||"").indexOf("forbidden-location")!==-1 || (tok.body||"").indexOf("403 ERROR")!==-1) return {unlocked:false,status:"No (IP Banned)",region:""};
var rt=""; try{ rt=JSON.parse(tok.body).refresh_token||""; }catch(e){ var rm=(tok.body||"").match(/"refresh_token"\s*:\s*"([^"]+)"/); rt=rm?rm[1]:""; }
if (!rt){ var reg=mainRegion(); if(reg) return {unlocked:true,status:"Yes",region:reg}; return {unlocked:false,status:"Failed",region:""}; }
var gql=JSON.stringify({query:"mutation refreshToken($input: RefreshTokenInput!) { refreshToken(refreshToken: $input) { activeSession { sessionId } } }",variables:{input:{refreshToken:rt}}});
var g=http_post("https://disney.api.edge.bamgrid.com/graph/v1/device/graphql",{headers:{"authorization":AUTH,"content-type":"application/json"},body:gql});
var prev=http_get("https://disneyplus.com"); var unavailable=prev.final_url.indexOf("preview")!==-1||prev.final_url.indexOf("unavailable")!==-1;
if (!g.body || g.status>=400){ var reg2=mainRegion(); if(reg2) return {unlocked:true,status:"Yes",region:reg2}; return {unlocked:false,status:"Failed",region:""}; }
var cm=g.body.match(/"countryCode"\s*:\s*"([^"]+)"/); var sm=g.body.match(/"inSupportedLocation"\s*:\s*(false|true)/);
if (!cm){ var reg3=mainRegion(); if(reg3) return {unlocked:true,status:"Yes",region:reg3}; return {unlocked:false,status:"No",region:""}; }
var region=cm[1];
if (region==="JP") return {unlocked:true,status:"Yes",region:region};
if (unavailable) return {unlocked:false,status:"No",region:region};
if (sm && sm[1]==="false") return {unlocked:false,status:"Soon",region:region};
if (sm && sm[1]==="true") return {unlocked:true,status:"Yes",region:region};
return {unlocked:false,status:"Failed",region:region};
`}},
	{name: "TikTok", key: "tiktok", icon: "simple-icons:tiktok", ruleType: "js", sortOrder: 9, def: ScriptDef{Code: `
function st(s,b){ if(s===403||s===451) return "No"; if(!(s>=200&&s<300)) return "Failed"; var t=b.toLowerCase(); if(t.indexOf("access denied")!==-1||t.indexOf("not available in your region")!==-1||t.indexOf("tiktok is not available")!==-1) return "No"; return "Yes"; }
function rg(b){ var m=b.match(/"region"\s*:\s*"([a-zA-Z-]+)"/); if(m) return m[1].split("-")[0].toUpperCase(); return ""; }
var r=http_get("https://www.tiktok.com/cdn-cgi/trace"); var region=""; var lm=(r.body||"").match(/(^|\n)loc=([A-Z]{2})/); if(lm) region=lm[2];
var status=st(r.status, r.body||"");
if (region==="" || status==="Failed"){ var r2=http_get("https://www.tiktok.com/"); var s2=st(r2.status,r2.body||""); var g2=rg(r2.body||""); if(status!=="No") status=s2; if(region==="") region=g2; }
var ok=status==="Yes";
return {unlocked:ok,status:status,region:region};
`}},
	{name: "哔哩哔哩大陆", key: "bilibili_cn", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 10, def: ScriptDef{Code: `
var r=http_get("https://api.bilibili.com/pgc/player/web/playurl?avid=82846771&qn=0&type=&otype=json&ep_id=307247&fourk=1&fnver=0&fnval=16&module=bangumi");
var code=null; try{ code=JSON.parse(r.body).code; }catch(e){ return {unlocked:false,status:"Failed",region:""}; }
if (code===0) return {unlocked:true,status:"Yes",region:""};
if (code===-10403) return {unlocked:false,status:"No",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "哔哩哔哩港澳台", key: "bilibili_hkmctw", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 11, def: ScriptDef{Code: `
var r=http_get("https://api.bilibili.com/pgc/player/web/playurl?avid=18281381&cid=29892777&qn=0&type=&otype=json&ep_id=183799&fourk=1&fnver=0&fnval=16&module=bangumi");
var code=null; try{ code=JSON.parse(r.body).code; }catch(e){ return {unlocked:false,status:"Failed",region:""}; }
if (code===0) return {unlocked:true,status:"Yes",region:""};
if (code===-10403) return {unlocked:false,status:"No",region:""};
return {unlocked:false,status:"Failed",region:""};
`}},
	{name: "巴哈姆特动画疯", key: "bahamut", icon: "simple-icons:bilibili", ruleType: "js", sortOrder: 12, def: ScriptDef{Code: `
var d=http_get("https://ani.gamer.com.tw/ajax/getdeviceid.php"); var dm=(d.body||"").match(/"deviceid"\s*:\s*"([^"]+)"/);
if (!dm) return {unlocked:false,status:"Failed",region:""};
var t=http_get("https://ani.gamer.com.tw/ajax/token.php?adID=89422&sn=37783&device="+dm[1]);
if ((t.body||"").indexOf("animeSn")===-1) return {unlocked:false,status:"No",region:""};
var region=""; var h=http_get("https://ani.gamer.com.tw/"); var rm=(h.body||"").match(/data-geo="([^"]+)"/); if(rm) region=rm[1];
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "Spotify", key: "spotify", icon: "simple-icons:spotify", ruleType: "js", sortOrder: 13, def: ScriptDef{Code: `
var r=http_get("https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json"); var region="";
try{ var path=(r.final_url||"").replace(/^https?:\/\/[^/]+/,""); var segs=path.split("/"); for(var i=0;i<segs.length;i++){ if(segs[i]&&segs[i]!=="api"){ region=segs[i].split("-")[0].toUpperCase(); break; } } }catch(e){}
if (region===""){ var m=(r.body||"").match(/"countryCode":"([^"]+)"/); if(m) region=m[1].toUpperCase(); }
if (r.status===403||r.status===451) return {unlocked:false,status:"No",region:region};
if (!(r.status>=200&&r.status<300)) return {unlocked:false,status:"Failed",region:region};
if ((r.body||"").toLowerCase().indexOf("not available in your country")!==-1) return {unlocked:false,status:"No",region:region};
return {unlocked:true,status:"Yes",region:region};
`}},
	{name: "Prime Video", key: "prime_video", icon: "simple-icons:primevideo", ruleType: "js", sortOrder: 14, def: ScriptDef{Code: `
var r=http_get("https://www.primevideo.com"); var body=r.body||"";
var blocked=body.indexOf("isServiceRestricted")!==-1; var m=body.match(/"currentTerritory":"([^"]+)"/);
if (blocked) return {unlocked:false,status:"No",region:""};
if (m) return {unlocked:true,status:"Yes",region:m[1]};
return {unlocked:false,status:"Failed",region:""};
`}},
}

// seedDefaultRules inserts the built-in platform rules for a new user.
func seedDefaultRules(ctx context.Context, userID string) error {
	now := time.Now()
	for _, dr := range defaultRules {
		defJSON, _ := json.Marshal(dr.def)
		if _, err := db.Exec(ctx, `
			INSERT INTO platform_rules (id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,true,$6,$7,true,$8,$9,$9)
			ON CONFLICT (user_id, key) DO NOTHING
		`, uuid.New().String(), userID, dr.name, dr.key, dr.icon, dr.ruleType, defJSON, dr.sortOrder, now); err != nil {
			return err
		}
	}
	return nil
}
