package app

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (a *API) loadResearchSnapshot(r *http.Request, seasonID, gameweek int) (ResearchSnapshot, bool, error) {
	if repository, ok := a.Repository.(TransferResearchRepository); ok {
		return repository.LoadResearchSnapshotAtCutoff(r.Context(), seasonID, gameweek)
	}
	item, err := researchSnapshotFromStore(a.Store, seasonID, gameweek)
	return item, err == nil, err
}

func researchSnapshotFreshness(snapshot ResearchSnapshot) Freshness {
	return Freshness{Dataset: "public-fpl", State: snapshot.State, Status: snapshot.State, SnapshotIDs: []string{snapshot.ID}, SourceFetchedAt: snapshot.ObservedAt, SnapshotAt: snapshot.ObservedAt, MissingInputs: append([]string{}, snapshot.MissingInputs...)}
}

func (a *API) transferSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST to simulate transfers.", nil)
		return
	}
	session, ok := a.requireMutationAuth(w, r)
	if !ok {
		return
	}
	var input TransferSimulationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", nil)
		return
	}
	if err := validateResearchRange(input.Gameweek, input.Horizon); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_research_range", err.Error(), nil)
		return
	}
	season, warnings, err := a.resolveSeason(r.Context(), input.SeasonID)
	if err != nil {
		writeSeasonResolutionError(w, requestIDFrom(w), err)
		return
	}
	input.SeasonID = season.ID
	snapshot, found, err := a.loadResearchSnapshot(r, season.ID, input.Gameweek)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "research_unavailable", "Transfer research is temporarily unavailable.", nil)
		return
	}
	if !found {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_snapshot_unavailable", "No warehouse snapshot exists at or before the selected gameweek deadline.", nil)
		return
	}
	if snapshot.State == "unavailable" {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_inputs_unavailable", "The warehouse cannot prove every required input existed before the selected deadline.", nil)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), season.ID, session.User.ID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "squad_required", "Save a valid squad before simulating transfers.", nil)
		return
	}
	domain.ApplySnapshot(snapshot.Snapshot.Season, snapshot.Snapshot.Gameweeks, snapshot.Snapshot.Teams, snapshot.Snapshot.Players, snapshot.Snapshot.Fixtures, snapshot.Snapshot.Histories)
	result, validation := simulateTransfers(domain, snapshot, input)
	if len(validation) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "simulation_invalid", "The transfer scenario is not a legal FPL plan.", validation)
		return
	}
	warnings = append(warnings, snapshot.MissingInputs...)
	writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), Scope{SeasonID: season.ID, Gameweek: input.Gameweek, Dataset: "public-fpl"}, researchSnapshotFreshness(snapshot), warnings, result)
}

func (a *API) fixtureSwing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET for fixture research.", nil)
		return
	}
	seasonID, gameweek, horizon := parseInt(r.URL.Query().Get("seasonId"), 0), parseInt(r.URL.Query().Get("gameweek"), 0), parseInt(r.URL.Query().Get("horizon"), 5)
	if err := validateResearchRange(gameweek, horizon); err != nil || seasonID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_research_range", "seasonId, gameweek 1-38, and horizon 1-8 are required.", nil)
		return
	}
	if _, _, err := a.resolveSeason(r.Context(), seasonID); err != nil {
		writeSeasonResolutionError(w, requestIDFrom(w), err)
		return
	}
	snapshot, found, err := a.loadResearchSnapshot(r, seasonID, gameweek)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "research_unavailable", "Fixture research is temporarily unavailable.", nil)
		return
	}
	if !found {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_snapshot_unavailable", "No warehouse snapshot exists at or before the selected gameweek deadline.", nil)
		return
	}
	result := calculateFixtureResearch(snapshot, gameweek, horizon)
	writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "public-fpl"}, researchSnapshotFreshness(snapshot), result.MissingInputs, result)
}

func (a *API) differentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET for differential research.", nil)
		return
	}
	q := r.URL.Query()
	seasonID, gameweek, horizon := parseInt(q.Get("seasonId"), 0), parseInt(q.Get("gameweek"), 0), parseInt(q.Get("horizon"), 5)
	if err := validateResearchRange(gameweek, horizon); err != nil || seasonID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_research_range", "seasonId, gameweek 1-38, and horizon 1-8 are required.", nil)
		return
	}
	position, minMinutes, limit := parseInt(q.Get("position"), 0), parseInt(q.Get("minMinutes"), 180), parseInt(q.Get("limit"), 25)
	if position < 0 || position > 4 || minMinutes < 180 || limit < 1 || limit > 50 {
		writeError(w, http.StatusBadRequest, "invalid_differential_filters", "Position must be 1-4, minimum minutes at least 180, and limit 1-50.", nil)
		return
	}
	minPrice, maxPrice := parseFloatParam(q.Get("minPrice")), parseFloatParam(q.Get("maxPrice"))
	maxOwnership := 10.0
	if q.Has("maxOwnership") {
		maxOwnership = parseFloatParam(q.Get("maxOwnership"))
	}
	if minPrice < 0 || maxPrice < 0 || (maxPrice > 0 && maxPrice < minPrice) || maxOwnership < 0 || maxOwnership > 100 {
		writeError(w, http.StatusBadRequest, "invalid_differential_filters", "Price and ownership filters are outside their supported range.", nil)
		return
	}
	snapshot, found, err := a.loadResearchSnapshot(r, seasonID, gameweek)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "research_unavailable", "Differential research is temporarily unavailable.", nil)
		return
	}
	if !found {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_snapshot_unavailable", "No warehouse snapshot exists at or before the selected gameweek deadline.", nil)
		return
	}
	fixture := calculateFixtureResearch(snapshot, gameweek, horizon)
	result := calculateDifferentials(snapshot, fixture, position, minPrice, maxPrice, maxOwnership, minMinutes, limit)
	writeEnvelopeWithWarnings(w, http.StatusOK, requestIDFrom(w), Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "public-fpl"}, researchSnapshotFreshness(snapshot), result.MissingInputs, result)
}

func (a *API) planningScenarios(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST to save a confirmed scenario.", nil)
		return
	}
	session, ok := a.requireMutationAuth(w, r)
	if !ok {
		return
	}
	var body struct {
		Name       string                  `json:"name"`
		Confirmed  bool                    `json:"confirmed"`
		Simulation TransferSimulationInput `json:"simulation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", nil)
		return
	}
	if !body.Confirmed {
		writeError(w, http.StatusUnprocessableEntity, "confirmation_required", "Confirm the scenario before saving it.", nil)
		return
	}
	if strings.TrimSpace(body.Name) == "" || len(strings.TrimSpace(body.Name)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_scenario_name", "Scenario name must contain 1-100 characters.", nil)
		return
	}
	if err := validateResearchRange(body.Simulation.Gameweek, body.Simulation.Horizon); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_research_range", err.Error(), nil)
		return
	}
	snapshot, found, err := a.loadResearchSnapshot(r, body.Simulation.SeasonID, body.Simulation.Gameweek)
	if err != nil || !found {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_snapshot_unavailable", "No warehouse snapshot exists at or before the selected gameweek deadline.", nil)
		return
	}
	if snapshot.State == "unavailable" {
		writeError(w, http.StatusUnprocessableEntity, "cutoff_inputs_unavailable", "The warehouse cannot prove every required input existed before the selected deadline.", nil)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), body.Simulation.SeasonID, session.User.ID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "squad_required", "Save a valid squad before saving a scenario.", nil)
		return
	}
	domain.ApplySnapshot(snapshot.Snapshot.Season, snapshot.Snapshot.Gameweeks, snapshot.Snapshot.Teams, snapshot.Snapshot.Players, snapshot.Snapshot.Fixtures, snapshot.Snapshot.Histories)
	result, validation := simulateTransfers(domain, snapshot, body.Simulation)
	if len(validation) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "simulation_invalid", "The transfer scenario is not a legal FPL plan.", validation)
		return
	}
	repository, ok := a.Repository.(TransferResearchRepository)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "planning_persistence_unavailable", "Scenario saving requires PostgreSQL.", nil)
		return
	}
	item, err := repository.SavePlanningScenario(r.Context(), session.User.ID, strings.TrimSpace(body.Name), body.Simulation, result)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scenario_save_failed", "The confirmed scenario could not be saved.", nil)
		return
	}
	writeEnvelope(w, http.StatusCreated, requestIDFrom(w), Scope{SeasonID: body.Simulation.SeasonID, Gameweek: body.Simulation.Gameweek, Dataset: "public-fpl"}, researchSnapshotFreshness(snapshot), item)
}
