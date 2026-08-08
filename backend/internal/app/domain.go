package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

var legalFormations = map[string][3]int{
	"3-5-2": {3, 5, 2}, "3-4-3": {3, 4, 3}, "4-5-1": {4, 5, 1}, "4-4-2": {4, 4, 2}, "4-3-3": {4, 3, 3}, "5-4-1": {5, 4, 1}, "5-3-2": {5, 3, 2}, "5-2-3": {5, 2, 3},
}

func (s *Store) ValidateSquad(input Squad) []ValidationError {
	input = s.EnrichSquad(input)
	errors := []ValidationError{}
	if len(input.PurchasePrices) != 15 {
		errors = append(errors, ValidationError{Code: "squad_size", Rule: "exactly_15_players", Current: len(input.PurchasePrices), Required: 15, Message: "A squad must contain exactly 15 distinct players."})
	}
	counts := map[int]int{}
	clubs := map[int]int{}
	seen := map[int]bool{}
	for id := range input.PurchasePrices {
		if seen[id] {
			errors = append(errors, ValidationError{Code: "duplicate_player", Rule: "distinct_players", PlayerID: id, Message: "A player cannot be selected twice."})
			continue
		}
		seen[id] = true
		player, ok := s.Player(id)
		if !ok {
			errors = append(errors, ValidationError{Code: "unknown_player", Rule: "active_season_player", PlayerID: id, Message: "Player is not in the active season."})
			continue
		}
		counts[player.Position]++
		clubs[player.TeamID]++
	}
	for _, wanted := range []struct{ position, required int }{{Goalkeeper, 2}, {Defender, 5}, {Midfielder, 5}, {Forward, 3}} {
		if counts[wanted.position] != wanted.required {
			errors = append(errors, ValidationError{Code: "position_count", Rule: positionName(wanted.position), Current: counts[wanted.position], Required: wanted.required, Message: fmt.Sprintf("Squad needs exactly %d %ss.", wanted.required, strings.ToLower(positionName(wanted.position)))})
		}
	}
	for teamID, count := range clubs {
		if count > 3 {
			errors = append(errors, ValidationError{Code: "club_limit", Rule: "max_3_from_club", Current: count, Required: 3, Message: fmt.Sprintf("Club %d has %d players; the maximum is 3.", teamID, count)})
		}
	}
	if input.TotalCost > input.Budget+0.001 {
		errors = append(errors, ValidationError{Code: "budget", Rule: "budget_limit", Current: round(input.TotalCost), Required: input.Budget, Message: fmt.Sprintf("Squad costs %.1f but the budget is %.1f.", input.TotalCost, input.Budget)})
	}
	return errors
}

func (s *Store) ValidateLineup(input Squad) []ValidationError {
	errors := []ValidationError{}
	selected := map[int]bool{}
	for id := range input.PurchasePrices {
		selected[id] = true
	}
	if len(input.StartingPlayerIDs) != 11 {
		errors = append(errors, ValidationError{Code: "starting_size", Rule: "starting_xi", Current: len(input.StartingPlayerIDs), Required: 11, Message: "Starting XI must contain 11 players."})
	}
	if len(input.BenchPlayerIDs) != 4 {
		errors = append(errors, ValidationError{Code: "bench_size", Rule: "bench", Current: len(input.BenchPlayerIDs), Required: 4, Message: "Bench must contain 4 players."})
	}
	starts := map[int]bool{}
	counts := map[int]int{}
	for _, id := range input.StartingPlayerIDs {
		if !selected[id] {
			errors = append(errors, ValidationError{Code: "starter_not_in_squad", Rule: "lineup_membership", PlayerID: id, Message: "Every starter must belong to the planning squad."})
		}
		if starts[id] {
			errors = append(errors, ValidationError{Code: "duplicate_starter", Rule: "starting_xi", PlayerID: id, Message: "A player cannot appear twice in the starting XI."})
			continue
		}
		starts[id] = true
		player, ok := s.Player(id)
		if !ok {
			errors = append(errors, ValidationError{Code: "unknown_starter", Rule: "active_season_player", PlayerID: id, Message: "Starting player is not in the active season."})
			continue
		}
		counts[player.Position]++
	}
	if counts[Goalkeeper] != 1 {
		errors = append(errors, ValidationError{Code: "goalkeeper_count", Rule: "one_starting_goalkeeper", Current: counts[Goalkeeper], Required: 1, Message: "Starting XI must contain exactly one goalkeeper."})
	}
	if input.Formation == "" {
		input.Formation = fmt.Sprintf("%d-%d-%d", counts[Defender], counts[Midfielder], counts[Forward])
	}
	wanted, known := legalFormations[input.Formation]
	if !known || counts[Defender] != wanted[0] || counts[Midfielder] != wanted[1] || counts[Forward] != wanted[2] {
		errors = append(errors, ValidationError{Code: "formation", Rule: "legal_formation", Current: input.Formation, Required: "3-5-2, 3-4-3, 4-5-1, 4-4-2, 4-3-3, 5-4-1, 5-3-2, or 5-2-3", Message: "Starting XI does not match a legal formation."})
	}
	bench := map[int]bool{}
	for _, id := range input.BenchPlayerIDs {
		if !selected[id] {
			errors = append(errors, ValidationError{Code: "bench_not_in_squad", Rule: "lineup_membership", PlayerID: id, Message: "Every bench player must belong to the planning squad."})
		}
		if bench[id] {
			errors = append(errors, ValidationError{Code: "duplicate_bench", Rule: "bench", PlayerID: id, Message: "A bench player cannot appear twice."})
		}
		bench[id] = true
		if starts[id] {
			errors = append(errors, ValidationError{Code: "starter_on_bench", Rule: "starting_bench_partition", PlayerID: id, Message: "A starter cannot also be on the bench."})
		}
	}
	if input.CaptainID == 0 || !starts[input.CaptainID] {
		errors = append(errors, ValidationError{Code: "captain", Rule: "captain_is_starter", PlayerID: input.CaptainID, Message: "Captain must be selected from the starting XI."})
	}
	if input.ViceCaptainID == 0 || !starts[input.ViceCaptainID] {
		errors = append(errors, ValidationError{Code: "vice_captain", Rule: "vice_captain_is_starter", PlayerID: input.ViceCaptainID, Message: "Vice-captain must be selected from the starting XI."})
	}
	if input.CaptainID != 0 && input.CaptainID == input.ViceCaptainID {
		errors = append(errors, ValidationError{Code: "captain_distinct", Rule: "captain_vice_captain_distinct", Message: "Captain and vice-captain must be different players."})
	}
	return errors
}

func (s *Store) ValidatePlan(input Squad) []ValidationError {
	errors := s.ValidateSquad(input)
	errors = append(errors, s.ValidateLineup(input)...)
	sort.SliceStable(errors, func(i, j int) bool { return errors[i].Code < errors[j].Code })
	return errors
}

func (s *Store) FixtureContext(player Player) string {
	fixtures := s.UpcomingFixtures(player.TeamID)
	if len(fixtures) == 0 {
		return "No upcoming fixture loaded"
	}
	fixture := fixtures[0]
	opponent := "TBD"
	if fixture.HomeTeam == player.TeamID {
		if team, ok := s.Team(fixture.AwayTeam); ok {
			opponent = team.ShortName
		}
		return fmt.Sprintf("H vs %s · difficulty %d", opponent, fixture.HomeDifficulty)
	}
	if team, ok := s.Team(fixture.HomeTeam); ok {
		opponent = team.ShortName
	}
	return fmt.Sprintf("A vs %s · difficulty %d", opponent, fixture.AwayDifficulty)
}

func round(value float64) float64 { return math.Round(value*100) / 100 }
