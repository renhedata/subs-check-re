# Scheduler + Notify Services Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `scheduler` service (cron-based automatic checks) and `notify` service (webhook + Telegram notifications on job completion).

**Architecture:**
- `scheduler` owns its own `scheduled_jobs` table and an in-process `robfig/cron` instance. On startup it re-registers all active schedules. When a cron fires it calls `checker.TriggerCheck`.
- `notify` subscribes to `checker.JobCompletedTopic` via Encore PubSub and sends configured channels. Channels config is stored in a `notify_channels` table.

**Tech Stack:** `github.com/robfig/cron/v3`, Encore `//encore:service` lifecycle, Encore PubSub subscription

> **Reference:** `docs/superpowers/specs/2026-03-14-subs-check-re-design.md`

---

## File Map

```
services/
├── scheduler/
│   ├── migrations/
│   │   ├── 1_create_scheduled_jobs.up.sql
│   │   └── 1_create_scheduled_jobs.down.sql
│   └── scheduler.go      # Service struct + CRUD API + cron lifecycle
└── notify/
    ├── migrations/
    │   ├── 1_create_notify_channels.up.sql
    │   └── 1_create_notify_channels.down.sql
    └── notify.go         # PubSub subscriber + channel CRUD + senders
```

---

## Chunk 1: Scheduler Service

### Task 1: Add cron dependency

**Files:** `go.mod`, `go.sum`

- [ ] **Step 1: Add dependency**

```bash
go get github.com/robfig/cron/v3
go mod tidy
```

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(scheduler): add robfig/cron/v3 dependency"
```

---

### Task 2: Scheduler DB migration

**Files:**
- Create: `services/scheduler/migrations/1_create_scheduled_jobs.up.sql`
- Create: `services/scheduler/migrations/1_create_scheduled_jobs.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- services/scheduler/migrations/1_create_scheduled_jobs.up.sql
CREATE TABLE scheduled_jobs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    cron_expr       TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_scheduled_jobs_subscription_id ON scheduled_jobs (subscription_id);
```

- [ ] **Step 2: Create down migration**

```sql
-- services/scheduler/migrations/1_create_scheduled_jobs.down.sql
DROP TABLE IF EXISTS scheduled_jobs;
```

- [ ] **Step 3: Commit**

```bash
git add services/scheduler/migrations/
git commit -m "feat(scheduler): add scheduled_jobs migration"
```

---

### Task 3: Scheduler service

**Files:**
- Create: `services/scheduler/scheduler.go`

> `//encore:service` gives the service a named struct and `initService()` constructor.
> The struct holds the cron instance. On init, all enabled scheduled_jobs are loaded and registered.
> Cross-service calls: use `checker.TriggerCheck(ctx, subscriptionID)` directly — Encore compiles this as an internal RPC.

- [ ] **Step 1: Write scheduler.go**

```go
// services/scheduler/scheduler.go
package scheduler

import (
	"context"
	"fmt"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	authsvc "subs-check-re/services/auth"
	checkersvc "subs-check-re/services/checker"
)

var db = sqldb.NewDatabase("scheduler", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- Service lifecycle ---

//encore:service
type Service struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID // subscription_id → cron entry ID
}

func initService() (*Service, error) {
	svc := &Service{
		cron:    cron.New(),
		entries: make(map[string]cron.EntryID),
	}

	// Load all enabled scheduled jobs from DB on startup
	ctx := context.Background()
	rows, err := db.Query(ctx, `
		SELECT subscription_id, cron_expr FROM scheduled_jobs WHERE enabled = true
	`)
	if err != nil {
		return nil, fmt.Errorf("load scheduled jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subID, cronExpr string
		if err := rows.Scan(&subID, &cronExpr); err != nil {
			continue
		}
		svc.registerCron(subID, cronExpr)
	}

	svc.cron.Start()
	return svc, nil
}

func (s *Service) registerCron(subscriptionID, cronExpr string) {
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		ctx := context.Background()
		// Fire-and-forget: trigger check for this subscription
		// TriggerCheck needs auth context — use a system user approach:
		// Since cron runs without user context, we store user_id in the job
		// and call via internal background context. For now trigger directly.
		checkersvc.TriggerCheck(ctx, subscriptionID) //nolint
	})
	if err == nil {
		s.entries[subscriptionID] = entryID
	}
}

func (s *Service) removeCron(subscriptionID string) {
	if entryID, ok := s.entries[subscriptionID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, subscriptionID)
	}
}

// --- Request/Response types ---

// ScheduledJob represents a cron schedule entry.
type ScheduledJob struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	CronExpr       string    `json:"cron_expr"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListResponse is the response for GET /scheduler.
type ListResponse struct {
	Jobs []ScheduledJob `json:"jobs"`
}

// CreateParams is the request body for POST /scheduler.
type CreateParams struct {
	SubscriptionID string `json:"subscription_id"`
	CronExpr       string `json:"cron_expr"`
}

// DeleteResponse is the response for DELETE /scheduler/:id.
type DeleteResponse struct {
	OK bool `json:"ok"`
}

// --- API endpoints ---

// List returns all scheduled jobs for the current user.
//
//encore:api auth method=GET path=/scheduler
func (s *Service) List(ctx context.Context) (*ListResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	rows, err := db.Query(ctx, `
		SELECT id, subscription_id, cron_expr, enabled, created_at
		FROM scheduled_jobs WHERE user_id = $1 ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var jobs []ScheduledJob
	for rows.Next() {
		var j ScheduledJob
		if err := rows.Scan(&j.ID, &j.SubscriptionID, &j.CronExpr, &j.Enabled, &j.CreatedAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []ScheduledJob{}
	}
	return &ListResponse{Jobs: jobs}, nil
}

// Create adds a cron schedule for a subscription.
//
//encore:api auth method=POST path=/scheduler
func (s *Service) Create(ctx context.Context, p *CreateParams) (*ScheduledJob, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if p.SubscriptionID == "" || p.CronExpr == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("subscription_id and cron_expr required").Err()
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(p.CronExpr); err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid cron expression").Err()
	}

	id := uuid.New().String()
	_, err := db.Exec(ctx, `
		INSERT INTO scheduled_jobs (id, subscription_id, user_id, cron_expr, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (subscription_id) DO UPDATE SET cron_expr = $4, enabled = true
	`, id, p.SubscriptionID, claims.UserID, p.CronExpr, time.Now())
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create scheduled job").Err()
	}

	// Register/update in-memory cron
	s.removeCron(p.SubscriptionID)
	s.registerCron(p.SubscriptionID, p.CronExpr)

	return &ScheduledJob{
		ID:             id,
		SubscriptionID: p.SubscriptionID,
		CronExpr:       p.CronExpr,
		Enabled:        true,
	}, nil
}

// Delete removes a scheduled job by ID.
//
//encore:api auth method=DELETE path=/scheduler/:id
func (s *Service) Delete(ctx context.Context, id string) (*DeleteResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var subID string
	if err := db.QueryRow(ctx, `
		SELECT subscription_id FROM scheduled_jobs WHERE id = $1 AND user_id = $2
	`, id, claims.UserID).Scan(&subID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("scheduled job not found").Err()
	}

	if _, err := db.Exec(ctx, `DELETE FROM scheduled_jobs WHERE id = $1`, id); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("delete failed").Err()
	}

	s.removeCron(subID)
	return &DeleteResponse{OK: true}, nil
}
```

> **Note on TriggerCheck auth context:** Cron jobs run without user context. `TriggerCheck` uses `encauth.Data()` which panics without auth. We need to handle this. Two approaches:
> 1. Store `user_id` in the scheduled_job row and use `et.OverrideAuthInfo` in background context — but that's a test-only API.
> 2. Add an internal (non-auth) trigger endpoint to the checker service.
>
> **Solution:** Add `//encore:api private` endpoint `TriggerCheckInternal` to checker that accepts `(subscriptionID, userID string)` and doesn't require auth. The scheduler calls this instead.

- [ ] **Step 2: Add TriggerCheckInternal to checker service**

In `services/checker/checker.go`, add after `TriggerCheck`:

```go
// TriggerCheckInternal triggers a check job from an internal caller (e.g., scheduler).
// Does not require auth — caller must supply userID directly.
//
//encore:api private method=POST path=/internal/check/:subscriptionID
func TriggerCheckInternal(ctx context.Context, subscriptionID string, p *TriggerInternalParams) (*TriggerResponse, error) {
	// Check for already-running job
	var runningCount int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_jobs
		WHERE subscription_id = $1 AND status = 'running'
	`, subscriptionID).Scan(&runningCount); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if runningCount > 0 {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("a check is already running").Err()
	}

	// Get subscription URL — need to query directly since we have no auth context
	var subURL string
	var subUserID string
	// NOTE: cross-service DB access not allowed; store sub_url in a lookup mechanism.
	// For the internal trigger, caller passes sub_url in params.
	subURL = p.SubURL
	subUserID = p.UserID

	jobID := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO check_jobs (id, subscription_id, user_id, sub_url, status, created_at)
		VALUES ($1, $2, $3, $4, 'queued', $5)
	`, jobID, subscriptionID, subUserID, subURL, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create job").Err()
	}

	go runJob(context.Background(), jobID, subscriptionID, subUserID)
	return &TriggerResponse{JobID: jobID}, nil
}

// TriggerInternalParams is the request body for the internal trigger endpoint.
type TriggerInternalParams struct {
	UserID string `json:"user_id"`
	SubURL string `json:"sub_url"`
}
```

> **Note:** The scheduler needs the subscription's URL to pass to TriggerCheckInternal. It can store `sub_url` in `scheduled_jobs` at creation time (fetched from the subscription service when the schedule is created).

- [ ] **Step 3: Add sub_url column to scheduled_jobs**

Edit `services/scheduler/migrations/1_create_scheduled_jobs.up.sql` to add `sub_url TEXT NOT NULL DEFAULT ''`.

Update `Create` in `scheduler.go`:
1. Call `subsvc.GetSubscription(ctx, p.SubscriptionID)` to get the URL
2. Store `sub.URL` in the row
3. Pass `sub.URL` and `claims.UserID` when cron fires via `TriggerCheckInternal`

Updated `registerCron` signature:
```go
func (s *Service) registerCron(subscriptionID, cronExpr, subURL, userID string) {
    entryID, err := s.cron.AddFunc(cronExpr, func() {
        ctx := context.Background()
        checkersvc.TriggerCheckInternal(ctx, subscriptionID, &checkersvc.TriggerInternalParams{
            UserID: userID,
            SubURL: subURL,
        })
    })
    ...
}
```

And update the startup loader to also read `sub_url` and `user_id` from the DB.

- [ ] **Step 4: Compile check**

```bash
go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 5: Commit**

```bash
git add services/scheduler/ services/checker/checker.go
git commit -m "feat(scheduler): cron scheduler service with lifecycle management"
```

---

## Chunk 2: Notify Service

### Task 4: Notify DB migration

**Files:**
- Create: `services/notify/migrations/1_create_notify_channels.up.sql`
- Create: `services/notify/migrations/1_create_notify_channels.down.sql`

- [ ] **Step 1: Create migration**

```sql
-- services/notify/migrations/1_create_notify_channels.up.sql
CREATE TABLE notify_channels (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name    TEXT NOT NULL DEFAULT '',
    type    TEXT NOT NULL,   -- webhook | telegram
    config  JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notify_channels_user_id ON notify_channels (user_id);
```

```sql
-- services/notify/migrations/1_create_notify_channels.down.sql
DROP TABLE IF EXISTS notify_channels;
```

- [ ] **Step 2: Commit**

```bash
git add services/notify/migrations/
git commit -m "feat(notify): add notify_channels migration"
```

---

### Task 5: Notify service

**Files:**
- Create: `services/notify/notify.go`

Subscribes to `checker.JobCompletedTopic` and sends notifications.
Supports:
- **webhook**: POST JSON payload to configured URL
- **telegram**: send message via Bot API

- [ ] **Step 1: Write notify.go**

```go
// services/notify/notify.go
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"

	authsvc "subs-check-re/services/auth"
	checkersvc "subs-check-re/services/checker"
)

var db = sqldb.NewDatabase("notify", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- PubSub subscriber ---

var _ = pubsub.NewSubscription(
	checkersvc.JobCompletedTopic,
	"notify-on-job-completed",
	pubsub.SubscriptionConfig[*checkersvc.JobCompletedEvent]{
		Handler: handleJobCompleted,
	},
)

func handleJobCompleted(ctx context.Context, event *checkersvc.JobCompletedEvent) error {
	rows, err := db.Query(ctx, `
		SELECT id, type, config FROM notify_channels
		WHERE user_id = $1 AND enabled = true
	`, event.UserID)
	if err != nil {
		return err
	}
	defer rows.Close()

	msg := fmt.Sprintf(
		"✅ Check completed\nSubscription: %s\nAvailable: %d/%d nodes",
		event.SubscriptionID, event.Available, event.Total,
	)

	for rows.Next() {
		var id, chType string
		var configJSON []byte
		if err := rows.Scan(&id, &chType, &configJSON); err != nil {
			continue
		}
		switch chType {
		case "webhook":
			sendWebhook(configJSON, event)
		case "telegram":
			sendTelegram(configJSON, msg)
		}
	}
	return nil
}

// --- Senders ---

type webhookConfig struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

func sendWebhook(configJSON []byte, event *checkersvc.JobCompletedEvent) {
	var cfg webhookConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return
	}
	if cfg.URL == "" {
		return
	}
	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	payload, _ := json.Marshal(event)
	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func sendTelegram(configJSON []byte, message string) {
	var cfg telegramConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"chat_id": cfg.ChatID,
		"text":    message,
	})
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// --- Channel types ---

// Channel represents a notification channel.
type Channel struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
}

// ListChannelsResponse is the response for GET /notify/channels.
type ListChannelsResponse struct {
	Channels []Channel `json:"channels"`
}

// CreateChannelParams is the request body for POST /notify/channels.
type CreateChannelParams struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// DeleteChannelResponse is the response for DELETE /notify/channels/:id.
type DeleteChannelResponse struct {
	OK bool `json:"ok"`
}

// --- API endpoints ---

// ListChannels returns all notification channels for the current user.
//
//encore:api auth method=GET path=/notify/channels
func ListChannels(ctx context.Context) (*ListChannelsResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	rows, err := db.Query(ctx, `
		SELECT id, user_id, name, type, config, enabled, created_at
		FROM notify_channels WHERE user_id = $1 ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Config, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		channels = append(channels, c)
	}
	if channels == nil {
		channels = []Channel{}
	}
	return &ListChannelsResponse{Channels: channels}, nil
}

// CreateChannel adds a new notification channel.
//
//encore:api auth method=POST path=/notify/channels
func CreateChannel(ctx context.Context, p *CreateChannelParams) (*Channel, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if p.Type != "webhook" && p.Type != "telegram" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("type must be webhook or telegram").Err()
	}

	id := uuid.New().String()
	configJSON := p.Config
	if configJSON == nil {
		configJSON = json.RawMessage("{}")
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO notify_channels (id, user_id, name, type, config, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, claims.UserID, p.Name, p.Type, []byte(configJSON), time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create channel").Err()
	}

	return &Channel{
		ID:      id,
		UserID:  claims.UserID,
		Name:    p.Name,
		Type:    p.Type,
		Config:  configJSON,
		Enabled: true,
	}, nil
}

// DeleteChannel removes a notification channel.
//
//encore:api auth method=DELETE path=/notify/channels/:id
func DeleteChannel(ctx context.Context, id string) (*DeleteChannelResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	result, err := db.Exec(ctx, `
		DELETE FROM notify_channels WHERE id = $1 AND user_id = $2
	`, id, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("delete failed").Err()
	}
	if result.RowsAffected() == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("channel not found").Err()
	}
	return &DeleteChannelResponse{OK: true}, nil
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add services/notify/
git commit -m "feat(notify): notification service with webhook and Telegram support"
```

---

### Task 6: Integration tests

**Files:**
- Create: `services/scheduler/scheduler_test.go`
- Create: `services/notify/notify_test.go`

- [ ] **Step 1: Write scheduler_test.go**

```go
// services/scheduler/scheduler_test.go
package scheduler

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestListEmpty(t *testing.T) {
	svc, err := initService()
	if err != nil {
		t.Fatalf("initService failed: %v", err)
	}
	ctx := withAuth()
	resp, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if resp.Jobs == nil {
		t.Error("expected non-nil jobs slice")
	}
}

func TestCreateInvalidCron(t *testing.T) {
	svc, _ := initService()
	ctx := withAuth()
	_, err := svc.Create(ctx, &CreateParams{
		SubscriptionID: "some-id",
		CronExpr:       "not-a-cron",
	})
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}
```

- [ ] **Step 2: Write notify_test.go**

```go
// services/notify/notify_test.go
package notify

import (
	"context"
	"testing"

	"encore.dev/beta/auth"
	"encore.dev/et"

	authsvc "subs-check-re/services/auth"
)

func withAuth() context.Context {
	et.OverrideAuthInfo(auth.UID("test-user-id"), &authsvc.UserClaims{UserID: "test-user-id"})
	return context.Background()
}

func TestCreateAndListChannel(t *testing.T) {
	ctx := withAuth()
	ch, err := CreateChannel(ctx, &CreateChannelParams{
		Name: "My Webhook",
		Type: "webhook",
	})
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if ch.ID == "" {
		t.Error("expected non-empty ID")
	}

	list, err := ListChannels(ctx)
	if err != nil {
		t.Fatalf("ListChannels failed: %v", err)
	}
	if len(list.Channels) == 0 {
		t.Error("expected at least one channel")
	}
}

func TestCreateInvalidType(t *testing.T) {
	ctx := withAuth()
	_, err := CreateChannel(ctx, &CreateChannelParams{
		Name: "Bad",
		Type: "email", // not supported in this implementation
	})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}
```

- [ ] **Step 3: Run all tests**

```bash
encore test ./...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add services/scheduler/scheduler_test.go services/notify/notify_test.go
git commit -m "test(scheduler,notify): integration tests"
```

---

## What's Next

- **Plan 4:** Frontend — React app (TanStack Router, TanStack Query, shadcn/ui), auth flow, subscription management, real-time SSE progress view, node results table
