package checker

import (
	"context"
)

// TopNode represents one high-performing node for notification summaries.
type TopNode struct {
	Name      string `json:"name"`
	SpeedKbps int    `json:"speed_kbps"`
	LatencyMs int    `json:"latency_ms"`
	Country   string `json:"country"`
}

// JobDetailedSummary contains detailed statistics for a completed job.
type JobDetailedSummary struct {
	JobID            string                `json:"job_id"`
	SubscriptionName string                `json:"subscription_name"`
	Available        int                   `json:"available"`
	Total            int                   `json:"total"`
	Platforms        PlatformUnlockSummary `json:"platforms"`
	AvgSpeedKbps     int                   `json:"avg_speed_kbps"`
	MaxSpeedKbps     int                   `json:"max_speed_kbps"`
	AvgLatencyMs     int                   `json:"avg_latency_ms"`
	TopNodes         []TopNode             `json:"top_nodes"`
	Countries        map[string]int        `json:"countries"`
}

// GetJobDetailedSummary returns detailed statistics for a completed check job.
//
//encore:api private method=GET path=/internal/check/:jobID/summary
func GetJobDetailedSummary(ctx context.Context, jobID string) (*JobDetailedSummary, error) {
	return loadJobSummary(ctx, jobID)
}
