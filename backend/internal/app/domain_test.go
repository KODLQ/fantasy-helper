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
	legalFormations := map[string]bool{"3-4-3": true, "3-5-2": true, "4-5-1": true, "4-4-2": true, "4-3-3": true, "5-4-1": true, "5-3-2": true, "5-2-3": true}
	if !legalFormations[first.Formation] || len(first.StartingXI) != 11 {
		t.Fatalf("recommendation must select a legal formation: %q %#v", first.Formation, first.StartingXI)
	}
	if len(first.Bench) != 4 || first.Bench[3].Player.Position != Goalkeeper {
		t.Fatalf("reserve goalkeeper must use the dedicated final bench slot: %#v", first.Bench)
	}
	for _, item := range first.Bench[:3] {
		if item.Player.Position == Goalkeeper {
			t.Fatalf("goalkeeper cannot receive an outfield substitution rank: %#v", first.Bench)
		}
	}
	second, errors := store.Recommend(plan, DefaultWeights())
	if len(errors) != 0 || first.Formation != second.Formation || first.Captain.Player.ID != second.Captain.Player.ID || first.ViceCaptain.Player.ID != second.ViceCaptain.Player.ID {
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

func TestSearchUsesStableNameAndIDTieBreakers(t *testing.T) {
	store := &Store{players: map[int]Player{
		3: {ID: 3, WebName: "Zulu", Form: 5},
		2: {ID: 2, WebName: "Alpha", Form: 5},
		1: {ID: 1, WebName: "Alpha", Form: 5},
	}}
	players, total := store.SearchPlayers(PlayerQuery{Sort: "form", Desc: true, PageSize: 100})
	if total != 3 || len(players) != 3 || players[0].ID != 1 || players[1].ID != 2 || players[2].ID != 3 {
		t.Fatalf("expected stable name and ID tie-breakers, got %#v", players)
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

func TestWarehouseCacheStartsWithoutFixtureData(t *testing.T) {
	cache := NewWarehouseCache()
	if len(cache.AllPlayers()) != 0 || cache.ExportSnapshot().Season.ID != 0 {
		t.Fatalf("production cache contains fixture data: %#v", cache.ExportSnapshot())
	}
	if len(NewStore().AllPlayers()) == 0 {
		t.Fatal("unit-test store should retain deterministic fixture data")
	}
}
