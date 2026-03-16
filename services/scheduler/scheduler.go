// services/scheduler/scheduler.go
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"
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
		SELECT subscription_id, cron_expr, sub_url, user_id, COALESCE(options_json, '{}')
		FROM scheduled_jobs WHERE enabled = true
	`)
	if err != nil {
		return nil, fmt.Errorf("load scheduled jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subID, cronExpr, subURL, userID string
		var optsJSON []byte
		if err := rows.Scan(&subID, &cronExpr, &subURL, &userID, &optsJSON); err != nil {
			continue
		}
		var opts checkersvc.CheckOptions
		if err := json.Unmarshal(optsJSON, &opts); err != nil {
			opts = defaultCheckOptions()
		}
		svc.registerCron(subID, cronExpr, subURL, userID, opts)
	}

	svc.cron.Start()
	return svc, nil
}

func (s *Service) registerCron(subscriptionID, cronExpr, subURL, userID string, opts checkersvc.CheckOptions) {
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		ctx := context.Background()
		checkersvc.TriggerCheckInternal(ctx, subscriptionID, &checkersvc.TriggerInternalParams{ //nolint
			UserID:  userID,
			SubURL:  subURL,
			Options: opts,
		})
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

func defaultCheckOptions() checkersvc.CheckOptions {
	return checkersvc.CheckOptions{
		SpeedTest: true,
		MediaApps: []string{"openai", "claude", "gemini", "netflix", "youtube", "disney", "tiktok"},
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

	// Register/update in-memory cron
	s.removeCron(p.SubscriptionID)
	s.registerCron(p.SubscriptionID, p.CronExpr, sub.URL, claims.UserID, opts)

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
