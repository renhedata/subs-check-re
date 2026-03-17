CREATE TABLE export_logs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    ip              TEXT NOT NULL DEFAULT '',
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_export_logs_sub ON export_logs (subscription_id, requested_at DESC);
