package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errNoSyncWork = errors.New("no sync work available")

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
	CreateDatasetSnapshot(context.Context, DatasetSnapshot) error
}

type ResearchReadRepository interface {
	SearchPlayers(context.Context, PlayerQuery) ([]Player, int, error)
	LoadPlayerDetail(context.Context, int) (PlayerDetail, bool, error)
}

type DatasetFreshnessRepository interface {
	CurrentDatasetFreshness(context.Context, Scope) (Freshness, error)
}

type SyncStatusRepository interface {
	LoadLatestSyncStatus(context.Context) (SyncStatus, error)
	RetrySyncRun(context.Context, int64) (SyncStatus, error)
}

type SyncStageRepository interface {
	RecordSyncStage(context.Context, SyncStage) error
}

type WarehouseFactRepository interface {
	UpsertFixtureStats(context.Context, int, time.Time, []SourceFixture) error
	UpsertLiveGameweek(context.Context, string, int, int, bool, time.Time, []LivePlayerStats) error
	LiveGameweekFactsUnchanged(context.Context, int, int, []LivePlayerStats) (bool, error)
}

type SourcePayloadRepository interface {
	RecordSourceObservation(context.Context, SourceObservation) error
}

type SyncWorkRepository interface {
	StartSyncRun(context.Context, Scope, string) (int64, error)
	FinishSyncRun(context.Context, int64, SyncStatus) error
	EnqueueSyncWork(context.Context, int64, []SyncWorkItem) error
	ClaimSyncWork(context.Context, int64) (SyncWorkItem, bool, error)
	CompleteSyncWork(context.Context, int64) error
	FailSyncWork(context.Context, int64, error, bool) error
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

func (r *PostgresRepository) StartSyncRun(ctx context.Context, scope Scope, correlationID string) (int64, error) {
	var id int64
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		// A process restart can leave a run and its claimed work marked running.
		// Requeue only claims that have exceeded the lease window, then close the
		// abandoned run so the scope lock cannot prevent recovery forever.
		if _, err := tx.ExecContext(ctx, `UPDATE sync_work_items SET status='pending', available_at=NOW(), claimed_at=NULL WHERE status='running' AND claimed_at < NOW() - INTERVAL '10 minutes' AND sync_run_id IN (SELECT id FROM sync_runs WHERE status='running' AND started_at < NOW() - INTERVAL '10 minutes')`); err != nil {
			return fmt.Errorf("recover abandoned sync work: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sync_runs SET status='partial', finished_at=NOW(), warning=COALESCE(warning, 'Sync run was recovered after its worker lease expired.') WHERE status='running' AND started_at < NOW() - INTERVAL '10 minutes'`); err != nil {
			return fmt.Errorf("recover abandoned sync runs: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO sync_runs (status, scope, season_source_id, gameweek_source_id, correlation_id) VALUES ('running',$1,$2,$3,$4) RETURNING id`, scope.Dataset, nullableInt(scope.SeasonID), nullableInt(scope.Gameweek), nullableString(correlationID)).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sync_work_items SET sync_run_id=$1 WHERE sync_run_id=(SELECT id FROM sync_runs WHERE status='partial' AND scope=$2 AND COALESCE(season_source_id,0)=COALESCE($3,0) AND COALESCE(gameweek_source_id,0)=COALESCE($4,0) ORDER BY started_at DESC, id DESC LIMIT 1) AND status IN ('pending','retryable')`, id, scope.Dataset, nullableInt(scope.SeasonID), nullableInt(scope.Gameweek)); err != nil {
			return fmt.Errorf("resume pending sync work: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("start sync run: %w", err)
	}
	return id, nil
}

func (r *PostgresRepository) FinishSyncRun(ctx context.Context, runID int64, status SyncStatus) error {
	checksum := nullableString(status.Checksum)
	_, err := r.db.ExecContext(ctx, `UPDATE sync_runs SET status=$1, finished_at=$2, warning=$3, checksum=$4 WHERE id=$5`, status.Status, nullableTime(status.FinishedAt), nullableString(status.Warning), checksum, runID)
	if err != nil {
		return fmt.Errorf("finish sync run: %w", err)
	}
	return nil
}

func (r *PostgresRepository) RecordSyncStage(ctx context.Context, stage SyncStage) error {
	if stage.RunID <= 0 || stage.Name == "" || stage.Status == "" {
		return fmt.Errorf("sync stage requires run ID, name, and status")
	}
	if stage.StartedAt.IsZero() {
		stage.StartedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `UPDATE sync_stages SET status=$1, processed_count=$2, failed_count=$3, error=$4, finished_at=$5 WHERE sync_run_id=$6 AND stage=$7`, stage.Status, stage.ProcessedCount, stage.FailedCount, nullableString(stage.Error), nullableTime(stage.FinishedAt), stage.RunID, stage.Name)
	if err != nil {
		return fmt.Errorf("update sync stage %s: %w", stage.Name, err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO sync_stages (sync_run_id, stage, status, processed_count, failed_count, error, started_at, finished_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, stage.RunID, stage.Name, stage.Status, stage.ProcessedCount, stage.FailedCount, nullableString(stage.Error), stage.StartedAt, nullableTime(stage.FinishedAt))
	if err != nil {
		return fmt.Errorf("insert sync stage %s: %w", stage.Name, err)
	}
	return nil
}

func (r *PostgresRepository) UpsertFixtureStats(ctx context.Context, seasonSourceID int, observedAt time.Time, fixtures []SourceFixture) error {
	if seasonSourceID <= 0 || observedAt.IsZero() {
		return fmt.Errorf("fixture statistics require season identity and observation time")
	}
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		for _, fixture := range fixtures {
			for _, statistic := range fixture.Stats {
				values := append([]SourceStatValue{}, statistic.Home...)
				values = append(values, statistic.Away...)
				for _, value := range values {
					raw, err := json.Marshal(value)
					if err != nil {
						return fmt.Errorf("encode fixture %d statistic %s: %w", fixture.ID, statistic.Identifier, err)
					}
					result, err := tx.ExecContext(ctx, `INSERT INTO fixture_stats (fixture_id, player_id, stat_type, stat_value, source_observed_at, raw) SELECT f.id, p.id, $3, $4, $5, $6 FROM fixtures f JOIN seasons s ON s.id=f.season_id JOIN players p ON p.season_id=s.id AND p.source_id=$7 WHERE s.source_id=$1 AND f.source_id=$2 ON CONFLICT (fixture_id, player_id, stat_type, source_observed_at) DO UPDATE SET stat_value=EXCLUDED.stat_value, raw=EXCLUDED.raw`, seasonSourceID, fixture.ID, statistic.Identifier, value.Value, observedAt, raw, value.Element)
					if err != nil {
						return fmt.Errorf("upsert fixture %d statistic %s for player %d: %w", fixture.ID, statistic.Identifier, value.Element, err)
					}
					if affected, err := result.RowsAffected(); err != nil || affected != 1 {
						if err != nil {
							return err
						}
						return fmt.Errorf("fixture %d statistic %s references unknown player %d", fixture.ID, statistic.Identifier, value.Element)
					}
				}
			}
		}
		return nil
	})
}

func (r *PostgresRepository) UpsertLiveGameweek(ctx context.Context, snapshotID string, seasonSourceID, gameweekSourceID int, finalized bool, observedAt time.Time, players []LivePlayerStats) error {
	if snapshotID == "" || seasonSourceID <= 0 || gameweekSourceID <= 0 || observedAt.IsZero() {
		return fmt.Errorf("live gameweek facts require snapshot, season, gameweek, and observation time")
	}
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		for _, player := range players {
			raw, err := json.Marshal(player)
			if err != nil {
				return fmt.Errorf("encode live facts for player %d: %w", player.PlayerID, err)
			}
			result, err := tx.ExecContext(ctx, `INSERT INTO player_gameweek_facts (snapshot_id, player_id, gameweek_id, source_observed_at, finalized, minutes, total_points, goals_scored, assists, clean_sheets, bonus, bps, saves, yellow_cards, red_cards, own_goals, penalties_saved, penalties_missed, expected_goals, expected_assists, raw) SELECT $1, p.id, g.id, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22 FROM seasons s JOIN players p ON p.season_id=s.id AND p.source_id=$4 JOIN gameweeks g ON g.season_id=s.id AND g.source_id=$3 WHERE s.source_id=$2 ON CONFLICT (snapshot_id, player_id, gameweek_id) DO UPDATE SET source_observed_at=EXCLUDED.source_observed_at, finalized=EXCLUDED.finalized, minutes=EXCLUDED.minutes, total_points=EXCLUDED.total_points, goals_scored=EXCLUDED.goals_scored, assists=EXCLUDED.assists, clean_sheets=EXCLUDED.clean_sheets, bonus=EXCLUDED.bonus, bps=EXCLUDED.bps, saves=EXCLUDED.saves, yellow_cards=EXCLUDED.yellow_cards, red_cards=EXCLUDED.red_cards, own_goals=EXCLUDED.own_goals, penalties_saved=EXCLUDED.penalties_saved, penalties_missed=EXCLUDED.penalties_missed, expected_goals=EXCLUDED.expected_goals, expected_assists=EXCLUDED.expected_assists, raw=EXCLUDED.raw`, snapshotID, seasonSourceID, gameweekSourceID, player.PlayerID, observedAt, finalized, player.Minutes, player.Points, player.Goals, player.Assists, player.CleanSheets, player.Bonus, player.BPS, player.Saves, player.YellowCards, player.RedCards, player.OwnGoals, player.PenaltiesSaved, player.PenaltiesMissed, parseFloat(player.ExpectedGoals), parseFloat(player.ExpectedAssists), raw)
			if err != nil {
				return fmt.Errorf("upsert live facts for player %d: %w", player.PlayerID, err)
			}
			if affected, err := result.RowsAffected(); err != nil || affected != 1 {
				if err != nil {
					return err
				}
				return fmt.Errorf("live facts reference unknown player %d or gameweek %d", player.PlayerID, gameweekSourceID)
			}
		}
		return nil
	})
}

func (r *PostgresRepository) LiveGameweekFactsUnchanged(ctx context.Context, seasonSourceID, gameweekSourceID int, incoming []LivePlayerStats) (bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT f.raw FROM player_gameweek_facts f JOIN dataset_snapshots d ON d.id=f.snapshot_id JOIN seasons s ON s.id=d.season_id JOIN gameweeks g ON g.id=f.gameweek_id WHERE s.source_id=$1 AND g.source_id=$2 AND d.id=(SELECT d2.id FROM dataset_snapshots d2 JOIN seasons s2 ON s2.id=d2.season_id JOIN gameweeks g2 ON g2.id=d2.gameweek_id WHERE s2.source_id=$1 AND g2.source_id=$2 ORDER BY d2.normalized_at DESC, d2.id DESC LIMIT 1) ORDER BY f.player_id`, seasonSourceID, gameweekSourceID)
	if err != nil {
		return false, fmt.Errorf("load prior live gameweek facts: %w", err)
	}
	defer rows.Close()
	prior := map[int]LivePlayerStats{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return false, err
		}
		var player LivePlayerStats
		if err := json.Unmarshal(raw, &player); err != nil {
			return false, fmt.Errorf("decode prior live gameweek fact: %w", err)
		}
		prior[player.PlayerID] = player
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(prior) == 0 || len(prior) != len(incoming) {
		return false, nil
	}
	for _, player := range incoming {
		if previous, ok := prior[player.PlayerID]; !ok || previous != player {
			return false, nil
		}
	}
	return true, nil
}

func (r *PostgresRepository) LoadLatestSyncStatus(ctx context.Context) (SyncStatus, error) {
	var status SyncStatus
	var scope, warning, checksum, correlationID sql.NullString
	var seasonID, gameweek sql.NullInt64
	var finished sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT id, status, scope, season_source_id, gameweek_source_id, started_at, finished_at, warning, checksum, correlation_id FROM sync_runs ORDER BY started_at DESC, id DESC LIMIT 1`).Scan(&status.RunID, &status.Status, &scope, &seasonID, &gameweek, &status.StartedAt, &finished, &warning, &checksum, &correlationID)
	if err == sql.ErrNoRows {
		return SyncStatus{Status: "empty", Freshness: Freshness{Status: "unavailable", State: "unavailable"}}, nil
	}
	if err != nil {
		return SyncStatus{}, fmt.Errorf("load latest sync status: %w", err)
	}
	status.Scope.Dataset = scope.String
	if seasonID.Valid {
		status.Scope.SeasonID = int(seasonID.Int64)
	}
	if gameweek.Valid {
		status.Scope.Gameweek = int(gameweek.Int64)
	}
	if finished.Valid {
		status.FinishedAt = finished.Time
	}
	if warning.Valid {
		status.Warning = warning.String
	}
	if checksum.Valid {
		status.Checksum = checksum.String
	}
	if correlationID.Valid {
		status.CorrelationID = correlationID.String
	}
	stageRows, err := r.db.QueryContext(ctx, `SELECT stage, status FROM sync_stages WHERE sync_run_id=$1 ORDER BY id`, status.RunID)
	if err != nil {
		return SyncStatus{}, err
	}
	for stageRows.Next() {
		var stage, stageStatus string
		if err := stageRows.Scan(&stage, &stageStatus); err != nil {
			stageRows.Close()
			return SyncStatus{}, err
		}
		if stageStatus == "success" {
			status.CompletedStages = append(status.CompletedStages, stage)
		} else if stageStatus == "running" {
			status.CurrentStage = stage
		} else {
			status.FailedStages = append(status.FailedStages, stage)
		}
	}
	stageRows.Close()
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status='success'), COUNT(*) FILTER (WHERE status='failed'), COUNT(*) FILTER (WHERE status='retryable') FROM sync_work_items WHERE sync_run_id=$1`, status.RunID).Scan(&status.TotalWork, &status.CompletedWork, &status.FailedWork, &status.RetryableWork); err != nil {
		return SyncStatus{}, err
	}
	freshness, err := r.CurrentDatasetFreshness(ctx, status.Scope)
	if err != nil {
		return SyncStatus{}, err
	}
	status.Freshness = freshness
	return status, nil
}

func (r *PostgresRepository) RetrySyncRun(ctx context.Context, runID int64) (SyncStatus, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE sync_work_items SET status='pending', available_at=NOW(), last_error=NULL, claimed_at=NULL, completed_at=NULL WHERE sync_run_id=$1 AND status IN ('failed','retryable')`, runID); err != nil {
		return SyncStatus{}, fmt.Errorf("retry sync work: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE sync_runs SET status='running', finished_at=NULL, warning=NULL WHERE id=$1 AND status IN ('failed','partial')`, runID); err != nil {
		return SyncStatus{}, fmt.Errorf("retry sync run: %w", err)
	}
	return r.LoadLatestSyncStatus(ctx)
}

func (r *PostgresRepository) EnqueueSyncWork(ctx context.Context, runID int64, items []SyncWorkItem) error {
	return r.WithTransaction(ctx, func(tx *sql.Tx) error {
		for _, item := range items {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sync_work_items (sync_run_id, scope, natural_key, endpoint, season_source_id, gameweek_source_id, entity_source_id, status, attempts, available_at) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',0,NOW()) ON CONFLICT (sync_run_id, natural_key) DO NOTHING`, runID, item.Scope, item.NaturalKey, item.Endpoint, nullableInt(item.SeasonSourceID), nullableInt(item.GameweekSourceID), nullableInt(item.EntitySourceID)); err != nil {
				return fmt.Errorf("enqueue sync work %s: %w", item.NaturalKey, err)
			}
		}
		return nil
	})
}

func (r *PostgresRepository) ClaimSyncWork(ctx context.Context, runID int64) (SyncWorkItem, bool, error) {
	var item SyncWorkItem
	err := r.WithTransaction(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `SELECT id, scope, natural_key, endpoint, COALESCE(season_source_id,0), COALESCE(gameweek_source_id,0), COALESCE(entity_source_id,0), attempts, available_at, COALESCE(last_error,'') FROM sync_work_items WHERE sync_run_id=$1 AND status IN ('pending','retryable') AND available_at <= NOW() ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1`, runID)
		if err := row.Scan(&item.ID, &item.Scope, &item.NaturalKey, &item.Endpoint, &item.SeasonSourceID, &item.GameweekSourceID, &item.EntitySourceID, &item.Attempts, &item.AvailableAt, &item.LastError); err != nil {
			if err == sql.ErrNoRows {
				return errNoSyncWork
			}
			return err
		}
		item.RunID = runID
		item.Status = "running"
		item.Attempts++
		_, err := tx.ExecContext(ctx, `UPDATE sync_work_items SET status='running', attempts=$1, claimed_at=NOW() WHERE id=$2`, item.Attempts, item.ID)
		return err
	})
	if err == errNoSyncWork {
		return SyncWorkItem{}, false, nil
	}
	if err != nil {
		return SyncWorkItem{}, false, fmt.Errorf("claim sync work: %w", err)
	}
	return item, true, nil
}

func (r *PostgresRepository) CompleteSyncWork(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sync_work_items SET status='success', completed_at=NOW(), last_error=NULL WHERE id=$1`, id)
	return err
}

func (r *PostgresRepository) FailSyncWork(ctx context.Context, id int64, failure error, retryable bool) error {
	status := "failed"
	if retryable {
		status = "retryable"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE sync_work_items SET status=$1, last_error=$2, available_at=CASE WHEN $1='retryable' THEN NOW() + LEAST((attempts * INTERVAL '5 seconds'), INTERVAL '5 minutes') ELSE available_at END WHERE id=$3`, status, failure.Error(), id)
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
	query := `SELECT d.id::text, d.dataset, d.state, s.source_id, COALESCE(g.source_id, 0), d.source_fetched_at, d.normalized_at, d.normalizer_version, d.missing_inputs FROM dataset_snapshots d JOIN seasons s ON s.id=d.season_id LEFT JOIN gameweeks g ON g.id=d.gameweek_id WHERE ($1=0 OR s.source_id=$1) AND ($2=0 OR g.source_id=$2) AND ($3='' OR d.dataset=$3) ORDER BY d.normalized_at DESC`
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

func (r *PostgresRepository) CreateDatasetSnapshot(ctx context.Context, item DatasetSnapshot) error {
	if item.ID == "" || item.Dataset == "" || item.SeasonID == 0 {
		return fmt.Errorf("dataset snapshot requires id, dataset, and season ID")
	}
	if item.State == "" {
		item.State = "provisional"
	}
	if item.NormalizerVersion == "" {
		item.NormalizerVersion = "fpl-public-v1"
	}
	if item.NormalizedAt.IsZero() {
		item.NormalizedAt = time.Now().UTC()
	}
	missing, err := json.Marshal(item.MissingInputs)
	if err != nil {
		return fmt.Errorf("encode snapshot missing inputs: %w", err)
	}
	var sourceFetched interface{}
	if !item.SourceFetchedAt.IsZero() {
		sourceFetched = item.SourceFetchedAt
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO dataset_snapshots (id, season_id, gameweek_id, dataset, state, source_fetched_at, normalized_at, normalizer_version, missing_inputs) VALUES ($1, (SELECT id FROM seasons WHERE source_id=$2 ORDER BY is_current DESC, updated_at DESC LIMIT 1), (SELECT g.id FROM gameweeks g JOIN seasons s ON s.id=g.season_id WHERE s.source_id=$2 AND g.source_id=$3 LIMIT 1), $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET state=EXCLUDED.state, source_fetched_at=EXCLUDED.source_fetched_at, normalized_at=EXCLUDED.normalized_at, normalizer_version=EXCLUDED.normalizer_version, missing_inputs=EXCLUDED.missing_inputs`, item.ID, item.SeasonID, item.Gameweek, item.Dataset, item.State, sourceFetched, item.NormalizedAt, item.NormalizerVersion, missing)
	if err != nil {
		return fmt.Errorf("create dataset snapshot: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CurrentDatasetFreshness(ctx context.Context, scope Scope) (Freshness, error) {
	var snapshotID, dataset, state, normalizer string
	var sourceFetched, normalized sql.NullTime
	var missing []byte
	err := r.db.QueryRowContext(ctx, `SELECT d.id::text, d.dataset, d.state, d.source_fetched_at, d.normalized_at, d.normalizer_version, d.missing_inputs FROM dataset_snapshots d JOIN seasons s ON s.id=d.season_id LEFT JOIN gameweeks g ON g.id=d.gameweek_id WHERE ($1=0 OR s.source_id=$1) AND ($2=0 OR g.source_id=$2) AND ($3='' OR d.dataset=$3) ORDER BY d.normalized_at DESC LIMIT 1`, scope.SeasonID, scope.Gameweek, scope.Dataset).Scan(&snapshotID, &dataset, &state, &sourceFetched, &normalized, &normalizer, &missing)
	if err == sql.ErrNoRows {
		return Freshness{Status: "unavailable", State: "unavailable", Dataset: scope.Dataset}, nil
	}
	if err != nil {
		return Freshness{}, fmt.Errorf("load dataset freshness: %w", err)
	}
	status := "fresh"
	if state == "partial" || state == "stale" {
		status = state
	}
	if state == "unavailable" {
		status = "unavailable"
	}
	freshness := Freshness{Status: status, State: state, Dataset: dataset, SnapshotIDs: []string{snapshotID}, NormalizerVersion: normalizer}
	if sourceFetched.Valid {
		freshness.SourceFetchedAt = sourceFetched.Time
	}
	if normalized.Valid {
		freshness.NormalizedAt = normalized.Time
		freshness.SnapshotAt = normalized.Time
		freshness.LastSuccessfulSync = normalized.Time
	}
	if len(missing) > 0 {
		_ = json.Unmarshal(missing, &freshness.MissingInputs)
	}
	return freshness, nil
}

func (r *PostgresRepository) SearchPlayers(ctx context.Context, q PlayerQuery) ([]Player, int, error) {
	where := []string{"s.is_current"}
	args := []interface{}{}
	add := func(clause string, value interface{}) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	term := strings.TrimSpace(q.Search)
	if term != "" {
		add("(LOWER(p.web_name || ' ' || p.first_name || ' ' || p.second_name) LIKE LOWER('%%' || $%d || '%%'))", term)
	}
	if q.Position > 0 {
		add("p.position=$%d", q.Position)
	}
	if q.TeamID > 0 {
		add("t.source_id=$%d", q.TeamID)
	}
	if q.MinPrice > 0 {
		add("p.price >= $%d", q.MinPrice)
	}
	if q.MaxPrice > 0 {
		add("p.price <= $%d", q.MaxPrice)
	}
	if q.MinMinutes > 0 {
		add("p.minutes >= $%d", q.MinMinutes)
	}
	if q.MinForm > 0 {
		add("p.form >= $%d", q.MinForm)
	}
	if q.MinPoints > 0 {
		add("p.total_points >= $%d", q.MinPoints)
	}
	if q.MinValue > 0 {
		add("p.value >= $%d", q.MinValue)
	}
	if q.Status != "" {
		add("p.status=$%d", q.Status)
	}
	sortColumn := map[string]string{"price": "p.price", "form": "p.form", "points": "p.total_points", "minutes": "p.minutes", "value": "p.value"}[q.Sort]
	if sortColumn == "" {
		sortColumn = "LOWER(p.web_name)"
	}
	direction := "ASC"
	if q.Desc {
		direction = "DESC"
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size < 1 {
		size = 25
	}
	if size > 100 {
		size = 100
	}
	args = append(args, size, (page-1)*size)
	query := fmt.Sprintf(`SELECT p.source_id, p.first_name, p.second_name, p.web_name, p.position, t.source_id, p.price, p.total_points, p.form, p.minutes, p.value, p.status, p.news, p.chance_of_playing_next_round, p.goals_scored, p.assists, p.clean_sheets, p.bonus, p.saves, p.expected_minutes, p.recent_returns, COUNT(*) OVER() FROM players p JOIN teams t ON t.id=p.team_id JOIN seasons s ON s.id=p.season_id WHERE %s ORDER BY %s %s, p.source_id LIMIT $%d OFFSET $%d`, strings.Join(where, " AND "), sortColumn, direction, len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search players: %w", err)
	}
	defer rows.Close()
	items := []Player{}
	total := 0
	for rows.Next() {
		item, rowTotal, err := scanPlayerWithTotal(rows)
		if err != nil {
			return nil, 0, err
		}
		total = rowTotal
		items = append(items, item)
	}
	return items, total, rows.Err()
}

type rowScanner interface{ Scan(...interface{}) error }

func scanPlayerWithTotal(scanner rowScanner) (Player, int, error) {
	var item Player
	var chance sql.NullInt64
	var total int
	err := scanner.Scan(&item.ID, &item.FirstName, &item.SecondName, &item.WebName, &item.Position, &item.TeamID, &item.Price, &item.TotalPoints, &item.Form, &item.Minutes, &item.Value, &item.Status, &item.News, &chance, &item.GoalsScored, &item.Assists, &item.CleanSheets, &item.Bonus, &item.Saves, &item.ExpectedMinutes, &item.RecentReturns, &total)
	if chance.Valid {
		value := int(chance.Int64)
		item.ChanceOfPlaying = &value
	}
	return item, total, err
}

func (r *PostgresRepository) LoadPlayerDetail(ctx context.Context, sourcePlayerID int) (PlayerDetail, bool, error) {
	var detail PlayerDetail
	var chance sql.NullInt64
	var teamID int
	err := r.db.QueryRowContext(ctx, `SELECT p.source_id, p.first_name, p.second_name, p.web_name, p.position, t.source_id, p.price, p.total_points, p.form, p.minutes, p.value, p.status, p.news, p.chance_of_playing_next_round, p.goals_scored, p.assists, p.clean_sheets, p.bonus, p.saves, p.expected_minutes, p.recent_returns, t.name, t.short_name FROM players p JOIN teams t ON t.id=p.team_id JOIN seasons s ON s.id=p.season_id WHERE s.is_current AND p.source_id=$1 ORDER BY s.updated_at DESC LIMIT 1`, sourcePlayerID).Scan(&detail.Player.ID, &detail.Player.FirstName, &detail.Player.SecondName, &detail.Player.WebName, &detail.Player.Position, &teamID, &detail.Player.Price, &detail.Player.TotalPoints, &detail.Player.Form, &detail.Player.Minutes, &detail.Player.Value, &detail.Player.Status, &detail.Player.News, &chance, &detail.Player.GoalsScored, &detail.Player.Assists, &detail.Player.CleanSheets, &detail.Player.Bonus, &detail.Player.Saves, &detail.Player.ExpectedMinutes, &detail.Player.RecentReturns, &detail.Team.Name, &detail.Team.ShortName)
	if err == sql.ErrNoRows {
		return PlayerDetail{}, false, nil
	}
	if err != nil {
		return PlayerDetail{}, false, fmt.Errorf("load player detail: %w", err)
	}
	detail.Player.TeamID = teamID
	if chance.Valid {
		value := int(chance.Int64)
		detail.Player.ChanceOfPlaying = &value
	}
	historyRows, err := r.db.QueryContext(ctx, `SELECT g.source_id, h.minutes, h.total_points, h.goals_scored, h.assists, h.clean_sheets, h.bonus, COALESCE(h.value,0) FROM player_gameweek_history h JOIN players p ON p.id=h.player_id JOIN gameweeks g ON g.id=h.gameweek_id JOIN seasons s ON s.id=h.season_id WHERE s.is_current AND p.source_id=$1 ORDER BY g.source_id`, sourcePlayerID)
	if err != nil {
		return PlayerDetail{}, false, err
	}
	for historyRows.Next() {
		var row PlayerHistory
		if err := historyRows.Scan(&row.Gameweek, &row.Minutes, &row.TotalPoints, &row.Goals, &row.Assists, &row.CleanSheets, &row.Bonus, &row.Value); err != nil {
			historyRows.Close()
			return PlayerDetail{}, false, err
		}
		detail.History = append(detail.History, row)
	}
	if err := historyRows.Err(); err != nil {
		historyRows.Close()
		return PlayerDetail{}, false, err
	}
	historyRows.Close()
	fixtureRows, err := r.db.QueryContext(ctx, `SELECT f.source_id, COALESCE(g.source_id,0), f.kickoff_time, f.finished, h.source_id, a.source_id, COALESCE(f.team_home_difficulty,0), COALESCE(f.team_away_difficulty,0), f.team_home_score, f.team_away_score FROM fixtures f JOIN teams h ON h.id=f.team_home_id JOIN teams a ON a.id=f.team_away_id JOIN seasons s ON s.id=f.season_id LEFT JOIN gameweeks g ON g.id=f.gameweek_id WHERE s.is_current AND f.finished=FALSE AND (h.source_id=$1 OR a.source_id=$1) ORDER BY f.kickoff_time NULLS LAST`, teamID)
	if err != nil {
		return PlayerDetail{}, false, err
	}
	for fixtureRows.Next() {
		var row Fixture
		if err := fixtureRows.Scan(&row.ID, &row.Gameweek, &row.KickoffTime, &row.Finished, &row.HomeTeam, &row.AwayTeam, &row.HomeDifficulty, &row.AwayDifficulty, &row.HomeScore, &row.AwayScore); err != nil {
			fixtureRows.Close()
			return PlayerDetail{}, false, err
		}
		detail.Fixtures = append(detail.Fixtures, row)
	}
	fixtureRows.Close()
	detail.Freshness = Freshness{Status: "fresh", State: "actual", Dataset: "public-fpl"}
	return detail, true, nil
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
		if err := tx.QueryRowContext(ctx, `INSERT INTO sync_runs (status, scope, season_source_id, gameweek_source_id, started_at, finished_at, warning, checksum) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, statusValue, status.Scope.Dataset, nullableInt(status.Scope.SeasonID), nullableInt(status.Scope.Gameweek), nullableTime(status.StartedAt), nullableTime(status.FinishedAt), nullableString(status.Warning), nullableString(checksum)).Scan(&runID); err != nil {
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
func nullableInt(value int) interface{} {
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
