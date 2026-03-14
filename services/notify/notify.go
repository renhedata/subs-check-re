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
			sendWebhook(configJSON, event)
		case "telegram":
			sendTelegram(configJSON, msg)
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

func sendWebhook(configJSON []byte, event *checkersvc.JobCompletedEvent) {
	var cfg webhookConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.URL == "" {
		return
	}
	method := cfg.Method
	if method == "" {
		method = "POST"
	}
	payload, _ := json.Marshal(event)
	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func sendTelegram(configJSON []byte, message string) {
	var cfg telegramConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg.BotToken == "" || cfg.ChatID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	})
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
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
