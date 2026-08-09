DROP INDEX IF EXISTS sync_runs_active_scope_idx;
DROP INDEX IF EXISTS sync_runs_scope_idx;
ALTER TABLE sync_runs DROP COLUMN IF EXISTS correlation_id;
ALTER TABLE sync_runs DROP COLUMN IF EXISTS gameweek_source_id;
ALTER TABLE sync_runs DROP COLUMN IF EXISTS season_source_id;
ALTER TABLE sync_runs DROP COLUMN IF EXISTS scope;
