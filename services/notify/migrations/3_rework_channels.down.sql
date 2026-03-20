ALTER TABLE notify_channels DROP COLUMN IF EXISTS on_check_complete;
ALTER TABLE notify_channels DROP COLUMN IF EXISTS unlock_cron;
ALTER TABLE notify_channels ADD COLUMN notify_prefs JSONB NOT NULL DEFAULT '{}';
