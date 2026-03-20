ALTER TABLE notify_channels DROP COLUMN IF EXISTS notify_prefs;
ALTER TABLE notify_channels ADD COLUMN on_check_complete BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE notify_channels ADD COLUMN unlock_cron TEXT;
