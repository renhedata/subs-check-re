package notify

import (
	"fmt"
	"sort"
	"strings"
)

var platformNames = map[string]string{
	"netflix":         "Netflix",
	"youtube":         "YouTube",
	"youtube_premium": "YouTube Premium",
	"openai":          "ChatGPT Web",
	"chatgpt_ios":     "ChatGPT iOS",
	"claude":          "Claude",
	"gemini":          "Gemini",
	"grok":            "Grok",
	"disney":          "Disney+",
	"tiktok":          "TikTok",
	"bilibili_cn":     "哔哩哔哩大陆",
	"bilibili_hkmctw": "哔哩哔哩港澳台",
	"bahamut":         "巴哈姆特动画疯",
	"spotify":         "Spotify",
	"prime_video":     "Prime Video",
}

// platformOrder controls display order in reports; keys not listed render last, sorted.
var platformOrder = []string{
	"netflix", "youtube", "youtube_premium", "openai", "chatgpt_ios",
	"claude", "gemini", "grok", "disney", "tiktok",
	"bilibili_cn", "bilibili_hkmctw", "bahamut", "spotify", "prime_video",
}

func platformDisplayName(key string) string {
	if n, ok := platformNames[key]; ok {
		return n
	}
	return key
}

// orderedPlatformKeys returns keys in platformOrder first, then any extra keys
// present in either map, sorted. Pass nil for the map you don't have.
func orderedPlatformKeys(boolM map[string]bool, intM map[string]int) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range platformOrder {
		if _, ok := boolM[k]; ok {
			out = append(out, k)
			seen[k] = true
			continue
		}
		if _, ok := intM[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var extra []string
	for k := range boolM {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	for k := range intM {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

func formatSpeed(kbps int) string {
	if kbps >= 1024 {
		return fmt.Sprintf("%.1f MB/s", float64(kbps)/1024)
	}
	return fmt.Sprintf("%d KB/s", kbps)
}

func formatCheckReport(s *JobReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("✅ <b>Check Completed</b>\n📋 %s\n", s.SubscriptionName))
	b.WriteString(fmt.Sprintf("📊 Available: <b>%d/%d</b> nodes\n", s.Available, s.Total))

	if s.MaxSpeedKbps > 0 {
		b.WriteString(fmt.Sprintf("\n⚡ <b>Speed:</b> avg %s, max %s\n",
			formatSpeed(s.AvgSpeedKbps), formatSpeed(s.MaxSpeedKbps)))
	}
	if s.AvgLatencyMs > 0 {
		b.WriteString(fmt.Sprintf("⏱ <b>Latency:</b> avg %dms\n", s.AvgLatencyMs))
	}

	var unlocked []string
	for _, key := range orderedPlatformKeys(map[string]bool(nil), s.Platforms) {
		if n := s.Platforms[key]; n > 0 {
			unlocked = append(unlocked, fmt.Sprintf("  %s: %d", platformDisplayName(key), n))
		}
	}
	if len(unlocked) > 0 {
		b.WriteString("\n🔓 <b>Platform unlocks:</b>\n")
		for _, line := range unlocked {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	if len(s.TopNodes) > 0 {
		b.WriteString("\n🏆 <b>Top fastest:</b>\n")
		for i, n := range s.TopNodes {
			country := ""
			if n.Country != "" {
				country = fmt.Sprintf(" (%s)", n.Country)
			}
			b.WriteString(fmt.Sprintf("  %d. %s — %s, %dms%s\n",
				i+1, n.Name, formatSpeed(n.SpeedKbps), n.LatencyMs, country))
		}
	}

	if len(s.Countries) > 0 {
		b.WriteString("\n🌍 <b>Countries:</b> ")
		type kv struct {
			k string
			v int
		}
		var pairs []kv
		for k, v := range s.Countries {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
		parts := make([]string, 0, len(pairs))
		for _, p := range pairs {
			parts = append(parts, fmt.Sprintf("%s(%d)", p.k, p.v))
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteByte('\n')
	}

	return b.String()
}

func formatUnlockReport(r *LocalUnlockReport) string {
	var b strings.Builder

	b.WriteString("🌐 <b>Network Unlock Report</b>\n")
	if r.IP != "" || r.Country != "" {
		b.WriteString(fmt.Sprintf("📍 %s %s\n", r.Country, r.IP))
	}

	b.WriteByte('\n')
	for _, key := range orderedPlatformKeys(r.Platforms, nil) {
		status := "❌"
		if r.Platforms[key] {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", platformDisplayName(key), status))
	}

	return b.String()
}

func formatPlatformAlert(subName string, lostPlatforms []string) string {
	names := make([]string, len(lostPlatforms))
	for i, p := range lostPlatforms {
		names[i] = platformDisplayName(p)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚠️ <b>Platform Alert</b>\n📋 %s\n\n", subName))
	b.WriteString("The following platforms are no longer accessible through this subscription:\n")
	for _, n := range names {
		b.WriteString(fmt.Sprintf("  ❌ %s\n", n))
	}
	b.WriteString("\n<i>Alert fires when a previously available platform becomes inaccessible after a check.</i>")
	return b.String()
}
