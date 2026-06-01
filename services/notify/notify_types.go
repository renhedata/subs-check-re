package notify

import (
	"encoding/json"
	"time"

	checkersvc "subs-check-re/services/checker"
)

// --- Channel DTOs (API surface) ---

// Channel represents a notification channel.
type Channel struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Config          json.RawMessage `json:"config"`
	Enabled         bool            `json:"enabled"`
	OnCheckComplete bool            `json:"on_check_complete"`
	UnlockCron      string          `json:"unlock_cron"`
	PlatformAlerts  []string        `json:"platform_alerts"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ListChannelsResponse is the response for GET /notify/channels.
type ListChannelsResponse struct {
	Channels []Channel `json:"channels"`
}

// CreateChannelParams is the request body for POST /notify/channels.
type CreateChannelParams struct {
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Config          json.RawMessage `json:"config"`
	OnCheckComplete bool            `json:"on_check_complete"`
	UnlockCron      string          `json:"unlock_cron"`
	PlatformAlerts  []string        `json:"platform_alerts"`
}

// UpdateChannelParams is the request body for PUT /notify/channels/:id.
type UpdateChannelParams struct {
	Name            *string         `json:"name"`
	Config          json.RawMessage `json:"config"`
	Enabled         *bool           `json:"enabled"`
	OnCheckComplete *bool           `json:"on_check_complete"`
	UnlockCron      *string         `json:"unlock_cron"`
	PlatformAlerts  []string        `json:"platform_alerts"`
}

// DeleteChannelResponse is the response for DELETE /notify/channels/:id.
type DeleteChannelResponse struct {
	OK bool `json:"ok"`
}

// TestChannelParams selects which report type to test.
type TestChannelParams struct {
	ReportType string `json:"report_type"` // "check", "unlock", or "platform_alert"
}

// TestChannelResponse is the response for POST /notify/channels/:id/test.
type TestChannelResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// --- Internal report DTOs ---
//
// These mirror the checker service's response types but live in notify so that
// formatters depend only on notify's own model. Cross-service data enters
// through [fromCheckerSummary] / [fromCheckerLocalUnlock] at the boundary.

// JobReport is the notification-side view of a completed check job.
type JobReport struct {
	JobID            string
	SubscriptionName string
	Available        int
	Total            int
	Platforms        PlatformCounts
	AvgSpeedKbps     int
	MaxSpeedKbps     int
	AvgLatencyMs     int
	TopNodes         []TopNode
	Countries        map[string]int
}

// PlatformCounts holds per-platform unlock counts for a job.
type PlatformCounts struct {
	Netflix        int
	YouTube        int
	YouTubePremium int
	OpenAI         int
	Claude         int
	Gemini         int
	Grok           int
	Disney         int
	TikTok         int
}

// TopNode is one high-performing node row in a JobReport.
type TopNode struct {
	Name      string
	SpeedKbps int
	LatencyMs int
	Country   string
}

// LocalUnlockReport is the notification-side view of a local-network unlock probe.
type LocalUnlockReport struct {
	IP             string
	Country        string
	Netflix        bool
	YouTube        bool
	YouTubePremium bool
	OpenAI         bool
	Claude         bool
	Gemini         bool
	Grok           bool
	Disney         bool
	TikTok         bool
}

// fromCheckerSummary copies the cross-service payload into the local DTO so that
// formatters and senders never depend on the checker package's types.
func fromCheckerSummary(s *checkersvc.JobDetailedSummary) *JobReport {
	r := &JobReport{
		JobID:            s.JobID,
		SubscriptionName: s.SubscriptionName,
		Available:        s.Available,
		Total:            s.Total,
		Platforms: PlatformCounts{
			Netflix:        s.Platforms.Netflix,
			YouTube:        s.Platforms.YouTube,
			YouTubePremium: s.Platforms.YouTubePremium,
			OpenAI:         s.Platforms.OpenAI,
			Claude:         s.Platforms.Claude,
			Gemini:         s.Platforms.Gemini,
			Grok:           s.Platforms.Grok,
			Disney:         s.Platforms.Disney,
			TikTok:         s.Platforms.TikTok,
		},
		AvgSpeedKbps: s.AvgSpeedKbps,
		MaxSpeedKbps: s.MaxSpeedKbps,
		AvgLatencyMs: s.AvgLatencyMs,
		Countries:    s.Countries,
		TopNodes:     make([]TopNode, 0, len(s.TopNodes)),
	}
	for _, n := range s.TopNodes {
		r.TopNodes = append(r.TopNodes, TopNode{
			Name: n.Name, SpeedKbps: n.SpeedKbps,
			LatencyMs: n.LatencyMs, Country: n.Country,
		})
	}
	if r.Countries == nil {
		r.Countries = map[string]int{}
	}
	return r
}

func fromCheckerLocalUnlock(r *checkersvc.LocalUnlockResult) *LocalUnlockReport {
	return &LocalUnlockReport{
		IP: r.IP, Country: r.Country,
		Netflix: r.Netflix, YouTube: r.YouTube, YouTubePremium: r.YouTubePremium,
		OpenAI: r.OpenAI, Claude: r.Claude, Gemini: r.Gemini,
		Grok: r.Grok, Disney: r.Disney, TikTok: r.TikTok,
	}
}
