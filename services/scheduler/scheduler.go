// services/scheduler/scheduler.go
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	authsvc "subs-check-re/services/auth"
	checkersvc "subs-check-re/services/checker"
	subsvc "subs-check-re/services/subscription"
)

var db = sqldb.NewDatabase("scheduler", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// --- Service lifecycle ---

//encore:service
type Service struct {
	cron    *cron.Cron
	mu      sync.Mutex              // guards entries (Create/Delete handlers run concurrently)
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
		SELECT subscription_id, cron_expr, COALESCE(options_json, '{}')
		FROM scheduled_jobs WHERE enabled = true
	`)
	if err != nil {
		return nil, fmt.Errorf("load scheduled jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subID, cronExpr string
		var optsJSON []byte
		if err := rows.Scan(&subID, &cronExpr, &optsJSON); err != nil {
			rlog.Error("skipping malformed scheduled job row", "err", err)
			continue
		}
		var opts checkersvc.CheckOptions
		if err := json.Unmarshal(optsJSON, &opts); err != nil {
			opts = defaultCheckOptions()
		}
		svc.registerCron(subID, cronExpr, opts)
	}

	svc.cron.Start()
	return svc, nil
}

func (s *Service) registerCron(subscriptionID, cronExpr string, opts checkersvc.CheckOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[subscriptionID]; ok {
		s.cron.Remove(old)
		delete(s.entries, subscriptionID)
	}
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.runScheduledCheck(subscriptionID, opts)
	})
	if err != nil {
		rlog.Error("failed to register cron entry", "subscription_id", subscriptionID, "cron", cronExpr, "err", err)
		return
	}
	s.entries[subscriptionID] = entryID
}

func (s *Service) removeCron(subscriptionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[subscriptionID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, subscriptionID)
	}
}

// runScheduledCheck fires one scheduled check. It resolves the subscription's
// CURRENT url/owner at fire time (the snapshot in scheduled_jobs goes stale
// when the user edits the subscription) and lazily removes the schedule when
// the subscription has been deleted.
func (s *Service) runScheduledCheck(subscriptionID string, opts checkersvc.CheckOptions) {
	ctx := context.Background()
	sub, err := subsvc.GetSubscriptionByID(ctx, &subsvc.GetByIDParams{ID: subscriptionID})
	if err != nil {
		var e *errs.Error
		if errors.As(err, &e) && e.Code == errs.NotFound {
			rlog.Info("subscription deleted; removing schedule", "subscription_id", subscriptionID)
			s.removeCron(subscriptionID)
			if _, derr := db.Exec(ctx, `DELETE FROM scheduled_jobs WHERE subscription_id = $1`, subscriptionID); derr != nil {
				rlog.Error("failed to delete stale scheduled job", "subscription_id", subscriptionID, "err", derr)
			}
			return
		}
		rlog.Error("scheduled check: subscription lookup failed", "subscription_id", subscriptionID, "err", err)
		return
	}
	if !sub.Enabled {
		return
	}
	if _, err := checkersvc.TriggerCheckInternal(ctx, subscriptionID, &checkersvc.TriggerInternalParams{
		UserID:  sub.UserID,
		SubURL:  sub.URL,
		Options: opts,
	}); err != nil {
		// FailedPrecondition (already running) is expected when runs overlap.
		rlog.Warn("scheduled check trigger failed", "subscription_id", subscriptionID, "err", err)
	}
}

func defaultCheckOptions() checkersvc.CheckOptions {
	return checkersvc.CheckOptions{
		SpeedTest: true,
		MediaApps: []string{"openai", "claude", "gemini", "grok", "netflix", "youtube", "disney", "tiktok"},
	}
}

// --- Types ---

// ScheduledJob represents a cron schedule entry.
type ScheduledJob struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	CronExpr       string    `json:"cron_expr"`
	Enabled        bool      `json:"enabled"`
	SpeedTest      bool      `json:"speed_test"`
	MediaApps      []string  `json:"media_apps"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListResponse is the response for GET /scheduler.
type ListResponse struct {
	Jobs []ScheduledJob `json:"jobs"`
}

// CreateParams is the request body for POST /scheduler.
type CreateParams struct {
	SubscriptionID string                    `json:"subscription_id"`
	CronExpr       string                    `json:"cron_expr"`
	Options        *checkersvc.CheckOptions  `json:"options"`
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
		SELECT id, subscription_id, cron_expr, enabled, created_at, COALESCE(options_json, '{}')
		FROM scheduled_jobs WHERE user_id = $1 ORDER BY created_at DESC
	`, claims.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	defer rows.Close()

	var jobs []ScheduledJob
	for rows.Next() {
		var j ScheduledJob
		var optsJSON []byte
		if err := rows.Scan(&j.ID, &j.SubscriptionID, &j.CronExpr, &j.Enabled, &j.CreatedAt, &optsJSON); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		var opts checkersvc.CheckOptions
		if err := json.Unmarshal(optsJSON, &opts); err != nil {
			opts = defaultCheckOptions()
		}
		j.SpeedTest = opts.SpeedTest
		j.MediaApps = opts.MediaApps
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []ScheduledJob{}
	}
	return &ListResponse{Jobs: jobs}, nil
}

// Create adds or updates a cron schedule for a subscription.
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

	// Get subscription to verify ownership and get URL
	sub, err := subsvc.GetSubscription(ctx, p.SubscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}

	// Resolve options with defaults
	opts := defaultCheckOptions()
	if p.Options != nil {
		opts = *p.Options
	}
	optsJSON, _ := json.Marshal(opts)

	id := uuid.New().String()
	if _, err := db.Exec(ctx, `
		INSERT INTO scheduled_jobs (id, subscription_id, user_id, sub_url, cron_expr, options_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (subscription_id) DO UPDATE SET cron_expr = $5, sub_url = $4, options_json = $6, enabled = true
	`, id, p.SubscriptionID, claims.UserID, sub.URL, p.CronExpr, optsJSON, time.Now()); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create scheduled job").Err()
	}

	// Register/update in-memory cron (registerCron replaces any existing entry)
	s.registerCron(p.SubscriptionID, p.CronExpr, opts)

	return &ScheduledJob{
		ID:             id,
		SubscriptionID: p.SubscriptionID,
		CronExpr:       p.CronExpr,
		Enabled:        true,
		SpeedTest:      opts.SpeedTest,
		MediaApps:      opts.MediaApps,
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

// SetEnabledParams is the request body for PATCH /scheduler/:id.
type SetEnabledParams struct {
	Enabled bool `json:"enabled"`
}

// SetEnabled pauses or resumes a scheduled job without deleting it.
//
//encore:api auth method=PATCH path=/scheduler/:id
func (s *Service) SetEnabled(ctx context.Context, id string, p *SetEnabledParams) (*ScheduledJob, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	var j ScheduledJob
	var optsJSON []byte
	if err := db.QueryRow(ctx, `
		SELECT id, subscription_id, cron_expr, created_at, COALESCE(options_json, '{}')
		FROM scheduled_jobs WHERE id = $1 AND user_id = $2
	`, id, claims.UserID).Scan(&j.ID, &j.SubscriptionID, &j.CronExpr, &j.CreatedAt, &optsJSON); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("scheduled job not found").Err()
	}

	if _, err := db.Exec(ctx,
		`UPDATE scheduled_jobs SET enabled = $2 WHERE id = $1`, id, p.Enabled); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}

	var opts checkersvc.CheckOptions
	if err := json.Unmarshal(optsJSON, &opts); err != nil {
		opts = defaultCheckOptions()
	}

	if p.Enabled {
		s.registerCron(j.SubscriptionID, j.CronExpr, opts)
	} else {
		s.removeCron(j.SubscriptionID)
	}

	j.Enabled = p.Enabled
	j.SpeedTest = opts.SpeedTest
	j.MediaApps = opts.MediaApps
	return &j, nil
}
