CREATE TABLE seasons (
  id BIGSERIAL PRIMARY KEY,
  source_id INTEGER NOT NULL UNIQUE,
  name TEXT NOT NULL,
  start_date DATE,
  end_date DATE,
  is_current BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE gameweeks (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  deadline_time TIMESTAMPTZ,
  finished BOOLEAN NOT NULL DEFAULT FALSE,
  is_current BOOLEAN NOT NULL DEFAULT FALSE,
  average_score NUMERIC(6,2),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, source_id)
);

CREATE TABLE teams (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  short_name TEXT NOT NULL,
  strength INTEGER,
  strength_overall_home INTEGER,
  strength_overall_away INTEGER,
  strength_attack_home INTEGER,
  strength_attack_away INTEGER,
  strength_defence_home INTEGER,
  strength_defence_away INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, source_id)
);

CREATE TABLE players (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  first_name TEXT NOT NULL,
  second_name TEXT NOT NULL,
  web_name TEXT NOT NULL,
  position SMALLINT NOT NULL,
  team_id BIGINT NOT NULL REFERENCES teams(id),
  price NUMERIC(5,1) NOT NULL,
  total_points INTEGER NOT NULL DEFAULT 0,
  form NUMERIC(6,2) NOT NULL DEFAULT 0,
  minutes INTEGER NOT NULL DEFAULT 0,
  value NUMERIC(8,2) NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'a',
  news TEXT NOT NULL DEFAULT '',
  chance_of_playing_next_round INTEGER,
  goals_scored INTEGER NOT NULL DEFAULT 0,
  assists INTEGER NOT NULL DEFAULT 0,
  clean_sheets INTEGER NOT NULL DEFAULT 0,
  bonus INTEGER NOT NULL DEFAULT 0,
  saves INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, source_id)
);

CREATE TABLE fixtures (
  id BIGSERIAL PRIMARY KEY,
  season_id BIGINT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL,
  gameweek_id BIGINT REFERENCES gameweeks(id),
  kickoff_time TIMESTAMPTZ,
  finished BOOLEAN NOT NULL DEFAULT FALSE,
  team_home_id BIGINT NOT NULL REFERENCES teams(id),
  team_away_id BIGINT NOT NULL REFERENCES teams(id),
  team_home_difficulty INTEGER,
  team_away_difficulty INTEGER,
  team_home_score INTEGER,
  team_away_score INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (season_id, source_id)
);

