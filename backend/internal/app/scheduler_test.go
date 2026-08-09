package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

type recordingSyncStarter struct {
	scopes       []Scope
	correlations []string
}

func (r *recordingSyncStarter) StartScopedSync(_ context.Context, scope Scope, correlationID string) (SyncStatus, error) {
	r.scopes = append(r.scopes, scope)
	r.correlations = append(r.correlations, correlationID)
	return SyncStatus{Status: "running", Scope: scope}, nil
}

func TestSyncSchedulerAppliesCadenceAndPriority(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	starter := &recordingSyncStarter{}
	scheduler := NewSyncScheduler(starter, func() Gameweek { return Gameweek{ID: 3, IsCurrent: true} }, SyncSchedule{Tick: time.Minute, Catalog: time.Hour, Fixtures: time.Hour, Live: 5 * time.Minute, Finalization: 10 * time.Minute, HistoricalReconcile: 24 * time.Hour})
	scheduler.now = func() time.Time { return now }

	for range 4 {
		if !scheduler.runDue(context.Background()) {
			t.Fatal("expected an initial due policy")
		}
	}
	if got := []string{starter.scopes[0].Dataset, starter.scopes[1].Dataset, starter.scopes[2].Dataset, starter.scopes[3].Dataset}; strings.Join(got, ",") != "live,fixtures,catalog,player-history" {
		t.Fatalf("unexpected policy priority: %v", got)
	}
	if scheduler.runDue(context.Background()) {
		t.Fatal("no policy should be due again at the same instant")
	}
	now = now.Add(4 * time.Minute)
	if scheduler.runDue(context.Background()) {
		t.Fatal("live policy ran before its cadence elapsed")
	}
	now = now.Add(2 * time.Minute)
	if !scheduler.runDue(context.Background()) || starter.scopes[len(starter.scopes)-1].Dataset != "live" {
		t.Fatal("live policy did not run after its cadence elapsed")
	}
	if !strings.HasPrefix(starter.correlations[0], "scheduler:live:") {
		t.Fatalf("missing scheduler correlation identity: %q", starter.correlations[0])
	}
}

func TestSyncSchedulerUsesFinalizationPolicyForFinishedGameweek(t *testing.T) {
	starter := &recordingSyncStarter{}
	scheduler := NewSyncScheduler(starter, func() Gameweek { return Gameweek{ID: 8, IsCurrent: true, Finished: true} }, SyncSchedule{Tick: time.Minute, Catalog: time.Hour, Fixtures: time.Hour, Live: 5 * time.Minute, Finalization: 10 * time.Minute, HistoricalReconcile: 24 * time.Hour})
	scheduler.now = func() time.Time { return time.Date(2026, time.August, 9, 18, 0, 0, 0, time.UTC) }
	if !scheduler.runDue(context.Background()) || len(starter.scopes) != 1 || starter.scopes[0].Dataset != "live" || starter.scopes[0].Gameweek != 8 {
		t.Fatalf("unexpected finalization scope: %#v", starter.scopes)
	}
	if !strings.HasPrefix(starter.correlations[0], "scheduler:finalization:") {
		t.Fatalf("finalization policy was not identified: %q", starter.correlations[0])
	}
}
