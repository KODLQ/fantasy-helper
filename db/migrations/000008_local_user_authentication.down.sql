BEGIN;

DROP INDEX IF EXISTS squad_plans_user_season_key;

-- The pre-authentication schema permits only one plan per season. Rolling back
-- from multiple user-owned plans is necessarily lossy, so retain the oldest.
DELETE FROM squad_plans candidate
USING squad_plans keeper
WHERE candidate.season_id = keeper.season_id
  AND candidate.id > keeper.id;

ALTER TABLE squad_plans DROP COLUMN IF EXISTS user_id;
CREATE UNIQUE INDEX IF NOT EXISTS squad_plans_season_idx ON squad_plans (season_id) WHERE season_id IS NOT NULL;
DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

COMMIT;
