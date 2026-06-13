ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_include_dead BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS export_sort TEXT NOT NULL DEFAULT 'speed_desc';
