// services/settings/settings.go
package settings

import (
	"context"

	"github.com/google/uuid"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"

	authsvc "subs-check-re/services/auth"
)

var db = sqldb.NewDatabase("settings", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// UserSettings holds configurable per-user settings.
type UserSettings struct {
	SpeedTestURL string `json:"speed_test_url"`
}

// GetSettings returns the current user's settings.
//
//encore:api auth method=GET path=/settings
func GetSettings(ctx context.Context) (*UserSettings, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var s UserSettings
	err := db.QueryRow(ctx,
		`SELECT COALESCE(speed_test_url, '') FROM user_settings WHERE user_id = $1`,
		claims.UserID,
	).Scan(&s.SpeedTestURL)
	if err != nil {
		// No row yet — return defaults
		return &UserSettings{SpeedTestURL: ""}, nil
	}
	return &s, nil
}

// UpdateSettings updates the current user's settings.
//
//encore:api auth method=PUT path=/settings
func UpdateSettings(ctx context.Context, p *UserSettings) (*UserSettings, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if _, err := db.Exec(ctx, `
		INSERT INTO user_settings (user_id, speed_test_url)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET speed_test_url = EXCLUDED.speed_test_url
	`, claims.UserID, p.SpeedTestURL); err != nil {
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
		`SELECT COALESCE(speed_test_url, '') FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(&s.SpeedTestURL)
	if err != nil {
		return &UserSettings{SpeedTestURL: ""}, nil
	}
	return &s, nil
}
