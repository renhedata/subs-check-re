CREATE TABLE scheduled_jobs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    sub_url         TEXT NOT NULL DEFAULT '',
    cron_expr       TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_scheduled_jobs_subscription_id ON scheduled_jobs (subscription_id);
