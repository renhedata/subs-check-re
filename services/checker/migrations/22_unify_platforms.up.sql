ALTER TABLE check_results ADD COLUMN platforms jsonb NOT NULL DEFAULT '{}';

-- Backfill builtin bool columns + extra_platforms into the unified map.
UPDATE check_results SET platforms = (
  SELECT COALESCE(jsonb_object_agg(key, value), '{}'::jsonb)
  FROM (
    SELECT k AS key,
           jsonb_build_object('unlocked', v, 'status', CASE WHEN v THEN 'Yes' ELSE 'No' END, 'region', '') AS value
    FROM (VALUES
      ('netflix', netflix), ('youtube', youtube), ('youtube_premium', youtube_premium),
      ('openai', openai), ('claude', claude), ('gemini', gemini), ('grok', grok),
      ('disney', disney), ('tiktok', tiktok)
    ) AS b(k, v)
    UNION ALL
    SELECT ep.key,
           jsonb_build_object('unlocked', (ep.value)::boolean,
                              'status', CASE WHEN (ep.value)::boolean THEN 'Yes' ELSE 'No' END,
                              'region', '')
    FROM jsonb_each(CASE WHEN jsonb_typeof(extra_platforms) = 'object' THEN extra_platforms ELSE '{}'::jsonb END) AS ep
  ) merged
);

ALTER TABLE check_results
  DROP COLUMN netflix, DROP COLUMN youtube, DROP COLUMN youtube_premium,
  DROP COLUMN openai, DROP COLUMN claude, DROP COLUMN gemini, DROP COLUMN grok,
  DROP COLUMN disney, DROP COLUMN tiktok, DROP COLUMN extra_platforms;
