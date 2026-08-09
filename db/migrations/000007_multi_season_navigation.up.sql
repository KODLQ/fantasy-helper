ALTER TABLE seasons
  ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'official-current'
    CHECK (source_kind IN ('official-current', 'retained-snapshot', 'historical-archive')),
  ADD COLUMN last_imported_at TIMESTAMPTZ,
  ADD COLUMN completeness JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN missing_inputs JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN warnings JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE seasons SET source_kind='retained-snapshot' WHERE NOT is_current;

-- These derived research fields were already written by the warehouse adapter,
-- but older fresh installs did not receive their schema migration.
ALTER TABLE players
  ADD COLUMN IF NOT EXISTS expected_minutes NUMERIC(6,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS recent_returns INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX seasons_one_current_idx ON seasons (is_current) WHERE is_current;

ALTER TABLE dataset_snapshots
  ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'official-current'
    CHECK (source_kind IN ('official-current', 'retained-snapshot', 'historical-archive')),
  ADD COLUMN source_version TEXT NOT NULL DEFAULT 'fpl-public-v1',
  ADD COLUMN supported_datasets JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN manifest_checksum TEXT;

ALTER TABLE squad_plans ADD COLUMN season_id BIGINT REFERENCES seasons(id) ON DELETE CASCADE;
UPDATE squad_plans SET season_id=(SELECT id FROM seasons WHERE is_current ORDER BY updated_at DESC LIMIT 1) WHERE season_id IS NULL;
CREATE UNIQUE INDEX squad_plans_season_idx ON squad_plans (season_id) WHERE season_id IS NOT NULL;

COMMENT ON COLUMN seasons.is_current IS 'Official source currency; never a browser or user selection.';
COMMENT ON COLUMN seasons.source_kind IS 'Origin of this normalized season catalogue.';
COMMENT ON COLUMN dataset_snapshots.manifest_checksum IS 'Checksum of the validated historical archive manifest when applicable.';
