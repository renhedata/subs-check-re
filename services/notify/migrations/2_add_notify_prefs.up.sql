ALTER TABLE notify_channels
    ADD COLUMN notify_prefs JSONB NOT NULL DEFAULT '{}';
