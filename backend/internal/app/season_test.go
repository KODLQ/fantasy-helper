package app

import "testing"

func TestDefaultGameweekRules(t *testing.T) {
	current := []Gameweek{{ID: 1, Finished: true}, {ID: 2, IsCurrent: true}, {ID: 3}}
	if selected := DefaultGameweek(SeasonCurrent, current); selected == nil || *selected != 2 {
		t.Fatalf("current default = %v", selected)
	}
	next := []Gameweek{{ID: 1, Finished: true}, {ID: 2}, {ID: 3}}
	if selected := DefaultGameweek(SeasonCurrent, next); selected == nil || *selected != 2 {
		t.Fatalf("next default = %v", selected)
	}
	historical := []Gameweek{{ID: 1, Finished: true}, {ID: 2, Finished: true}, {ID: 3}}
	if selected := DefaultGameweek(SeasonHistorical, historical); selected == nil || *selected != 2 {
		t.Fatalf("historical default = %v", selected)
	}
	if selected := DefaultGameweek(SeasonHistorical, nil); selected != nil {
		t.Fatalf("empty default = %v", selected)
	}
}

func TestDefaultSeasonPrefersCurrentThenNewest(t *testing.T) {
	items := []SeasonCatalogueItem{{ID: 2026, State: SeasonHistorical}, {ID: 2024, State: SeasonCurrent}, {ID: 2025, State: SeasonHistorical}}
	if selected, ok := DefaultSeason(items); !ok || selected.ID != 2024 {
		t.Fatalf("selected = %#v ok=%v", selected, ok)
	}
	items[1].State = SeasonHistorical
	if selected, ok := DefaultSeason(items); !ok || selected.ID != 2026 {
		t.Fatalf("newest selected = %#v ok=%v", selected, ok)
	}
}
