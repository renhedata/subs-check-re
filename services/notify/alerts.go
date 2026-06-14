package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// checkPlatformAlerts compares the current job's platform counts against the stored
// last-known state and sends alerts for any platform that dropped from available to zero.
func checkPlatformAlerts(ctx context.Context, userID, subID string, summary *JobReport) {
	current := map[string]bool{}
	for key, count := range summary.Platforms {
		current[key] = count > 0
	}

	prev := loadPreviousPlatformState(ctx, subID)
	upsertPlatformState(ctx, userID, subID, current)

	var lostPlatforms []string
	for platform, nowAvail := range current {
		if wasAvail, hadPrev := prev[platform]; hadPrev && wasAvail && !nowAvail {
			lostPlatforms = append(lostPlatforms, platform)
		}
	}
	if len(lostPlatforms) == 0 {
		return
	}

	deliverPlatformAlerts(ctx, userID, summary.SubscriptionName, lostPlatforms)
}

func loadPreviousPlatformState(ctx context.Context, subID string) map[string]bool {
	prev := map[string]bool{}
	rows, err := db.Query(ctx,
		`SELECT platform, available FROM subscription_platform_state WHERE subscription_id=$1`, subID)
	if err != nil {
		return prev
	}
	defer rows.Close()
	for rows.Next() {
		var platform string
		var available bool
		if rows.Scan(&platform, &available) == nil {
			prev[platform] = available
		}
	}
	return prev
}

func upsertPlatformState(ctx context.Context, userID, subID string, current map[string]bool) {
	for platform, available := range current {
		_, _ = db.Exec(ctx, `
			INSERT INTO subscription_platform_state (subscription_id, user_id, platform, available, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (subscription_id, platform) DO UPDATE SET available=$4, updated_at=NOW()
		`, subID, userID, platform, available)
	}
}

func deliverPlatformAlerts(ctx context.Context, userID, subName string, lostPlatforms []string) {
	chRows, err := db.Query(ctx, `
		SELECT type, config, platform_alerts FROM notify_channels
		WHERE user_id=$1 AND enabled=true AND platform_alerts != '[]'::jsonb
	`, userID)
	if err != nil {
		return
	}
	defer chRows.Close()

	for chRows.Next() {
		var chType string
		var configJSON, alertsJSON []byte
		if err := chRows.Scan(&chType, &configJSON, &alertsJSON); err != nil {
			continue
		}
		var watched []string
		_ = json.Unmarshal(alertsJSON, &watched)

		matched := intersect(lostPlatforms, watched)
		if len(matched) == 0 {
			continue
		}

		sender := senderFor(chType, configJSON)
		if sender == nil {
			continue
		}

		names := make([]string, len(matched))
		for i, p := range matched {
			names[i] = platformDisplayName(p)
		}
		subject := fmt.Sprintf("⚠️ Platform Alert — %s", strings.Join(names, ", "))
		msg := formatPlatformAlert(subName, matched)

		switch chType {
		case "webhook":
			_ = sender.SendPayload(ctx, userID, map[string]any{
				"type":           "platform_alert",
				"subscription":   subName,
				"lost_platforms": matched,
				"timestamp":      time.Now().UTC(),
			})
		default:
			_ = sender.SendMessage(ctx, userID, subject, msg)
		}
	}
}

func intersect(lost, watched []string) []string {
	var matched []string
	for _, l := range lost {
		for _, w := range watched {
			if l == w {
				matched = append(matched, l)
				break
			}
		}
	}
	return matched
}
