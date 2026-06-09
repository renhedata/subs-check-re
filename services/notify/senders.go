package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	settingssvc "subs-check-re/services/settings"
)

// Sender is the abstraction for one notification delivery channel.
// Each adapter is responsible for parsing its own config blob, sending a
// formatted message (HTML/text), and the channel-specific payload shape
// when payload-based delivery is needed (e.g. webhook JSON body).
type Sender interface {
	// SendMessage delivers a pre-formatted message. Channels that don't have
	// a notion of "message body" (e.g. webhooks) may ignore the message and
	// rely on SendPayload instead.
	SendMessage(ctx context.Context, userID, subject, body string) error
	// SendPayload delivers an arbitrary JSON-serializable payload. Used by
	// channels (webhook) where the consumer expects a structured envelope.
	SendPayload(ctx context.Context, userID string, payload any) error
}

// senderFor returns a sender for the given channel type wired with the
// channel's config blob. Returns nil for unknown types.
func senderFor(chType string, configJSON []byte) Sender {
	switch chType {
	case "webhook":
		return &webhookSender{config: configJSON}
	case "telegram":
		return &telegramSender{config: configJSON}
	case "email":
		return &emailSender{config: configJSON}
	}
	return nil
}

// --- Webhook ---

type webhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type webhookSender struct {
	config []byte
}

func (w *webhookSender) SendMessage(ctx context.Context, _ string, _, body string) error {
	return w.send(ctx, []byte(body))
}

func (w *webhookSender) SendPayload(ctx context.Context, _ string, payload any) error {
	data, _ := json.Marshal(payload)
	return w.send(ctx, data)
}

func (w *webhookSender) send(ctx context.Context, payload []byte) error {
	var cfg webhookConfig
	if err := json.Unmarshal(w.config, &cfg); err != nil || cfg.URL == "" {
		return fmt.Errorf("invalid webhook config: url is required")
	}
	method := cfg.Method
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader(payload))
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

// --- Telegram ---

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type telegramSender struct {
	config []byte
}

func (t *telegramSender) SendMessage(ctx context.Context, _ string, _, body string) error {
	return t.send(ctx, body, "HTML")
}

func (t *telegramSender) SendPayload(ctx context.Context, _ string, payload any) error {
	data, _ := json.Marshal(payload)
	return t.send(ctx, string(data), "")
}

func (t *telegramSender) send(ctx context.Context, message, parseMode string) error {
	var cfg telegramConfig
	if err := json.Unmarshal(t.config, &cfg); err != nil || cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("invalid telegram config: bot_token and chat_id are required")
	}
	body := map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	}
	if parseMode != "" {
		body["parse_mode"] = parseMode
	}
	payload, _ := json.Marshal(body)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
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

// --- Email (SMTP, reads global config from settings service) ---

type emailConfig struct {
	To string `json:"to_email"`
}

type emailSender struct {
	config []byte
}

func (e *emailSender) SendMessage(ctx context.Context, userID, subject, body string) error {
	var cfg emailConfig
	if err := json.Unmarshal(e.config, &cfg); err != nil || cfg.To == "" {
		return fmt.Errorf("email channel not configured: set a recipient in the channel settings")
	}
	return sendSMTP(ctx, userID, cfg.To, subject, htmlWrap(body))
}

func (e *emailSender) SendPayload(ctx context.Context, userID string, payload any) error {
	var cfg emailConfig
	if err := json.Unmarshal(e.config, &cfg); err != nil || cfg.To == "" {
		return fmt.Errorf("email channel not configured: set a recipient in the channel settings")
	}
	data, _ := json.Marshal(payload)
	return sendSMTP(ctx, userID, cfg.To, "Notification", string(data))
}

func sendSMTP(ctx context.Context, userID, to, subject, htmlBody string) error {
	cfg, err := settingssvc.GetEmailConfigForUser(ctx, userID)
	if err != nil || cfg.SMTPHost == "" {
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

	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	header := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		from, to, subject,
	)
	msg := []byte(header + htmlBody)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)

	if port == 465 {
		return sendSMTPS(addr, cfg.SMTPHost, cfg.SMTPUser, cfg.SMTPPass, from, recipients, msg)
	}

	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, from, recipients, msg)
}

func sendSMTPS(addr, host, user, pass, from string, recipients []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("tls connect: %w", err)
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if user != "" {
		if err := c.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
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
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

// htmlWrap turns a Telegram HTML message into a minimal HTML email body.
func htmlWrap(telegramHTML string) string {
	body := strings.NewReplacer("\n", "<br>\n").Replace(telegramHTML)
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8">
<style>body{font-family:monospace;line-height:1.6;padding:24px;max-width:640px;color:#222;}
b{font-weight:bold;}</style></head><body>%s</body></html>`, body)
}
