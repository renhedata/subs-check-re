// services/auth/auth.go
package auth

import (
	"context"
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

type RegisterParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	UserID string `json:"user_id"`
}

//encore:api public method=POST path=/auth/register
func Register(ctx context.Context, p *RegisterParams) (*RegisterResponse, error) {
	if p.Username == "" || p.Password == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("username and password required").Err()
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
	token, err := generateJWT(id)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to generate token").Err()
	}
	return &LoginResponse{Token: token, UserID: id}, nil
}

type MeResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

//encore:api auth method=GET path=/auth/me
func Me(ctx context.Context) (*MeResponse, error) {
	claims, ok := encauth.Data().(*UserClaims)
	if !ok || claims == nil {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth data").Err()
	}
	var username string
	err := db.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, claims.UserID).Scan(&username)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("user not found").Err()
	}
	return &MeResponse{UserID: claims.UserID, Username: username}, nil
}
