// services/settings/settings.go
package settings

import (
	"context"

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
