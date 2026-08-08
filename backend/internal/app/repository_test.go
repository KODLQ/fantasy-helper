package app

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestPostgresRepositoryPersistence(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL persistence tests")
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
