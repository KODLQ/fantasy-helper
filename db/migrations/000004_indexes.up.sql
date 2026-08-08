CREATE INDEX idx_players_active_research ON players (season_id, position, status, form DESC, total_points DESC);
CREATE INDEX idx_players_team ON players (season_id, team_id);
CREATE INDEX idx_players_price ON players (season_id, price);
CREATE INDEX idx_fixtures_upcoming ON fixtures (season_id, kickoff_time, finished);
CREATE INDEX idx_fixtures_team_home ON fixtures (season_id, team_home_id, kickoff_time);
CREATE INDEX idx_fixtures_team_away ON fixtures (season_id, team_away_id, kickoff_time);
CREATE INDEX idx_player_history_gameweek ON player_gameweek_history (player_id, season_id, gameweek_id);

