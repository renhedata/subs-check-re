CREATE TABLE check_results (
    id          TEXT PRIMARY KEY,
    job_id      TEXT NOT NULL,
    node_id     TEXT NOT NULL,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alive       BOOL NOT NULL DEFAULT FALSE,
    latency_ms  INT,
    speed_kbps  INT,
    country     TEXT,
    ip          TEXT,
    openai      BOOL,
    netflix     BOOL,
    youtube     TEXT,
    disney      BOOL,
    claude      BOOL,
    gemini      BOOL,
    tiktok      TEXT
);

CREATE INDEX idx_check_results_job_id ON check_results (job_id);
