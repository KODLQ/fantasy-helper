package app

import (
	"context"
	"log/slog"
	"time"
)

func RunRetentionCleanup(ctx context.Context, repository RetentionRepository, cadence, successfulRetention, finalizedLiveRetention time.Duration, logger *slog.Logger) {
	if repository == nil || cadence <= 0 || successfulRetention <= 0 || finalizedLiveRetention <= 0 {
		return
	}
	cleanup := func() {
		now := time.Now().UTC()
		count, err := repository.CleanupSourcePayloads(ctx, now.Add(-successfulRetention), now.Add(-finalizedLiveRetention))
		if logger == nil {
			return
		}
		if err != nil {
			logger.Warn("source payload retention cleanup failed", "error", err)
			return
		}
		logger.Info("source payload retention cleanup completed", "purgedBodies", count)
	}
	cleanup()
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
