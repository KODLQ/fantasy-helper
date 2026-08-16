package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (r *PostgresRepository) LoadResearchSnapshotAtCutoff(ctx context.Context, seasonSourceID, gameweekSourceID int) (ResearchSnapshot, bool, error) {
	var seasonDBID, gameweekDBID int64
	var deadline sql.NullTime
	if err := r.db.QueryRowContext(ctx, `SELECT s.id, g.id, g.deadline_time FROM seasons s JOIN gameweeks g ON g.season_id=s.id WHERE s.source_id=$1 AND g.source_id=$2`, seasonSourceID, gameweekSourceID).Scan(&seasonDBID, &gameweekDBID, &deadline); err != nil {
		if err == sql.ErrNoRows {
			return ResearchSnapshot{}, false, nil
		}
		return ResearchSnapshot{}, false, fmt.Errorf("load research deadline: %w", err)
	}
	if !deadline.Valid {
		return ResearchSnapshot{}, false, nil
	}
	var result ResearchSnapshot
	var state string
	var missingJSON []byte
	if err := r.db.QueryRowContext(ctx, `SELECT id::text, state, source_fetched_at, missing_inputs FROM dataset_snapshots WHERE season_id=$1 AND dataset='public-fpl' AND source_fetched_at IS NOT NULL AND source_fetched_at <= $2 ORDER BY source_fetched_at DESC, normalized_at DESC LIMIT 1`, seasonDBID, deadline.Time).Scan(&result.ID, &state, &result.ObservedAt, &missingJSON); err != nil {
		if err == sql.ErrNoRows {
			return ResearchSnapshot{}, false, nil
		}
		return ResearchSnapshot{}, false, fmt.Errorf("select research snapshot at cutoff: %w", err)
	}
	result.SeasonID, result.Gameweek, result.Deadline, result.State = seasonSourceID, gameweekSourceID, deadline.Time, state
	result.MissingInputs = []string{}
	_ = json.Unmarshal(missingJSON, &result.MissingInputs)
	var snapshot Snapshot
	var found bool
	var err error
	snapshot, found, err = r.LoadSnapshotForSeason(ctx, seasonSourceID)
	if err != nil || !found {
		return ResearchSnapshot{}, false, err
	}
	players, err := r.loadResearchPlayers(ctx, seasonDBID, result.ID)
	if err != nil {
		return ResearchSnapshot{}, false, err
	}
	if len(players) == 0 {
		return ResearchSnapshot{}, false, nil
	}
	snapshot.Players = players
	fixtures, excluded, err := r.loadFixturesAtCutoff(ctx, seasonDBID, deadline.Time)
	if err != nil {
		return ResearchSnapshot{}, false, err
	}
	snapshot.Fixtures = fixtures
	if excluded > 0 {
		result.State = "unavailable"
		result.MissingInputs = append(result.MissingInputs, "fixture_observations_at_deadline")
	}
	result.Snapshot = snapshot
	return result, true, nil
}

func (r *PostgresRepository) loadResearchPlayers(ctx context.Context, seasonDBID int64, snapshotID string) ([]Player, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id, p.first_name, p.second_name, p.web_name, p.position, t.source_id, ps.price, COALESCE(ps.total_points,0), COALESCE(ps.form,0), COALESCE(ps.minutes,0), COALESCE(ps.value,0), ps.status, p.news, ps.chance_of_playing_next_round, p.goals_scored, p.assists, p.clean_sheets, p.bonus, p.saves, p.expected_minutes, p.recent_returns, COALESCE(ps.selected_by_percent,0), ps.selected_by_percent IS NOT NULL FROM player_snapshots ps JOIN players p ON p.id=ps.player_id JOIN teams t ON t.id=ps.team_id WHERE p.season_id=$1 AND ps.snapshot_id=$2::uuid ORDER BY p.source_id`, seasonDBID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("load cutoff players: %w", err)
	}
	defer rows.Close()
	items := []Player{}
	for rows.Next() {
		var item Player
		if err := rows.Scan(&item.ID, &item.FirstName, &item.SecondName, &item.WebName, &item.Position, &item.TeamID, &item.Price, &item.TotalPoints, &item.Form, &item.Minutes, &item.Value, &item.Status, &item.News, &item.ChanceOfPlaying, &item.GoalsScored, &item.Assists, &item.CleanSheets, &item.Bonus, &item.Saves, &item.ExpectedMinutes, &item.RecentReturns, &item.SelectedByPercent, &item.OwnershipKnown); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) loadFixturesAtCutoff(ctx context.Context, seasonDBID int64, deadline time.Time) ([]Fixture, int, error) {
	var excluded int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM fixtures WHERE season_id=$1 AND updated_at>$2`, seasonDBID, deadline).Scan(&excluded); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT f.source_id, COALESCE(g.source_id,0), f.kickoff_time, f.finished, h.source_id, a.source_id, COALESCE(f.team_home_difficulty,0), COALESCE(f.team_away_difficulty,0), f.team_home_score, f.team_away_score FROM fixtures f JOIN teams h ON h.id=f.team_home_id JOIN teams a ON a.id=f.team_away_id LEFT JOIN gameweeks g ON g.id=f.gameweek_id WHERE f.season_id=$1 AND f.updated_at<=$2 ORDER BY f.source_id`, seasonDBID, deadline)
	if err != nil {
		return nil, 0, fmt.Errorf("load cutoff fixtures: %w", err)
	}
	defer rows.Close()
	items := []Fixture{}
	for rows.Next() {
		var item Fixture
		if err := rows.Scan(&item.ID, &item.Gameweek, &item.KickoffTime, &item.Finished, &item.HomeTeam, &item.AwayTeam, &item.HomeDifficulty, &item.AwayDifficulty, &item.HomeScore, &item.AwayScore); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, excluded, rows.Err()
}

func (r *PostgresRepository) SavePlanningScenario(ctx context.Context, userID int64, name string, input TransferSimulationInput, result TransferSimulation) (PlanningScenario, error) {
	if userID <= 0 {
		return PlanningScenario{}, fmt.Errorf("planning scenario requires an authenticated owner")
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return PlanningScenario{}, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return PlanningScenario{}, err
	}
	item := PlanningScenario{Name: name, SimulationID: result.SimulationID, SeasonID: input.SeasonID, Gameweek: input.Gameweek, Result: result}
	err = r.db.QueryRowContext(ctx, `INSERT INTO planning_scenarios (user_id, season_id, gameweek_id, simulation_id, name, input, result) SELECT $1, s.id, g.id, $4, $5, $6, $7 FROM seasons s JOIN gameweeks g ON g.season_id=s.id WHERE s.source_id=$2 AND g.source_id=$3 ON CONFLICT (user_id, simulation_id) DO UPDATE SET name=planning_scenarios.name RETURNING id, created_at`, userID, input.SeasonID, input.Gameweek, result.SimulationID, name, inputJSON, resultJSON).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return PlanningScenario{}, fmt.Errorf("save planning scenario: %w", err)
	}
	return item, nil
}
