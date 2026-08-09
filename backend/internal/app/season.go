package app

import "sort"

func DefaultGameweek(state SeasonState, gameweeks []Gameweek) *int {
	if len(gameweeks) == 0 {
		return nil
	}
	ordered := append([]Gameweek(nil), gameweeks...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	if state == SeasonCurrent {
		for _, gameweek := range ordered {
			if gameweek.IsCurrent {
				value := gameweek.ID
				return &value
			}
		}
		for _, gameweek := range ordered {
			if !gameweek.Finished {
				value := gameweek.ID
				return &value
			}
		}
	}
	for index := len(ordered) - 1; index >= 0; index-- {
		if ordered[index].Finished {
			value := ordered[index].ID
			return &value
		}
	}
	value := ordered[len(ordered)-1].ID
	return &value
}

func DefaultSeason(items []SeasonCatalogueItem) (SeasonCatalogueItem, bool) {
	for _, item := range items {
		if item.State == SeasonCurrent {
			return item, true
		}
	}
	if len(items) == 0 {
		return SeasonCatalogueItem{}, false
	}
	selected := items[0]
	for _, item := range items[1:] {
		if item.ID > selected.ID {
			selected = item
		}
	}
	return selected, true
}
