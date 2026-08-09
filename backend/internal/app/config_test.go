package app

import (
	"testing"
	"time"
)

func TestLoadConfigSourceTransportPolicy(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "8")
	t.Setenv("DB_MAX_IDLE_CONNS", "4")
	t.Setenv("FPL_SOURCE_SEASON_ID", "")
	t.Setenv("FPL_SOURCE_SEASON_NAME", "")
	t.Setenv("FPL_SOURCE_RETRIES", "4")
	t.Setenv("FPL_SOURCE_RETRY_JITTER", "275ms")
	t.Setenv("FPL_SOURCE_MAX_CONCURRENT", "3")
	t.Setenv("SYNC_WORKERS", "5")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceRetries != 4 || cfg.SourceRetryJitter != 275*time.Millisecond || cfg.SourceMaxConcurrent != 3 || cfg.SyncWorkers != 5 {
		t.Fatalf("unexpected source transport configuration: %#v", cfg)
	}
	t.Setenv("FPL_SOURCE_MAX_CONCURRENT", "0")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected zero source concurrency to be rejected")
	}
}
