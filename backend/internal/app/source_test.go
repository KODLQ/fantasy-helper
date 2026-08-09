package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSourceNormalizesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bootstrap-static/":
			_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[{"id":1,"name":"Gameweek 1","is_current":true}],"phases":[],"game_settings":{},"element_types":[{"id":3,"singular_name":"Midfielder","plural_name":"Midfielders"}],"teams":[{"id":1,"name":"Example","short_name":"EXA"}],"elements":[{"id":10,"first_name":"A","second_name":"Player","web_name":"Player","element_type":3,"team":1,"now_cost":55,"total_points":42,"form":"6.2","minutes":500,"value_form":"11.1","status":"a"}]}`))
		case "/fixtures/":
			_, _ = w.Write([]byte(`[{"id":99,"event":1,"team_h":1,"team_a":1,"team_h_difficulty":3,"team_a_difficulty":4,"finished":false}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	season, weeks, teams, players, fixtures, checksum, err := NewFPLSourceWithSeason(server.URL, 2026, "2026/27").Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if season.ID != 2026 || season.Name != "2026/27" || len(weeks) != 1 || len(teams) != 1 || len(players) != 1 || len(fixtures) != 1 || checksum == "" {
		t.Fatalf("unexpected normalized snapshot: %#v %#v %#v %#v %#v", season, weeks, teams, players, fixtures)
	}
	if players[0].Price != 5.5 || players[0].Form != 6.2 {
		t.Fatalf("player was not normalized: %#v", players[0])
	}
}

func TestSourceReportsRawObservations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[],"phases":[],"game_settings":{},"element_types":[],"teams":[],"elements":[]}`))
	}))
	defer server.Close()
	observations := []SourceObservation{}
	source := NewFPLSource(server.URL)
	source.OnObservation = func(observation SourceObservation) { observations = append(observations, observation) }
	if _, _, err := source.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 || observations[0].Endpoint != "/bootstrap-static/" || observations[0].ValidationState != "valid" || len(observations[0].Payload) == 0 || observations[0].Checksum == "" {
		t.Fatalf("unexpected source observation: %#v", observations)
	}
}

func TestSourceRequiresExplicitOrDiscoverableSeasonIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bootstrap-static/":
			_, _ = w.Write([]byte(`{"events":[],"teams":[],"elements":[]}`))
		case "/fixtures/":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if _, _, _, _, _, _, err := NewFPLSource(server.URL).Snapshot(context.Background()); err == nil {
		t.Fatal("expected a missing season identity error")
	}
}

func TestSourceRetriesRateLimitsAndServerErrors(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					if status == http.StatusTooManyRequests {
						w.Header().Set("Retry-After", "0")
					}
					w.WriteHeader(status)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[],"phases":[],"game_settings":{},"element_types":[],"teams":[],"elements":[]}`))
			}))
			defer server.Close()
			source := NewFPLSource(server.URL)
			source.Retries = 1
			if _, _, err := source.Bootstrap(context.Background()); err != nil {
				t.Fatal(err)
			}
			if attempts != 2 {
				t.Fatalf("attempts = %d, want 2", attempts)
			}
			metrics := source.Metrics()
			var expectedRateLimited, expectedServerErrors uint64
			if status == http.StatusTooManyRequests {
				expectedRateLimited = 1
			} else {
				expectedServerErrors = 1
			}
			if metrics.Requests != 2 || metrics.Retries != 1 || metrics.RateLimited != expectedRateLimited || metrics.ServerErrors != expectedServerErrors {
				t.Fatalf("unexpected source metrics: %#v", metrics)
			}
		})
	}
}

func TestSourceCapturesMalformedPayloadDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":`))
	}))
	defer server.Close()
	var observation SourceObservation
	source := NewFPLSource(server.URL)
	source.OnObservation = func(value SourceObservation) { observation = value }
	if _, _, err := source.Bootstrap(context.Background()); err == nil {
		t.Fatal("expected malformed payload error")
	}
	if observation.ValidationState != "invalid" || observation.Diagnostic == "" || observation.Checksum == "" {
		t.Fatalf("missing malformed diagnostic: %#v", observation)
	}
}

func TestSourceNormalizesFixtureLiveAndPlayerSummaryFeeds(t *testing.T) {
	fixtureQuery := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/fixtures/":
			fixtureQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`[{"id":99,"code":123,"event":4,"kickoff_time":"2026-09-12T15:00:00Z","finished":false,"provisional_start_time":false,"started":false,"team_h":1,"team_a":2,"team_h_difficulty":3,"team_a_difficulty":4,"stats":[{"identifier":"goals_scored","h":[{"element":10,"value":2}],"a":[]}]}]`))
		case "/event/4/live/":
			_, _ = w.Write([]byte(`{"finished":true,"elements":[{"id":10,"stats":{"minutes":90,"total_points":12,"goals_scored":2,"bps":88,"expected_goals":"1.20","expected_assists":"0.10"},"explain":[]}]}`))
		case "/element-summary/10/":
			_, _ = w.Write([]byte(`{"history":[{"element":10,"round":4,"fixture":99,"opponent_team":2,"was_home":true,"kickoff_time":"2026-09-12T15:00:00Z","minutes":90,"total_points":12,"goals_scored":2,"value":55}],"history_past":[{"season_name":"2025/26","total_points":120,"minutes":1800}],"fixtures":[{"id":100,"event":5,"kickoff_time":"2026-09-19T15:00:00Z","team_h":1,"team_a":3,"is_home":true,"difficulty":2}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewFPLSourceWithSeason(server.URL, 2026, "2026/27")
	fixtures, _, err := source.Fixtures(context.Background(), 4)
	if err != nil || fixtureQuery != "event=4" || len(fixtures.Fixtures) != 1 || len(fixtures.Fixtures[0].Stats) != 1 || fixtures.Fixtures[0].Stats[0].Home[0].Value != 2 {
		t.Fatalf("unexpected fixture feed: %#v err=%v", fixtures, err)
	}
	live, _, err := source.EventLive(context.Background(), 4)
	if err != nil || live.Finalized == nil || !*live.Finalized || len(live.Elements) != 1 || live.Elements[0].PlayerID != 10 || live.Elements[0].BPS != 88 {
		t.Fatalf("unexpected live feed: %#v err=%v", live, err)
	}
	summary, _, err := source.ElementSummary(context.Background(), 10)
	if err != nil || len(summary.History) != 1 || summary.History[0].Fixture != 99 || len(summary.HistoryPast) != 1 || len(summary.Fixtures) != 1 {
		t.Fatalf("unexpected element summary: %#v err=%v", summary, err)
	}
}

func TestSourceBoundsPerHostConcurrency(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		for observed := peak.Load(); current > observed && !peak.CompareAndSwap(observed, current); observed = peak.Load() {
		}
		<-release
		active.Add(-1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[],"phases":[],"game_settings":{},"element_types":[],"teams":[],"elements":[]}`))
	}))
	defer server.Close()
	source := NewFPLSource(server.URL)
	source.SetMaxConcurrent(2)
	source.RetryJitter = 0
	var workers sync.WaitGroup
	for index := 0; index < 6; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, _, err := source.Bootstrap(context.Background()); err != nil {
				t.Errorf("bootstrap failed: %v", err)
			}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for peak.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(release)
	workers.Wait()
	metrics := source.Metrics()
	if peak.Load() != 2 || metrics.PeakConcurrent != 2 || metrics.InFlight != 0 || metrics.Requests != 6 {
		t.Fatalf("concurrency was not bounded: serverPeak=%d metrics=%#v", peak.Load(), metrics)
	}
}
