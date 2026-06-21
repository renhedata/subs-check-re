// services/subscription/subscription.go
package subscription

import (
	"context"
	"encoding/json"
	"strings"
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
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	Name              string     `json:"name"`
	URL               string     `json:"url"`
	Enabled           bool       `json:"enabled"`
	CronExpr          *string    `json:"cron_expr"`
	CreatedAt         time.Time  `json:"created_at"`
	LastRunAt         *time.Time `json:"last_run_at"`
	ExportIncludeDead bool       `json:"export_include_dead"`
	ExportSort        string     `json:"export_sort"`
	// FetchProxyConfig, when non-empty, is the JSON of a node's proxy config
	// used to tunnel the subscription fetch through that node (for URLs the
	// server can't reach directly). Empty means fetch directly. Stored as
	// jsonb, surfaced as a JSON string so it crosses the Encore API boundary.
	FetchProxyConfig string `json:"fetch_proxy_config,omitempty"`
}

// normalizeExportSort guards the export_sort enum, defaulting unknown values.
func normalizeExportSort(s string) string {
	if s == "latency_asc" {
		return "latency_asc"
	}
	return "speed_desc"
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
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at, export_include_dead, export_sort, COALESCE(fetch_proxy_config::text, '')
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
			&s.CronExpr, &s.CreatedAt, &s.LastRunAt, &s.ExportIncludeDead, &s.ExportSort, &s.FetchProxyConfig); err != nil {
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
	Name              string  `json:"name"`
	URL               string  `json:"url"`
	CronExpr          *string `json:"cron_expr"`
	ExportIncludeDead bool    `json:"export_include_dead"`
	ExportSort        string  `json:"export_sort"`
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
	sort := normalizeExportSort(p.ExportSort)
	_, err := db.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, name, url, cron_expr, created_at, export_include_dead, export_sort)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, uid, p.Name, p.URL, p.CronExpr, time.Now(), p.ExportIncludeDead, sort)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create subscription").Err()
	}
	return &Subscription{
		ID:                id,
		UserID:            uid,
		Name:              p.Name,
		URL:               p.URL,
		Enabled:           true,
		CronExpr:          p.CronExpr,
		ExportIncludeDead: p.ExportIncludeDead,
		ExportSort:        sort,
	}, nil
}

// UpdateParams is the request body for PUT /subscriptions/:id.
// Use ClearCronExpr=true to remove the cron schedule (set cron_expr to NULL).
type UpdateParams struct {
	Name              *string `json:"name"`
	URL               *string `json:"url"`
	Enabled           *bool   `json:"enabled"`
	CronExpr          *string `json:"cron_expr"`
	ClearCronExpr     bool    `json:"clear_cron_expr"`
	ExportIncludeDead *bool   `json:"export_include_dead"`
	ExportSort        *string `json:"export_sort"`
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

	var exportSortSQL any
	if p.ExportSort != nil {
		exportSortSQL = normalizeExportSort(*p.ExportSort)
	}

	_, err := db.Exec(ctx, `
		UPDATE subscriptions SET
			name                = COALESCE($2, name),
			url                 = COALESCE($3, url),
			enabled             = COALESCE($4, enabled),
			cron_expr           = CASE WHEN $6::boolean THEN NULL ELSE COALESCE($5, cron_expr) END,
			export_include_dead = COALESCE($7, export_include_dead),
			export_sort         = COALESCE($8, export_sort)
		WHERE id = $1
	`, id, p.Name, p.URL, p.Enabled, cronExprSQL, p.ClearCronExpr, p.ExportIncludeDead, exportSortSQL)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}

	var s Subscription
	if err := db.QueryRow(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at, export_include_dead, export_sort, COALESCE(fetch_proxy_config::text, '')
		FROM subscriptions WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled, &s.CronExpr, &s.CreatedAt, &s.LastRunAt, &s.ExportIncludeDead, &s.ExportSort, &s.FetchProxyConfig); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("fetch after update failed").Err()
	}
	return &s, nil
}

// SetFetchProxyParams sets (or clears) the node used to tunnel subscription
// fetches. Config is the chosen node's proxy config as JSON; an empty string
// clears it (fetch directly).
type SetFetchProxyParams struct {
	Config string `json:"config"`
}

// SetFetchProxy stores the chosen node's proxy config on the subscription so
// refresh / test-fetch tunnel the download through that node. Pass an empty
// config to go back to a direct fetch.
//
//encore:api auth method=PUT path=/subscriptions/:id/fetch-proxy
func SetFetchProxy(ctx context.Context, id string, p *SetFetchProxyParams) (*Subscription, error) {
	uid := encauth.Data().(*authsvc.UserClaims).UserID
	var ownerID string
	if err := db.QueryRow(ctx, `SELECT user_id FROM subscriptions WHERE id = $1`, id).Scan(&ownerID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	if ownerID != uid {
		return nil, errs.B().Code(errs.PermissionDenied).Msg("access denied").Err()
	}

	var cfgSQL any
	if trimmed := strings.TrimSpace(p.Config); trimmed != "" {
		var probe map[string]any
		if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
			return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid node config JSON").Err()
		}
		cfgSQL = trimmed
	}
	if _, err := db.Exec(ctx,
		`UPDATE subscriptions SET fetch_proxy_config = $2::jsonb WHERE id = $1`, id, cfgSQL); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("update failed").Err()
	}
	return GetSubscription(ctx, id)
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

// GetSubscription returns a subscription by ID, verifying ownership.
// Used internally by the checker service.
//
//encore:api auth method=GET path=/subscriptions/:id
func GetSubscription(ctx context.Context, id string) (*Subscription, error) {
	uid := encauth.Data().(*authsvc.UserClaims).UserID
	var s Subscription
	err := db.QueryRow(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at, export_include_dead, export_sort, COALESCE(fetch_proxy_config::text, '')
		FROM subscriptions WHERE id = $1 AND user_id = $2
	`, id, uid).Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled, &s.CronExpr, &s.CreatedAt, &s.LastRunAt, &s.ExportIncludeDead, &s.ExportSort, &s.FetchProxyConfig)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	return &s, nil
}

// GetByIDParams is the request for the internal subscription lookup endpoint.
type GetByIDParams struct {
	ID string `json:"id"`
}

// GetSubscriptionByID returns a subscription by ID without an auth context.
// Internal-only: used by the scheduler at cron-fire time to resolve the
// subscription's current URL and owner (the stored sub_url snapshot in
// scheduled_jobs goes stale when the user edits the subscription).
//
//encore:api private method=POST path=/internal/subscriptions/get
func GetSubscriptionByID(ctx context.Context, p *GetByIDParams) (*Subscription, error) {
	var s Subscription
	err := db.QueryRow(ctx, `
		SELECT id, user_id, name, url, enabled, cron_expr, created_at, last_run_at, export_include_dead, export_sort, COALESCE(fetch_proxy_config::text, '')
		FROM subscriptions WHERE id = $1
	`, p.ID).Scan(&s.ID, &s.UserID, &s.Name, &s.URL, &s.Enabled, &s.CronExpr, &s.CreatedAt, &s.LastRunAt, &s.ExportIncludeDead, &s.ExportSort, &s.FetchProxyConfig)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	return &s, nil
}

// GetSubscriptionNamesParams is the request for the internal name-lookup endpoint.
type GetSubscriptionNamesParams struct {
	UserID string   `json:"user_id"`
	IDs    []string `json:"ids"`
}

// GetSubscriptionNamesResponse maps subscription ID → name.
type GetSubscriptionNamesResponse struct {
	Names map[string]string `json:"names"`
}

// GetSubscriptionNames resolves subscription names by ID for internal service calls.
//
//encore:api private method=POST path=/internal/subscriptions/names
func GetSubscriptionNames(ctx context.Context, p *GetSubscriptionNamesParams) (*GetSubscriptionNamesResponse, error) {
	if len(p.IDs) == 0 {
		return &GetSubscriptionNamesResponse{Names: map[string]string{}}, nil
	}
	rows, err := db.Query(ctx, `
		SELECT id, name FROM subscriptions
		WHERE id = ANY($1) AND user_id = $2
	`, p.IDs, p.UserID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("db query failed").Err()
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("scan failed").Err()
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("rows iteration failed").Err()
	}
	return &GetSubscriptionNamesResponse{Names: names}, nil
}
