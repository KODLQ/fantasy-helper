package app

import (
	"context"
	"fmt"
	"time"
)

type SyncStarter interface {
	StartScopedSync(context.Context, Scope, string) (SyncStatus, error)
}

type SyncSchedule struct {
	Tick                time.Duration
	Catalog             time.Duration
	Fixtures            time.Duration
	Live                time.Duration
	Finalization        time.Duration
	HistoricalReconcile time.Duration
}

type scheduledPolicy struct {
	name     string
	interval time.Duration
	scope    Scope
	eligible func(Gameweek) bool
}

type SyncScheduler struct {
	starter         SyncStarter
	currentGameweek func() Gameweek
	schedule        SyncSchedule
	lastAttempt     map[string]time.Time
	now             func() time.Time
}

func NewSyncScheduler(starter SyncStarter, currentGameweek func() Gameweek, schedule SyncSchedule) *SyncScheduler {
	return &SyncScheduler{starter: starter, currentGameweek: currentGameweek, schedule: schedule, lastAttempt: map[string]time.Time{}, now: time.Now}
}

func (s *SyncScheduler) Run(ctx context.Context) {
	if s == nil || s.starter == nil || s.schedule.Tick <= 0 {
		return
	}
	ticker := time.NewTicker(s.schedule.Tick)
	defer ticker.Stop()
	s.runDue(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDue(ctx)
		}
	}
}

func (s *SyncScheduler) runDue(ctx context.Context) bool {
	now := s.now().UTC()
	gameweek := s.currentGameweek()
	for _, policy := range s.policies(gameweek) {
		if policy.interval <= 0 || !policy.eligible(gameweek) || now.Sub(s.lastAttempt[policy.name]) < policy.interval {
			continue
		}
		s.lastAttempt[policy.name] = now
		policy.scope.Gameweek = gameweek.ID
		_, err := s.starter.StartScopedSync(ctx, policy.scope, fmt.Sprintf("scheduler:%s:%d", policy.name, now.Unix()))
		return err == nil
	}
	return false
}

func (s *SyncScheduler) policies(gameweek Gameweek) []scheduledPolicy {
	always := func(Gameweek) bool { return true }
	return []scheduledPolicy{
		{name: "finalization", interval: s.schedule.Finalization, scope: Scope{Dataset: "live"}, eligible: func(item Gameweek) bool { return item.ID > 0 && item.Finished }},
		{name: "live", interval: s.schedule.Live, scope: Scope{Dataset: "live"}, eligible: func(item Gameweek) bool { return item.ID > 0 && !item.Finished }},
		{name: "fixtures", interval: s.schedule.Fixtures, scope: Scope{Dataset: "fixtures"}, eligible: always},
		{name: "catalog", interval: s.schedule.Catalog, scope: Scope{Dataset: "catalog"}, eligible: always},
		{name: "historical-reconciliation", interval: s.schedule.HistoricalReconcile, scope: Scope{Dataset: "player-history"}, eligible: always},
	}
}
