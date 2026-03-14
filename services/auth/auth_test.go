// services/auth/auth_test.go
package auth

import (
	"context"
	"testing"
)

func TestRegister(t *testing.T) {
	ctx := context.Background()
	resp, err := Register(ctx, &RegisterParams{
		Username: "testuser",
		Password: "testpass123",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if resp.UserID == "" {
		t.Error("expected non-empty user ID")
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	params := &RegisterParams{Username: "dupuser", Password: "pass"}
	_, _ = Register(ctx, params)
	_, err := Register(ctx, params)
	if err == nil {
		t.Error("expected error for duplicate username")
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	_, _ = Register(ctx, &RegisterParams{Username: "loginuser", Password: "mypassword"})
	resp, err := Login(ctx, &LoginParams{Username: "loginuser", Password: "mypassword"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ctx := context.Background()
	_, _ = Register(ctx, &RegisterParams{Username: "authuser", Password: "correct"})
	_, err := Login(ctx, &LoginParams{Username: "authuser", Password: "wrong"})
	if err == nil {
		t.Error("expected error for wrong password")
	}
}
