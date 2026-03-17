// services/notify/notify.go
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
	checkersvc "subs-check-re/services/checker"
)

var db = sqldb.NewDatabase("notify", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- PubSub subscriber ---

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
		WHERE user_id = $1 AND enabled = true
	`, event.UserID)
	if err != nil {
		return err
	}
	defer rows.Close()

	msg := fmt.Sprintf(
		"✅ Check completed\nSubscription: %s\nAvailable: %d/%d nodes",
		event.SubscriptionID, event.Available, event.Total,
	)

	for rows.Next() {
		var chType string
		var configJSON []byte
		if err := rows.Scan(&chType, &configJSON); err != nil {
			continue
		}
		switch chType {
		case "webhook":
			sendWebhook(configJSON, event) //nolint:errcheck
		case "telegram":
			sendTelegram(configJSON, msg) //nolint:errcheck
		}
	}
	return nil
}

// --- Senders ---

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

func sendWebhook(configJSON []byte, event *checkersvc.JobCompletedEvent) error {
	var cfg webhookConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.URL == "" {
		return fmt.Errorf("invalid webhook config: url is required")
	}
	method := cfg.Method
	if method == "" {
		method = "POST"
	}
	payload, _ := json.Marshal(event)
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

func sendTelegram(configJSON []byte, message string) error {
	var cfg telegramConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("invalid telegram config: bot_token and chat_id are required")
	}
	payload, _ := json.Marshal(map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	})
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
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
}

// ListChannelsResponse is the response for GET /notify/channels.
type ListChannelsResponse struct {
	Channels []Channel `json:"channels"`
}

// CreateChannelParams is the request body for POST /notify/channels.
type CreateChannelParams struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
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

// --- API endpoints ---

// ListChannels returns all notification channels for the current user.
//
//encore:api auth method=GET path=/notify/channels
func ListChannels(ctx context.Context) (*ListChannelsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	rows, err := db.Query(ctx, `
		SELECT id, user_id, name, type, config, enabled, created_at
		FROM notify_channels WHERE user_id = $1 ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
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
func CreateChannel(ctx context.Context, p *CreateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if p.Type != "webhook" && p.Type != "telegram" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("type must be webhook or telegram").Err()
	}

	id := uuid.New().String()
	configJSON := p.Config
	if configJSON == nil {
		configJSON = json.RawMessage("{}")
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO notify_channels (id, user_id, name, type, config, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, claims.UserID, p.Name, p.Type, []byte(configJSON), time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create channel").Err()
	}

	return &Channel{
		ID:      id,
		UserID:  claims.UserID,
		Name:    p.Name,
		Type:    p.Type,
		Config:  configJSON,
		Enabled: true,
	}, nil
}

// UpdateChannelParams is the request body for PUT /notify/channels/:id.
type UpdateChannelParams struct {
	Name    *string         `json:"name"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled"`
}

// UpdateChannel modifies an existing notification channel.
//
//encore:api auth method=PUT path=/notify/channels/:id
func UpdateChannel(ctx context.Context, id string, p *UpdateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	result, err := db.Exec(ctx, `
		UPDATE notify_channels
		SET
			name    = COALESCE($3, name),
			config  = COALESCE($4, config),
			enabled = COALESCE($5, enabled)
		WHERE id=$1 AND user_id=$2
	`, id, claims.UserID, p.Name, nullableJSON(p.Config), p.Enabled)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}
	if result.RowsAffected() == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}

	var c Channel
	if err := db.QueryRow(ctx, `
		SELECT id, user_id, name, type, config, enabled, created_at
		FROM notify_channels WHERE id=$1
	`, id).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("fetch after update failed").Err()
	}
	return &c, nil
}

// TestChannel sends a test notification to verify the channel configuration.
//
//encore:api auth method=POST path=/notify/channels/:id/test
func TestChannel(ctx context.Context, id string) (*TestChannelResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var chType string
	var configJSON []byte
	if err := db.QueryRow(ctx,
		`SELECT type, config FROM notify_channels WHERE id=$1 AND user_id=$2`,
		id, claims.UserID).Scan(&chType, &configJSON); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}

	testEvent := &checkersvc.JobCompletedEvent{
		JobID:          "test",
		SubscriptionID: "test-subscription",
		UserID:         claims.UserID,
		Available:      42,
		Total:          100,
	}
	testMsg := "🔔 Test notification — your channel is working!\nAvailable: 42/100 nodes"

	var sendErr error
	switch chType {
	case "webhook":
		sendErr = sendWebhook(configJSON, testEvent)
	case "telegram":
		sendErr = sendTelegram(configJSON, testMsg)
	default:
		return nil, errs.B().Code(errs.InvalidArgument).Msg("unknown channel type").Err()
	}

	if sendErr != nil {
		return &TestChannelResponse{OK: false, Error: sendErr.Error()}, nil
	}
	return &TestChannelResponse{OK: true}, nil
}

// DeleteChannel removes a notification channel.
//
//encore:api auth method=DELETE path=/notify/channels/:id
func DeleteChannel(ctx context.Context, id string) (*DeleteChannelResponse, error) {
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
	return &DeleteChannelResponse{OK: true}, nil
}

// nullableJSON returns nil if the raw message is empty, otherwise the raw bytes.
// Used so that an absent JSON field does not overwrite an existing DB value.
func nullableJSON(r json.RawMessage) []byte {
	if len(r) == 0 {
		return nil
	}
	return []byte(r)
}
