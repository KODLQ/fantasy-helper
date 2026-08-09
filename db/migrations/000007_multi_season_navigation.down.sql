DROP INDEX IF EXISTS squad_plans_season_idx;
ALTER TABLE squad_plans DROP COLUMN IF EXISTS season_id;

ALTER TABLE players
  DROP COLUMN IF EXISTS recent_returns,
  DROP COLUMN IF EXISTS expected_minutes;

ALTER TABLE dataset_snapshots
  DROP COLUMN IF EXISTS manifest_checksum,
  DROP COLUMN IF EXISTS supported_datasets,
  DROP COLUMN IF EXISTS source_version,
  DROP COLUMN IF EXISTS source_kind;

DROP INDEX IF EXISTS seasons_one_current_idx;
ALTER TABLE seasons
  DROP COLUMN IF EXISTS warnings,
  DROP COLUMN IF EXISTS missing_inputs,
  DROP COLUMN IF EXISTS completeness,
  DROP COLUMN IF EXISTS last_imported_at,
  DROP COLUMN IF EXISTS source_kind;
