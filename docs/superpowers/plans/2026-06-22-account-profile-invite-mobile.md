# Account Profile, Invite Code, and Mobile Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let signed-in users edit username/password/email/display-name, gate registration behind an env-sourced invite code (default `ashark`), and make every route render correctly on mobile.

**Architecture:** Extend the existing Encore `auth` service with a 2-column migration plus three endpoints (`UpdateProfile`, `ChangePassword`, extended `Me`); each auth endpoint extracts `userID` from JWT claims then delegates to a plain helper so the logic is unit-testable. The frontend adds an invite field on the login page, two mutation hooks, and a new `/settings/account` page that mirrors the existing settings-subpage pattern. A final browser pass audits and fixes each route at 375/768px.

**Tech Stack:** Go + Encore, PostgreSQL (Encore-managed), bcrypt, `net/mail` (stdlib email check); React 19 + TanStack Router/Query, react-hook-form + zod, Tailwind v4, Biome.

**Spec:** `docs/superpowers/specs/2026-06-22-account-profile-invite-mobile-design.md`

**Conventions:**
- Run Go tests **without** `-race` (the harness hangs with it): `encore test ./services/auth/...`
- Commits: conventional-commit prefixes; no attribution footer (disabled globally).
- Client gen app name is `subs-check-uqti`; client output path is `frontend/src/lib/client.gen.ts`.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `services/auth/migrations/2_add_profile_fields.up.sql` / `.down.sql` | Add nullable `email`, `display_name` to `users` |
| `services/auth/auth.go` | Invite-code helper + validation in `Register`; extend `Me`; new `UpdateProfile` / `ChangePassword` endpoints + their plain helpers |
| `services/auth/auth_test.go` | Update existing `Register` calls for invite code; add tests for invite gating, profile update, password change |
| `frontend/src/lib/client.gen.ts` | Regenerated typed client (do not hand-edit) |
| `frontend/src/routes/login.tsx` | Invite-code input in register mode |
| `frontend/src/queries/auth.ts` | `useUpdateProfile`, `useChangePassword` hooks |
| `frontend/src/routes/settings/account.tsx` | **New** account page (profile + password sections) |
| `frontend/src/routes/settings.tsx` | Add "Account" tab |
| `frontend/src/components/rail.tsx` | Account-settings link; prefer display name |
| (various routes) | Mobile fixes from the audit |

---

## Phase 1 — Backend (auth service)

### Task 1: Migration — add profile columns

**Files:**
- Create: `services/auth/migrations/2_add_profile_fields.up.sql`
- Create: `services/auth/migrations/2_add_profile_fields.down.sql`

- [ ] **Step 1: Write the up migration**

`services/auth/migrations/2_add_profile_fields.up.sql`:
```sql
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN display_name TEXT;
```

- [ ] **Step 2: Write the down migration**

`services/auth/migrations/2_add_profile_fields.down.sql`:
```sql
ALTER TABLE users DROP COLUMN display_name;
ALTER TABLE users DROP COLUMN email;
```

- [ ] **Step 3: Apply the migration**

Run: `encore db migrate` (or start `encore run`, which auto-migrates).
Expected: no error; migration `2_add_profile_fields` applied.

- [ ] **Step 4: Verify the schema**

Run: `encore db shell auth` then `\d users`
Expected: columns `email` and `display_name` of type `text`, both nullable.

- [ ] **Step 5: Commit**

```bash
git add services/auth/migrations/2_add_profile_fields.up.sql services/auth/migrations/2_add_profile_fields.down.sql
git commit -m "feat(auth): add email and display_name columns to users"
```

---

### Task 2: Invite-code gate on registration

**Files:**
- Modify: `services/auth/auth.go` (RegisterParams, new `inviteCode()` helper, validation in `Register`)
- Modify: `services/auth/auth_test.go` (existing calls + new gating tests)

- [ ] **Step 1: Update existing tests to pass an invite code and add gating tests**

In `services/auth/auth_test.go`, add `InviteCode: "ashark"` to **every** existing `Register` call (in `TestRegister`, `TestRegisterDuplicateUsername`, `TestLogin`, `TestLoginWrongPassword`), and append these two tests:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `encore test ./services/auth/... -run 'TestRegister'`
Expected: FAIL — `RegisterParams` has no field `InviteCode` (compile error).

- [ ] **Step 3: Add the field, helper, and validation**

In `services/auth/auth.go`, add `"os"` to the import block. Add `InviteCode` to `RegisterParams`:
```go
type RegisterParams struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}
```

Add the helper (near the top, after the `db` var):
```go
// inviteCode returns the registration invite code. Overridable per deployment
// via REGISTER_INVITE_CODE; defaults to "ashark" so the app runs with no config.
func inviteCode() string {
	if v := os.Getenv("REGISTER_INVITE_CODE"); v != "" {
		return v
	}
	return "ashark"
}
```

In `Register`, immediately after the empty-username/password check, add:
```go
	if p.InviteCode != inviteCode() {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid invite code").Err()
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `encore test ./services/auth/...`
Expected: PASS (all register/login tests, including the two new gating tests).

- [ ] **Step 5: Commit**

```bash
git add services/auth/auth.go services/auth/auth_test.go
git commit -m "feat(auth): require invite code for registration"
```

---

### Task 3: Extend `Me` with email and display name

**Files:**
- Modify: `services/auth/auth.go` (`MeResponse`, `Me`, add `meByID` helper)

- [ ] **Step 1: Add the `meByID` helper and extend `MeResponse`**

Replace the `MeResponse` struct and `Me` function in `services/auth/auth.go` with:
```go
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
```

- [ ] **Step 2: Add a test for `meByID`**

Append to `services/auth/auth_test.go`:
```go
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
```

- [ ] **Step 3: Run the test**

Run: `encore test ./services/auth/... -run TestMeByIDDefaults`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add services/auth/auth.go services/auth/auth_test.go
git commit -m "feat(auth): return email and display_name from /auth/me"
```

---

### Task 4: `UpdateProfile` endpoint

**Files:**
- Modify: `services/auth/auth.go` (params, `updateProfile` helper, endpoint)
- Modify: `services/auth/auth_test.go`

- [ ] **Step 1: Write failing tests**

Append to `services/auth/auth_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `encore test ./services/auth/... -run TestUpdateProfile`
Expected: FAIL — `updateProfile` / `UpdateProfileParams` undefined.

- [ ] **Step 3: Implement params, helper, and endpoint**

Add `"net/mail"` to the imports in `services/auth/auth.go`. Add:
```go
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
		if _, err := mail.ParseAddress(p.Email); err != nil {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `encore test ./services/auth/... -run TestUpdateProfile`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add services/auth/auth.go services/auth/auth_test.go
git commit -m "feat(auth): add UpdateProfile endpoint"
```

---

### Task 5: `ChangePassword` endpoint

**Files:**
- Modify: `services/auth/auth.go` (params, `changePassword` helper, endpoint)
- Modify: `services/auth/auth_test.go`

- [ ] **Step 1: Write failing tests**

Append to `services/auth/auth_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `encore test ./services/auth/... -run TestChangePassword`
Expected: FAIL — `changePassword` / `ChangePasswordParams` undefined.

- [ ] **Step 3: Implement params, helper, and endpoint**

Add to `services/auth/auth.go`:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `encore test ./services/auth/...`
Expected: PASS (full auth suite).

- [ ] **Step 5: Commit**

```bash
git add services/auth/auth.go services/auth/auth_test.go
git commit -m "feat(auth): add ChangePassword endpoint"
```

---

### Task 6: Regenerate the frontend client

**Files:**
- Modify: `frontend/src/lib/client.gen.ts` (generated)

- [ ] **Step 1: Ensure the backend is running**

Run: `encore run` (in a separate terminal; leave it running).

- [ ] **Step 2: Regenerate the client**

Run:
```bash
encore gen client subs-check-uqti --lang=typescript --output=./frontend/src/lib/client.gen.ts
```

- [ ] **Step 3: Verify new symbols exist**

Run: `grep -nE "UpdateProfile|ChangePassword|invite_code|display_name" frontend/src/lib/client.gen.ts`
Expected: matches for `UpdateProfile`, `ChangePassword`, `invite_code`, `display_name`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/client.gen.ts
git commit -m "chore(web): regenerate API client for account endpoints"
```

---

## Phase 2 — Frontend

### Task 7: Invite-code field on the login page

**Files:**
- Modify: `frontend/src/routes/login.tsx`

- [ ] **Step 1: Add invite-code state**

In `LoginPage`, add after the `password` state:
```tsx
	const [inviteCode, setInviteCode] = useState("");
```

- [ ] **Step 2: Send the invite code on register**

In `submit`, replace the `registerMut.mutate({ username, password }, …)` payload first argument with:
```tsx
				registerMut.mutate(
					{ username, password, invite_code: inviteCode },
```
(leave the `onSuccess`/`onError` block unchanged).

- [ ] **Step 3: Render the field in register mode**

Inside the `<div className="space-y-3">` block, after the password field's closing `</div>` and before the `mode === "login" ? (…)` remember-me block, add:
```tsx
						{mode === "register" ? (
							<div className="space-y-1.5">
								<Label htmlFor="invite" className="text-xs">
									Invite code
								</Label>
								<Input
									id="invite"
									value={inviteCode}
									onChange={(e) => setInviteCode(e.target.value)}
								/>
							</div>
						) : null}
```

- [ ] **Step 4: Require the field before enabling submit in register mode**

Change the submit `Button`'s `disabled` prop to:
```tsx
						disabled={!username || !password || (mode === "register" && !inviteCode)}
```

- [ ] **Step 5: Verify types**

Run: `cd frontend && bun run check-types`
Expected: PASS (`invite_code` is a known field on `RegisterParams`).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/login.tsx
git commit -m "feat(web): add invite code field to registration"
```

---

### Task 8: Profile and password mutation hooks

**Files:**
- Modify: `frontend/src/queries/auth.ts`

- [ ] **Step 1: Add the two hooks**

Change the first import line of `frontend/src/queries/auth.ts` to:
```tsx
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
```
Then append:
```tsx
export function useUpdateProfile() {
	const qc = useQueryClient();
	return useMutation({
		mutationFn: (p: auth.UpdateProfileParams) => client.auth.UpdateProfile(p),
		onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.me() }),
	});
}

export function useChangePassword() {
	return useMutation({
		mutationFn: (p: auth.ChangePasswordParams) => client.auth.ChangePassword(p),
	});
}
```

- [ ] **Step 2: Verify types**

Run: `cd frontend && bun run check-types`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/queries/auth.ts
git commit -m "feat(web): add profile and password mutation hooks"
```

---

### Task 9: Account settings page + tab + rail link

**Files:**
- Create: `frontend/src/routes/settings/account.tsx`
- Modify: `frontend/src/routes/settings.tsx`
- Modify: `frontend/src/components/rail.tsx`

- [ ] **Step 1: Create the account page**

`frontend/src/routes/settings/account.tsx`:
```tsx
import { zodResolver } from "@hookform/resolvers/zod";
import { createFileRoute } from "@tanstack/react-router";
import type * as React from "react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { isApiError } from "@/lib/client";
import { useChangePassword, useMe, useUpdateProfile } from "@/queries";

export const Route = createFileRoute("/settings/account")({
	component: AccountSettingsPage,
});

const profileSchema = z.object({
	username: z.string().min(1, "Username is required"),
	display_name: z.string(),
	email: z.union([z.string().email("Invalid email"), z.literal("")]),
});
type ProfileValues = z.infer<typeof profileSchema>;

const passwordSchema = z
	.object({
		current_password: z.string().min(1, "Required"),
		new_password: z.string().min(8, "At least 8 characters"),
		confirm_password: z.string().min(1, "Required"),
	})
	.refine((v) => v.new_password === v.confirm_password, {
		message: "Passwords do not match",
		path: ["confirm_password"],
	});
type PasswordValues = z.infer<typeof passwordSchema>;

function Section({
	title,
	description,
	children,
}: {
	title: string;
	description: string;
	children: React.ReactNode;
}) {
	return (
		<section className="rounded-lg border border-border bg-card p-4 md:p-5">
			<h2 className="font-semibold text-foreground text-sm">{title}</h2>
			<p className="mt-0.5 mb-4 text-muted-foreground text-xs">{description}</p>
			<div className="space-y-3">{children}</div>
		</section>
	);
}

function Field({
	label,
	error,
	children,
}: {
	label: string;
	error?: string;
	children: React.ReactNode;
}) {
	return (
		<div className="space-y-1.5">
			<Label className="text-xs">{label}</Label>
			{children}
			{error ? <p className="text-danger text-xs">⚠ {error}</p> : null}
		</div>
	);
}

function AccountSettingsPage() {
	const meQuery = useMe();
	const updateMut = useUpdateProfile();
	const passwordMut = useChangePassword();

	const profileForm = useForm<ProfileValues>({
		resolver: zodResolver(profileSchema),
		defaultValues: { username: "", display_name: "", email: "" },
	});
	const passwordForm = useForm<PasswordValues>({
		resolver: zodResolver(passwordSchema),
		defaultValues: { current_password: "", new_password: "", confirm_password: "" },
	});

	const me = meQuery.data;
	useEffect(() => {
		if (me) {
			profileForm.reset({
				username: me.username,
				display_name: me.display_name,
				email: me.email,
			});
		}
	}, [me, profileForm]);

	const onProfile = (values: ProfileValues) => {
		updateMut.mutate(values, {
			onSuccess: () => toast.success("Profile saved"),
			onError: (e) => toast.error(isApiError(e) ? e.message : "Failed to save"),
		});
	};

	const onPassword = (values: PasswordValues) => {
		passwordMut.mutate(
			{ current_password: values.current_password, new_password: values.new_password },
			{
				onSuccess: () => {
					toast.success("Password changed");
					passwordForm.reset();
				},
				onError: (e) =>
					toast.error(isApiError(e) ? e.message : "Failed to change password"),
			},
		);
	};

	if (meQuery.isLoading) {
		return (
			<div className="space-y-3">
				<Skeleton className="h-48 w-full" />
				<Skeleton className="h-48 w-full" />
			</div>
		);
	}

	const pErr = profileForm.formState.errors;
	const wErr = passwordForm.formState.errors;

	return (
		<div className="space-y-4">
			<form onSubmit={profileForm.handleSubmit(onProfile)}>
				<Section title="Profile" description="Your account name and contact details.">
					<Field label="Username" error={pErr.username?.message}>
						<Input {...profileForm.register("username")} autoComplete="username" />
					</Field>
					<div className="grid gap-3 sm:grid-cols-2">
						<Field label="Display name" error={pErr.display_name?.message}>
							<Input {...profileForm.register("display_name")} />
						</Field>
						<Field label="Email" error={pErr.email?.message}>
							<Input {...profileForm.register("email")} type="email" autoComplete="email" />
						</Field>
					</div>
					<div className="flex justify-end pt-1">
						<Button type="submit" variant="success" loading={updateMut.isPending}>
							Save profile
						</Button>
					</div>
				</Section>
			</form>

			<form onSubmit={passwordForm.handleSubmit(onPassword)}>
				<Section title="Password" description="Change the password you use to sign in.">
					<Field label="Current password" error={wErr.current_password?.message}>
						<Input
							type="password"
							autoComplete="current-password"
							{...passwordForm.register("current_password")}
						/>
					</Field>
					<div className="grid gap-3 sm:grid-cols-2">
						<Field label="New password" error={wErr.new_password?.message}>
							<Input
								type="password"
								autoComplete="new-password"
								{...passwordForm.register("new_password")}
							/>
						</Field>
						<Field label="Confirm new password" error={wErr.confirm_password?.message}>
							<Input
								type="password"
								autoComplete="new-password"
								{...passwordForm.register("confirm_password")}
							/>
						</Field>
					</div>
					<div className="flex justify-end pt-1">
						<Button type="submit" variant="success" loading={passwordMut.isPending}>
							Change password
						</Button>
					</div>
				</Section>
			</form>
		</div>
	);
}
```

- [ ] **Step 2: Add the "Account" tab**

In `frontend/src/routes/settings.tsx`, append to the `TABS` array:
```tsx
	{ to: "/settings/account" as const, label: "Account" },
```

- [ ] **Step 3: Add a rail link to account settings and prefer display name**

In `frontend/src/components/rail.tsx`:
- Inside `DropdownMenuContent`, add an account item directly above the existing logout `DropdownMenuItem`:
```tsx
						<DropdownMenuItem onClick={() => navigate({ to: "/settings/account" })}>
							<Settings size={14} /> Account settings
						</DropdownMenuItem>
```
- Change the label source to prefer display name:
```tsx
	const username = meQuery.data?.display_name || meQuery.data?.username || "…";
```
(`Settings` and `navigate` are already imported/defined in this file.)

- [ ] **Step 4: Verify routes generate and types pass**

Run: `cd frontend && bun run check-types`
Expected: PASS (TanStack regenerates `routeTree.gen.ts`; `/settings/account` becomes a known route).

- [ ] **Step 5: Build**

Run: `cd frontend && bun run build`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/settings/account.tsx frontend/src/routes/settings.tsx frontend/src/components/rail.tsx frontend/src/routeTree.gen.ts
git commit -m "feat(web): add account settings page with profile and password editing"
```

---

### Task 10: Manual functional verification (browser)

**Files:** none (verification only)

- [ ] **Step 1: Start both servers**

Run: `encore run` and, in another terminal, `cd frontend && bun dev` (web on port 3001).

- [ ] **Step 2: Register with the invite code**

Open `/login`, switch to register, try a wrong invite code → expect toast "invalid invite code". Then register with code `ashark` → expect "Account created — please sign in".

- [ ] **Step 3: Edit profile**

Sign in, go to `/settings/account`, change display name + email, Save → expect "Profile saved" and the rail avatar label switches to the display name.

- [ ] **Step 4: Change password**

Use a wrong current password → expect "current password is incorrect". Use the correct current password + matching new (≥8 chars) → expect "Password changed". Log out, sign in with the new password → succeeds.

- [ ] **Step 5: No commit**

This task changes no files. If a bug was found, fix it under the relevant task above and re-verify.

---

## Phase 3 — Mobile Audit

### Task 11: Per-route mobile audit and fixes at 375 / 768px

**Files:** route/component files as needed (fixes only where a problem is found)

> Audit in a real browser (Chrome DevTools device toolbar or the chrome-devtools MCP). A green `build`/`check-types` does **not** prove Tailwind utilities rendered — confirm actual computed layout. For each route, check: no horizontal page overflow, tap targets ≥ ~32–36px, dialogs/tables fit or reflow, no clipped controls.

- [ ] **Step 1: Resize to 375px and walk every route**

Visit and inspect at 375px: `/login` (with invite field), `/` (subscriptions), `/subscriptions/$id` (node table/cards + live feed), `/scheduler`, `/rules`, `/settings/general`, `/settings/notify`, `/settings/export`, `/settings/export-tags`, `/settings/account`. Note any route with horizontal overflow, clipped controls, or unreadable density.

- [ ] **Step 2: Repeat at 768px**

Confirm the `md` breakpoint transition looks correct (rail vs tabbar, table vs cards) and nothing is half-collapsed.

- [ ] **Step 3: Fix each issue found, smallest change first**

Apply targeted Tailwind fixes (e.g. `flex-wrap`, `min-w-0`, `overflow-x-auto`, responsive `grid-cols`, `min-h-*` on dense controls). Make one fix, re-check that route in the browser, then move on. Do **not** alter the desktop (`>= md`) layout.

- [ ] **Step 4: Re-verify the whole list**

Re-walk all routes at 375px and 768px; confirm no remaining horizontal overflow and all primary actions are reachable and tappable.

- [ ] **Step 5: Type-check, build, lint**

Run: `cd frontend && bun run check-types && bun run build && bun check`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add -A frontend/src
git commit -m "fix(web): mobile layout fixes across all routes"
```

---

## Self-Review (completed during authoring)

- **Spec coverage:** migration (T1), invite gate (T2), `Me` extension (T3), `UpdateProfile` (T4), `ChangePassword` (T5), client regen (T6), login invite field (T7), hooks (T8), account page + tab + rail (T9), functional check (T10), mobile audit (T11). All spec sections mapped.
- **Placeholders:** none — every code step contains full code; the mobile task is verification-driven and lists concrete checks/fixes rather than fabricated diffs.
- **Type consistency:** `UpdateProfileParams` / `ChangePasswordParams` / `MeResponse` (`user_id`, `username`, `email`, `display_name`) and the `invite_code` JSON tag are used identically across backend, generated client, and frontend hooks/forms; helper names (`meByID`, `updateProfile`, `changePassword`) match between endpoints and tests.
