ALTER TABLE scheduled_jobs ADD COLUMN IF NOT EXISTS options_json JSONB;
