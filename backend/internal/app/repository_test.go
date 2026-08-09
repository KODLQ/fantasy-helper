package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPostgresRepositoryPersistence(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL persistence tests")
	}
	if err := assertDisposableTestDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	database, err := OpenDatabase(ctx, Config{DatabaseURL: dsn, DatabaseMaxConns: 4, DatabaseMaxIdle: 2, DatabasePing: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewPostgresRepository(database, nil)
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `TRUNCATE sync_diagnostics, sync_stages, sync_runs, squad_lineups, squad_plan_players, squad_plans, player_gameweek_history, player_season_history, fixtures, players, teams, gameweeks, seasons RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	snapshot := store.ExportSnapshot()
	snapshot.Fixtures = []Fixture{{ID: 101, Gameweek: 1, HomeTeam: 1, AwayTeam: 2, HomeDifficulty: 2, AwayDifficulty: 4}}
	snapshot.Checksum = "demo-checksum"
	if err := repository.UpsertSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Season.Name = "2025/26 Demo Updated"
	if err := repository.UpsertSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	var seasonCount, playerCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM seasons`).Scan(&seasonCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&playerCount); err != nil {
		t.Fatal(err)
	}
	if seasonCount != 1 || playerCount != len(snapshot.Players) {
		t.Fatalf("upsert duplicated data: seasons=%d players=%d", seasonCount, playerCount)
	}
	datasetSnapshotID := newSnapshotID()
	if err := repository.CreateDatasetSnapshot(ctx, DatasetSnapshot{ID: datasetSnapshotID, Dataset: "player-gameweek", State: "actual", SeasonID: snapshot.Season.ID, Gameweek: 1, SourceFetchedAt: time.Now().UTC(), NormalizedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1"}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := repository.ListDatasetSnapshots(ctx, Scope{SeasonID: snapshot.Season.ID, Gameweek: 1, Dataset: "player-gameweek"})
	if err != nil || len(snapshots) != 1 || snapshots[0].State != "actual" || snapshots[0].NormalizerVersion != "fpl-public-v1" {
		t.Fatalf("unexpected dataset snapshots: %#v err=%v", snapshots, err)
	}
	freshness, err := repository.CurrentDatasetFreshness(ctx, Scope{SeasonID: snapshot.Season.ID, Gameweek: 1, Dataset: "player-gameweek"})
	if err != nil || freshness.State != "actual" || len(freshness.SnapshotIDs) != 1 || freshness.NormalizerVersion != "fpl-public-v1" {
		t.Fatalf("unexpected dataset freshness: %#v err=%v", freshness, err)
	}
	fixture := snapshot.Fixtures[0]
	player := snapshot.Players[0]
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repository.UpsertFixtureStats(ctx, snapshot.Season.ID, observedAt, []SourceFixture{{ID: fixture.ID, Stats: []SourceFixtureStat{{Identifier: "goals_scored", Home: []SourceStatValue{{Element: player.ID, Value: 1}}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertLiveGameweek(ctx, datasetSnapshotID, snapshot.Season.ID, 1, true, observedAt, []LivePlayerStats{{PlayerID: player.ID, Minutes: 90, Points: 9, Goals: 1, ExpectedGoals: "0.75"}}); err != nil {
		t.Fatal(err)
	}
	var fixtureStatCount, liveFactCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM fixture_stats WHERE stat_type='goals_scored' AND stat_value=1`).Scan(&fixtureStatCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM player_gameweek_facts WHERE snapshot_id=$1 AND finalized AND total_points=9`, datasetSnapshotID).Scan(&liveFactCount); err != nil {
		t.Fatal(err)
	}
	if fixtureStatCount != 1 || liveFactCount != 1 {
		t.Fatalf("warehouse facts not persisted: fixture=%d live=%d", fixtureStatCount, liveFactCount)
	}
	players, total, err := repository.SearchPlayers(ctx, PlayerQuery{Sort: "form", Desc: true, Page: 1, PageSize: 3})
	if err != nil || len(players) != 3 || total != len(snapshot.Players) {
		t.Fatalf("unexpected PostgreSQL research result: count=%d total=%d err=%v", len(players), total, err)
	}
	detail, found, err := repository.LoadPlayerDetail(ctx, players[0].ID)
	if err != nil || !found || detail.Player.ID != players[0].ID || detail.Team.ShortName == "" {
		t.Fatalf("unexpected PostgreSQL player detail: %#v found=%v err=%v", detail, found, err)
	}
	loaded, ok, err := repository.LoadSnapshot(ctx)
	if err != nil || !ok {
		t.Fatalf("load snapshot: ok=%v err=%v", ok, err)
	}
	if loaded.Season.Name != "2025/26 Demo Updated" || len(loaded.Players) != len(snapshot.Players) {
		t.Fatalf("unexpected loaded snapshot: %#v", loaded.Season)
	}

	squad := demoSquad()
	if err := repository.SaveSquad(ctx, squad); err != nil {
		t.Fatal(err)
	}
	loadedSquad, ok, err := repository.LoadSquad(ctx)
	if err != nil || !ok {
		t.Fatalf("load squad: ok=%v err=%v", ok, err)
	}
	if len(loadedSquad.PurchasePrices) != 15 || loadedSquad.Formation != squad.Formation || loadedSquad.CaptainID != squad.CaptainID {
		t.Fatalf("unexpected loaded squad: %#v", loadedSquad)
	}

	status := SyncStatus{Status: "partial", StartedAt: time.Now().Add(-time.Minute), FinishedAt: time.Now(), CompletedStages: []string{"snapshot"}, FailedStages: []string{"player-history:8"}, Warning: "one history batch failed", Checksum: "sync-checksum"}
	if err := repository.RecordSyncStatus(ctx, status); err != nil {
		t.Fatal(err)
	}
	var runCount, stageCount, diagnosticCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_runs WHERE status='partial'`).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_stages`).Scan(&stageCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_diagnostics`).Scan(&diagnosticCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 1 || stageCount != 2 || diagnosticCount != 1 {
		t.Fatalf("sync diagnostics not persisted: runs=%d stages=%d diagnostics=%d", runCount, stageCount, diagnosticCount)
	}
	if err := repository.RecordSourceObservation(ctx, SourceObservation{Endpoint: "/bootstrap-static/", FetchedAt: time.Now().UTC(), HTTPStatus: 200, Checksum: "source-checksum", ValidationState: "valid", SchemaVersion: "fpl-public-v1", Payload: []byte(`{"events":[]}`)}); err != nil {
		t.Fatal(err)
	}
	var payloadCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_payloads WHERE checksum='source-checksum'`).Scan(&payloadCount); err != nil {
		t.Fatal(err)
	}
	if payloadCount != 1 {
		t.Fatalf("source observation not persisted: %d", payloadCount)
	}

	transactional := interface{}(repository).(TransactionalRepository)
	if err := transactional.WithTransaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO seasons (source_id, name, is_current) VALUES (999, 'rollback', FALSE)`); err != nil {
			return err
		}
		return context.Canceled
	}); err == nil {
		t.Fatal("expected transaction rollback error")
	}
	var rolledBack int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM seasons WHERE source_id=999`).Scan(&rolledBack); err != nil {
		t.Fatal(err)
	}
	if rolledBack != 0 {
		t.Fatalf("transaction did not roll back: %d rows", rolledBack)
	}
}

func assertDisposableTestDatabase(dsn string) error {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("TEST_DATABASE_URL is invalid: %w", err)
	}
	database := strings.ToLower(strings.TrimPrefix(parsed.Path, "/"))
	if database == "" || (!strings.Contains(database, "test") && !strings.Contains(database, "ci")) {
		return fmt.Errorf("refusing integration test database %q: name must contain test or ci", database)
	}
	if strings.Contains(database, "prod") || strings.Contains(database, "production") {
		return fmt.Errorf("refusing production-like integration test database %q", database)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("TEST_DATABASE_URL must include a host")
	}
	return nil
}

func TestPostgresSyncWorkQueueClaimsIdempotently(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL persistence tests")
	}
	if err := assertDisposableTestDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	database, err := OpenDatabase(ctx, Config{DatabaseURL: dsn, DatabaseMaxConns: 4, DatabaseMaxIdle: 2, DatabasePing: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewPostgresRepository(database, nil)
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Dataset: fmt.Sprintf("test-work-queue-%d", time.Now().UnixNano())}
	runID, err := repository.StartSyncRun(ctx, scope, "test-request")
	if err != nil {
		t.Fatal(err)
	}
	if _, duplicateErr := repository.StartSyncRun(ctx, scope, "duplicate-request"); duplicateErr == nil {
		t.Fatal("expected duplicate active sync scope to be rejected")
	}
	if err := repository.RecordSyncStage(ctx, SyncStage{RunID: runID, Name: "catalog", Status: "running", StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordSyncStage(ctx, SyncStage{RunID: runID, Name: "catalog", Status: "success", ProcessedCount: 3, FinishedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.EnqueueSyncWork(ctx, runID, []SyncWorkItem{{Scope: "player-history", NaturalKey: "test:1", Endpoint: "/element-summary/1/", EntitySourceID: 1}}); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := repository.ClaimSyncWork(ctx, runID)
	if err != nil || !ok || claimed.Attempts != 1 {
		t.Fatalf("unexpected claim: %#v ok=%v err=%v", claimed, ok, err)
	}
	if err := repository.FailSyncWork(ctx, claimed.ID, fmt.Errorf("temporary test failure"), false); err != nil {
		t.Fatal(err)
	}
	if err := repository.FinishSyncRun(ctx, runID, SyncStatus{Status: "failed", FinishedAt: time.Now().UTC(), Scope: scope}); err != nil {
		t.Fatal(err)
	}
	retried, err := repository.RetrySyncRun(ctx, runID)
	if err != nil || retried.Status != "running" {
		t.Fatalf("retry did not reopen run: %#v err=%v", retried, err)
	}
	reclaimed, ok, err := repository.ClaimSyncWork(ctx, runID)
	if err != nil || !ok || reclaimed.Attempts != 2 {
		t.Fatalf("retry did not requeue work: %#v ok=%v err=%v", reclaimed, ok, err)
	}
	if err := repository.CompleteSyncWork(ctx, reclaimed.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repository.ClaimSyncWork(ctx, runID); err != nil || ok {
		t.Fatalf("completed work was claimable: ok=%v err=%v", ok, err)
	}
	if err := repository.FinishSyncRun(ctx, runID, SyncStatus{Status: "success", FinishedAt: time.Now().UTC(), Scope: scope}); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.LoadLatestSyncStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RunID != runID || loaded.Status != "success" || loaded.CorrelationID != "test-request" || loaded.TotalWork != 1 || loaded.CompletedWork != 1 || loaded.FailedWork != 0 || loaded.RetryableWork != 0 || strings.Join(loaded.CompletedStages, ",") != "catalog" {
		t.Fatalf("unexpected persisted sync status: %#v", loaded)
	}
	afterCompletionRun, err := repository.StartSyncRun(ctx, scope, "after-completion")
	if err != nil {
		t.Fatalf("completed scope remained locked: %v", err)
	}
	if err := repository.FinishSyncRun(ctx, afterCompletionRun, SyncStatus{Status: "success", FinishedAt: time.Now().UTC(), Scope: scope}); err != nil {
		t.Fatal(err)
	}

	recoveryScope := Scope{Dataset: fmt.Sprintf("test-recovery-%d", time.Now().UnixNano())}
	abandonedRun, err := repository.StartSyncRun(ctx, recoveryScope, "crashed-worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.EnqueueSyncWork(ctx, abandonedRun, []SyncWorkItem{{Scope: "player-history", NaturalKey: "recovery:1", Endpoint: "/element-summary/1/", EntitySourceID: 1}}); err != nil {
		t.Fatal(err)
	}
	abandonedItem, ok, err := repository.ClaimSyncWork(ctx, abandonedRun)
	if err != nil || !ok {
		t.Fatalf("expected work to be claimed before simulated restart: %#v ok=%v err=%v", abandonedItem, ok, err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE sync_runs SET started_at=NOW() - INTERVAL '20 minutes' WHERE id=$1`, abandonedRun); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE sync_work_items SET claimed_at=NOW() - INTERVAL '20 minutes' WHERE id=$1`, abandonedItem.ID); err != nil {
		t.Fatal(err)
	}
	recoveredRun, err := repository.StartSyncRun(ctx, recoveryScope, "after-restart")
	if err != nil {
		t.Fatalf("restart did not recover the abandoned scope: %v", err)
	}
	var abandonedStatus string
	if err := database.QueryRowContext(ctx, `SELECT status FROM sync_runs WHERE id=$1`, abandonedRun).Scan(&abandonedStatus); err != nil {
		t.Fatal(err)
	}
	if abandonedStatus != "partial" {
		t.Fatalf("abandoned run status = %q, want partial", abandonedStatus)
	}
	recoveredItem, ok, err := repository.ClaimSyncWork(ctx, recoveredRun)
	if err != nil || !ok || recoveredItem.Attempts != 2 {
		t.Fatalf("restart did not make abandoned work claimable: %#v ok=%v err=%v", recoveredItem, ok, err)
	}
	if err := repository.CompleteSyncWork(ctx, recoveredItem.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.FinishSyncRun(ctx, recoveredRun, SyncStatus{Status: "success", FinishedAt: time.Now().UTC(), Scope: recoveryScope}); err != nil {
		t.Fatal(err)
	}
}
