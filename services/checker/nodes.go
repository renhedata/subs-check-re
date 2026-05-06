// services/checker/nodes.go
package checker

import (
	"context"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
)

// SetNodeEnabledParams is the request body for PATCH /nodes/:nodeID.
type SetNodeEnabledParams struct {
	Enabled bool `json:"enabled"`
}

// SetNodeEnabled enables or disables a single node for the authenticated user.
// Disabled nodes are excluded from subscription exports but still appear in results.
//
//encore:api auth method=PATCH path=/nodes/:nodeID
func SetNodeEnabled(ctx context.Context, nodeID string, p *SetNodeEnabledParams) error {
	claims := encauth.Data().(*authsvc.UserClaims)

	res, err := db.Exec(ctx, `
		UPDATE nodes SET enabled = $3
		WHERE id = $1
		  AND subscription_id IN (
		      SELECT DISTINCT subscription_id FROM check_jobs WHERE user_id = $2
		  )
	`, nodeID, claims.UserID, p.Enabled)
	if err != nil {
		return errs.B().Code(errs.Internal).Msg("db error").Err()
	}
	if res.RowsAffected() == 0 {
		return errs.B().Code(errs.NotFound).Msg("node not found").Err()
	}
	return nil
}
