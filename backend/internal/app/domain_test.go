package app

import "testing"

func demoSquad() Squad {
	ids := []int{1, 2, 4, 5, 6, 7, 22, 8, 9, 10, 11, 12, 13, 14, 15}
	prices := map[int]float64{}
	for _, id := range ids {
		prices[id] = 5
	}
	return Squad{Name: "Test", Budget: 100, PurchasePrices: prices, StartingPlayerIDs: []int{1, 4, 5, 6, 8, 9, 10, 11, 13, 14, 15}, BenchPlayerIDs: []int{2, 7, 12, 22}, CaptainID: 13, ViceCaptainID: 8, Formation: "3-4-3"}
}

func TestValidSquadAndLineup(t *testing.T) {
	store := NewStore()
	plan := demoSquad()
	if errors := store.ValidatePlan(plan); len(errors) != 0 {
		t.Fatalf("expected valid plan, got %#v", errors)
	}
}

func TestBudgetErrorDoesNotPassValidation(t *testing.T) {
	store := NewStore()
	plan := demoSquad()
	plan.Budget = 50
	errors := store.ValidateSquad(plan)
	if len(errors) == 0 || errors[len(errors)-1].Code != "budget" {
		t.Fatalf("expected budget error, got %#v", errors)
	}
}

func TestWeightsAreValidated(t *testing.T) {
	errors := ValidateWeights(Weights{Form: 1, Minutes: 1})
	if len(errors) != 1 || errors[0].Code != "weight_sum" {
		t.Fatalf("expected sum error, got %#v", errors)
	}
}

func TestRecommendationIsReproducibleAndRejectsInvalidWeights(t *testing.T) {
	store := NewStore()
	plan := demoSquad()
	first, errors := store.Recommend(plan, DefaultWeights())
	if len(errors) != 0 {
		t.Fatalf("unexpected recommendation errors: %#v", errors)
	}
	second, errors := store.Recommend(plan, DefaultWeights())
	if len(errors) != 0 || first.Captain.Player.ID != second.Captain.Player.ID || first.ViceCaptain.Player.ID != second.ViceCaptain.Player.ID {
		t.Fatalf("recommendation is not reproducible: %#v %#v", first, second)
	}
	if _, errors := store.Recommend(plan, Weights{Form: 1, Minutes: 1}); len(errors) == 0 {
		t.Fatal("expected invalid weights to be rejected")
	}
}

func TestLineupRejectsCaptainOnBench(t *testing.T) {
	store := NewStore()
	plan := demoSquad()
	plan.CaptainID = 2
	errors := store.ValidateLineup(plan)
	if len(errors) == 0 || errors[0].Code != "captain" {
		t.Fatalf("expected captain error, got %#v", errors)
	}
}

func TestSearchSortsByFormDescending(t *testing.T) {
	store := NewStore()
	players, total := store.SearchPlayers(PlayerQuery{Sort: "form", Desc: true, PageSize: 100})
	if total < 18 || len(players) != total {
		t.Fatalf("expected demo players, got %d/%d", len(players), total)
	}
	if players[0].Form < players[len(players)-1].Form {
		t.Fatalf("results are not descending: %#v", players)
	}
}

func TestApplySnapshotRetainsLastKnownGoodHistoryForFailedPlayers(t *testing.T) {
	store := NewStore()
	before := store.History(8)
	if len(before) == 0 {
		t.Fatal("expected seeded player history")
	}
	snapshot := store.ExportSnapshot()
	snapshot.Histories = map[int][]PlayerHistory{9: {{Gameweek: 2, TotalPoints: 7}}}
	store.ApplySnapshot(snapshot.Season, snapshot.Gameweeks, snapshot.Teams, snapshot.Players, snapshot.Fixtures, snapshot.Histories)
	if len(store.History(8)) != len(before) {
		t.Fatalf("last-known-good history was lost: before=%d after=%d", len(before), len(store.History(8)))
	}
	if len(store.History(9)) != 1 || store.History(9)[0].TotalPoints != 7 {
		t.Fatalf("successful history was not refreshed: %#v", store.History(9))
	}
}
