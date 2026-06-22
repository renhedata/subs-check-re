// services/auth/auth_test.go
package auth

import (
	"context"
	"testing"
)

func TestRegister(t *testing.T) {
	ctx := context.Background()
	resp, err := Register(ctx, &RegisterParams{
		Username:   "testuser",
		Password:   "testpass123",
		InviteCode: "ashark",
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
	params := &RegisterParams{Username: "dupuser", Password: "pass", InviteCode: "ashark"}
	_, _ = Register(ctx, params)
	_, err := Register(ctx, params)
	if err == nil {
		t.Error("expected error for duplicate username")
	}
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	_, _ = Register(ctx, &RegisterParams{Username: "loginuser", Password: "mypassword", InviteCode: "ashark"})
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
	_, _ = Register(ctx, &RegisterParams{Username: "authuser", Password: "correct", InviteCode: "ashark"})
	_, err := Login(ctx, &LoginParams{Username: "authuser", Password: "wrong"})
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestRegisterWrongInviteCode(t *testing.T) {
	ctx := context.Background()
	_, err := Register(ctx, &RegisterParams{
		Username:   "badinvite",
		Password:   "testpass123",
		InviteCode: "not-the-code",
	})
	if err == nil {
		t.Error("expected error for wrong invite code")
	}
}

func TestRegisterMissingInviteCode(t *testing.T) {
	ctx := context.Background()
	_, err := Register(ctx, &RegisterParams{
		Username: "noinvite",
		Password: "testpass123",
	})
	if err == nil {
		t.Error("expected error for missing invite code")
	}
}

func TestMeByIDDefaults(t *testing.T) {
	ctx := context.Background()
	reg, err := Register(ctx, &RegisterParams{
		Username: "meuser", Password: "pass1234", InviteCode: "ashark",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	me, err := meByID(ctx, reg.UserID)
	if err != nil {
		t.Fatalf("meByID failed: %v", err)
	}
	if me.Username != "meuser" || me.Email != "" || me.DisplayName != "" {
		t.Errorf("unexpected me: %+v", me)
	}
}

func TestUpdateProfile(t *testing.T) {
	ctx := context.Background()
	reg, _ := Register(ctx, &RegisterParams{
		Username: "profuser", Password: "pass1234", InviteCode: "ashark",
	})
	me, err := updateProfile(ctx, reg.UserID, &UpdateProfileParams{
		Username: "profuser2", Email: "p@example.com", DisplayName: "Prof",
	})
	if err != nil {
		t.Fatalf("updateProfile failed: %v", err)
	}
	if me.Username != "profuser2" || me.Email != "p@example.com" || me.DisplayName != "Prof" {
		t.Errorf("unexpected profile: %+v", me)
	}
}

func TestUpdateProfileDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	_, _ = Register(ctx, &RegisterParams{Username: "taken", Password: "pass1234", InviteCode: "ashark"})
	reg, _ := Register(ctx, &RegisterParams{Username: "mover", Password: "pass1234", InviteCode: "ashark"})
	_, err := updateProfile(ctx, reg.UserID, &UpdateProfileParams{Username: "taken"})
	if err == nil {
		t.Error("expected error renaming to a taken username")
	}
}

func TestUpdateProfileInvalidEmail(t *testing.T) {
	ctx := context.Background()
	reg, _ := Register(ctx, &RegisterParams{Username: "emailuser", Password: "pass1234", InviteCode: "ashark"})
	_, err := updateProfile(ctx, reg.UserID, &UpdateProfileParams{Username: "emailuser", Email: "not-an-email"})
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	reg, _ := Register(ctx, &RegisterParams{Username: "pwuser", Password: "oldpass12", InviteCode: "ashark"})
	if err := changePassword(ctx, reg.UserID, &ChangePasswordParams{
		CurrentPassword: "oldpass12", NewPassword: "newpass12",
	}); err != nil {
		t.Fatalf("changePassword failed: %v", err)
	}
	if _, err := Login(ctx, &LoginParams{Username: "pwuser", Password: "newpass12"}); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	ctx := context.Background()
	reg, _ := Register(ctx, &RegisterParams{Username: "pwuser2", Password: "oldpass12", InviteCode: "ashark"})
	err := changePassword(ctx, reg.UserID, &ChangePasswordParams{
		CurrentPassword: "wrongpass", NewPassword: "newpass12",
	})
	if err == nil {
		t.Error("expected error for wrong current password")
	}
}
