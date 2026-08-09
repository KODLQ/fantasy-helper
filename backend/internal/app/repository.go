package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Repository interface {
	EnsureSchema(context.Context) error
	LoadSnapshot(context.Context) (Snapshot, bool, error)
	LoadSquad(context.Context) (Squad, bool, error)
	SaveSquad(context.Context, Squad) error
	UpsertSnapshot(context.Context, Snapshot) error
	RecordSyncStatus(context.Context, SyncStatus) error
}

type TransactionalRepository interface {
	Repository
	WithTransaction(context.Context, func(*sql.Tx) error) error
}

type DatasetSnapshotRepository interface {
	ListDatasetSnapshots(context.Context, Scope) ([]DatasetSnapshot, error)
}

type SourcePayloadRepository interface {
	RecordSourceObservation(context.Context, SourceObservation) error
}

type PostgresRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPostgresRepository(database *sql.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{db: database, logger: logger}
}

func (r *PostgresRepository) RecordSourceObservation(ctx context.Context, observation SourceObservation) error {
	var payload interface{}
	if len(observation.Payload) > 0 {
		payload = string(observation.Payload)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO source_payloads (endpoint, fetched_at, http_status, checksum, validation_state, schema_version, payload, diagnostic) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, observation.Endpoint, observation.FetchedAt, observation.HTTPStatus, observation.Checksum, observation.ValidationState, observation.SchemaVersion, payload, nullableString(observation.Diagnostic))
	return err
}

func (r *PostgresRepository) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	var applied int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		return fmt.Errorf("check migration state: %w", err)
	}
	if applied == 0 {
		return fmt.Errorf("database has no applied migrations; run db/migrate.sh before starting the backend")
	}
	var missing string
	if err := r.db.QueryRowContext(ctx, `SELECT required.table_name FROM (VALUES ('dataset_snapshots'), ('source_payloads'), ('sync_work_items'), ('player_snapshots'), ('player_gameweek_facts')) AS required(table_name) LEFT JOIN information_schema.tables actual ON actual.table_schema='public' AND actual.table_name=required.table_name WHERE actual.table_name IS NULL ORDER BY required.table_name LIMIT 1`).Scan(&missing); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("check warehouse schema: %w", err)
	}
	return fmt.Errorf("required warehouse table %q is missing; run db/migrate.sh", missing)
}

func (r *PostgresRepository) LoadSnapshot(ctx context.Context) (Snapshot, bool, error) {
	var snapshot Snapshot
	var seasonSourceID int
	if err := r.db.QueryRowContext(ctx, `SELECT source_id, name, is_current, updated_at FROM seasons WHERE is_current ORDER BY updated_at DESC LIMIT 1`).Scan(&seasonSourceID, &snapshot.Season.Name, &snapshot.Season.IsCurrent, &snapshot.Season.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("load current season: %w", err)
	}
	snapshot.Season.ID = seasonSourceID
	seasonDBID, err := r.currentSeasonID(ctx)
	if err != nil {
		return Snapshot{}, false, err
	}
	snapshot.Gameweeks, err = r.loadGameweeks(ctx, seasonDBID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snapshot.Teams, err = r.loadTeams(ctx, seasonDBID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snapshot.Players, err = r.loadPlayers(ctx, seasonDBID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snapshot.Fixtures, err = r.loadFixtures(ctx, seasonDBID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snapshot.Histories, err = r.loadHistories(ctx, seasonDBID)
	if err != nil {
		return Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func (r *PostgresRepository) ListDatasetSnapshots(ctx context.Context, scope Scope) ([]DatasetSnapshot, error) {
	query := `SELECT id::text, dataset, state, s.source_id, COALESCE(g.source_id, 0), source_fetched_at, normalized_at, normalizer_version, missing_inputs FROM dataset_snapshots d JOIN seasons s ON s.id=d.season_id LEFT JOIN gameweeks g ON g.id=d.gameweek_id WHERE ($1=0 OR s.source_id=$1) AND ($2=0 OR g.source_id=$2) AND ($3='' OR d.dataset=$3) ORDER BY d.normalized_at DESC`
	rows, err := r.db.QueryContext(ctx, query, scope.SeasonID, scope.Gameweek, scope.Dataset)
	if err != nil {
		return nil, fmt.Errorf("list dataset snapshots: %w", err)
	}
	defer rows.Close()
	items := []DatasetSnapshot{}
	for rows.Next() {
		var item DatasetSnapshot
		var sourceFetched sql.NullTime
		var missing []byte
		if err := rows.Scan(&item.ID, &item.Dataset, &item.State, &item.SeasonID, &item.Gameweek, &sourceFetched, &item.NormalizedAt, &item.NormalizerVersion, &missing); err != nil {
			return nil, err
		}
		if sourceFetched.Valid {
			item.SourceFetchedAt = sourceFetched.Time
		}
		if len(missing) > 0 {
			_ = json.Unmarshal(missing, &item.MissingInputs)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) currentSeasonID(ctx context.Context) (int64, error) {
	var id int64
	if err := r.db.QueryRowContext(ctx, `SELECT id FROM seasons WHERE is_current ORDER BY updated_at DESC LIMIT 1`).Scan(&id); err != nil {
		return 0, fmt.Errorf("load current season id: %w", err)
	}
	return id, nil
}

func (r *PostgresRepository) loadGameweeks(ctx context.Context, seasonID int64) ([]Gameweek, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT source_id, name, deadline_time, finished, is_current, COALESCE(average_score, 0) FROM gameweeks WHERE season_id=$1 ORDER BY source_id`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Gameweek{}
	for rows.Next() {
		var item Gameweek
		if err := rows.Scan(&item.ID, &item.Name, &item.DeadlineTime, &item.Finished, &item.IsCurrent, &item.AverageScore); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) loadTeams(ctx context.Context, seasonID int64) ([]Team, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT source_id, name, short_name FROM teams WHERE season_id=$1 ORDER BY source_id`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Team{}
	for rows.Next() {
		var item Team
		if err := rows.Scan(&item.ID, &item.Name, &item.ShortName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) loadPlayers(ctx context.Context, seasonID int64) ([]Player, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id, p.first_name, p.second_name, p.web_name, p.position, t.source_id, p.price, p.total_points, p.form, p.minutes, p.value, p.status, p.news, p.chance_of_playing_next_round, p.goals_scored, p.assists, p.clean_sheets, p.bonus, p.saves, p.expected_minutes, p.recent_returns FROM players p JOIN teams t ON t.id=p.team_id WHERE p.season_id=$1 ORDER BY p.source_id`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Player{}
	for rows.Next() {
		var item Player
		if err := rows.Scan(&item.ID, &item.FirstName, &item.SecondName, &item.WebName, &item.Position, &item.TeamID, &item.Price, &item.TotalPoints, &item.Form, &item.Minutes, &item.Value, &item.Status, &item.News, &item.ChanceOfPlaying, &item.GoalsScored, &item.Assists, &item.CleanSheets, &item.Bonus, &item.Saves, &item.ExpectedMinutes, &item.RecentReturns); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) loadFixtures(ctx context.Context, seasonID int64) ([]Fixture, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT f.source_id, COALESCE(g.source_id, 0), f.kickoff_time, f.finished, h.source_id, a.source_id, COALESCE(f.team_home_difficulty, 0), COALESCE(f.team_away_difficulty, 0), f.team_home_score, f.team_away_score FROM fixtures f JOIN teams h ON h.id=f.team_home_id JOIN teams a ON a.id=f.team_away_id LEFT JOIN gameweeks g ON g.id=f.gameweek_id WHERE f.season_id=$1 ORDER BY f.source_id`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Fixture{}
	for rows.Next() {
		var item Fixture
		if err := rows.Scan(&item.ID, &item.Gameweek, &item.KickoffTime, &item.Finished, &item.HomeTeam, &item.AwayTeam, &item.HomeDifficulty, &item.AwayDifficulty, &item.HomeScore, &item.AwayScore); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) loadHistories(ctx context.Context, seasonID int64) (map[int][]PlayerHistory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id, g.source_id, h.minutes, h.total_points, h.goals_scored, h.assists, h.clean_sheets, h.bonus, COALESCE(h.value, 0) FROM player_gameweek_history h JOIN players p ON p.id=h.player_id JOIN gameweeks g ON g.id=h.gameweek_id WHERE h.season_id=$1 ORDER BY p.source_id, g.source_id`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := map[int][]PlayerHistory{}
	for rows.Next() {
		var playerID int
		var item PlayerHistory
		if err := rows.Scan(&playerID, &item.Gameweek, &item.Minutes, &item.TotalPoints, &item.Goals, &item.Assists, &item.CleanSheets, &item.Bonus, &item.Value); err != nil {
			return nil, err
		}
		items[playerID] = append(items[playerID], item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) UpsertSnapshot(ctx context.Context, snapshot Snapshot) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		var seasonID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO seasons (source_id, name, is_current, updated_at) VALUES ($1,$2,$3,NOW()) ON CONFLICT (source_id) DO UPDATE SET name=EXCLUDED.name, is_current=EXCLUDED.is_current, updated_at=NOW() RETURNING id`, snapshot.Season.ID, snapshot.Season.Name, snapshot.Season.IsCurrent).Scan(&seasonID); err != nil {
			return fmt.Errorf("upsert season: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE seasons SET is_current=FALSE WHERE id<>$1`, seasonID); err != nil {
			return err
		}
		weeks := map[int]int64{}
		for _, week := range snapshot.Gameweeks {
			var id int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO gameweeks (season_id, source_id, name, deadline_time, finished, is_current, average_score, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW()) ON CONFLICT (season_id,source_id) DO UPDATE SET name=EXCLUDED.name, deadline_time=EXCLUDED.deadline_time, finished=EXCLUDED.finished, is_current=EXCLUDED.is_current, average_score=EXCLUDED.average_score, updated_at=NOW() RETURNING id`, seasonID, week.ID, week.Name, week.DeadlineTime, week.Finished, week.IsCurrent, week.AverageScore).Scan(&id); err != nil {
				return fmt.Errorf("upsert gameweek %d: %w", week.ID, err)
			}
			weeks[week.ID] = id
		}
		teams := map[int]int64{}
		for _, team := range snapshot.Teams {
			var id int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO teams (season_id, source_id, name, short_name, updated_at) VALUES ($1,$2,$3,$4,NOW()) ON CONFLICT (season_id,source_id) DO UPDATE SET name=EXCLUDED.name, short_name=EXCLUDED.short_name, updated_at=NOW() RETURNING id`, seasonID, team.ID, team.Name, team.ShortName).Scan(&id); err != nil {
				return fmt.Errorf("upsert team %d: %w", team.ID, err)
			}
			teams[team.ID] = id
		}
		players := map[int]int64{}
		for _, player := range snapshot.Players {
			teamID, ok := teams[player.TeamID]
			if !ok {
				return fmt.Errorf("player %d references unknown team %d", player.ID, player.TeamID)
			}
			var id int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO players (season_id, source_id, first_name, second_name, web_name, position, team_id, price, total_points, form, minutes, value, status, news, chance_of_playing_next_round, goals_scored, assists, clean_sheets, bonus, saves, expected_minutes, recent_returns, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,NOW()) ON CONFLICT (season_id,source_id) DO UPDATE SET first_name=EXCLUDED.first_name, second_name=EXCLUDED.second_name, web_name=EXCLUDED.web_name, position=EXCLUDED.position, team_id=EXCLUDED.team_id, price=EXCLUDED.price, total_points=EXCLUDED.total_points, form=EXCLUDED.form, minutes=EXCLUDED.minutes, value=EXCLUDED.value, status=EXCLUDED.status, news=EXCLUDED.news, chance_of_playing_next_round=EXCLUDED.chance_of_playing_next_round, goals_scored=EXCLUDED.goals_scored, assists=EXCLUDED.assists, clean_sheets=EXCLUDED.clean_sheets, bonus=EXCLUDED.bonus, saves=EXCLUDED.saves, expected_minutes=EXCLUDED.expected_minutes, recent_returns=EXCLUDED.recent_returns, updated_at=NOW() RETURNING id`, seasonID, player.ID, player.FirstName, player.SecondName, player.WebName, player.Position, teamID, player.Price, player.TotalPoints, player.Form, player.Minutes, player.Value, player.Status, player.News, player.ChanceOfPlaying, player.GoalsScored, player.Assists, player.CleanSheets, player.Bonus, player.Saves, player.ExpectedMinutes, player.RecentReturns).Scan(&id); err != nil {
				return fmt.Errorf("upsert player %d: %w", player.ID, err)
			}
			players[player.ID] = id
		}
		for _, fixture := range snapshot.Fixtures {
			homeID, homeOK := teams[fixture.HomeTeam]
			awayID, awayOK := teams[fixture.AwayTeam]
			if !homeOK || !awayOK {
				return fmt.Errorf("fixture %d references unknown team", fixture.ID)
			}
			weekID := weeks[fixture.Gameweek]
			if _, err := tx.ExecContext(ctx, `INSERT INTO fixtures (season_id, source_id, gameweek_id, kickoff_time, finished, team_home_id, team_away_id, team_home_difficulty, team_away_difficulty, team_home_score, team_away_score, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW()) ON CONFLICT (season_id,source_id) DO UPDATE SET gameweek_id=EXCLUDED.gameweek_id, kickoff_time=EXCLUDED.kickoff_time, finished=EXCLUDED.finished, team_home_id=EXCLUDED.team_home_id, team_away_id=EXCLUDED.team_away_id, team_home_difficulty=EXCLUDED.team_home_difficulty, team_away_difficulty=EXCLUDED.team_away_difficulty, team_home_score=EXCLUDED.team_home_score, team_away_score=EXCLUDED.team_away_score, updated_at=NOW()`, seasonID, fixture.ID, nullableInt64(weekID), fixture.KickoffTime, fixture.Finished, homeID, awayID, fixture.HomeDifficulty, fixture.AwayDifficulty, fixture.HomeScore, fixture.AwayScore); err != nil {
				return fmt.Errorf("upsert fixture %d: %w", fixture.ID, err)
			}
		}
		for playerID, history := range snapshot.Histories {
			dbPlayerID, ok := players[playerID]
			if !ok {
				continue
			}
			var minutes, goals, assists, cleanSheets, points int
			for _, item := range history {
				minutes += item.Minutes
				goals += item.Goals
				assists += item.Assists
				cleanSheets += item.CleanSheets
				points += item.TotalPoints
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO player_season_history (player_id, season_id, source_id, minutes, goals_scored, assists, clean_sheets, total_points) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (player_id,season_id,source_id) DO UPDATE SET minutes=EXCLUDED.minutes, goals_scored=EXCLUDED.goals_scored, assists=EXCLUDED.assists, clean_sheets=EXCLUDED.clean_sheets, total_points=EXCLUDED.total_points`, dbPlayerID, seasonID, playerID, minutes, goals, assists, cleanSheets, points); err != nil {
				return err
			}
			for _, item := range history {
				weekID, ok := weeks[item.Gameweek]
				if !ok {
					continue
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO player_gameweek_history (player_id, season_id, gameweek_id, minutes, total_points, goals_scored, assists, clean_sheets, bonus, value) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (player_id,gameweek_id) DO UPDATE SET minutes=EXCLUDED.minutes, total_points=EXCLUDED.total_points, goals_scored=EXCLUDED.goals_scored, assists=EXCLUDED.assists, clean_sheets=EXCLUDED.clean_sheets, bonus=EXCLUDED.bonus, value=EXCLUDED.value`, dbPlayerID, seasonID, weekID, item.Minutes, item.TotalPoints, item.Goals, item.Assists, item.CleanSheets, item.Bonus, item.Value); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *PostgresRepository) LoadSquad(ctx context.Context) (Squad, bool, error) {
	var squad Squad
	var planID int64
	if err := r.db.QueryRowContext(ctx, `SELECT id, name, budget FROM squad_plans ORDER BY id LIMIT 1`).Scan(&planID, &squad.Name, &squad.Budget); err != nil {
		if err == sql.ErrNoRows {
			return Squad{}, false, nil
		}
		return Squad{}, false, err
	}
	squad.PurchasePrices = map[int]float64{}
	rows, err := r.db.QueryContext(ctx, `SELECT p.source_id, spp.purchase_price FROM squad_plan_players spp JOIN players p ON p.id=spp.player_id WHERE spp.plan_id=$1`, planID)
	if err != nil {
		return Squad{}, false, err
	}
	for rows.Next() {
		var id int
		var price float64
		if err := rows.Scan(&id, &price); err != nil {
			rows.Close()
			return Squad{}, false, err
		}
		squad.PurchasePrices[id] = price
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Squad{}, false, err
	}
	rows.Close()
	var starts, bench string
	var captain, vice sql.NullInt64
	var formation sql.NullString
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(array_to_string(starting_player_ids, ','), ''), COALESCE(array_to_string(bench_player_ids, ','), ''), captain_player_id, vice_captain_player_id, formation FROM squad_lineups WHERE plan_id=$1`, planID).Scan(&starts, &bench, &captain, &vice, &formation); err == nil {
		squad.StartingPlayerIDs = r.sourceIDsFromDBIDs(ctx, parseIntList(starts))
		squad.BenchPlayerIDs = r.sourceIDsFromDBIDs(ctx, parseIntList(bench))
		if captain.Valid {
			squad.CaptainID = r.sourceIDFromDBID(ctx, captain.Int64)
		}
		if vice.Valid {
			squad.ViceCaptainID = r.sourceIDFromDBID(ctx, vice.Int64)
		}
		if formation.Valid {
			squad.Formation = formation.String
		}
	}
	return squad, true, nil
}

func (r *PostgresRepository) SaveSquad(ctx context.Context, squad Squad) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		var planID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM squad_plans ORDER BY id LIMIT 1`).Scan(&planID); err != nil {
			if err != sql.ErrNoRows {
				return err
			}
			if _, insertErr := tx.ExecContext(ctx, `INSERT INTO squad_plans (name, budget) VALUES ($1,$2)`, squad.Name, squad.Budget); insertErr != nil {
				return insertErr
			}
			if queryErr := tx.QueryRowContext(ctx, `SELECT id FROM squad_plans ORDER BY id LIMIT 1`).Scan(&planID); queryErr != nil {
				return queryErr
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE squad_plans SET name=$1, budget=$2, updated_at=NOW() WHERE id=$3`, squad.Name, squad.Budget, planID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM squad_plan_players WHERE plan_id=$1`, planID); err != nil {
			return err
		}
		ids := make([]int, 0, len(squad.PurchasePrices))
		for id := range squad.PurchasePrices {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, sourceID := range ids {
			dbID, err := r.resolvePlayerID(ctx, tx, sourceID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO squad_plan_players (plan_id, player_id, purchase_price) VALUES ($1,$2,$3)`, planID, dbID, squad.PurchasePrices[sourceID]); err != nil {
				return err
			}
		}
		starts, err := r.resolvePlayerIDs(ctx, tx, squad.StartingPlayerIDs)
		if err != nil {
			return err
		}
		bench, err := r.resolvePlayerIDs(ctx, tx, squad.BenchPlayerIDs)
		if err != nil {
			return err
		}
		captain, err := r.resolveOptionalPlayerID(ctx, tx, squad.CaptainID)
		if err != nil {
			return err
		}
		vice, err := r.resolveOptionalPlayerID(ctx, tx, squad.ViceCaptainID)
		if err != nil {
			return err
		}
		query := `INSERT INTO squad_lineups (plan_id, starting_player_ids, bench_player_ids, captain_player_id, vice_captain_player_id, formation, updated_at) VALUES ($1,` + postgresArray(starts) + `,` + postgresArray(bench) + `,$2,$3,$4,NOW()) ON CONFLICT (plan_id) DO UPDATE SET starting_player_ids=EXCLUDED.starting_player_ids, bench_player_ids=EXCLUDED.bench_player_ids, captain_player_id=EXCLUDED.captain_player_id, vice_captain_player_id=EXCLUDED.vice_captain_player_id, formation=EXCLUDED.formation, updated_at=NOW()`
		_, err = tx.ExecContext(ctx, query, planID, nullableInt64(captain), nullableInt64(vice), nullableString(squad.Formation))
		return err
	})
}

func (r *PostgresRepository) resolvePlayerID(ctx context.Context, tx *sql.Tx, sourceID int) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT p.id FROM players p JOIN seasons s ON s.id=p.season_id WHERE p.source_id=$1 AND s.is_current ORDER BY s.updated_at DESC LIMIT 1`, sourceID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("resolve player %d: %w", sourceID, err)
	}
	return id, nil
}

func (r *PostgresRepository) resolvePlayerIDs(ctx context.Context, tx *sql.Tx, sourceIDs []int) ([]int64, error) {
	ids := make([]int64, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		id, err := r.resolvePlayerID(ctx, tx, sourceID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *PostgresRepository) resolveOptionalPlayerID(ctx context.Context, tx *sql.Tx, sourceID int) (int64, error) {
	if sourceID == 0 {
		return 0, nil
	}
	return r.resolvePlayerID(ctx, tx, sourceID)
}

func (r *PostgresRepository) sourceIDsFromDBIDs(ctx context.Context, ids []int) []int {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.sourceIDFromDBID(ctx, int64(id)))
	}
	return result
}

func (r *PostgresRepository) sourceIDFromDBID(ctx context.Context, id int64) int {
	var sourceID int
	if err := r.db.QueryRowContext(ctx, `SELECT source_id FROM players WHERE id=$1`, id).Scan(&sourceID); err != nil {
		return 0
	}
	return sourceID
}

func (r *PostgresRepository) RecordSyncStatus(ctx context.Context, status SyncStatus) error {
	statusValue := status.Status
	if statusValue == "empty" || statusValue == "unavailable" {
		return nil
	}
	checksum := ""
	if !status.Freshness.SnapshotAt.IsZero() {
		checksum = status.Freshness.SnapshotAt.UTC().Format(time.RFC3339Nano)
	}
	var runID int64
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `INSERT INTO sync_runs (status, started_at, finished_at, warning, checksum) VALUES ($1,$2,$3,$4,$5) RETURNING id`, statusValue, nullableTime(status.StartedAt), nullableTime(status.FinishedAt), nullableString(status.Warning), nullableString(checksum)).Scan(&runID); err != nil {
			return err
		}
		for _, stage := range append(append([]string{}, status.CompletedStages...), status.FailedStages...) {
			stageStatus := "success"
			if strings.HasPrefix(stage, "player-history:") || contains(status.FailedStages, stage) {
				stageStatus = "failed"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO sync_stages (sync_run_id, stage, status, error, finished_at) VALUES ($1,$2,$3,$4,$5)`, runID, stage, stageStatus, nullableString(status.Warning), nullableTime(status.FinishedAt)); err != nil {
				return err
			}
		}
		if status.Warning != "" {
			payload, _ := json.Marshal(map[string]string{"warning": status.Warning})
			_, err := tx.ExecContext(ctx, `INSERT INTO sync_diagnostics (sync_run_id, endpoint, checksum, error, payload) VALUES ($1,$2,$3,$4,$5)`, runID, "sync", checksum, status.Warning, payload)
			return err
		}
		return nil
	})
	if err != nil && r.logger != nil {
		r.logger.Error("persist sync status failed", "error", err)
	}
	return err
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
func nullableInt64(value int64) interface{} {
	if value == 0 {
		return nil
	}
	return value
}
func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
func nullableTime(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value
}
func parseIntList(value string) []int {
	if value == "" {
		return []int{}
	}
	parts := strings.Split(value, ",")
	result := []int{}
	for _, part := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			result = append(result, id)
		}
	}
	return result
}
func postgresArray(values []int64) string {
	if len(values) == 0 {
		return "ARRAY[]::bigint[]"
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.FormatInt(value, 10)
	}
	return "ARRAY[" + strings.Join(parts, ",") + "]::bigint[]"
}
func postgresArrayJSON(values []int64) []byte { value, _ := json.Marshal(values); return value }
