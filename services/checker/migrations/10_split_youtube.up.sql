ALTER TABLE check_results
  ALTER COLUMN youtube TYPE BOOL USING (youtube != ''),
  ADD COLUMN IF NOT EXISTS youtube_premium BOOL NOT NULL DEFAULT false;
