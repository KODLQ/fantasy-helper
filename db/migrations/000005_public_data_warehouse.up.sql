-- Public FPL warehouse extensions. Natural keys are season-scoped because
-- upstream source IDs are not guaranteed to be globally stable.
CREATE TABLE dataset_snapshots (
  id UUID PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  gameweek_id BIGINT REFERENCES gameweeks(id) ON DELETE CASCADE,
  dataset TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('actual', 'provisional', 'estimated', 'partial', 'stale', 'unavailable')),
  source_fetched_at TIMESTAMPTZ,
  normalized_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  normalizer_version TEXT NOT NULL,
  completeness JSONB NOT NULL DEFAULT '{}'::jsonb,
  missing_inputs JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_checksums JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (season_id, gameweek_id, dataset, normalized_at)
);

CREATE TABLE source_payloads (
  id BIGSERIAL PRIMARY KEY,
  snapshot_id UUID REFERENCES dataset_snapshots(id) ON DELETE SET NULL,
  endpoint TEXT NOT NULL,
  request_params JSONB NOT NULL DEFAULT '{}'::jsonb,
  fetched_at TIMESTAMPTZ NOT NULL,
  http_status INTEGER NOT NULL,
  checksum TEXT NOT NULL,
  validation_state TEXT NOT NULL CHECK (validation_state IN ('valid', 'invalid', 'partial')),
  schema_version TEXT NOT NULL,
  payload JSONB,
  diagnostic TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX source_payloads_endpoint_fetched_idx ON source_payloads (endpoint, fetched_at DESC);
CREATE INDEX source_payloads_snapshot_idx ON source_payloads (snapshot_id);

CREATE TABLE sync_work_items (
  id BIGSERIAL PRIMARY KEY,
  sync_run_id BIGINT NOT NULL REFERENCES sync_runs(id) ON DELETE CASCADE,
  scope TEXT NOT NULL,
  natural_key TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  season_source_id INTEGER,
  gameweek_source_id INTEGER,
  entity_source_id INTEGER,
  status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'success', 'retryable', 'failed', 'cancelled')),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  claimed_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  last_error TEXT,
  UNIQUE (sync_run_id, natural_key)
);

CREATE INDEX sync_work_items_claim_idx ON sync_work_items (status, available_at);
CREATE INDEX sync_work_items_scope_idx ON sync_work_items (scope, season_source_id, gameweek_source_id);

CREATE TABLE season_phases (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  start_event INTEGER,
  stop_event INTEGER,
  UNIQUE (season_id, source_id)
);

CREATE TABLE game_settings (
  season_id BIGINT PRIMARY KEY REFERENCES seasons(id) ON DELETE CASCADE,
  settings JSONB NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE element_types (
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  singular_name TEXT NOT NULL,
  plural_name TEXT NOT NULL,
  squad_select INTEGER,
  squad_min_select INTEGER,
  squad_max_select INTEGER,
  UNIQUE (season_id, source_id)
);

CREATE TABLE season_source_identities (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  entity_type TEXT NOT NULL,
  source_id INTEGER NOT NULL,
  source_key TEXT,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, entity_type, source_id)
);

CREATE TABLE player_snapshots (
  id BIGSERIAL PRIMARY KEY,
  snapshot_id UUID NOT NULL REFERENCES dataset_snapshots(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  team_id BIGINT NOT NULL REFERENCES teams(id),
  observed_at TIMESTAMPTZ NOT NULL,
  price NUMERIC(5,1) NOT NULL,
  status TEXT NOT NULL,
  chance_of_playing_next_round INTEGER,
  selected_by_percent NUMERIC(8,3),
  form NUMERIC(6,2),
  total_points INTEGER,
  minutes INTEGER,
  value NUMERIC(8,2),
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (snapshot_id, player_id)
);

CREATE INDEX player_snapshots_player_time_idx ON player_snapshots (player_id, observed_at DESC);

CREATE TABLE player_gameweek_facts (
  id BIGSERIAL PRIMARY KEY,
  snapshot_id UUID NOT NULL REFERENCES dataset_snapshots(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  source_observed_at TIMESTAMPTZ NOT NULL,
  finalized BOOLEAN NOT NULL DEFAULT FALSE,
  minutes INTEGER NOT NULL DEFAULT 0,
  total_points INTEGER NOT NULL DEFAULT 0,
  goals_scored INTEGER NOT NULL DEFAULT 0,
  assists INTEGER NOT NULL DEFAULT 0,
  clean_sheets INTEGER NOT NULL DEFAULT 0,
  bonus INTEGER NOT NULL DEFAULT 0,
  bps INTEGER,
  saves INTEGER,
  yellow_cards INTEGER,
  red_cards INTEGER,
  own_goals INTEGER,
  penalties_saved INTEGER,
  penalties_missed INTEGER,
  expected_goals NUMERIC(8,4),
  expected_assists NUMERIC(8,4),
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (snapshot_id, player_id, gameweek_id)
);

CREATE INDEX player_gameweek_facts_query_idx ON player_gameweek_facts (player_id, gameweek_id, finalized);

CREATE TABLE fixture_stats (
  id BIGSERIAL PRIMARY KEY,
  fixture_id BIGINT NOT NULL REFERENCES fixtures(id) ON DELETE CASCADE,
  player_id BIGINT REFERENCES players(id) ON DELETE CASCADE,
  stat_type TEXT NOT NULL,
  stat_value NUMERIC(12,4),
  source_observed_at TIMESTAMPTZ NOT NULL,
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (fixture_id, player_id, stat_type, source_observed_at)
);

CREATE TABLE player_future_fixtures (
  id BIGSERIAL PRIMARY KEY,
  snapshot_id UUID NOT NULL REFERENCES dataset_snapshots(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  fixture_id BIGINT REFERENCES fixtures(id) ON DELETE SET NULL,
  source_fixture_id INTEGER NOT NULL,
  event_source_id INTEGER,
  opponent_source_id INTEGER,
  is_home BOOLEAN,
  difficulty INTEGER,
  kickoff_time TIMESTAMPTZ,
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (snapshot_id, player_id, source_fixture_id)
);

COMMENT ON TABLE dataset_snapshots IS 'Point-in-time analytical dataset identity; all historical reads must select a snapshot.';
COMMENT ON TABLE source_payloads IS 'Raw public FPL observations retained for replay, validation, and diagnostics.';
COMMENT ON TABLE sync_work_items IS 'Durable endpoint/entity work queue; natural_key makes retries idempotent.';
COMMENT ON TABLE player_snapshots IS 'Time-varying player price, team, availability, ownership, and research values.';
COMMENT ON TABLE player_gameweek_facts IS 'Live or finalized player performance facts scoped to a season gameweek snapshot.';
COMMENT ON TABLE fixture_stats IS 'Fixture-level source statistics retained independently from fixture identity.';
COMMENT ON TABLE player_future_fixtures IS 'Player-oriented upcoming fixture context from element-summary.';
