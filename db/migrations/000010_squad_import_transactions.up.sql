CREATE TABLE squad_import_drafts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  snapshot_id BIGINT NOT NULL REFERENCES active_team_snapshots(id) ON DELETE RESTRICT,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  squad JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, snapshot_id)
);

ALTER TABLE squad_import_events ALTER COLUMN plan_id DROP NOT NULL;
ALTER TABLE squad_import_events ADD COLUMN draft_id BIGINT REFERENCES squad_import_drafts(id) ON DELETE CASCADE;
ALTER TABLE squad_import_events ADD CONSTRAINT squad_import_event_target_check CHECK (
  (mode='draft' AND draft_id IS NOT NULL AND plan_id IS NULL) OR
  (mode='replace' AND plan_id IS NOT NULL AND draft_id IS NULL)
);
