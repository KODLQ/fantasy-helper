CREATE TABLE planning_scenarios (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  gameweek_id BIGINT NOT NULL REFERENCES gameweeks(id) ON DELETE RESTRICT,
  simulation_id TEXT NOT NULL,
  name TEXT NOT NULL,
  input JSONB NOT NULL,
  result JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, simulation_id)
);

CREATE INDEX planning_scenarios_owner_created_idx ON planning_scenarios (user_id, created_at DESC);

COMMENT ON TABLE planning_scenarios IS 'User-owned immutable copies of explicitly confirmed transfer simulations.';
