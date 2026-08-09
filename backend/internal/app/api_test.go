package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
			Items []Player `json:"items"`
			Total int      `json:"total"`
		} `json:"data"`
		Meta ResponseMeta `json:"meta"`
	}
	if err := json.NewDecoder(research.Body).Decode(&researchBody); err != nil {
		t.Fatal(err)
	}
	if len(researchBody.Data.Items) != 3 || researchBody.Data.Total < 3 || researchBody.Meta.RequestID == "" || researchBody.Meta.Freshness.State == "" {
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
