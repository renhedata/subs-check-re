// services/checker/service.go
package checker

import (
	"context"

	"encore.dev/rlog"
)

//encore:service
type Service struct{}

// initService runs once at process startup. Check jobs execute as in-process
// goroutines (see jobrunner.go), so any job still queued/running at startup
// belongs to a dead process and can never finish — mark it failed so the
// one-active-job index doesn't block the subscription forever.
// NOTE: this assumes a single checker instance (see docs/adr/0001).
func initService() (*Service, error) {
	n, err := recoverOrphanedJobs(context.Background())
	if err != nil {
		return nil, err
	}
	if n > 0 {
		rlog.Info("recovered orphaned check jobs from previous process", "count", n)
	}
	return &Service{}, nil
}

// recoverOrphanedJobs marks all queued/running jobs as failed and returns how
// many rows were affected.
func recoverOrphanedJobs(ctx context.Context) (int, error) {
	res, err := db.Exec(ctx, `
		UPDATE check_jobs SET status = 'failed', finished_at = NOW()
		WHERE status IN ('queued', 'running')
	`)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}
