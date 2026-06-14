// services/settings/settings.go
package settings

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"

	authsvc "subs-check-re/services/auth"
)

var db = sqldb.NewDatabase("settings", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// EmailConfig holds global SMTP settings for email notifications.
// The recipient address is configured per-channel in the notify service.
type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"`
	From     string `json:"from"`
}

// PlatformTag is one platform's export-tag rule. Key is a built-in platform
// (netflix, openai, …) or a custom rule key (e.g. spotify).
type PlatformTag struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

// ExportTagConfig controls the tags appended to node names in exports.
type ExportTagConfig struct {
	ShowCountry bool          `json:"show_country"`
	ShowSpeed   bool          `json:"show_speed"`
	Platforms   []PlatformTag `json:"platforms"`
}

// DefaultExportTags is the exported default config, for callers (e.g. the
// checker export path) that need a safe fallback on a settings lookup error.
func DefaultExportTags() ExportTagConfig {
	return defaultExportTags()
}

// defaultExportTags reproduces the legacy taggedName behavior: built-in short
// tags, speed on, country off.
func defaultExportTags() ExportTagConfig {
	return ExportTagConfig{
		ShowCountry: false,
		ShowSpeed:   true,
		Platforms: []PlatformTag{
			{Key: "netflix", Label: "NF", Enabled: true},
			{Key: "openai", Label: "GPT", Enabled: true},
			{Key: "gemini", Label: "GM", Enabled: true},
			{Key: "claude", Label: "CL", Enabled: true},
			{Key: "grok", Label: "GK", Enabled: true},
			{Key: "youtube", Label: "YT", Enabled: true},
			{Key: "disney", Label: "D+", Enabled: true},
			{Key: "tiktok", Label: "TK", Enabled: true},
			{Key: "spotify", Label: "SP", Enabled: true},
			{Key: "prime_video", Label: "PV", Enabled: true},
			{Key: "bahamut", Label: "BH", Enabled: true},
			{Key: "bilibili_cn", Label: "B站", Enabled: true},
			{Key: "bilibili_hkmctw", Label: "B站港", Enabled: true},
		},
	}
}

// mergeExportTags overlays a stored config onto the defaults: built-in entries
// take the stored label/enabled when present (defaults otherwise, in default
// order), then any custom (non-built-in) stored entries are appended in their
// stored order. Scalar flags come straight from the stored config.
func mergeExportTags(stored ExportTagConfig) ExportTagConfig {
	storedByKey := map[string]PlatformTag{}
	for _, p := range stored.Platforms {
		storedByKey[p.Key] = p
	}
	out := ExportTagConfig{ShowCountry: stored.ShowCountry, ShowSpeed: stored.ShowSpeed}
	builtinKeys := map[string]bool{}
	for _, def := range defaultExportTags().Platforms {
		builtinKeys[def.Key] = true
		if s, ok := storedByKey[def.Key]; ok {
			out.Platforms = append(out.Platforms, s)
		} else {
			out.Platforms = append(out.Platforms, def)
		}
	}
	for _, p := range stored.Platforms {
		if !builtinKeys[p.Key] {
			out.Platforms = append(out.Platforms, p)
		}
	}
	return out
}

// UserSettings holds configurable per-user settings.
type UserSettings struct {
	SpeedTestURL   string          `json:"speed_test_url"`
	UploadTestURL  string          `json:"upload_test_url"`
	LatencyTestURL string          `json:"latency_test_url"`
	EmailConfig    EmailConfig     `json:"email_config"`
	ExportTags     ExportTagConfig `json:"export_tags"`
}

// GetSettings returns the current user's settings.
//
//encore:api auth method=GET path=/settings
func GetSettings(ctx context.Context) (*UserSettings, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var s UserSettings
	var emailConfigJSON []byte
	var exportTagsJSON []byte
	err := db.QueryRow(ctx,
		`SELECT COALESCE(speed_test_url, ''), COALESCE(upload_test_url, ''), COALESCE(latency_test_url, ''), COALESCE(email_config, 'null'::jsonb), COALESCE(export_tags, 'null'::jsonb) FROM user_settings WHERE user_id = $1`,
		claims.UserID,
	).Scan(&s.SpeedTestURL, &s.UploadTestURL, &s.LatencyTestURL, &emailConfigJSON, &exportTagsJSON)
	if err != nil {
		if errors.Is(err, sqldb.ErrNoRows) {
			return &UserSettings{ExportTags: defaultExportTags()}, nil
		}
		return nil, errs.B().Code(errs.Internal).Msg("failed to load settings").Err()
	}
	if len(emailConfigJSON) > 0 {
		json.Unmarshal(emailConfigJSON, &s.EmailConfig) //nolint:errcheck
	}
	if len(exportTagsJSON) > 0 && string(exportTagsJSON) != "null" {
		var stored ExportTagConfig
		if json.Unmarshal(exportTagsJSON, &stored) == nil {
			s.ExportTags = mergeExportTags(stored)
		} else {
			s.ExportTags = defaultExportTags()
		}
	} else {
		s.ExportTags = defaultExportTags()
	}
	return &s, nil
}

// UpdateSettings updates the current user's settings.
//
//encore:api auth method=PUT path=/settings
func UpdateSettings(ctx context.Context, p *UserSettings) (*UserSettings, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	emailConfigJSON, _ := json.Marshal(p.EmailConfig)
	exportTagsJSON, _ := json.Marshal(p.ExportTags)

	if _, err := db.Exec(ctx, `
		INSERT INTO user_settings (user_id, speed_test_url, upload_test_url, latency_test_url, email_config, export_tags)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		  SET speed_test_url   = EXCLUDED.speed_test_url,
		      upload_test_url  = EXCLUDED.upload_test_url,
		      latency_test_url = EXCLUDED.latency_test_url,
		      email_config     = EXCLUDED.email_config,
		      export_tags      = EXCLUDED.export_tags
	`, claims.UserID, p.SpeedTestURL, p.UploadTestURL, p.LatencyTestURL, emailConfigJSON, exportTagsJSON); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to save settings").Err()
	}
	return p, nil
}

// APIKeyResponse is returned by GET /settings/api-key and POST /settings/api-key/regenerate.
type APIKeyResponse struct {
	APIKey string `json:"api_key"`
}

// GetAPIKey returns the user's API key, generating one if it doesn't exist yet.
//
//encore:api auth method=GET path=/settings/api-key
func GetAPIKey(ctx context.Context) (*APIKeyResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var key string
	err := db.QueryRow(ctx,
		`SELECT api_key FROM user_settings WHERE user_id=$1`,
		claims.UserID).Scan(&key)

	if err != nil || key == "" {
		key = uuid.New().String()
		if _, err := db.Exec(ctx, `
			INSERT INTO user_settings (user_id, api_key)
			VALUES ($1, $2)
			ON CONFLICT (user_id) DO UPDATE SET api_key = EXCLUDED.api_key
		`, claims.UserID, key); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to store API key").Err()
		}
	}
	return &APIKeyResponse{APIKey: key}, nil
}

// RegenerateAPIKey creates a new API key, invalidating the old one.
//
//encore:api auth method=POST path=/settings/api-key/regenerate
func RegenerateAPIKey(ctx context.Context) (*APIKeyResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	key := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO user_settings (user_id, api_key)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET api_key = EXCLUDED.api_key
	`, claims.UserID, key); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to regenerate API key").Err()
	}
	return &APIKeyResponse{APIKey: key}, nil
}

// UserIDResponse is the response for internal API key lookups.
type UserIDResponse struct {
	UserID string `json:"user_id"`
}

// GetUserIDByAPIKey resolves an API key to a user ID. Used by the checker export endpoint.
//
//encore:api private method=GET path=/internal/resolve-api-key/:apiKey
func GetUserIDByAPIKey(ctx context.Context, apiKey string) (*UserIDResponse, error) {
	var userID string
	err := db.QueryRow(ctx,
		`SELECT user_id FROM user_settings WHERE api_key=$1`, apiKey).Scan(&userID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("invalid API key").Err()
	}
	return &UserIDResponse{UserID: userID}, nil
}

// GetSpeedTestURLForUser is an internal helper used by the checker service.
//
//encore:api private method=GET path=/internal/settings/:userID/speed-test-url
func GetSpeedTestURLForUser(ctx context.Context, userID string) (*UserSettings, error) {
	var s UserSettings
	err := db.QueryRow(ctx,
		`SELECT COALESCE(speed_test_url, ''), COALESCE(upload_test_url, ''), COALESCE(latency_test_url, '') FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&s.SpeedTestURL, &s.UploadTestURL, &s.LatencyTestURL)
	if err != nil {
		return &UserSettings{}, nil
	}
	return &s, nil
}

// GetEmailConfigForUser is an internal helper used by the notify service.
//
//encore:api private method=GET path=/internal/settings/:userID/email-config
func GetEmailConfigForUser(ctx context.Context, userID string) (*EmailConfig, error) {
	var emailConfigJSON []byte
	err := db.QueryRow(ctx,
		`SELECT COALESCE(email_config, 'null'::jsonb) FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&emailConfigJSON)
	if err != nil {
		return &EmailConfig{}, nil
	}
	var cfg EmailConfig
	if len(emailConfigJSON) > 0 {
		json.Unmarshal(emailConfigJSON, &cfg) //nolint:errcheck
	}
	return &cfg, nil
}

// GetExportTagsForUser is an internal helper used by the checker export path
// (token-auth, no user claims). Returns the user's merged tag config, or
// defaults when unset.
//
//encore:api private method=GET path=/internal/settings/:userID/export-tags
func GetExportTagsForUser(ctx context.Context, userID string) (*ExportTagConfig, error) {
	var exportTagsJSON []byte
	err := db.QueryRow(ctx,
		`SELECT COALESCE(export_tags, 'null'::jsonb) FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&exportTagsJSON)
	if err != nil || len(exportTagsJSON) == 0 || string(exportTagsJSON) == "null" {
		d := defaultExportTags()
		return &d, nil
	}
	var stored ExportTagConfig
	if json.Unmarshal(exportTagsJSON, &stored) != nil {
		d := defaultExportTags()
		return &d, nil
	}
	merged := mergeExportTags(stored)
	return &merged, nil
}
