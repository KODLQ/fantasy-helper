package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFullSyncPersistsSourceParityThroughPostgresQueue(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL sync integration tests")
	}
	if err := assertDisposableTestDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := OpenDatabase(ctx, Config{DatabaseURL: dsn, DatabaseMaxConns: 8, DatabaseMaxIdle: 4, DatabasePing: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewPostgresRepository(database, nil)
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `TRUNCATE sync_diagnostics, sync_stages, sync_runs, squad_lineups, squad_plan_players, squad_plans, player_gameweek_history, player_season_history, fixtures, players, teams, gameweeks, seasons RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bootstrap-static/":
			_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[{"id":1,"name":"Gameweek 1","is_current":true,"finished":false,"data_checked":false}],"phases":[],"game_settings":{},"element_types":[{"id":3,"singular_name":"Midfielder","plural_name":"Midfielders"}],"teams":[{"id":1,"name":"North","short_name":"NOR"},{"id":2,"name":"South","short_name":"SOU"}],"elements":[{"id":10,"first_name":"A","second_name":"One","web_name":"One","element_type":3,"team":1,"now_cost":75,"total_points":9,"form":"4.5","minutes":90,"value_form":"1.2","status":"a"},{"id":20,"first_name":"B","second_name":"Two","web_name":"Two","element_type":3,"team":2,"now_cost":65,"total_points":5,"form":"2.5","minutes":90,"value_form":"0.8","status":"a"}]}`))
		case "/fixtures/":
			_, _ = w.Write([]byte(`[{"id":100,"event":1,"finished":false,"team_h":1,"team_a":2,"team_h_difficulty":2,"team_a_difficulty":4,"stats":[{"identifier":"goals_scored","h":[{"element":10,"value":1}],"a":[]}]}]`))
		case "/event/1/live/":
			_, _ = w.Write([]byte(`{"finished":false,"elements":[{"id":10,"stats":{"minutes":90,"total_points":9,"goals_scored":1,"expected_goals":"0.80"}},{"id":20,"stats":{"minutes":90,"total_points":5,"expected_goals":"0.25"}}]}`))
		case "/element-summary/10/":
			_, _ = w.Write([]byte(`{"history":[{"element":10,"round":1,"fixture":100,"opponent_team":2,"was_home":true,"minutes":90,"total_points":9,"goals_scored":1,"value":75}]}`))
		case "/element-summary/20/":
			_, _ = w.Write([]byte(`{"history":[{"element":20,"round":1,"fixture":100,"opponent_team":1,"was_home":false,"minutes":90,"total_points":5,"value":65}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	scope := Scope{Dataset: "full", SeasonID: 2026, Gameweek: 1}
	runID, err := repository.StartSyncRun(ctx, scope, "integration-full-sync")
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	store.SetSyncStatus(SyncStatus{Status: "running", RunID: runID, Scope: scope, StartedAt: time.Now().UTC(), CompletedStages: []string{}, FailedStages: []string{}, Freshness: Freshness{Status: "unavailable", State: "unavailable"}})
	api := NewAPI(store, NewFPLSourceWithSeason(server.URL, 2026, "2026/27"), nil, nil, repository)
	api.SyncWorkers = 2
	api.runSync(ctx, scope, runID)
	status := store.SyncStatus()
	if status.Status != "success" || strings.Join(status.CompletedStages, ",") != "catalog,fixtures,live,player-history" {
		t.Fatalf("full sync did not complete: %#v", status)
	}

	counts := map[string]int{}
	for name, query := range map[string]string{
		"players":         `SELECT COUNT(*) FROM players`,
		"fixtures":        `SELECT COUNT(*) FROM fixtures`,
		"histories":       `SELECT COUNT(*) FROM player_gameweek_history`,
		"playerSnapshots": `SELECT COUNT(*) FROM player_snapshots`,
		"liveFacts":       `SELECT COUNT(*) FROM player_gameweek_facts`,
		"fixtureStats":    `SELECT COUNT(*) FROM fixture_stats`,
		"sourcePayloads":  `SELECT COUNT(*) FROM source_payloads`,
	} {
		var count int
		if err := database.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		counts[name] = count
	}
	if counts["players"] != 2 || counts["fixtures"] != 1 || counts["histories"] != 2 || counts["playerSnapshots"] != 2 || counts["liveFacts"] != 2 || counts["fixtureStats"] != 1 || counts["sourcePayloads"] != 5 {
		t.Fatalf("source/database count parity failed: %#v", counts)
	}
	var totalPoints, livePoints int
	if err := database.QueryRowContext(ctx, `SELECT total_points FROM players WHERE source_id=10`).Scan(&totalPoints); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT f.total_points FROM player_gameweek_facts f JOIN players p ON p.id=f.player_id WHERE p.source_id=10`).Scan(&livePoints); err != nil {
		t.Fatal(err)
	}
	if totalPoints != 9 || livePoints != 9 {
		t.Fatalf("selected metric parity failed: player=%d live=%d", totalPoints, livePoints)
	}
	var stages string
	if err := database.QueryRowContext(ctx, `SELECT string_agg(stage, ',' ORDER BY id) FROM sync_stages WHERE sync_run_id=$1`, runID).Scan(&stages); err != nil {
		t.Fatal(err)
	}
	if stages != "catalog,fixtures,live,player-history" {
		t.Fatalf("persisted stage order = %q", stages)
	}
	if status.RunID != runID || status.Scope != scope {
		t.Fatal(fmt.Sprintf("sync identity changed: %#v", status))
	}
	readAPI := NewAPI(NewStore(), NewFPLSourceWithSeason(server.URL, 2026, "2026/27"), nil, nil, repository)
	response := httptest.NewRecorder()
	readAPI.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/players/10", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"webName":"One"`) || strings.Contains(response.Body.String(), `"webName":"Fox"`) {
		t.Fatalf("production player read did not use PostgreSQL: status=%d body=%s", response.Code, response.Body.String())
	}
	analysisResponse := httptest.NewRecorder()
	readAPI.Handler().ServeHTTP(analysisResponse, httptest.NewRequest(http.MethodGet, "/api/v1/analysis/players/10?seasonId=2026&gameweek=1", nil))
	if analysisResponse.Code != http.StatusOK || !strings.Contains(analysisResponse.Body.String(), `"rollingPoints":9`) || !strings.Contains(analysisResponse.Body.String(), `"upcomingFixtures":[{"id":100`) {
		t.Fatalf("unexpected scoped analysis response: status=%d body=%s", analysisResponse.Code, analysisResponse.Body.String())
	}
	missingScopeResponse := httptest.NewRecorder()
	readAPI.Handler().ServeHTTP(missingScopeResponse, httptest.NewRequest(http.MethodGet, "/api/v1/analysis/players/10", nil))
	if missingScopeResponse.Code != http.StatusBadRequest {
		t.Fatalf("historical analysis accepted an implicit scope: status=%d body=%s", missingScopeResponse.Code, missingScopeResponse.Body.String())
	}
}
