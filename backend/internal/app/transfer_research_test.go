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

type unavailableTransferResearchRepository struct {
	archiveTestRepository
	research ResearchSnapshot
}

func (r *unavailableTransferResearchRepository) LoadResearchSnapshotAtCutoff(context.Context, int, int) (ResearchSnapshot, bool, error) {
	return r.research, true, nil
}
func (r *unavailableTransferResearchRepository) SavePlanningScenario(context.Context, int64, string, TransferSimulationInput, TransferSimulation) (PlanningScenario, error) {
	return PlanningScenario{}, nil
}

func transferResearchSnapshot(t *testing.T) (ResearchSnapshot, *Store) {
	t.Helper()
	store := NewStore()
	base := store.ExportSnapshot()
	deadline := time.Date(2026, 8, 14, 17, 30, 0, 0, time.UTC)
	base.Gameweeks = []Gameweek{{ID: 3, Name: "Gameweek 3", DeadlineTime: &deadline}, {ID: 4, Name: "Gameweek 4"}, {ID: 5, Name: "Gameweek 5"}}
	base.Fixtures = []Fixture{
		{ID: 1, Gameweek: 3, HomeTeam: 1, AwayTeam: 2, HomeDifficulty: 2, AwayDifficulty: 4},
		{ID: 2, Gameweek: 4, HomeTeam: 3, AwayTeam: 1, HomeDifficulty: 3, AwayDifficulty: 3},
		{ID: 3, Gameweek: 4, HomeTeam: 1, AwayTeam: 4, HomeDifficulty: 2, AwayDifficulty: 4},
		{ID: 4, Gameweek: 3, HomeTeam: 3, AwayTeam: 4, HomeDifficulty: 5, AwayDifficulty: 1},
	}
	for index := range base.Players {
		base.Players[index].SelectedByPercent = float64((index % 9) + 1)
	}
	store.ApplySnapshot(base.Season, base.Gameweeks, base.Teams, base.Players, base.Fixtures, base.Histories)
	store.SaveSquad(demoSquad())
	return ResearchSnapshot{ID: "snapshot-before-deadline", SeasonID: 1, Gameweek: 3, Deadline: deadline, ObservedAt: deadline.Add(-time.Minute), State: "actual", MissingInputs: []string{}, Snapshot: base}, store
}

func TestFixtureResearchHandlesBlanksDoublesAndStableRanking(t *testing.T) {
	snapshot, _ := transferResearchSnapshot(t)
	result := calculateFixtureResearch(snapshot, 3, 3)
	if result.FormulaVersion != fixtureResearchFormulaVersion || result.GameweekTo != 5 || len(result.Items) != 5 {
		t.Fatalf("unexpected fixture result: %#v", result)
	}
	var teamOne FixtureResearchRow
	for _, row := range result.Items {
		if row.Team.ID == 1 {
			teamOne = row
		}
	}
	if teamOne.FixtureCount != 3 || len(teamOne.DoubleGameweeks) != 1 || teamOne.DoubleGameweeks[0] != 4 || len(teamOne.BlankGameweeks) != 1 || teamOne.BlankGameweeks[0] != 5 {
		t.Fatalf("blank/double accounting is wrong: %#v", teamOne)
	}
	if teamOne.Ease != 0.74 {
		t.Fatalf("fixture formula = %.2f, want 0.74", teamOne.Ease)
	}
	for index := 1; index < len(result.Items); index++ {
		if result.Items[index-1].Ease < result.Items[index].Ease {
			t.Fatal("fixture rows are not ranked descending")
		}
	}
}

func TestDifferentialsUseDocumentedFormulaAndPlayerIDTieBreak(t *testing.T) {
	snapshot, _ := transferResearchSnapshot(t)
	fixture := calculateFixtureResearch(snapshot, 3, 3)
	result := calculateDifferentials(snapshot, fixture, Midfielder, 0, 0, 10, 180, 50)
	if result.FormulaVersion != differentialFormulaVersion || result.PeerCount == 0 || len(result.Items) == 0 {
		t.Fatalf("unexpected differential result: %#v", result)
	}
	for _, row := range result.Items {
		if row.Score < 0 || row.Score > 1 || row.Components.OwnershipSignal < 0 || row.Components.OwnershipSignal > 1 {
			t.Fatalf("unnormalized differential row: %#v", row)
		}
	}
	for index := 1; index < len(result.Items); index++ {
		if result.Items[index-1].Score < result.Items[index].Score {
			t.Fatal("differentials are not ranked descending")
		}
	}
}

func TestDifferentialTieBreakMissingInputsAndAvailabilitySignals(t *testing.T) {
	snapshot, _ := transferResearchSnapshot(t)
	players := snapshot.Snapshot.Players[:2]
	for index := range players {
		players[index].Position = Goalkeeper
		players[index].TeamID = 1
		players[index].Price = 5
		players[index].Minutes = 360
		players[index].TotalPoints = 40
		players[index].SelectedByPercent = 5
		players[index].OwnershipKnown = true
	}
	snapshot.Snapshot.Players = players
	result := calculateDifferentials(snapshot, calculateFixtureResearch(snapshot, 3, 2), Goalkeeper, 0, 0, 10, 180, 10)
	if len(result.Items) != 2 || result.Items[0].Player.ID > result.Items[1].Player.ID {
		t.Fatalf("tied rows did not use player ID: %#v", result.Items)
	}
	if result.Items[0].Components.NormalizedPointsPer90 != .5 || result.Items[0].Components.NormalizedMinutesShare != .5 {
		t.Fatalf("constant peer normalization should be 0.5: %#v", result.Items[0])
	}
	unknown := players[0]
	unknown.SelectedByPercent = 0
	unknown.OwnershipKnown = false
	snapshot.Snapshot.Players = append(players, unknown)
	partial := calculateDifferentials(snapshot, calculateFixtureResearch(snapshot, 3, 2), Goalkeeper, 0, 0, 10, 180, 10)
	if partial.State != "partial" || len(partial.MissingInputs) == 0 {
		t.Fatalf("missing ownership was not disclosed: %#v", partial)
	}
	chance := 75
	if availabilitySignal(Player{Status: "a"}) != 1 || availabilitySignal(Player{Status: "d", ChanceOfPlaying: &chance}) != .5 || availabilitySignal(Player{Status: "u"}) != 0 {
		t.Fatal("availability factors do not match the formula registry")
	}
}

func TestFixtureResearchEmptyRunKeepsZeroDenominator(t *testing.T) {
	snapshot, _ := transferResearchSnapshot(t)
	snapshot.Snapshot.Fixtures = nil
	result := calculateFixtureResearch(snapshot, 3, 2)
	if len(result.Items) == 0 || result.Items[0].Denominator != 0 || result.Items[0].Ease != 0 || len(result.Items[0].BlankGameweeks) != 2 {
		t.Fatalf("empty fixture run was not explicit: %#v", result.Items)
	}
}

func TestTransferSimulationIsDeterministicReadOnlyAndChargesHits(t *testing.T) {
	snapshot, store := transferResearchSnapshot(t)
	input := TransferSimulationInput{SeasonID: 1, Gameweek: 3, Horizon: 3, FreeTransfers: 0, Transfers: []TransferMove{{PlayerOut: 5, PlayerIn: 17}}}
	first, validation := simulateTransfers(store, snapshot, input)
	if len(validation) > 0 {
		t.Fatalf("valid simulation rejected: %#v", validation)
	}
	second, validation := simulateTransfers(store, snapshot, input)
	if len(validation) > 0 || first.SimulationID != second.SimulationID {
		t.Fatalf("simulation is not deterministic: %#v %#v", first, second)
	}
	if first.PointsHit != 4 || first.PaidTransfers != 1 || first.After.PurchasePrices[17] != 4 {
		t.Fatalf("transfer accounting is wrong: %#v", first)
	}
	if _, owned := store.GetSquad().PurchasePrices[17]; owned {
		t.Fatal("simulation mutated the saved squad")
	}
	encoded, _ := json.Marshal(first)
	if len(encoded) == 0 || first.SnapshotID != snapshot.ID || len(first.FormulaVersions) != 2 {
		t.Fatal("simulation lost provenance")
	}
}

func TestTransferSimulationRejectsInvalidBoundsAndIllegalSquad(t *testing.T) {
	snapshot, store := transferResearchSnapshot(t)
	_, validation := simulateTransfers(store, snapshot, TransferSimulationInput{SeasonID: 1, Gameweek: 3, Horizon: 3, FreeTransfers: 1, Transfers: []TransferMove{{PlayerOut: 999, PlayerIn: 17}}})
	if len(validation) == 0 {
		t.Fatal("unknown outgoing player was accepted")
	}
	if err := validateResearchRange(3, 9); err == nil {
		t.Fatal("oversized horizon was accepted")
	}
}

func TestTransferResearchAPIUsesEnvelopeAndDoesNotPersistSimulation(t *testing.T) {
	store := NewStore()
	store.SaveSquad(demoSquad())
	handler := NewAPI(store, nil, nil, nil).Handler()
	fixture := httptest.NewRecorder()
	handler.ServeHTTP(fixture, httptest.NewRequest(http.MethodGet, "/api/v1/analysis/fixtures/swing?seasonId=1&gameweek=1&horizon=5", nil))
	if fixture.Code != http.StatusOK || !bytes.Contains(fixture.Body.Bytes(), []byte(`"formulaVersion":"fixture-research-v1"`)) || !bytes.Contains(fixture.Body.Bytes(), []byte(`"meta"`)) {
		t.Fatalf("unexpected fixture API response: %d %s", fixture.Code, fixture.Body.String())
	}
	differential := httptest.NewRecorder()
	handler.ServeHTTP(differential, httptest.NewRequest(http.MethodGet, "/api/v1/analysis/differentials?seasonId=1&gameweek=1&horizon=5&maxOwnership=10", nil))
	if differential.Code != http.StatusOK || !bytes.Contains(differential.Body.Bytes(), []byte(`"formulaVersion":"differential-opportunity-v1"`)) {
		t.Fatalf("unexpected differential API response: %d %s", differential.Code, differential.Body.String())
	}
	input := `{"seasonId":1,"gameweek":1,"horizon":5,"freeTransfers":0,"transfers":[{"playerOut":5,"playerIn":17}]}`
	simulation := httptest.NewRecorder()
	handler.ServeHTTP(simulation, httptest.NewRequest(http.MethodPost, "/api/v1/analysis/transfers/simulate", bytes.NewBufferString(input)))
	if simulation.Code != http.StatusOK || !bytes.Contains(simulation.Body.Bytes(), []byte(`"pointsHit":4`)) {
		t.Fatalf("unexpected simulation response: %d %s", simulation.Code, simulation.Body.String())
	}
	if _, changed := store.GetSquad().PurchasePrices[17]; changed {
		t.Fatal("API simulation changed the saved squad")
	}
}

func TestTransferResearchAPIRejectsRangeAndRequiresSaveConfirmation(t *testing.T) {
	handler := NewAPI(NewStore(), nil, nil, nil).Handler()
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/analysis/fixtures/swing?seasonId=1&gameweek=1&horizon=9", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid horizon status = %d", invalid.Code)
	}
	unconfirmed := httptest.NewRecorder()
	handler.ServeHTTP(unconfirmed, httptest.NewRequest(http.MethodPost, "/api/v1/planning/scenarios", bytes.NewBufferString(`{"name":"Maybe","confirmed":false,"simulation":{"seasonId":1,"gameweek":1,"horizon":5}}`)))
	if unconfirmed.Code != http.StatusUnprocessableEntity || !bytes.Contains(unconfirmed.Body.Bytes(), []byte(`confirmation_required`)) {
		t.Fatalf("unconfirmed save response: %d %s", unconfirmed.Code, unconfirmed.Body.String())
	}
}

func TestTransferResearchAPIRejectsFutureLeakingInputs(t *testing.T) {
	snapshot, _ := transferResearchSnapshot(t)
	snapshot.State = "unavailable"
	snapshot.MissingInputs = []string{"fixture_observations_at_deadline"}
	repository := &unavailableTransferResearchRepository{archiveTestRepository: archiveTestRepository{snapshot: snapshot.Snapshot}, research: snapshot}
	handler := NewAPI(NewStore(), nil, nil, nil, repository).Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/analysis/transfers/simulate", bytes.NewBufferString(`{"seasonId":1,"gameweek":3,"horizon":3,"freeTransfers":1,"transfers":[{"playerOut":5,"playerIn":17}]}`)))
	if response.Code != http.StatusUnprocessableEntity || !bytes.Contains(response.Body.Bytes(), []byte(`cutoff_inputs_unavailable`)) {
		t.Fatalf("future-leaking inputs were accepted: %d %s", response.Code, response.Body.String())
	}
}
