CREATE TABLE squad_plans (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL DEFAULT 'My FPL squad',
  budget NUMERIC(5,1) NOT NULL DEFAULT 100.0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE squad_plan_players (
  plan_id BIGINT NOT NULL REFERENCES squad_plans(id) ON DELETE CASCADE,
  player_id BIGINT NOT NULL REFERENCES players(id),
  purchase_price NUMERIC(5,1) NOT NULL,
  PRIMARY KEY (plan_id, player_id)
);

CREATE TABLE squad_lineups (
  plan_id BIGINT PRIMARY KEY REFERENCES squad_plans(id) ON DELETE CASCADE,
  starting_player_ids BIGINT[] NOT NULL DEFAULT '{}',
  bench_player_ids BIGINT[] NOT NULL DEFAULT '{}',
  captain_player_id BIGINT REFERENCES players(id),
  vice_captain_player_id BIGINT REFERENCES players(id),
  formation TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO squad_plans (name) VALUES ('My FPL squad');

