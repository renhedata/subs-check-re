// services/auth/authhandler.go
package auth

import (
	"context"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
)

// UserClaims holds per-request auth data, available in all authenticated endpoints
// via encauth.Data[*UserClaims]().
type UserClaims struct {
	UserID string
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (encauth.UID, *UserClaims, error) {
	claims, err := validateJWT(token)
	if err != nil {
		return "", nil, errs.B().Code(errs.Unauthenticated).Msg("invalid or expired token").Err()
	}
	return encauth.UID(claims.UserID), &UserClaims{UserID: claims.UserID}, nil
}
