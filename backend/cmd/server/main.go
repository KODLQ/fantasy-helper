package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fantasy-helper/backend/internal/app"
)

func main() {
	logger := app.NewLogger(os.Getenv("APP_ENV"))
	cfg, err := app.LoadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	database, err := app.OpenDatabase(context.Background(), cfg)
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	repository := app.NewPostgresRepository(database, logger)
	if err := repository.EnsureSchema(context.Background()); err != nil {
		logger.Error("database schema unavailable", "error", err)
		os.Exit(1)
	}
	store := app.NewWarehouseCache()
	if snapshot, ok, err := repository.LoadSnapshot(context.Background()); err != nil {
		logger.Error("load persisted snapshot failed", "error", err)
	} else if ok {
		store.ApplySnapshot(snapshot.Season, snapshot.Gameweeks, snapshot.Teams, snapshot.Players, snapshot.Fixtures, snapshot.Histories)
	}
	if squad, ok, err := repository.LoadSquad(context.Background()); err != nil {
		logger.Error("load persisted squad failed", "error", err)
	} else if ok {
		store.SaveSquad(squad)
	}
	source := app.NewFPLSource(cfg.FPLBaseURL)
	source.SeasonID = cfg.SourceSeasonID
	source.SeasonName = cfg.SourceSeasonName
	source.AllowDiscovery = cfg.SourceDiscovery
	source.Client.Timeout = cfg.SourceTimeout
	source.Retries = cfg.SourceRetries
	source.RetryJitter = cfg.SourceRetryJitter
	source.SetMaxConcurrent(cfg.SourceMaxConcurrent)
	api := app.NewAPI(store, source, func(ctx context.Context) bool {
		return database.PingContext(ctx) == nil
	}, logger, repository)
	api.SyncWorkers = cfg.SyncWorkers
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	if cfg.SchedulerEnabled {
		scheduler := app.NewSyncScheduler(api, store.CurrentGameweek, app.SyncSchedule{Tick: cfg.SchedulerTick, Catalog: cfg.CatalogCadence, Fixtures: cfg.FixtureCadence, Live: cfg.LiveCadence, Finalization: cfg.FinalizationCadence, HistoricalReconcile: cfg.ReconcileCadence})
		go scheduler.Run(schedulerCtx)
		logger.Info("sync scheduler enabled", "catalogCadence", cfg.CatalogCadence, "fixtureCadence", cfg.FixtureCadence, "liveCadence", cfg.LiveCadence, "finalizationCadence", cfg.FinalizationCadence, "reconcileCadence", cfg.ReconcileCadence)
	}
	if cfg.RetentionEnabled {
		go app.RunRetentionCleanup(schedulerCtx, repository, cfg.RetentionCadence, cfg.RawPayloadRetention, cfg.LivePayloadRetention, logger)
	}
	server := &http.Server{Addr: ":" + cfg.Port, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	stopScheduler()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = api.Shutdown(ctx)
	_ = server.Shutdown(ctx)
}
