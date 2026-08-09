package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port                string
	DatabaseURL         string
	FPLBaseURL          string
	SourceSeasonID      int
	SourceSeasonName    string
	SourceDiscovery     bool
	SourceTimeout       time.Duration
	SourceRetries       int
	SourceRetryJitter   time.Duration
	SourceMaxConcurrent int
	SyncWorkers         int
	SchedulerEnabled    bool
	SchedulerTick       time.Duration
	CatalogCadence      time.Duration
	FixtureCadence      time.Duration
	LiveCadence         time.Duration
	FinalizationCadence time.Duration
	ReconcileCadence    time.Duration
	Environment         string
	DatabaseMaxConns    int
	DatabaseMaxIdle     int
	DatabasePing        time.Duration
	ShutdownTimeout     time.Duration
}

func LoadConfig() (Config, error) {
	maxConns, err := envInt("DB_MAX_CONNS", 8)
	if err != nil {
		return Config{}, err
	}
	maxIdle, err := envInt("DB_MAX_IDLE_CONNS", 4)
	if err != nil {
		return Config{}, err
	}
	if maxConns < 1 || maxIdle < 0 || maxIdle > maxConns {
		return Config{}, fmt.Errorf("invalid database pool limits: max=%d idle=%d", maxConns, maxIdle)
	}
	seasonID, err := envInt("FPL_SOURCE_SEASON_ID", 0)
	if err != nil {
		return Config{}, err
	}
	seasonName := os.Getenv("FPL_SOURCE_SEASON_NAME")
	discovery := envBool("FPL_SOURCE_SEASON_DISCOVERY", true)
	if (seasonID == 0) != (seasonName == "") {
		return Config{}, fmt.Errorf("FPL_SOURCE_SEASON_ID and FPL_SOURCE_SEASON_NAME must be provided together")
	}
	if seasonID > 0 {
		discovery = false
	}
	retries, err := envInt("FPL_SOURCE_RETRIES", 2)
	if err != nil {
		return Config{}, err
	}
	workers, err := envInt("SYNC_WORKERS", 6)
	if err != nil {
		return Config{}, err
	}
	if retries < 0 || workers < 1 {
		return Config{}, fmt.Errorf("FPL_SOURCE_RETRIES must be non-negative and SYNC_WORKERS must be positive")
	}
	maxConcurrent, err := envInt("FPL_SOURCE_MAX_CONCURRENT", workers)
	if err != nil {
		return Config{}, err
	}
	if maxConcurrent < 1 {
		return Config{}, fmt.Errorf("FPL_SOURCE_MAX_CONCURRENT must be positive")
	}
	schedulerTick := envDuration("SYNC_SCHEDULER_TICK", time.Minute)
	catalogCadence := envDuration("SYNC_CATALOG_CADENCE", time.Hour)
	fixtureCadence := envDuration("SYNC_FIXTURE_CADENCE", time.Hour)
	liveCadence := envDuration("SYNC_LIVE_CADENCE", 5*time.Minute)
	finalizationCadence := envDuration("SYNC_FINALIZATION_CADENCE", 15*time.Minute)
	reconcileCadence := envDuration("SYNC_RECONCILE_CADENCE", 24*time.Hour)
	if schedulerTick <= 0 || catalogCadence <= 0 || fixtureCadence <= 0 || liveCadence <= 0 || finalizationCadence <= 0 || reconcileCadence <= 0 {
		return Config{}, fmt.Errorf("sync scheduler tick and cadence durations must be positive")
	}
	return Config{
		Port:                configEnv("BACKEND_PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		FPLBaseURL:          configEnv("FPL_BASE_URL", "https://fantasy.premierleague.com/api"),
		SourceSeasonID:      seasonID,
		SourceSeasonName:    seasonName,
		SourceDiscovery:     discovery,
		SourceTimeout:       envDuration("FPL_SOURCE_TIMEOUT", 20*time.Second),
		SourceRetries:       retries,
		SourceRetryJitter:   envDuration("FPL_SOURCE_RETRY_JITTER", 100*time.Millisecond),
		SourceMaxConcurrent: maxConcurrent,
		SyncWorkers:         workers,
		SchedulerEnabled:    envBool("SYNC_SCHEDULER_ENABLED", false),
		SchedulerTick:       schedulerTick,
		CatalogCadence:      catalogCadence,
		FixtureCadence:      fixtureCadence,
		LiveCadence:         liveCadence,
		FinalizationCadence: finalizationCadence,
		ReconcileCadence:    reconcileCadence,
		Environment:         configEnv("APP_ENV", "local"),
		DatabaseMaxConns:    maxConns,
		DatabaseMaxIdle:     maxIdle,
		DatabasePing:        envDuration("DB_PING_TIMEOUT", 3*time.Second),
		ShutdownTimeout:     envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}, nil
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func OpenDatabase(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	database, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	database.SetMaxOpenConns(cfg.DatabaseMaxConns)
	database.SetMaxIdleConns(cfg.DatabaseMaxIdle)
	database.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, cfg.DatabasePing)
	defer cancel()
	if err := database.PingContext(pingCtx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return database, nil
}

func NewLogger(environment string) *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("environment", environment)
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func configEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
