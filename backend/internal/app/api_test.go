package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type failingSnapshotRepository struct{}

type catalogueTestRepository struct {
	archiveTestRepository
	items []SeasonCatalogueItem
	err   error
}

func (repository *catalogueTestRepository) ListSeasons(context.Context) ([]SeasonCatalogueItem, error) {
	return repository.items, repository.err
}

func (repository *catalogueTestRepository) LoadSnapshotForSeason(context.Context, int) (Snapshot, bool, error) {
	return repository.snapshot, repository.snapshot.Season.ID > 0, nil
}

func (failingSnapshotRepository) EnsureSchema(context.Context) error { return nil }
func (failingSnapshotRepository) LoadSnapshot(context.Context) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}
func (failingSnapshotRepository) LoadSquad(context.Context) (Squad, bool, error) {
	return Squad{}, false, nil
}
func (failingSnapshotRepository) SaveSquad(context.Context, Squad) error { return nil }
func (failingSnapshotRepository) UpsertSnapshot(context.Context, Snapshot) error {
	return fmt.Errorf("forced database failure")
}
func (failingSnapshotRepository) RecordSyncStatus(context.Context, SyncStatus) error { return nil }

func TestAPIResearchSquadAndRecommendationFlow(t *testing.T) {
	store := NewStore()
	api := NewAPI(store, NewFPLSource("http://127.0.0.1:1"), nil, nil)
	handler := api.Handler()

	research := httptest.NewRecorder()
	handler.ServeHTTP(research, httptest.NewRequest(http.MethodGet, "/api/v1/players?sort=form&direction=desc&pageSize=3", nil))
	if research.Code != http.StatusOK {
		t.Fatalf("research status = %d", research.Code)
	}
	var researchBody struct {
		Data struct {
			Items []PlayerResearchItem `json:"items"`
			Teams []Team               `json:"teams"`
			Total int                  `json:"total"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	if err := json.NewDecoder(research.Body).Decode(&researchBody); err != nil {
		t.Fatal(err)
	}
	if len(researchBody.Data.Items) != 3 || len(researchBody.Data.Teams) != 5 || researchBody.Data.Items[0].Player.ID == 0 || researchBody.Data.Items[0].Player.TeamID != researchBody.Data.Items[0].Team.ID || researchBody.Data.Total < 3 || researchBody.Meta.RequestID == "" || researchBody.Meta.Freshness.State == "" {
		t.Fatalf("unexpected research response: %#v", researchBody)
	}

	body, _ := json.Marshal(demoSquad())
	save := httptest.NewRecorder()
	handler.ServeHTTP(save, httptest.NewRequest(http.MethodPut, "/api/v1/squad", bytes.NewReader(body)))
	if save.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", save.Code, save.Body.String())
	}

	recommendation := httptest.NewRecorder()
	handler.ServeHTTP(recommendation, httptest.NewRequest(http.MethodPost, "/api/v1/recommendations", bytes.NewBufferString(`{}`)))
	if recommendation.Code != http.StatusOK {
		t.Fatalf("recommendation status = %d, body = %s", recommendation.Code, recommendation.Body.String())
	}
	var recommendationBody struct {
		Data struct {
			Recommendation Recommendation `json:"recommendation"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	if err := json.NewDecoder(recommendation.Body).Decode(&recommendationBody); err != nil {
		t.Fatal(err)
	}
	if recommendationBody.Data.Recommendation.AlgorithmVersion == "" || recommendationBody.Meta.Freshness.State == "" {
		t.Fatalf("recommendation response lost common freshness metadata: %#v", recommendationBody)
	}
}

func TestAPIRejectsMoreThanFourComparisonPlayers(t *testing.T) {
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil)
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/players/compare?ids=1,2,3,4,5", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestAPIHandlesEmptyAndUnknownResearchResults(t *testing.T) {
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil)
	handler := api.Handler()
	empty := httptest.NewRecorder()
	handler.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/api/v1/players?search=does-not-exist", nil))
	if empty.Code != http.StatusOK || !bytes.Contains(empty.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("unexpected empty result: %d %s", empty.Code, empty.Body.String())
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/v1/players/9999", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown player status = %d", unknown.Code)
	}
}

func TestAPIRejectsMissingPlayerTeamRelationship(t *testing.T) {
	store := NewStore()
	store.mu.Lock()
	delete(store.teams, 1)
	store.mu.Unlock()
	recorder := httptest.NewRecorder()
	NewAPI(store, NewFPLSource("http://127.0.0.1:1"), nil, nil).Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/players?search=Mason", nil))
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":"player_team_inconsistent"`) {
		t.Fatalf("unexpected inconsistent team response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAPIListsSeasonsAndRejectsUnknownExplicitSeason(t *testing.T) {
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil)
	seasons := httptest.NewRecorder()
	api.Handler().ServeHTTP(seasons, httptest.NewRequest(http.MethodGet, "/api/v1/seasons", nil))
	if seasons.Code != http.StatusOK || !strings.Contains(seasons.Body.String(), `"items":[{"id":1`) || !strings.Contains(seasons.Body.String(), `"defaultGameweek":1`) {
		t.Fatalf("unexpected season catalogue: status=%d body=%s", seasons.Code, seasons.Body.String())
	}
	unknown := httptest.NewRecorder()
	api.Handler().ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/v1/players?seasonId=999", nil))
	if unknown.Code != http.StatusNotFound || !strings.Contains(unknown.Body.String(), `"code":"SEASON_NOT_FOUND"`) {
		t.Fatalf("unexpected unknown season response: status=%d body=%s", unknown.Code, unknown.Body.String())
	}
}

func TestAPISeasonContractCoversEmptyPartialUnavailableAndOmittedScope(t *testing.T) {
	current := SeasonCatalogueItem{ID: 2026, Name: "2026/27", State: SeasonCurrent, SourceKind: SourceOfficialCurrent, Freshness: Freshness{State: "actual", Status: "fresh"}, Completeness: map[string]interface{}{"catalogue": true}, MissingInputs: []string{}, Warnings: []string{}}
	partial := SeasonCatalogueItem{ID: 2025, Name: "2025/26", State: SeasonHistorical, SourceKind: SourceHistoricalArchive, Freshness: Freshness{State: "partial", Status: "partial"}, Completeness: map[string]interface{}{"catalogue": true}, MissingInputs: []string{"live"}, Warnings: []string{"live unavailable"}}
	unavailable := SeasonCatalogueItem{ID: 2024, Name: "2024/25", State: SeasonHistorical, SourceKind: SourceHistoricalArchive, Freshness: Freshness{State: "unavailable", Status: "unavailable"}, Completeness: map[string]interface{}{"catalogue": false}, MissingInputs: []string{"catalogue"}, Warnings: []string{}}
	repository := &catalogueTestRepository{items: []SeasonCatalogueItem{current, partial, unavailable}}
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil, repository)

	listed := httptest.NewRecorder()
	api.Handler().ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/v1/seasons", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"id":2026`) || !strings.Contains(listed.Body.String(), `"state":"partial"`) || !strings.Contains(listed.Body.String(), `"missingInputs":["catalogue"]`) {
		t.Fatalf("partial/unavailable catalogue contract failed: %d %s", listed.Code, listed.Body.String())
	}
	unavailableResponse := httptest.NewRecorder()
	api.Handler().ServeHTTP(unavailableResponse, httptest.NewRequest(http.MethodGet, "/api/v1/players?seasonId=2024", nil))
	if unavailableResponse.Code != http.StatusConflict || !strings.Contains(unavailableResponse.Body.String(), `"code":"SEASON_DATA_UNAVAILABLE"`) {
		t.Fatalf("unavailable season contract failed: %d %s", unavailableResponse.Code, unavailableResponse.Body.String())
	}

	emptyAPI := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil, &catalogueTestRepository{})
	empty := httptest.NewRecorder()
	emptyAPI.Handler().ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/api/v1/seasons", nil))
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"items":[]`) || !strings.Contains(empty.Body.String(), `"state":"unavailable"`) {
		t.Fatalf("empty catalogue contract failed: %d %s", empty.Code, empty.Body.String())
	}

	omitted := httptest.NewRecorder()
	NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil).Handler().ServeHTTP(omitted, httptest.NewRequest(http.MethodGet, "/api/v1/players", nil))
	if omitted.Code != http.StatusOK || !strings.Contains(omitted.Body.String(), `seasonId was omitted`) || !strings.Contains(omitted.Body.String(), `"seasonId":1`) {
		t.Fatalf("omitted season compatibility contract failed: %d %s", omitted.Code, omitted.Body.String())
	}
}

func TestSyncPolicyRejectsHistoricalAndMismatchedSources(t *testing.T) {
	current := NewFPLSourceWithSeason("http://127.0.0.1:1", 2026, "2026/27")
	current.Kind = SourceOfficialCurrent
	api := NewAPI(NewStore(), current, nil, nil)
	if _, err := api.StartScopedSync(context.Background(), Scope{SeasonID: 2025, Dataset: "full"}, "test"); err == nil || err.(SyncPolicyError).Code != "HISTORICAL_SOURCE_UNAVAILABLE" {
		t.Fatalf("expected mismatched historical season to be rejected, got %v", err)
	}
	historical := NewFPLSourceWithSeason("archive://local", 2025, "2025/26")
	historical.Kind = SourceHistoricalArchive
	api = NewAPI(NewStore(), historical, nil, nil)
	if _, err := api.StartScopedSync(context.Background(), Scope{SeasonID: 2025, Dataset: "live"}, "test"); err == nil || err.(SyncPolicyError).Code != "HISTORICAL_LIVE_SYNC_FORBIDDEN" {
		t.Fatalf("expected historical live refresh to be rejected, got %v", err)
	}
}

func TestDatasetSnapshotsUseCommonResponseEnvelope(t *testing.T) {
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil)
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/data/snapshots", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Data struct {
			Items []DatasetSnapshot `json:"items"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Items) != 1 || body.Meta.RequestID == "" || body.Meta.Freshness.Status == "" || body.Meta.Freshness.State == "" {
		t.Fatalf("unexpected common response: %#v", body)
	}
}

func TestDatasetSnapshotsUseCommonErrorEnvelope(t *testing.T) {
	api := NewAPI(NewStore(), NewFPLSource("http://127.0.0.1:1"), nil, nil)
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/data/snapshots", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Error ResponseError `json:"error"`
		Meta  ResponseMeta  `json:"meta"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "method_not_allowed" || body.Error.Retryable || body.Meta.RequestID == "" {
		t.Fatalf("unexpected common error response: %#v", body)
	}
}

func TestResponseMetaSupportsProvenancePaginationAndCoverage(t *testing.T) {
	meta := ResponseMeta{RequestID: "req-test", Scope: Scope{SeasonID: 2026, Gameweek: 1}, Provenance: []string{"snapshot-1", "normalizer:fpl-public-v1"}, Pagination: &Pagination{Limit: 25, Offset: 0, Returned: 2, Total: 2}, Coverage: &Coverage{Complete: false, MissingIDs: []string{"player:99"}, Warning: "one source item is unavailable"}}
	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ResponseMeta
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Provenance) != 2 || decoded.Pagination == nil || decoded.Pagination.Returned != 2 || decoded.Coverage == nil || decoded.Coverage.Complete {
		t.Fatalf("metadata contract lost fields: %#v", decoded)
	}
}

func TestAPISyncCoordinatorCancelsAndWaits(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bootstrap-static/" {
			started <- struct{}{}
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	source := NewFPLSourceWithSeason(server.URL, 2026, "2026/27")
	api := NewAPI(NewStore(), source, nil, nil)
	api.startSync(Scope{Dataset: "full"}, 0)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := api.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	metrics := api.Metrics()
	if metrics.Started != 1 || metrics.Cancelled != 1 {
		t.Fatalf("unexpected sync metrics: %#v", metrics)
	}
}

func TestAPISyncRunsDependencyOrderedStages(t *testing.T) {
	var mu sync.Mutex
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.RequestURI())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bootstrap-static/":
			_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[{"id":1,"name":"Gameweek 1","is_current":true}],"phases":[],"game_settings":{},"element_types":[{"id":1,"singular_name":"Goalkeeper","plural_name":"Goalkeepers"}],"teams":[{"id":1,"name":"Home","short_name":"HOM"},{"id":2,"name":"Away","short_name":"AWY"}],"elements":[{"id":10,"first_name":"A","second_name":"Keeper","web_name":"Keeper","element_type":1,"team":1,"now_cost":50,"form":"1.0","value_form":"1.0","status":"a"}]}`))
		case "/fixtures/":
			_, _ = w.Write([]byte(`[{"id":99,"event":1,"team_h":1,"team_a":2,"team_h_difficulty":2,"team_a_difficulty":4,"stats":[]}]`))
		case "/event/1/live/":
			_, _ = w.Write([]byte(`{"finished":false,"elements":[{"id":10,"stats":{"minutes":90,"total_points":6}}]}`))
		case "/element-summary/10/":
			_, _ = w.Write([]byte(`{"history":[{"element":10,"round":1,"fixture":99,"opponent_team":2,"was_home":true,"minutes":90,"total_points":6}],"history_past":[],"fixtures":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	api := NewAPI(NewStore(), NewFPLSourceWithSeason(server.URL, 2026, "2026/27"), nil, nil)
	api.runSync(context.Background(), Scope{Dataset: "full"}, 0)
	status := api.Store.SyncStatus()
	if status.Status != "success" || strings.Join(status.CompletedStages, ",") != "catalog,fixtures,live,player-history" {
		t.Fatalf("unexpected stage result: %#v", status)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 4 || requests[0] != "/bootstrap-static/" || requests[1] != "/fixtures/" || requests[2] != "/event/1/live/" || requests[3] != "/element-summary/10/" {
		t.Fatalf("source requests were not dependency ordered: %#v", requests)
	}
}

func TestMergeFixturesReplacesOnlyRequestedGameweek(t *testing.T) {
	merged := mergeFixtures([]Fixture{{ID: 1, Gameweek: 1}, {ID: 2, Gameweek: 2}}, []Fixture{{ID: 3, Gameweek: 2}}, 2)
	if len(merged) != 2 || merged[0].ID != 1 || merged[1].ID != 3 {
		t.Fatalf("unexpected scoped fixture merge: %#v", merged)
	}
}

func TestLiveFinalizedUsesExplicitSourceThenCheckedEvent(t *testing.T) {
	value := false
	gameweek := 1
	finishedFixtures := []SourceFixture{{ID: 1, Event: &gameweek, Finished: true}}
	unfinishedFixtures := []SourceFixture{{ID: 1, Event: &gameweek, Finished: false}}
	if liveFinalized([]SourceEvent{{ID: 1}}, finishedFixtures, 1, &value, true) {
		t.Fatal("explicit provisional source state must remain provisional")
	}
	if liveFinalized([]SourceEvent{{ID: 1, Finished: true, DataChecked: true}}, unfinishedFixtures, 1, nil, true) {
		t.Fatal("unfinished fixtures must prevent finalization")
	}
	if liveFinalized([]SourceEvent{{ID: 1, Finished: true, DataChecked: true}}, finishedFixtures, 1, nil, false) {
		t.Fatal("changed facts must require a confirming refresh")
	}
	if !liveFinalized([]SourceEvent{{ID: 1, Finished: true, DataChecked: true}}, finishedFixtures, 1, nil, true) {
		t.Fatal("stable checked event with finished fixtures should finalize")
	}
}

func TestSyncRefreshesCacheOnlyAfterDatabaseCommit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/bootstrap-static/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"season_id":2026,"season_name":"2026/27","events":[{"id":1,"name":"Gameweek 1","is_current":true}],"phases":[],"game_settings":{},"element_types":[{"id":1,"singular_name":"Goalkeeper","plural_name":"Goalkeepers"}],"teams":[{"id":1,"name":"Home","short_name":"HOM"}],"elements":[{"id":1000,"first_name":"New","second_name":"Player","web_name":"New","element_type":1,"team":1,"now_cost":50,"form":"1","value_form":"1","status":"a"}]}`))
	}))
	defer server.Close()
	store := NewStore()
	store.SetSyncStatus(SyncStatus{Status: "running", Scope: Scope{Dataset: "catalog"}, CompletedStages: []string{}, FailedStages: []string{}, Freshness: store.Freshness()})
	api := NewAPI(store, NewFPLSourceWithSeason(server.URL, 2026, "2026/27"), nil, nil, failingSnapshotRepository{})
	api.runSync(context.Background(), Scope{Dataset: "catalog"}, 0)
	if _, found := store.Player(1000); found {
		t.Fatal("uncommitted source data leaked into the cache")
	}
	if status := store.SyncStatus(); status.Status != "failed" || !strings.Contains(status.Warning, "database snapshot persistence failed") {
		t.Fatalf("unexpected failed sync status: %#v", status)
	}
}
