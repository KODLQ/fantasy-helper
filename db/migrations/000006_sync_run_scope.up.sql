ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'full';
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS season_source_id INTEGER;
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS gameweek_source_id INTEGER;
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS correlation_id TEXT;

CREATE INDEX IF NOT EXISTS sync_runs_scope_idx ON sync_runs (scope, season_source_id, gameweek_source_id, started_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS sync_runs_active_scope_idx ON sync_runs (scope, COALESCE(season_source_id, 0), COALESCE(gameweek_source_id, 0)) WHERE status = 'running';

COMMENT ON COLUMN sync_runs.scope IS 'Public sync scope: catalog, fixtures, live, player-history, or full.';
COMMENT ON COLUMN sync_runs.correlation_id IS 'Request or scheduler correlation ID; never contains credentials.';
