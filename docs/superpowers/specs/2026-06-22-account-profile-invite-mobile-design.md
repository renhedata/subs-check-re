# Account Profile Editing, Registration Invite Code, and Mobile Audit

**Status:** Approved — 2026-06-22

## Problem

Three gaps in the current product:

1. **No way to edit your account.** The `users` table holds only
   `id, username, password_hash, created_at`. After registering, a user can never
   change their username or password, and there is no place to store contact info
   (email) or a friendly display name. The Rail account dropdown shows only
   "Signed in as X" and "Log out".
2. **Registration is open to anyone.** `auth.Register` (`services/auth/auth.go`)
   accepts any `username` + `password` and creates a user. There is no gate, so
   the deployment cannot be limited to invited users.
3. **Mobile support is only partially verified.** Responsive infrastructure
   exists (`MobileTabbar`, a `md:`-gated `Rail`, `flex-col md:flex-row` shell in
   `__root.tsx`), but not every route has been audited at phone widths. The
   requirement is that *all* pages work on mobile. (The original mobile design,
   `2026-06-09-mobile-responsive-design.md`, predates the shipped `Rail` +
   `MobileTabbar` layout — the implementation diverged to a bottom tab bar — so
   this pass audits the routes as they exist today, including ones added since.)

## Goal

- A signed-in user can change their **username**, **password**, **email**, and
  **display name** from a dedicated account page.
- New registrations must supply a correct **invite code** (default `ashark`,
  overridable by environment variable) or are rejected.
- Every route is audited and fixed to render correctly on mobile (375px and
  768px), verified in a real browser.

**Non-goals:** password reset / "forgot password" flows, email verification or
sending mail to the new `email` field, a managed multi-code invite system, and
account deletion. Email and display name are free-form profile fields only.

## Approach

Extend the existing `auth` service with a profile migration and three endpoints,
follow the established settings-subpage pattern on the frontend for a new
`/settings/account` page, gate registration on an env-sourced invite code, and
do a page-by-page mobile pass. Username changes do **not** force re-login because
the JWT is keyed on `user_id` (`sub`), not username (`services/auth/jwt.go`).

### Backend — `auth` service

**1. Migration** `services/auth/migrations/2_add_profile_fields.up.sql`
```sql
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN display_name TEXT;
```
`.down.sql` drops both columns. Both are nullable; existing rows get `NULL`,
surfaced to the API as empty strings via `COALESCE`.

**2. Invite code source** — a small helper in `auth.go`:
```go
func inviteCode() string {
    if v := os.Getenv("REGISTER_INVITE_CODE"); v != "" {
        return v
    }
    return "ashark"
}
```
Low-sensitivity by design: not committed to git, overridable per deployment, and
works with zero config. If stronger management is wanted later, swap to an Encore
secret (the `secrets struct` pattern already used for `JWTSecret`).

**3. `Register` (changed)** — add `InviteCode string` to `RegisterParams`.
Validate **before** hashing/insert: if `p.InviteCode != inviteCode()` return
`errs.InvalidArgument` "invalid invite code". Registration still creates only
username + password; email/display_name are filled in later on the profile page.

**4. `Me` (changed)** — `MeResponse` gains `Email string` and `DisplayName string`.
Query becomes `SELECT username, COALESCE(email,''), COALESCE(display_name,'')`.

**5. `UpdateProfile` (new)** — `//encore:api auth method=PATCH path=/auth/profile`
```go
type UpdateProfileParams struct {
    Username    string `json:"username"`
    Email       string `json:"email"`        // "" allowed (clears)
    DisplayName string `json:"display_name"` // "" allowed (clears)
}
```
- `Username` required (non-empty); on unique violation return `AlreadyExists`
  "username already taken".
- `Email` validated only when non-empty (simple format check); stored as `NULL`
  when empty.
- `DisplayName` stored as `NULL` when empty.
- Returns the updated `MeResponse`. Operates on `claims.UserID`.

**6. `ChangePassword` (new)** — `//encore:api auth method=POST path=/auth/change-password`
```go
type ChangePasswordParams struct {
    CurrentPassword string `json:"current_password"`
    NewPassword     string `json:"new_password"`
}
```
- Load `password_hash` for `claims.UserID`; `bcrypt.CompareHashAndPassword`
  against `CurrentPassword`. Mismatch → `InvalidArgument` "current password is
  incorrect".
- `NewPassword` minimum length (8). Hash and `UPDATE`. Returns empty success.
- The current JWT remains valid (keyed on `user_id`); no forced re-login.

### Frontend

**1. Regenerate client** after backend changes:
`encore gen client subs-check-uqti --lang=typescript --output=./frontend/src/lib/client.gen.ts`
(client output path is `frontend/src/lib/` per the flat-monorepo layout).

**2. Login page** (`routes/login.tsx`) — in **register mode only**, add an
"Invite code" `Input`; include `invite_code` in the register mutation payload.
Login mode is unchanged.

**3. Query hooks** (`queries/auth.ts`) — add `useUpdateProfile` and
`useChangePassword` mutations; both invalidate `queryKeys.me()` on success.
`useMe`'s type updates automatically from the regenerated client.

**4. New route** `routes/settings/account.tsx` — mirror `settings/general.tsx`
(react-hook-form + zod + the local `Section`/`Field` helpers):
- **Profile** section: `username`, `display_name`, `email` → "Save profile"
  (`UpdateProfile`).
- **Password** section: `current_password`, `new_password`, `confirm_password`
  (client-side confirm match) → "Change password" (`ChangePassword`).
- Prefill profile fields from `useMe`; toast on success/error.

**5. Settings tab nav** (`routes/settings.tsx` layout) — add an "Account" tab
linking to `/settings/account`.

**6. Rail dropdown** (`components/rail.tsx`) — make the account area link to
`/settings/account` (e.g. an "Account settings" item) and prefer `display_name`
over `username` for the label when present.

### Mobile audit + fixes

Audit each route at **375px** and **768px** in a real browser; fix overflow,
horizontal scrolling, too-small tap targets, and dialogs/tables that exceed the
viewport. Routes:

- `/` (subscriptions index)
- `/subscriptions/$id` (detail, incl. node table/cards and live progress feed)
- `/scheduler`
- `/rules`
- `/settings/general`, `/settings/notify`, `/settings/export`,
  `/settings/export-tags`, **`/settings/account`** (new)
- `/login` (now with the invite-code field)

Per prior experience, a passing `build`/`tsc`/lint does **not** prove Tailwind
utilities rendered — confirm actual computed styles in the browser, not just a
green build.

## Testing

- **Go** (`services/auth/auth_test.go`, run **without** `-race` — the harness
  hangs with it): register with correct vs. wrong invite code; `UpdateProfile`
  success, duplicate-username conflict, invalid email; `ChangePassword` success
  and wrong-current-password rejection.
- **Frontend**: `bun run check-types` + `bun run build`; manual mobile
  verification of every route at 375/768px in the browser.

## Risks / Tradeoffs

- **Invite code in env, not a secret.** Acceptable for a low-sensitivity gate;
  documented path to upgrade to an Encore secret later.
- **Email is unverified, free-form.** Intentional non-goal; it is a profile field
  today, not an auth or recovery channel.
- **Username editable.** Safe for sessions (JWT keyed on `user_id`); uniqueness
  enforced at the DB and surfaced as `AlreadyExists`.
