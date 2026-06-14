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

// PlatformCounts maps a platform key to how many nodes unlocked it.
type PlatformCounts map[string]int

// TopNode is one high-performing node row in a JobReport.
type TopNode struct {
	Name      string
	SpeedKbps int
	LatencyMs int
	Country   string
}

// LocalUnlockReport is the notification-side view of a local-network unlock probe.
type LocalUnlockReport struct {
	IP        string
	Country   string
	Platforms map[string]bool
}

// fromCheckerSummary copies the cross-service payload into the local DTO so that
// formatters and senders never depend on the checker package's types.
func fromCheckerSummary(s *checkersvc.JobDetailedSummary) *JobReport {
	r := &JobReport{
		JobID:            s.JobID,
		SubscriptionName: s.SubscriptionName,
		Available:        s.Available,
		Total:            s.Total,
		Platforms:        PlatformCounts(s.Platforms),
		AvgSpeedKbps:     s.AvgSpeedKbps,
		MaxSpeedKbps:     s.MaxSpeedKbps,
		AvgLatencyMs:     s.AvgLatencyMs,
		Countries:        s.Countries,
		TopNodes:         make([]TopNode, 0, len(s.TopNodes)),
	}
	if r.Platforms == nil {
		r.Platforms = PlatformCounts{}
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
	out := &LocalUnlockReport{IP: r.IP, Country: r.Country, Platforms: map[string]bool{}}
	for k, v := range r.Platforms {
		out.Platforms[k] = v.Unlocked
	}
	return out
}
