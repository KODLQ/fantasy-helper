CREATE TABLE manager_sync_scopes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  scope_type TEXT NOT NULL CHECK (scope_type IN ('entry','league')),
  source_id INTEGER NOT NULL CHECK (source_id > 0),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  member_limit INTEGER NOT NULL DEFAULT 50 CHECK (member_limit BETWEEN 1 AND 200),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, scope_type, source_id)
);

CREATE TABLE manager_connections (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  entry_source_id INTEGER NOT NULL CHECK (entry_source_id > 0),
  provider_type TEXT NOT NULL CHECK (provider_type IN ('memory','environment','os-keychain')),
  provider_reference TEXT,
  state TEXT NOT NULL DEFAULT 'disconnected' CHECK (state IN ('disconnected','validating','connected','reauth_required','permission_denied','revoked')),
  last_validated_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, entry_source_id)
);
COMMENT ON TABLE manager_connections IS 'Redacted FPL connection metadata only. Raw cookies, tokens, and passwords are forbidden.';

CREATE TABLE manager_entries (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  player_first_name TEXT NOT NULL DEFAULT '',
  player_last_name TEXT NOT NULL DEFAULT '',
  entry_name TEXT NOT NULL DEFAULT '',
  started_gameweek INTEGER,
  normalized_at TIMESTAMPTZ NOT NULL,
  UNIQUE (user_id, season_id, source_id)
);

CREATE TABLE manager_season_summaries (
  id BIGSERIAL PRIMARY KEY,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  overall_points INTEGER NOT NULL DEFAULT 0,
  overall_rank INTEGER,
  value INTEGER NOT NULL DEFAULT 0,
  bank INTEGER NOT NULL DEFAULT 0,
  transfers INTEGER NOT NULL DEFAULT 0,
  source_endpoint TEXT NOT NULL,
  source_checksum CHAR(64) NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL,
  normalized_at TIMESTAMPTZ NOT NULL,
  normalization_version TEXT NOT NULL DEFAULT 'fpl-manager-v1',
  conflict_state TEXT NOT NULL DEFAULT 'none',
  UNIQUE (entry_id, source_checksum)
);

CREATE TABLE manager_gameweek_summaries (
  id BIGSERIAL PRIMARY KEY,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  points INTEGER NOT NULL DEFAULT 0,
  rank INTEGER,
  overall_rank INTEGER,
  bank INTEGER NOT NULL DEFAULT 0,
  value INTEGER NOT NULL DEFAULT 0,
  transfers INTEGER NOT NULL DEFAULT 0,
  transfer_cost INTEGER NOT NULL DEFAULT 0,
  bench_points INTEGER NOT NULL DEFAULT 0,
  source_checksum CHAR(64) NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL,
  normalized_at TIMESTAMPTZ NOT NULL,
  normalization_version TEXT NOT NULL DEFAULT 'fpl-manager-v1',
  finalized BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE (entry_id, gameweek_id, source_checksum)
);

CREATE TABLE manager_pick_snapshots (
  id BIGSERIAL PRIMARY KEY,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  source_endpoint TEXT NOT NULL,
  source_checksum CHAR(64) NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL,
  normalized_at TIMESTAMPTZ NOT NULL,
  normalization_version TEXT NOT NULL DEFAULT 'fpl-manager-v1',
  state TEXT NOT NULL DEFAULT 'complete',
  missing_inputs TEXT[] NOT NULL DEFAULT '{}',
  conflict_state TEXT NOT NULL DEFAULT 'none',
  UNIQUE (entry_id, gameweek_id, source_checksum)
);

CREATE TABLE manager_picks (
  snapshot_id BIGINT NOT NULL REFERENCES manager_pick_snapshots(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id),
  position SMALLINT NOT NULL,
  multiplier SMALLINT NOT NULL,
  is_captain BOOLEAN NOT NULL DEFAULT FALSE,
  is_vice_captain BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (snapshot_id, player_id)
);

CREATE TABLE manager_automatic_substitutions (
  snapshot_id BIGINT NOT NULL REFERENCES manager_pick_snapshots(id) ON DELETE CASCADE,
  player_in_id BIGINT NOT NULL REFERENCES players(id),
  player_out_id BIGINT NOT NULL REFERENCES players(id),
  PRIMARY KEY (snapshot_id, player_in_id, player_out_id)
);

CREATE TABLE manager_chips (
  id BIGSERIAL PRIMARY KEY,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  gameweek_id BIGINT REFERENCES gameweeks(id) ON DELETE CASCADE,
  chip_name TEXT NOT NULL,
  played_at TIMESTAMPTZ,
  UNIQUE (entry_id, chip_name, gameweek_id)
);

CREATE TABLE manager_transfers (
  id BIGSERIAL PRIMARY KEY,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  player_in_id BIGINT NOT NULL REFERENCES players(id),
  player_out_id BIGINT NOT NULL REFERENCES players(id),
  player_in_cost INTEGER NOT NULL,
  player_out_cost INTEGER NOT NULL,
  transfer_cost INTEGER NOT NULL DEFAULT 0,
  made_at TIMESTAMPTZ NOT NULL,
  source_checksum CHAR(64) NOT NULL,
  UNIQUE (entry_id, made_at, player_in_id, player_out_id)
);

CREATE TABLE active_team_snapshots (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  entry_id BIGINT NOT NULL REFERENCES manager_entries(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  bank INTEGER NOT NULL DEFAULT 0,
  team_value INTEGER NOT NULL DEFAULT 0,
  active_chip TEXT,
  source_endpoint_set TEXT[] NOT NULL DEFAULT '{}',
  source_checksum CHAR(64) NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL,
  normalized_at TIMESTAMPTZ NOT NULL,
  state TEXT NOT NULL DEFAULT 'complete',
  missing_inputs TEXT[] NOT NULL DEFAULT '{}',
  conflict_state TEXT NOT NULL DEFAULT 'none',
  UNIQUE (user_id, entry_id, gameweek_id, source_checksum)
);

CREATE TABLE active_team_snapshot_players (
  snapshot_id BIGINT NOT NULL REFERENCES active_team_snapshots(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id),
  position SMALLINT NOT NULL,
  multiplier SMALLINT NOT NULL,
  purchase_price INTEGER NOT NULL,
  selling_price INTEGER NOT NULL,
  is_captain BOOLEAN NOT NULL DEFAULT FALSE,
  is_vice_captain BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (snapshot_id, player_id)
);

CREATE TABLE classic_leagues (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  closed BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, season_id, source_id)
);

CREATE TABLE league_standing_snapshots (
  id BIGSERIAL PRIMARY KEY,
  league_id BIGINT NOT NULL REFERENCES classic_leagues(id) ON DELETE CASCADE,
  gameweek_id BIGINT REFERENCES gameweeks(id) ON DELETE CASCADE,
  phase INTEGER NOT NULL DEFAULT 1,
  page INTEGER NOT NULL,
  has_next BOOLEAN NOT NULL DEFAULT FALSE,
  source_checksum CHAR(64) NOT NULL,
  source_fetched_at TIMESTAMPTZ NOT NULL,
  normalized_at TIMESTAMPTZ NOT NULL,
  state TEXT NOT NULL DEFAULT 'complete',
  UNIQUE (league_id, gameweek_id, phase, page, source_checksum)
);

CREATE TABLE league_standing_members (
  snapshot_id BIGINT NOT NULL REFERENCES league_standing_snapshots(id) ON DELETE CASCADE,
  entry_source_id INTEGER NOT NULL,
  entry_name TEXT NOT NULL,
  player_name TEXT NOT NULL,
  rank INTEGER NOT NULL,
  last_rank INTEGER,
  total_points INTEGER NOT NULL,
  PRIMARY KEY (snapshot_id, entry_source_id)
);

CREATE TABLE league_member_pick_links (
  league_id BIGINT NOT NULL REFERENCES classic_leagues(id) ON DELETE CASCADE,
  entry_source_id INTEGER NOT NULL,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE CASCADE,
  pick_snapshot_id BIGINT REFERENCES manager_pick_snapshots(id) ON DELETE SET NULL,
  state TEXT NOT NULL DEFAULT 'pending',
  last_error TEXT,
  PRIMARY KEY (league_id, entry_source_id, gameweek_id)
);

CREATE TABLE manager_sync_runs (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'running',
  correlation_id TEXT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  warning TEXT
);

CREATE TABLE manager_sync_work_items (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT NOT NULL REFERENCES manager_sync_runs(id) ON DELETE CASCADE,
  natural_key TEXT NOT NULL,
  stage TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  checkpoint JSONB NOT NULL DEFAULT '{}'::JSONB,
  last_error TEXT,
  UNIQUE (run_id, natural_key)
);

CREATE TABLE squad_import_events (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  snapshot_id BIGINT NOT NULL REFERENCES active_team_snapshots(id) ON DELETE RESTRICT,
  plan_id BIGINT NOT NULL REFERENCES squad_plans(id) ON DELETE CASCADE,
  mode TEXT NOT NULL CHECK (mode IN ('draft','replace')),
  resulting_version BIGINT NOT NULL,
  confirmed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, snapshot_id, mode)
);

CREATE INDEX manager_entries_owner_scope_idx ON manager_entries (user_id, season_id, source_id);
CREATE INDEX manager_pick_scope_idx ON manager_pick_snapshots (entry_id, gameweek_id, normalized_at DESC);
CREATE INDEX active_team_owner_scope_idx ON active_team_snapshots (user_id, entry_id, gameweek_id, normalized_at DESC);
CREATE INDEX league_snapshot_scope_idx ON league_standing_snapshots (league_id, gameweek_id, phase, page, normalized_at DESC);
CREATE INDEX manager_work_claim_idx ON manager_sync_work_items (run_id, state, available_at);
