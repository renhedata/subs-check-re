// services/notify/notify.go
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	authsvc "subs-check-re/services/auth"
	checkersvc "subs-check-re/services/checker"
)

var db = sqldb.NewDatabase("notify", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- Service lifecycle ---

//encore:service
type Service struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID // channel_id → cron entry ID
	mu      sync.Mutex
}

func initService() (*Service, error) {
	svc := &Service{
		cron:    cron.New(),
		entries: make(map[string]cron.EntryID),
	}

	ctx := context.Background()
	rows, err := db.Query(ctx, `
		SELECT id, user_id, type, config, unlock_cron
		FROM notify_channels
		WHERE enabled = true AND unlock_cron IS NOT NULL AND unlock_cron != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("load notify cron: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chID, userID, chType, cronExpr string
		var configJSON []byte
		if err := rows.Scan(&chID, &userID, &chType, &configJSON, &cronExpr); err != nil {
			continue
		}
		svc.registerCron(chID, cronExpr, userID, chType, configJSON)
	}

	svc.cron.Start()
	return svc, nil
}

func (s *Service) registerCron(channelID, cronExpr, userID, chType string, configJSON []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.entries[channelID]; ok {
		s.cron.Remove(old)
		delete(s.entries, channelID)
	}

	entryID, err := s.cron.AddFunc(cronExpr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sendUnlockReport(ctx, userID, chType, configJSON)
	})
	if err == nil {
		s.entries[channelID] = entryID
	}
}

func (s *Service) removeCron(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[channelID]; ok {
		s.cron.Remove(old)
		delete(s.entries, channelID)
	}
}

// --- PubSub subscriber: check job completion ---

var _ = pubsub.NewSubscription(
	checkersvc.JobCompletedTopic,
	"notify-on-job-completed",
	pubsub.SubscriptionConfig[*checkersvc.JobCompletedEvent]{
		Handler: handleJobCompleted,
	},
)

// handleJobCompleted is the cross-service boundary: it pulls the summary from
// checker (typed call) and immediately converts to notify's own DTO so the
// rest of the pipeline (formatters, alerts, senders) depends only on notify types.
func handleJobCompleted(ctx context.Context, event *checkersvc.JobCompletedEvent) error {
	rows, err := db.Query(ctx, `
		SELECT type, config FROM notify_channels
		WHERE user_id = $1 AND enabled = true AND on_check_complete = true
	`, event.UserID)
	if err != nil {
		return err
	}
	defer rows.Close()

	checkerSummary, err := checkersvc.GetJobDetailedSummary(ctx, event.JobID)
	var report *JobReport
	if err != nil {
		rlog.Warn("job summary unavailable; sending minimal report", "job_id", event.JobID, "err", err)
		report = &JobReport{
			JobID:            event.JobID,
			SubscriptionName: event.SubscriptionID,
			Available:        event.Available,
			Total:            event.Total,
			TopNodes:         []TopNode{},
			Countries:        map[string]int{},
		}
	} else {
		report = fromCheckerSummary(checkerSummary)
	}

	body := formatCheckReport(report)
	subject := fmt.Sprintf("✅ Check Complete — %s (%d/%d alive)", report.SubscriptionName, report.Available, report.Total)

	for rows.Next() {
		var chType string
		var configJSON []byte
		if err := rows.Scan(&chType, &configJSON); err != nil {
			continue
		}
		sender := senderFor(chType, configJSON)
		if sender == nil {
			continue
		}
		var sendErr error
		if chType == "webhook" {
			sendErr = sender.SendPayload(ctx, event.UserID, report)
		} else {
			sendErr = sender.SendMessage(ctx, event.UserID, subject, body)
		}
		if sendErr != nil {
			rlog.Warn("notification failed", "job_id", event.JobID, "channel", chType, "err", sendErr)
		}
	}

	checkPlatformAlerts(ctx, event.UserID, event.SubscriptionID, report)
	return nil
}

// --- Unlock report (scheduled by per-channel cron) ---

func sendUnlockReport(ctx context.Context, userID, chType string, configJSON []byte) {
	result, err := checkersvc.GetLocalUnlock(ctx)
	if err != nil {
		rlog.Error("unlock report: failed to get local unlock status", "user_id", userID, "err", err)
		return
	}
	report := fromCheckerLocalUnlock(result)

	sender := senderFor(chType, configJSON)
	if sender == nil {
		rlog.Error("unlock report: unknown channel type", "user_id", userID, "type", chType)
		return
	}
	var sendErr error
	if chType == "webhook" {
		sendErr = sender.SendPayload(ctx, userID, report)
	} else {
		sendErr = sender.SendMessage(ctx, userID, "🌐 Network Unlock Report", formatUnlockReport(report))
	}
	if sendErr != nil {
		rlog.Error("unlock report: send failed", "user_id", userID, "type", chType, "err", sendErr)
	}
}

// --- API endpoints ---

// ListChannels returns all notification channels for the current user.
//
//encore:api auth method=GET path=/notify/channels
func ListChannels(ctx context.Context) (*ListChannelsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	rows, err := db.Query(ctx, `
		SELECT id, user_id, name, type, config, enabled, on_check_complete,
		       COALESCE(unlock_cron, ''), platform_alerts, created_at
		FROM notify_channels WHERE user_id = $1 ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		var alertsJSON []byte
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled,
			&c.OnCheckComplete, &c.UnlockCron, &alertsJSON, &c.CreatedAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		if err := json.Unmarshal(alertsJSON, &c.PlatformAlerts); err != nil || c.PlatformAlerts == nil {
			c.PlatformAlerts = []string{}
		}
		channels = append(channels, c)
	}
	if channels == nil {
		channels = []Channel{}
	}
	return &ListChannelsResponse{Channels: channels}, nil
}

// CreateChannel adds a new notification channel.
//
//encore:api auth method=POST path=/notify/channels
func (s *Service) CreateChannel(ctx context.Context, p *CreateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if p.Type != "webhook" && p.Type != "telegram" && p.Type != "email" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("type must be webhook, telegram, or email").Err()
	}
	if p.UnlockCron != "" {
		if _, err := cron.ParseStandard(p.UnlockCron); err != nil {
			return nil, errs.B().Code(errs.InvalidArgument).Msgf("invalid cron: %v", err).Err()
		}
	}

	id := uuid.New().String()
	configJSON := p.Config
	if configJSON == nil {
		configJSON = json.RawMessage("{}")
	}

	var unlockCron *string
	if p.UnlockCron != "" {
		unlockCron = &p.UnlockCron
	}

	alerts := p.PlatformAlerts
	if alerts == nil {
		alerts = []string{}
	}
	alertsJSON, _ := json.Marshal(alerts)

	if _, err := db.Exec(ctx, `
		INSERT INTO notify_channels (id, user_id, name, type, config, on_check_complete, unlock_cron, platform_alerts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, claims.UserID, p.Name, p.Type, []byte(configJSON),
		p.OnCheckComplete, unlockCron, alertsJSON, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create channel").Err()
	}

	if p.UnlockCron != "" {
		s.registerCron(id, p.UnlockCron, claims.UserID, p.Type, configJSON)
	}

	return &Channel{
		ID:              id,
		UserID:          claims.UserID,
		Name:            p.Name,
		Type:            p.Type,
		Config:          configJSON,
		Enabled:         true,
		OnCheckComplete: p.OnCheckComplete,
		UnlockCron:      p.UnlockCron,
		PlatformAlerts:  alerts,
	}, nil
}

// UpdateChannel modifies an existing notification channel.
//
//encore:api auth method=PUT path=/notify/channels/:id
func (s *Service) UpdateChannel(ctx context.Context, id string, p *UpdateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if p.UnlockCron != nil && *p.UnlockCron != "" {
		if _, err := cron.ParseStandard(*p.UnlockCron); err != nil {
			return nil, errs.B().Code(errs.InvalidArgument).Msgf("invalid cron: %v", err).Err()
		}
	}

	updAlerts := p.PlatformAlerts
	if updAlerts == nil {
		updAlerts = []string{}
	}
	updAlertsJSON, _ := json.Marshal(updAlerts)

	result, err := db.Exec(ctx, `
		UPDATE notify_channels
		SET
			name              = COALESCE($3, name),
			config            = COALESCE($4, config),
			enabled           = COALESCE($5, enabled),
			on_check_complete = COALESCE($6, on_check_complete),
			unlock_cron       = COALESCE($7, unlock_cron),
			platform_alerts   = $8::jsonb
		WHERE id=$1 AND user_id=$2
	`, id, claims.UserID, p.Name, nullableJSON(p.Config), p.Enabled, p.OnCheckComplete, p.UnlockCron, updAlertsJSON)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}
	if result.RowsAffected() == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}

	var c Channel
	var unlockCron *string
	var retAlertsJSON []byte
	if err := db.QueryRow(ctx, `
		SELECT id, user_id, name, type, config, enabled, on_check_complete, unlock_cron, platform_alerts, created_at
		FROM notify_channels WHERE id=$1
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled,
		&c.OnCheckComplete, &unlockCron, &retAlertsJSON, &c.CreatedAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("fetch after update failed").Err()
	}
	if unlockCron != nil {
		c.UnlockCron = *unlockCron
	}
	if err := json.Unmarshal(retAlertsJSON, &c.PlatformAlerts); err != nil || c.PlatformAlerts == nil {
		c.PlatformAlerts = []string{}
	}

	if c.Enabled && c.UnlockCron != "" {
		s.registerCron(c.ID, c.UnlockCron, claims.UserID, c.Type, c.Config)
	} else {
		s.removeCron(c.ID)
	}

	return &c, nil
}

// TestChannel sends a test notification to verify the channel configuration.
//
//encore:api auth method=POST path=/notify/channels/:id/test
func TestChannel(ctx context.Context, id string, p *TestChannelParams) (*TestChannelResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var chType string
	var configJSON []byte
	if err := db.QueryRow(ctx,
		`SELECT type, config FROM notify_channels WHERE id=$1 AND user_id=$2`,
		id, claims.UserID).Scan(&chType, &configJSON); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}

	reportType := "unlock"
	if p != nil && p.ReportType != "" {
		reportType = p.ReportType
	}

	sender := senderFor(chType, configJSON)
	if sender == nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("unknown channel type").Err()
	}

	var sendErr error
	switch reportType {
	case "unlock":
		result, err := checkersvc.GetLocalUnlock(ctx)
		if err != nil {
			return &TestChannelResponse{OK: false, Error: "failed to get local unlock status"}, nil
		}
		report := fromCheckerLocalUnlock(result)
		if chType == "webhook" {
			sendErr = sender.SendPayload(ctx, claims.UserID, report)
		} else {
			sendErr = sender.SendMessage(ctx, claims.UserID, "🌐 Network Unlock Report (test)", formatUnlockReport(report))
		}

	case "check":
		sample := sampleJobReport()
		if chType == "webhook" {
			sendErr = sender.SendPayload(ctx, claims.UserID, sample)
		} else {
			subject := fmt.Sprintf("✅ Check Complete (test) — %s", sample.SubscriptionName)
			sendErr = sender.SendMessage(ctx, claims.UserID, subject, formatCheckReport(sample))
		}

	case "platform_alert":
		msg := formatPlatformAlert("Test Subscription", []string{"netflix", "openai"})
		if chType == "webhook" {
			sendErr = sender.SendPayload(ctx, claims.UserID, map[string]any{
				"type": "platform_alert", "subscription": "Test Subscription",
				"lost_platforms": []string{"netflix", "openai"},
			})
		} else {
			sendErr = sender.SendMessage(ctx, claims.UserID, "⚠️ Platform Alert (test) — Netflix, OpenAI", msg)
		}

	default:
		return nil, errs.B().Code(errs.InvalidArgument).Msg("report_type must be 'check', 'unlock', or 'platform_alert'").Err()
	}

	if sendErr != nil {
		return &TestChannelResponse{OK: false, Error: sendErr.Error()}, nil
	}
	return &TestChannelResponse{OK: true}, nil
}

// DeleteChannel removes a notification channel.
//
//encore:api auth method=DELETE path=/notify/channels/:id
func (s *Service) DeleteChannel(ctx context.Context, id string) (*DeleteChannelResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	result, err := db.Exec(ctx, `
		DELETE FROM notify_channels WHERE id = $1 AND user_id = $2
	`, id, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("delete failed").Err()
	}
	if result.RowsAffected() == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}
	s.removeCron(id)
	return &DeleteChannelResponse{OK: true}, nil
}

// nullableJSON returns nil if the raw message is empty, otherwise the raw bytes.
func nullableJSON(r json.RawMessage) []byte {
	if len(r) == 0 {
		return nil
	}
	return []byte(r)
}

// sampleJobReport returns a fixture used by TestChannel's "check" path.
func sampleJobReport() *JobReport {
	return &JobReport{
		JobID:            "test",
		SubscriptionName: "Test Subscription",
		Available:        42,
		Total:            100,
		Platforms: PlatformCounts{
			Netflix: 10, YouTube: 15, OpenAI: 8,
			Claude: 5, Gemini: 7, Disney: 3,
		},
		AvgSpeedKbps: 5120,
		MaxSpeedKbps: 15360,
		AvgLatencyMs: 85,
		TopNodes: []TopNode{
			{Name: "HK-Node-01", SpeedKbps: 15360, LatencyMs: 32, Country: "HK"},
			{Name: "JP-Node-03", SpeedKbps: 12288, LatencyMs: 45, Country: "JP"},
			{Name: "SG-Node-02", SpeedKbps: 10240, LatencyMs: 55, Country: "SG"},
		},
		Countries: map[string]int{"HK": 15, "JP": 10, "SG": 8, "US": 5, "TW": 4},
	}
}
