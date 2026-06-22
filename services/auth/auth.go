// services/auth/auth.go
package auth

import (
	"context"
	"net/mail"
	"os"
	"strings"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var db = sqldb.NewDatabase("auth", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// inviteCode returns the registration invite code. Overridable per deployment
// via REGISTER_INVITE_CODE; defaults to "ashark" so the app runs with no config.
func inviteCode() string {
	if v := os.Getenv("REGISTER_INVITE_CODE"); v != "" {
		return v
	}
	return "ashark"
}

type RegisterParams struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

type RegisterResponse struct {
	UserID string `json:"user_id"`
}

//encore:api public method=POST path=/auth/register
func Register(ctx context.Context, p *RegisterParams) (*RegisterResponse, error) {
	if p.Username == "" || p.Password == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("username and password required").Err()
	}
	if p.InviteCode != inviteCode() {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid invite code").Err()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to hash password").Err()
	}
	id := uuid.New().String()
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES ($1, $2, $3, $4)
	`, id, p.Username, string(hash), time.Now())
	if err != nil {
		rlog.Error("register db error", "err", err)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, errs.B().Code(errs.AlreadyExists).Msg("username already taken").Err()
		}
		return nil, errs.B().Code(errs.Internal).Msg("failed to create user").Err()
	}
	return &RegisterResponse{UserID: id}, nil
}

type LoginParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

//encore:api public method=POST path=/auth/login
func Login(ctx context.Context, p *LoginParams) (*LoginResponse, error) {
	var id, hash string
	err := db.QueryRow(ctx, `
		SELECT id, password_hash FROM users WHERE username = $1
	`, p.Username).Scan(&id, &hash)
	if err != nil {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("invalid username or password").Err()
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p.Password)); err != nil {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("invalid username or password").Err()
	}
	expiry := 24 * time.Hour
	if p.Remember {
		expiry = 30 * 24 * time.Hour
	}
	token, err := generateJWT(id, expiry)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to generate token").Err()
	}
	return &LoginResponse{Token: token, UserID: id}, nil
}

type MeResponse struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func meByID(ctx context.Context, userID string) (*MeResponse, error) {
	var username, email, displayName string
	err := db.QueryRow(ctx, `
		SELECT username, COALESCE(email, ''), COALESCE(display_name, '')
		FROM users WHERE id = $1
	`, userID).Scan(&username, &email, &displayName)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("user not found").Err()
	}
	return &MeResponse{UserID: userID, Username: username, Email: email, DisplayName: displayName}, nil
}

//encore:api auth method=GET path=/auth/me
func Me(ctx context.Context) (*MeResponse, error) {
	claims, ok := encauth.Data().(*UserClaims)
	if !ok || claims == nil {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth data").Err()
	}
	return meByID(ctx, claims.UserID)
}

type UpdateProfileParams struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func updateProfile(ctx context.Context, userID string, p *UpdateProfileParams) (*MeResponse, error) {
	if p.Username == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("username required").Err()
	}
	if p.Email != "" {
		addr, err := mail.ParseAddress(p.Email)
		if err != nil || addr.Address != p.Email {
			return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid email address").Err()
		}
	}
	var email, displayName any // nil -> SQL NULL
	if p.Email != "" {
		email = p.Email
	}
	if p.DisplayName != "" {
		displayName = p.DisplayName
	}
	_, err := db.Exec(ctx, `
		UPDATE users SET username = $1, email = $2, display_name = $3 WHERE id = $4
	`, p.Username, email, displayName, userID)
	if err != nil {
		rlog.Error("update profile db error", "err", err)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, errs.B().Code(errs.AlreadyExists).Msg("username already taken").Err()
		}
		return nil, errs.B().Code(errs.Internal).Msg("failed to update profile").Err()
	}
	return meByID(ctx, userID)
}

//encore:api auth method=PATCH path=/auth/profile
func UpdateProfile(ctx context.Context, p *UpdateProfileParams) (*MeResponse, error) {
	claims, ok := encauth.Data().(*UserClaims)
	if !ok || claims == nil {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth data").Err()
	}
	return updateProfile(ctx, claims.UserID, p)
}

type ChangePasswordParams struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func changePassword(ctx context.Context, userID string, p *ChangePasswordParams) error {
	if len(p.NewPassword) < 8 {
		return errs.B().Code(errs.InvalidArgument).Msg("new password must be at least 8 characters").Err()
	}
	var hash string
	if err := db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return errs.B().Code(errs.NotFound).Msg("user not found").Err()
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p.CurrentPassword)); err != nil {
		return errs.B().Code(errs.InvalidArgument).Msg("current password is incorrect").Err()
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(p.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errs.B().Code(errs.Internal).Msg("failed to hash password").Err()
	}
	if _, err := db.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, string(newHash), userID); err != nil {
		return errs.B().Code(errs.Internal).Msg("failed to update password").Err()
	}
	return nil
}

//encore:api auth method=POST path=/auth/change-password
func ChangePassword(ctx context.Context, p *ChangePasswordParams) error {
	claims, ok := encauth.Data().(*UserClaims)
	if !ok || claims == nil {
		return errs.B().Code(errs.Unauthenticated).Msg("missing auth data").Err()
	}
	return changePassword(ctx, claims.UserID, p)
}
