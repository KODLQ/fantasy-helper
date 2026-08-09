package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSourceNormalizesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bootstrap-static/":
			_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[{"id":1,"name":"Gameweek 1","is_current":true}],"teams":[{"id":1,"name":"Example","short_name":"EXA"}],"elements":[{"id":10,"first_name":"A","second_name":"Player","web_name":"Player","element_type":3,"team":1,"now_cost":55,"total_points":42,"form":"6.2","minutes":500,"value_form":"11.1","status":"a"}]}`))
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
		_, _ = w.Write([]byte(`{"events":[],"teams":[],"elements":[]}`))
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
				_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[],"teams":[],"elements":[]}`))
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
