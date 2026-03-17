ALTER TABLE check_results
  DROP COLUMN IF EXISTS youtube_premium,
  ALTER COLUMN youtube TYPE TEXT USING (CASE WHEN youtube THEN 'YES' ELSE '' END);
