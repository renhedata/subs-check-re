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
	"openai":          "OpenAI",
	"claude":          "Claude",
	"gemini":          "Gemini",
	"grok":            "Grok",
	"disney":          "Disney+",
	"tiktok":          "TikTok",
}

func platformDisplayName(key string) string {
	if n, ok := platformNames[key]; ok {
		return n
	}
	return key
}

type platformEntry struct {
	emoji string
	label string
	count int
}

func platformEntries(p PlatformCounts) []platformEntry {
	return []platformEntry{
		{"🎬", "Netflix", p.Netflix},
		{"▶️", "YouTube", p.YouTube},
		{"⭐", "YouTube Premium", p.YouTubePremium},
		{"🤖", "OpenAI", p.OpenAI},
		{"🧠", "Claude", p.Claude},
		{"💎", "Gemini", p.Gemini},
		{"⚡", "Grok", p.Grok},
		{"🏰", "Disney+", p.Disney},
		{"🎵", "TikTok", p.TikTok},
	}
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

	entries := platformEntries(s.Platforms)
	var unlocked []string
	for _, e := range entries {
		if e.count > 0 {
			unlocked = append(unlocked, fmt.Sprintf("  %s %s: %d", e.emoji, e.label, e.count))
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

	type item struct {
		emoji string
		name  string
		ok    bool
	}
	platforms := []item{
		{"🎬", "Netflix", r.Netflix},
		{"▶️", "YouTube", r.YouTube},
		{"⭐", "YouTube Premium", r.YouTubePremium},
		{"🤖", "OpenAI", r.OpenAI},
		{"🧠", "Claude", r.Claude},
		{"💎", "Gemini", r.Gemini},
		{"⚡", "Grok", r.Grok},
		{"🏰", "Disney+", r.Disney},
		{"🎵", "TikTok", r.TikTok},
	}

	b.WriteByte('\n')
	for _, p := range platforms {
		status := "❌"
		if p.ok {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s %s %s\n", p.emoji, p.name, status))
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
