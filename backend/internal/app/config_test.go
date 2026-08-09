package app

import (
	"strings"
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
	t.Setenv("SYNC_SCHEDULER_ENABLED", "true")
	t.Setenv("SYNC_SCHEDULER_TICK", "30s")
	t.Setenv("SYNC_CATALOG_CADENCE", "1h")
	t.Setenv("SYNC_FIXTURE_CADENCE", "45m")
	t.Setenv("SYNC_LIVE_CADENCE", "5m")
	t.Setenv("SYNC_FINALIZATION_CADENCE", "10m")
	t.Setenv("SYNC_RECONCILE_CADENCE", "12h")
	t.Setenv("RETENTION_CLEANUP_ENABLED", "true")
	t.Setenv("RETENTION_CLEANUP_CADENCE", "6h")
	t.Setenv("RAW_PAYLOAD_RETENTION", "2160h")
	t.Setenv("LIVE_PAYLOAD_RETENTION", "720h")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceRetries != 4 || cfg.SourceRetryJitter != 275*time.Millisecond || cfg.SourceMaxConcurrent != 3 || cfg.SyncWorkers != 5 || !cfg.SchedulerEnabled || cfg.SchedulerTick != 30*time.Second || cfg.FixtureCadence != 45*time.Minute || cfg.ReconcileCadence != 12*time.Hour || !cfg.RetentionEnabled || cfg.RetentionCadence != 6*time.Hour || cfg.LivePayloadRetention != 720*time.Hour {
		t.Fatalf("unexpected source transport configuration: %#v", cfg)
	}
	t.Setenv("FPL_SOURCE_MAX_CONCURRENT", "0")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected zero source concurrency to be rejected")
	}
	t.Setenv("FPL_SOURCE_MAX_CONCURRENT", "3")
	t.Setenv("SYNC_LIVE_CADENCE", "-1m")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected negative scheduler cadence to be rejected")
	}
}

func TestLoadConfigValidatesSourceProfiles(t *testing.T) {
	t.Setenv("FPL_SOURCE_SEASON_ID", "")
	t.Setenv("FPL_SOURCE_SEASON_NAME", "")
	t.Setenv("FPL_SOURCE_PROFILES_JSON", `[{"seasonId":2026,"seasonName":"2026/27","kind":"official-current","baseLocation":"https://fantasy.premierleague.com/api","supportedDatasets":["catalogue"],"allowLiveRefresh":true}]`)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceSeasonID != 2026 || cfg.SourceSeasonName != "2026/27" || len(cfg.SourceProfiles) != 1 {
		t.Fatalf("unexpected source profile config: %#v", cfg)
	}

	for name, value := range map[string]string{
		"missing official profile": `[{"seasonId":2025,"seasonName":"2025/26","kind":"historical-archive","baseLocation":"/archive","allowLiveRefresh":false}]`,
		"duplicate season":         `[{"seasonId":2026,"seasonName":"2026/27","kind":"official-current","allowLiveRefresh":true},{"seasonId":2026,"seasonName":"duplicate","kind":"retained-snapshot","allowLiveRefresh":false}]`,
		"historical live":          `[{"seasonId":2026,"seasonName":"2026/27","kind":"official-current","allowLiveRefresh":true},{"seasonId":2025,"seasonName":"2025/26","kind":"historical-archive","allowLiveRefresh":true}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FPL_SOURCE_PROFILES_JSON", value)
			if _, err := LoadConfig(); err == nil || strings.TrimSpace(err.Error()) == "" {
				t.Fatal("expected invalid source profile configuration to fail")
			}
		})
	}
}

func TestLoadConfigAuthenticationLifecycle(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_REGISTRATION_ENABLED", "")
	t.Setenv("AUTH_COOKIE_SECURE", "")
	t.Setenv("AUTH_ALLOWED_ORIGIN", "https://fantasy.local")
	t.Setenv("AUTH_SESSION_IDLE_TIMEOUT", "2h")
	t.Setenv("AUTH_SESSION_ABSOLUTE_TIMEOUT", "48h")
	t.Setenv("AUTH_BOOTSTRAP_EMAIL", " owner@example.test ")
	t.Setenv("AUTH_BOOTSTRAP_PASSWORD", "Bootstrap-password-value-42")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthRegistration || !cfg.AuthCookieSecure || cfg.AuthAllowedOrigin != "https://fantasy.local" || cfg.AuthIdleTimeout != 2*time.Hour || cfg.AuthAbsoluteTimeout != 48*time.Hour || cfg.AuthBootstrapEmail != "owner@example.test" {
		t.Fatalf("unexpected auth configuration: %#v", cfg)
	}
	t.Setenv("AUTH_SESSION_IDLE_TIMEOUT", "72h")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected idle timeout beyond absolute lifetime to fail")
	}
}
