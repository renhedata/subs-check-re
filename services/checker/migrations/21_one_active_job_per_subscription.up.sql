-- Any job still queued/running predates this migration's process and its
-- goroutine is gone — mark failed so the unique index below can be created
-- and the subscription is unblocked.
UPDATE check_jobs SET status = 'failed', finished_at = NOW()
WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX idx_check_jobs_one_active
ON check_jobs (subscription_id)
WHERE status IN ('queued', 'running');
