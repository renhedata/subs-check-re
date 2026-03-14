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
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	Enabled   bool       `json:"enabled"`
	CronExpr  *string    `json:"cron_expr"`
	CreatedAt time.Time  `json:"created_at"`
	LastRunAt *time.Time `json:"last_run_at"`
}

// ListResponse is the response for GET /subscriptions.
type ListResponse struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// List returns all subscriptions for the current user.
//
//encore:api auth method=GET path=/subscriptions
func List(ctx context.Context) (*ListResponse, error) {
	uid := encauth.Data().(*authsvc.UserClaims).UserID
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
	uid := encauth.Data().(*authsvc.UserClaims).UserID
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
	Name          *string `json:"name"`
	URL           *string `json:"url"`
	Enabled       *bool   `json:"enabled"`
	CronExpr      *string `json:"cron_expr"`
	ClearCronExpr bool    `json:"clear_cron_expr"`
}

// Update modifies a subscription owned by the current user.
//
//encore:api auth method=PUT path=/subscriptions/:id
func Update(ctx context.Context, id string, p *UpdateParams) (*Subscription, error) {
	uid := encauth.Data().(*authsvc.UserClaims).UserID
	var ownerID string
	if err := db.QueryRow(ctx, `SELECT user_id FROM subscriptions WHERE id = $1`, id).Scan(&ownerID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	if ownerID != uid {
		return nil, errs.B().Code(errs.PermissionDenied).Msg("access denied").Err()
	}

	var cronExprSQL any
	if p.ClearCronExpr {
		cronExprSQL = nil
	} else {
		cronExprSQL = p.CronExpr
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
	uid := encauth.Data().(*authsvc.UserClaims).UserID
	result, err := db.Exec(ctx, `
		DELETE FROM subscriptions WHERE id = $1 AND user_id = $2
	`, id, uid)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("delete failed").Err()
	}
	n := result.RowsAffected()
	if n == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	return &DeleteResponse{OK: true}, nil
}
