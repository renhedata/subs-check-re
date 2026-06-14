ALTER TABLE check_results
  ADD COLUMN netflix boolean NOT NULL DEFAULT false,
  ADD COLUMN youtube boolean NOT NULL DEFAULT false,
  ADD COLUMN youtube_premium boolean NOT NULL DEFAULT false,
  ADD COLUMN openai boolean NOT NULL DEFAULT false,
  ADD COLUMN claude boolean NOT NULL DEFAULT false,
  ADD COLUMN gemini boolean NOT NULL DEFAULT false,
  ADD COLUMN grok boolean NOT NULL DEFAULT false,
  ADD COLUMN disney boolean NOT NULL DEFAULT false,
  ADD COLUMN tiktok boolean NOT NULL DEFAULT false,
  ADD COLUMN extra_platforms jsonb NOT NULL DEFAULT '{}';

UPDATE check_results SET
  netflix         = COALESCE((platforms->'netflix'->>'unlocked')::boolean, false),
  youtube         = COALESCE((platforms->'youtube'->>'unlocked')::boolean, false),
  youtube_premium = COALESCE((platforms->'youtube_premium'->>'unlocked')::boolean, false),
  openai          = COALESCE((platforms->'openai'->>'unlocked')::boolean, false),
  claude          = COALESCE((platforms->'claude'->>'unlocked')::boolean, false),
  gemini          = COALESCE((platforms->'gemini'->>'unlocked')::boolean, false),
  grok            = COALESCE((platforms->'grok'->>'unlocked')::boolean, false),
  disney          = COALESCE((platforms->'disney'->>'unlocked')::boolean, false),
  tiktok          = COALESCE((platforms->'tiktok'->>'unlocked')::boolean, false);

ALTER TABLE check_results DROP COLUMN platforms;
