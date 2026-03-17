ALTER TABLE check_results
  ALTER COLUMN tiktok TYPE BOOL USING (tiktok IN ('YES', 'true', 't'));
