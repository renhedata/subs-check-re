// services/notify/notify.go
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"sort"
	"strings"
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
	settingssvc "subs-check-re/services/settings"
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

	// Load all channels with unlock_cron on startup
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

	// Remove existing entry if any
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

func handleJobCompleted(ctx context.Context, event *checkersvc.JobCompletedEvent) error {
	rows, err := db.Query(ctx, `
		SELECT type, config FROM notify_channels
		WHERE user_id = $1 AND enabled = true AND on_check_complete = true
	`, event.UserID)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Fetch detailed summary from checker
	summary, err := checkersvc.GetJobDetailedSummary(ctx, event.JobID)
	if err != nil {
		summary = &checkersvc.JobDetailedSummary{
			JobID:            event.JobID,
			SubscriptionName: event.SubscriptionID,
			Available:        event.Available,
			Total:            event.Total,
			TopNodes:         []checkersvc.TopNode{},
			Countries:        map[string]int{},
		}
	}

	for rows.Next() {
		var chType string
		var configJSON []byte
		if err := rows.Scan(&chType, &configJSON); err != nil {
			continue
		}
		switch chType {
		case "webhook":
			if err := sendWebhook(configJSON, summary); err != nil {
				rlog.Warn("webhook notification failed", "job_id", event.JobID, "err", err)
			}
		case "telegram":
			if err := sendTelegram(configJSON, formatCheckReport(summary), "HTML"); err != nil {
				rlog.Warn("telegram notification failed", "job_id", event.JobID, "err", err)
			}
		case "email":
			subject := fmt.Sprintf("✅ Check Complete — %s (%d/%d alive)", summary.SubscriptionName, summary.Available, summary.Total)
			if err := sendEmail(ctx, event.UserID, subject, htmlWrap(formatCheckReport(summary))); err != nil {
				rlog.Warn("email notification failed", "job_id", event.JobID, "err", err)
			}
		}
	}

	checkPlatformAlerts(ctx, event.UserID, event.SubscriptionID, summary)
	return nil
}

// --- Unlock report (scheduled) ---

func sendUnlockReport(ctx context.Context, userID, chType string, configJSON []byte) {
	result, err := checkersvc.GetLocalUnlock(ctx)
	if err != nil {
		return
	}

	switch chType {
	case "webhook":
		payload, _ := json.Marshal(result)
		sendWebhookRaw(configJSON, payload) //nolint:errcheck
	case "telegram":
		sendTelegram(configJSON, formatUnlockReport(result), "HTML") //nolint:errcheck
	case "email":
		sendEmail(ctx, userID, "🌐 Network Unlock Report", htmlWrap(formatUnlockReport(result))) //nolint:errcheck
	}
}

// --- Message formatting ---

func formatCheckReport(s *checkersvc.JobDetailedSummary) string {
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

func formatUnlockReport(r *checkersvc.LocalUnlockResult) string {
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

func formatSpeed(kbps int) string {
	if kbps >= 1024 {
		return fmt.Sprintf("%.1f MB/s", float64(kbps)/1024)
	}
	return fmt.Sprintf("%d KB/s", kbps)
}

type platformEntry struct {
	emoji string
	label string
	count int
}

func platformEntries(p checkersvc.PlatformUnlockSummary) []platformEntry {
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

// --- Senders ---

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

func sendWebhook(configJSON []byte, payload any) error {
	data, _ := json.Marshal(payload)
	return sendWebhookRaw(configJSON, data)
}

func sendWebhookRaw(configJSON []byte, payload []byte) error {
	var cfg webhookConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.URL == "" {
		return fmt.Errorf("invalid webhook config: url is required")
	}
	method := cfg.Method
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func sendTelegram(configJSON []byte, message string, parseMode ...string) error {
	var cfg telegramConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("invalid telegram config: bot_token and chat_id are required")
	}
	body := map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	}
	if len(parseMode) > 0 && parseMode[0] != "" {
		body["parse_mode"] = parseMode[0]
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// --- Types ---

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

// DeleteChannelResponse is the response for DELETE /notify/channels/:id.
type DeleteChannelResponse struct {
	OK bool `json:"ok"`
}

// TestChannelResponse is the response for POST /notify/channels/:id/test.
type TestChannelResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// TestChannelParams selects which report type to test.
type TestChannelParams struct {
	ReportType string `json:"report_type"` // "check", "unlock", or "platform_alert"
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

// UpdateChannelParams is the request body for PUT /notify/channels/:id.
type UpdateChannelParams struct {
	Name            *string         `json:"name"`
	Config          json.RawMessage `json:"config"`
	Enabled         *bool           `json:"enabled"`
	OnCheckComplete *bool           `json:"on_check_complete"`
	UnlockCron      *string         `json:"unlock_cron"`
	PlatformAlerts  []string        `json:"platform_alerts"`
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

	var sendErr error
	switch reportType {
	case "unlock":
		result, err := checkersvc.GetLocalUnlock(ctx)
		if err != nil {
			return &TestChannelResponse{OK: false, Error: "failed to get local unlock status"}, nil
		}
		switch chType {
		case "webhook":
			data, _ := json.Marshal(result)
			sendErr = sendWebhookRaw(configJSON, data)
		case "telegram":
			sendErr = sendTelegram(configJSON, formatUnlockReport(result), "HTML")
		case "email":
			sendErr = sendEmail(ctx, claims.UserID, "🌐 Network Unlock Report (test)", htmlWrap(formatUnlockReport(result)))
		}

	case "check":
		sample := &checkersvc.JobDetailedSummary{
			JobID:            "test",
			SubscriptionName: "Test Subscription",
			Available:        42,
			Total:            100,
			Platforms: checkersvc.PlatformUnlockSummary{
				Netflix: 10, YouTube: 15, OpenAI: 8,
				Claude: 5, Gemini: 7, Disney: 3,
			},
			AvgSpeedKbps: 5120,
			MaxSpeedKbps: 15360,
			AvgLatencyMs: 85,
			TopNodes: []checkersvc.TopNode{
				{Name: "HK-Node-01", SpeedKbps: 15360, LatencyMs: 32, Country: "HK"},
				{Name: "JP-Node-03", SpeedKbps: 12288, LatencyMs: 45, Country: "JP"},
				{Name: "SG-Node-02", SpeedKbps: 10240, LatencyMs: 55, Country: "SG"},
			},
			Countries: map[string]int{"HK": 15, "JP": 10, "SG": 8, "US": 5, "TW": 4},
		}
		switch chType {
		case "webhook":
			sendErr = sendWebhook(configJSON, sample)
		case "telegram":
			sendErr = sendTelegram(configJSON, formatCheckReport(sample), "HTML")
		case "email":
			sendErr = sendEmail(ctx, claims.UserID, "✅ Check Complete (test) — Test Subscription", htmlWrap(formatCheckReport(sample)))
		}

	case "platform_alert":
		msg := formatPlatformAlert("Test Subscription", []string{"netflix", "openai"})
		switch chType {
		case "webhook":
			sendErr = sendWebhook(configJSON, map[string]any{
				"type": "platform_alert", "subscription": "Test Subscription",
				"lost_platforms": []string{"netflix", "openai"},
			})
		case "telegram":
			sendErr = sendTelegram(configJSON, msg, "HTML")
		case "email":
			sendErr = sendEmail(ctx, claims.UserID, "⚠️ Platform Alert (test) — Netflix, OpenAI", htmlWrap(msg))
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

// --- Email sender (reads global SMTP config from settings service) ---

func sendEmail(ctx context.Context, userID, subject, htmlBody string) error {
	cfg, err := settingssvc.GetEmailConfigForUser(ctx, userID)
	if err != nil || cfg.SMTPHost == "" || cfg.To == "" {
		return fmt.Errorf("email not configured: set SMTP settings in General Settings")
	}

	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}
	from := cfg.From
	if from == "" {
		from = cfg.SMTPUser
	}

	recipients := strings.Split(cfg.To, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	header := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		from, cfg.To, subject,
	)
	msg := []byte(header + htmlBody)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)

	if port == 465 {
		tlsConfig := &tls.Config{ServerName: cfg.SMTPHost}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls connect: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer c.Close()
		if cfg.SMTPUser != "" {
			if err := c.Auth(smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return fmt.Errorf("smtp mail from: %w", err)
		}
		for _, to := range recipients {
			if err := c.Rcpt(to); err != nil {
				return fmt.Errorf("smtp rcpt: %w", err)
			}
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		_, err = w.Write(msg)
		w.Close()
		return err
	}

	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, from, recipients, msg)
}

// htmlWrap turns a Telegram HTML message into a minimal HTML email body.
func htmlWrap(telegramHTML string) string {
	body := strings.NewReplacer("\n", "<br>\n").Replace(telegramHTML)
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8">
<style>body{font-family:monospace;line-height:1.6;padding:24px;max-width:640px;color:#222;}
b{font-weight:bold;}</style></head><body>%s</body></html>`, body)
}

// --- Platform availability alerts ---

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

// checkPlatformAlerts compares the current job's platform counts against the stored
// last-known state and sends alerts for any platform that dropped from available to zero.
func checkPlatformAlerts(ctx context.Context, userID, subID string, summary *checkersvc.JobDetailedSummary) {
	current := map[string]bool{
		"netflix":         summary.Platforms.Netflix > 0,
		"youtube":         summary.Platforms.YouTube > 0,
		"youtube_premium": summary.Platforms.YouTubePremium > 0,
		"openai":          summary.Platforms.OpenAI > 0,
		"claude":          summary.Platforms.Claude > 0,
		"gemini":          summary.Platforms.Gemini > 0,
		"grok":            summary.Platforms.Grok > 0,
		"disney":          summary.Platforms.Disney > 0,
		"tiktok":          summary.Platforms.TikTok > 0,
	}

	prev := map[string]bool{}
	rows, err := db.Query(ctx,
		`SELECT platform, available FROM subscription_platform_state WHERE subscription_id=$1`, subID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var platform string
			var available bool
			if rows.Scan(&platform, &available) == nil {
				prev[platform] = available
			}
		}
	}

	for platform, available := range current {
		db.Exec(ctx, `
			INSERT INTO subscription_platform_state (subscription_id, user_id, platform, available, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (subscription_id, platform) DO UPDATE SET available=$4, updated_at=NOW()
		`, subID, userID, platform, available) //nolint:errcheck
	}

	var lostPlatforms []string
	for platform, nowAvail := range current {
		if wasAvail, hadPrev := prev[platform]; hadPrev && wasAvail && !nowAvail {
			lostPlatforms = append(lostPlatforms, platform)
		}
	}
	if len(lostPlatforms) == 0 {
		return
	}

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
		json.Unmarshal(alertsJSON, &watched) //nolint:errcheck

		var matched []string
		for _, lost := range lostPlatforms {
			for _, w := range watched {
				if lost == w {
					matched = append(matched, lost)
					break
				}
			}
		}
		if len(matched) == 0 {
			continue
		}

		names := make([]string, len(matched))
		for i, p := range matched {
			names[i] = platformDisplayName(p)
		}
		subject := fmt.Sprintf("⚠️ Platform Alert — %s", strings.Join(names, ", "))
		msg := formatPlatformAlert(summary.SubscriptionName, matched)

		switch chType {
		case "webhook":
			sendWebhook(configJSON, map[string]any{ //nolint:errcheck
				"type":           "platform_alert",
				"subscription":   summary.SubscriptionName,
				"lost_platforms": matched,
				"timestamp":      time.Now().UTC(),
			})
		case "telegram":
			sendTelegram(configJSON, msg, "HTML") //nolint:errcheck
		case "email":
			sendEmail(ctx, userID, subject, htmlWrap(msg)) //nolint:errcheck
		}
	}
}
