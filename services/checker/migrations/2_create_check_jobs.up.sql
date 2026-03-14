CREATE TABLE check_jobs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    sub_url         TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'queued',
    total           INT  NOT NULL DEFAULT 0,
    progress        INT  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_check_jobs_subscription_id ON check_jobs (subscription_id);
CREATE INDEX idx_check_jobs_user_id ON check_jobs (user_id);
