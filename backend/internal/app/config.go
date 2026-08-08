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
	Port             string
	DatabaseURL      string
	FPLBaseURL       string
	Environment      string
	DatabaseMaxConns int
	DatabaseMaxIdle  int
	DatabasePing     time.Duration
	ShutdownTimeout  time.Duration
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
	return Config{
		Port:             configEnv("BACKEND_PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		FPLBaseURL:       configEnv("FPL_BASE_URL", "https://fantasy.premierleague.com/api"),
		Environment:      configEnv("APP_ENV", "local"),
		DatabaseMaxConns: maxConns,
		DatabaseMaxIdle:  maxIdle,
		DatabasePing:     envDuration("DB_PING_TIMEOUT", 3*time.Second),
		ShutdownTimeout:  envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}, nil
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
