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
	if err := repository.CreateDatasetSnapshot(ctx, DatasetSnapshot{ID: newSnapshotID(), Dataset: "player-gameweek", State: "actual", SeasonID: snapshot.Season.ID, Gameweek: 1, SourceFetchedAt: time.Now().UTC(), NormalizedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1"}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := repository.ListDatasetSnapshots(ctx, Scope{SeasonID: snapshot.Season.ID, Gameweek: 1, Dataset: "player-gameweek"})
	if err != nil || len(snapshots) != 1 || snapshots[0].State != "actual" || snapshots[0].NormalizerVersion != "fpl-public-v1" {
		t.Fatalf("unexpected dataset snapshots: %#v err=%v", snapshots, err)
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
	if err := repository.EnqueueSyncWork(ctx, runID, []SyncWorkItem{{Scope: "player-history", NaturalKey: "test:1", Endpoint: "/element-summary/1/", EntitySourceID: 1}}); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := repository.ClaimSyncWork(ctx, runID)
	if err != nil || !ok || claimed.Attempts != 1 {
		t.Fatalf("unexpected claim: %#v ok=%v err=%v", claimed, ok, err)
	}
	if err := repository.CompleteSyncWork(ctx, claimed.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repository.ClaimSyncWork(ctx, runID); err != nil || ok {
		t.Fatalf("completed work was claimable: ok=%v err=%v", ok, err)
	}
	if err := repository.FinishSyncRun(ctx, runID, SyncStatus{Status: "success", FinishedAt: time.Now().UTC(), Scope: scope}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.StartSyncRun(ctx, scope, "after-completion"); err != nil {
		t.Fatalf("completed scope remained locked: %v", err)
	}
}
