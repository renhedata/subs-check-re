# Backend Foundation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundational Go backend — Go module setup, auth service (register/login/JWT), and subscription CRUD service — all running on Encore with PostgreSQL.

**Architecture:** Two Encore services (`auth`, `subscription`) each with their own Encore-managed PostgreSQL database. JWT signing uses a secret stored in Encore's secrets system. Subscription service enforces multi-tenancy by filtering all queries with the authenticated user's ID obtained via `auth.Data()`.

**Tech Stack:** Go 1.26, Encore framework, PostgreSQL (Encore-managed), `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `github.com/google/uuid`

> **Reference:** `docs/superpowers/specs/2026-03-14-subs-check-re-design.md`
> **Encore docs:** Use `context7` MCP → resolve `encore.dev` for exact API syntax as needed.

---

## File Map

```
services/
├── auth/
│   ├── migrations/
│   │   ├── 1_create_users.up.sql
│   │   └── 1_create_users.down.sql
│   ├── auth.go          # Register + Login API endpoints
│   ├── authhandler.go   # //encore:authhandler JWT validator
│   ├── jwt.go           # generateJWT / validateJWT helpers
│   └── auth_test.go     # Integration tests
└── subscription/
    ├── migrations/
    │   ├── 1_create_subscriptions.up.sql
    │   └── 1_create_subscriptions.down.sql
    ├── subscription.go  # CRUD + ListNodes stub
    └── subscription_test.go
go.mod
go.sum
```

---

## Prerequisites: Encore CLI Verification

- [ ] **Verify Encore CLI is installed and working**

```bash
encore version
```

Expected: version string printed. If not installed: `brew install encoredev/tap/encore`

- [ ] **Verify local app ID exists**

```bash
cat .encore/manifest.json
```

Expected: `{"local_id":"hy6u8",...}` — this ID is used for `encore secret set --local`.
If missing, run `encore run` once to generate it.

---

## Chunk 1: Go Setup + Auth Service

### Task 1: Add Go dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add required dependencies**

```bash
cd /Users/ashark/Code/subs-check-re
go get encore.dev@latest
go get github.com/golang-jwt/jwt/v5@latest
go get github.com/google/uuid@latest
go get golang.org/x/crypto@latest
```

- [ ] **Step 2: Verify go.mod has all dependencies**

```bash
cat go.mod
```

Expected: `encore.dev`, `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`, `golang.org/x/crypto` all present.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add backend Go dependencies"
```

---

### Task 2: Auth DB migration

**Files:**
- Create: `services/auth/migrations/1_create_users.up.sql`
- Create: `services/auth/migrations/1_create_users.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- services/auth/migrations/1_create_users.up.sql
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 2: Create down migration**

```sql
-- services/auth/migrations/1_create_users.down.sql
DROP TABLE IF EXISTS users;
```

- [ ] **Step 3: Commit**

```bash
git add services/auth/migrations/
git commit -m "feat(auth): add users table migration"
```

---

### Task 3: JWT utilities

**Files:**
- Create: `services/auth/jwt.go`

> Check `context7` MCP (`encore.dev`) for the exact secrets syntax before writing this file.

- [ ] **Step 1: Write JWT helpers**

```go
// services/auth/jwt.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Encore-managed secret. Set locally with:
//   encore secret set --local JWTSecret <any-random-string>
var secrets struct {
	JWTSecret string
}

type jwtClaims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

func generateJWT(userID string) (string, error) {
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secrets.JWTSecret))
}

func validateJWT(tokenStr string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secrets.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./services/auth/...
```

Expected: no errors (may warn about unused package until auth.go is added).

- [ ] **Step 3: Commit**

```bash
git add services/auth/jwt.go
git commit -m "feat(auth): JWT generate/validate helpers"
```

---

### Task 4: Auth handler (Encore JWT middleware)

**Files:**
- Create: `services/auth/authhandler.go`

> `//encore:authhandler` is Encore's hook to validate tokens on every authenticated request.
> The `UserClaims` type is exported so other services can call `auth.Data[*auth.UserClaims]()`.

- [ ] **Step 1: Write auth handler**

```go
// services/auth/authhandler.go
package auth

import (
	"context"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
)

// UserClaims holds per-request auth data, available in all authenticated endpoints
// via auth.Data[*auth.UserClaims]().
type UserClaims struct {
	UserID string
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, *UserClaims, error) {
	claims, err := validateJWT(token)
	if err != nil {
		return "", nil, errs.B().Code(errs.Unauthenticated).Msg("invalid or expired token").Err()
	}
	return auth.UID(claims.UserID), &UserClaims{UserID: claims.UserID}, nil
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./services/auth/...
```

- [ ] **Step 3: Commit**

```bash
git add services/auth/authhandler.go
git commit -m "feat(auth): Encore JWT auth handler"
```

---

### Task 5: Register and Login endpoints

**Files:**
- Create: `services/auth/auth.go`

- [ ] **Step 1: Write auth service**

```go
// services/auth/auth.go
package auth

import (
	"context"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ensure encauth is used (via Me endpoint)
var _ = encauth.Data[*UserClaims]

var db = sqldb.NewDatabase("auth", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// RegisterParams is the request body for POST /auth/register.
type RegisterParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterResponse is the response for POST /auth/register.
type RegisterResponse struct {
	UserID string `json:"user_id"`
}

// Register creates a new user account.
//
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
		// Encore wraps pg errors; check for unique violation
		return nil, errs.B().Code(errs.AlreadyExists).Msg("username already taken").Err()
	}
	return &RegisterResponse{UserID: id}, nil
}

// LoginParams is the request body for POST /auth/login.
type LoginParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response for POST /auth/login.
type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

// Login authenticates a user and returns a JWT.
//
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

// MeResponse is the response for GET /auth/me.
type MeResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// Me returns the current authenticated user's info.
//
//encore:api auth method=GET path=/auth/me
func Me(ctx context.Context) (*MeResponse, error) {
	// NOTE: package is named "auth" so we alias the Encore import as "encauth" above
	claims := encauth.Data[*UserClaims]()
	var username string
	err := db.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, claims.UserID).Scan(&username)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("user not found").Err()
	}
	return &MeResponse{UserID: claims.UserID, Username: username}, nil
}
```

> **Note on `auth.Data[*UserClaims]()`:** The import is `encore.dev/beta/auth`. Verify the generic syntax matches your Encore version via `context7` MCP.

- [ ] **Step 2: Compile check**

```bash
go build ./services/auth/...
```

Expected: compiles cleanly.

- [ ] **Step 3: Set JWT secret for local dev**

```bash
encore secret set --local JWTSecret supersecretdevkey123
```

- [ ] **Step 4: Commit**

```bash
git add services/auth/auth.go
git commit -m "feat(auth): register, login, and me endpoints"
```

---

### Task 6: Auth integration tests

**Files:**
- Create: `services/auth/auth_test.go`

- [ ] **Step 1: Write failing tests**

```go
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
	_, _ = Register(ctx, params) // first registration
	_, err := Register(ctx, params) // second must fail
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
```

- [ ] **Step 2: Run tests (expect FAIL — service not fully wired yet)**

```bash
encore test ./services/auth/...
```

Expected: tests run (Encore spins up local DB), all pass once the service compiles.

- [ ] **Step 3: Commit**

```bash
git add services/auth/auth_test.go
git commit -m "test(auth): register and login integration tests"
```

---

## Chunk 2: Subscription Service

### Task 7: Subscription DB migration

**Files:**
- Create: `services/subscription/migrations/1_create_subscriptions.up.sql`
- Create: `services/subscription/migrations/1_create_subscriptions.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- services/subscription/migrations/1_create_subscriptions.up.sql
CREATE TABLE subscriptions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    cron_expr   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_run_at TIMESTAMPTZ
);

CREATE INDEX idx_subscriptions_user_id ON subscriptions (user_id);
```

- [ ] **Step 2: Create down migration**

```sql
-- services/subscription/migrations/1_create_subscriptions.down.sql
DROP TABLE IF EXISTS subscriptions;
```

- [ ] **Step 3: Commit**

```bash
git add services/subscription/migrations/
git commit -m "feat(subscription): add subscriptions table migration"
```

---

### Task 8: Subscription CRUD service

**Files:**
- Create: `services/subscription/subscription.go`

> All endpoints are `//encore:api auth` — require valid JWT.
> Get current user ID via: `auth.Data[*authsvc.UserClaims]().UserID`
> Import the `auth` package as `authsvc "subs-check-re/services/auth"`.

- [ ] **Step 1: Write subscription service**

```go
// services/subscription/subscription.go
package subscription

import (
	"context"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
)

var db = sqldb.NewDatabase("subscription", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// Subscription represents a proxy subscription link.
type Subscription struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Enabled    bool       `json:"enabled"`
	CronExpr   *string    `json:"cron_expr"`
	CreatedAt  time.Time  `json:"created_at"`
	LastRunAt  *time.Time `json:"last_run_at"`
}

// ListResponse is the response for GET /subscriptions.
type ListResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// List returns all subscriptions for the current user.
//
//encore:api auth method=GET path=/subscriptions
func List(ctx context.Context) (*ListResponse, error) {
	uid := encauth.Data[*authsvc.UserClaims]().UserID
	rows, err := db.Query(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at
		FROM subscriptions WHERE user_id = $1 ORDER BY created_at DESC
	`, uid)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db query failed").Err()
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled,
			&s.CronExpr, &s.CreatedAt, &s.LastRunAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = []Subscription{}
	}
	return &ListResponse{Subscriptions: subs}, nil
}

// CreateParams is the request body for POST /subscriptions.
type CreateParams struct {
	Name     string  `json:"name"`
	URL      string  `json:"url"`
	CronExpr *string `json:"cron_expr"`
}

// Create adds a new subscription for the current user.
//
//encore:api auth method=POST path=/subscriptions
func Create(ctx context.Context, p *CreateParams) (*Subscription, error) {
	if p.URL == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("url is required").Err()
	}
	uid := encauth.Data[*authsvc.UserClaims]().UserID
	id := uuid.New().String()
	_, err := db.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, name, url, cron_expr, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, uid, p.Name, p.URL, p.CronExpr, time.Now())
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create subscription").Err()
	}
	return &Subscription{
		ID:       id,
		UserID:   uid,
		Name:     p.Name,
		URL:      p.URL,
		Enabled:  true,
		CronExpr: p.CronExpr,
	}, nil
}

// UpdateParams is the request body for PUT /subscriptions/:id.
// Use ClearCronExpr=true to remove the cron schedule (set cron_expr to NULL).
type UpdateParams struct {
	Name         *string `json:"name"`
	URL          *string `json:"url"`
	Enabled      *bool   `json:"enabled"`
	CronExpr     *string `json:"cron_expr"`
	ClearCronExpr bool   `json:"clear_cron_expr"`
}

// Update modifies a subscription owned by the current user.
//
//encore:api auth method=PUT path=/subscriptions/:id
func Update(ctx context.Context, id string, p *UpdateParams) (*Subscription, error) {
	uid := encauth.Data[*authsvc.UserClaims]().UserID
	// Verify ownership
	var ownerID string
	if err := db.QueryRow(ctx, `SELECT user_id FROM subscriptions WHERE id = $1`, id).Scan(&ownerID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	if ownerID != uid {
		return nil, errs.B().Code(errs.PermissionDenied).Msg("access denied").Err()
	}

	// Build cron_expr value: explicit NULL if ClearCronExpr=true, else use provided value or keep existing
	var cronExprSQL interface{}
	if p.ClearCronExpr {
		cronExprSQL = nil // forces NULL
	} else {
		cronExprSQL = p.CronExpr // nil = keep existing via COALESCE
	}

	_, err := db.Exec(ctx, `
		UPDATE subscriptions SET
			name      = COALESCE($2, name),
			url       = COALESCE($3, url),
			enabled   = COALESCE($4, enabled),
			cron_expr = CASE WHEN $6::boolean THEN NULL ELSE COALESCE($5, cron_expr) END
		WHERE id = $1
	`, id, p.Name, p.URL, p.Enabled, cronExprSQL, p.ClearCronExpr)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}

	var s Subscription
	if err := db.QueryRow(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at
		FROM subscriptions WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled, &s.CronExpr, &s.CreatedAt, &s.LastRunAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("fetch after update failed").Err()
	}
	return &s, nil
}

// DeleteResponse is the response for DELETE /subscriptions/:id.
type DeleteResponse struct {
	OK bool `json:"ok"`
}

// Delete removes a subscription owned by the current user.
//
//encore:api auth method=DELETE path=/subscriptions/:id
func Delete(ctx context.Context, id string) (*DeleteResponse, error) {
	uid := encauth.Data[*authsvc.UserClaims]().UserID
	result, err := db.Exec(ctx, `
		DELETE FROM subscriptions WHERE id = $1 AND user_id = $2
	`, id, uid)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("delete failed").Err()
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	return &DeleteResponse{OK: true}, nil
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./services/subscription/...
```

Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add services/subscription/subscription.go
git commit -m "feat(subscription): CRUD endpoints"
```

---

### Task 9: Subscription integration tests

**Files:**
- Create: `services/subscription/subscription_test.go`

> These tests require auth to work. Encore test framework supports calling other service functions directly.
> The `Me` endpoint auth test may need to simulate auth context — check Encore test docs for `et.WithAuth`.

- [ ] **Step 1: Write failing tests**

```go
// services/subscription/subscription_test.go
package subscription

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

// testCtx returns a context with a fake authenticated user injected.
// encore.dev/et provides test utilities for injecting auth data.
func testCtx() context.Context {
	uid := auth.UID("test-user-id")
	data := &authsvc.UserClaims{UserID: "test-user-id"}
	return et.WithAuthData(context.Background(), uid, data)
}

func TestCreateAndListSubscription(t *testing.T) {
	ctx := testCtx()
	created, err := Create(ctx, &CreateParams{
		Name: "Test Sub",
		URL:  "https://example.com/sub.yaml",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	list, err := List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list.Subscriptions) == 0 {
		t.Error("expected at least one subscription")
	}
}

func TestDeleteSubscription(t *testing.T) {
	ctx := testCtx()
	created, err := Create(ctx, &CreateParams{URL: "https://example.com/sub2.yaml"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	resp, err := Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !resp.OK {
		t.Error("expected OK=true")
	}
}
```

> **Note on `et.WithAuthData`:** Verify exact signature with `context7` MCP → `encore.dev/et` if the above doesn't compile. The pattern is: inject auth UID + typed auth data into the context before calling authenticated service functions.

- [ ] **Step 2: Run tests**

```bash
encore test ./services/subscription/...
```

Expected: tests pass.

- [ ] **Step 3: Commit**

```bash
git add services/subscription/subscription_test.go
git commit -m "test(subscription): CRUD integration tests"
```

---

### Task 10: Smoke test with `encore run`

- [ ] **Step 1: Start dev server**

```bash
encore run
```

Expected: server starts on port 4000, Encore auto-starts local PostgreSQL, migrations run.

- [ ] **Step 2: Register a user**

```bash
curl -X POST http://localhost:4000/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

Expected: `{"user_id":"..."}`

- [ ] **Step 3: Login**

```bash
curl -X POST http://localhost:4000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

Expected: `{"token":"...","user_id":"..."}`  — copy the token.

- [ ] **Step 4: Create a subscription**

```bash
curl -X POST http://localhost:4000/subscriptions \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Sub","url":"https://example.com/sub.yaml"}'
```

Expected: `{"id":"...","name":"My Sub",...}`

- [ ] **Step 5: List subscriptions**

```bash
curl http://localhost:4000/subscriptions \
  -H "Authorization: Bearer <token>"
```

Expected: `{"subscriptions":[...]}`

- [ ] **Step 6: Commit final state**

```bash
git add -A
git commit -m "feat: backend foundation complete — auth + subscription services"
```

---

## What's Next

After this plan is complete and smoke-tested:

- **Plan 2:** `checker` service — mihomo proxy engine integration, node fetching/replacement, concurrent check jobs, SSE progress stream
- **Plan 3:** `scheduler` + `notify` services — cron job management, webhook/telegram/email notifications
- **Plan 4:** Frontend — React app with TanStack Router, auth flow, subscription management, real-time node check progress
