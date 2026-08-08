package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
		Items []Player `json:"items"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(research.Body).Decode(&researchBody); err != nil {
		t.Fatal(err)
	}
	if len(researchBody.Items) != 3 || researchBody.Total < 3 {
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
