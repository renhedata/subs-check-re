ALTER TABLE notify_channels
    ADD COLUMN IF NOT EXISTS platform_alerts JSONB NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS subscription_platform_state (
    subscription_id TEXT        NOT NULL,
    user_id         TEXT        NOT NULL,
    platform        TEXT        NOT NULL,
    available       BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (subscription_id, platform)
);
