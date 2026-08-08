package app

import (
	"fmt"
	"math"
	"sort"
)

const AlgorithmVersion = "baseline-1"

func ValidateWeights(weights Weights) []ValidationError {
	values := []struct {
		name  string
		value float64
	}{{"form", weights.Form}, {"minutes", weights.Minutes}, {"fixture", weights.Fixture}, {"recentReturns", weights.RecentReturns}, {"value", weights.Value}}
	errors := []ValidationError{}
	total := 0.0
	for _, item := range values {
		if item.value < 0 || item.value > 1 {
			errors = append(errors, ValidationError{Code: "invalid_weight", Rule: item.name, Current: item.value, Required: "0..1", Message: fmt.Sprintf("Weight %s must be between 0 and 1.", item.name)})
		}
		total += item.value
	}
	if math.Abs(total-1) > 0.001 {
		errors = append(errors, ValidationError{Code: "weight_sum", Rule: "weights_sum_to_one", Current: round(total), Required: 1, Message: "Recommendation weights must sum to 1."})
	}
	return errors
}

func (s *Store) Recommend(squad Squad, requested Weights) (Recommendation, []ValidationError) {
	squad = s.EnrichSquad(squad)
	if requested == (Weights{}) {
		requested = DefaultWeights()
	}
	errors := s.ValidateSquad(squad)
	if len(errors) > 0 {
		return Recommendation{}, errors
	}
	if lineErrors := s.ValidateLineup(squad); len(lineErrors) > 0 && len(squad.StartingPlayerIDs) > 0 {
		return Recommendation{}, lineErrors
	}
	if weightErrors := ValidateWeights(requested); len(weightErrors) > 0 {
		return Recommendation{}, weightErrors
	}
	scores := map[int]RecommendationPlayer{}
	for _, player := range squad.Players {
		factors := s.scorePlayer(player, requested)
		score := 0.0
		for _, factor := range factors {
			score += factor.Contribution
		}
		scores[player.ID] = RecommendationPlayer{Player: player, Score: round(score), Factors: factors, Fixture: s.FixtureContext(player), Explanation: explain(player, score)}
	}
	ordered := make([]RecommendationPlayer, 0, len(scores))
	for _, item := range scores {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Score == ordered[j].Score {
			if ordered[i].Player.Position == ordered[j].Player.Position {
				return ordered[i].Player.ID < ordered[j].Player.ID
			}
			return ordered[i].Player.Position < ordered[j].Player.Position
		}
		return ordered[i].Score > ordered[j].Score
	})
	starting := []RecommendationPlayer{}
	bench := []RecommendationPlayer{}
	for _, position := range []int{Goalkeeper, Defender, Midfielder, Forward} {
		count := map[int]int{Goalkeeper: 1, Defender: 3, Midfielder: 4, Forward: 3}[position]
		candidates := []RecommendationPlayer{}
		for _, item := range ordered {
			if item.Player.Position == position {
				candidates = append(candidates, item)
			}
		}
		for i := 0; i < len(candidates); i++ {
			if i < count {
				starting = append(starting, candidates[i])
			} else {
				bench = append(bench, candidates[i])
			}
		}
	}
	sort.Slice(starting, func(i, j int) bool {
		return starting[i].Player.Position < starting[j].Player.Position || (starting[i].Player.Position == starting[j].Player.Position && starting[i].Score > starting[j].Score)
	})
	sort.Slice(bench, func(i, j int) bool { return bench[i].Score > bench[j].Score })
	captain := starting[0]
	vice := starting[1]
	for _, item := range starting {
		if item.Player.Position != Goalkeeper && item.Score > captain.Score {
			vice = captain
			captain = item
		} else if item.Player.ID != captain.Player.ID && item.Score > vice.Score {
			vice = item
		}
	}
	season, gameweek, snapshot := s.Snapshot()
	if snapshot.IsZero() {
		snapshot = s.season.UpdatedAt
	}
	return Recommendation{Season: season, Gameweek: gameweek, SnapshotAt: snapshot, AlgorithmVersion: AlgorithmVersion, Weights: requested, StartingXI: starting, Bench: bench, Captain: captain, ViceCaptain: vice, HeuristicNotice: "This is a transparent heuristic, not a guaranteed point projection."}, nil
}

func (s *Store) scorePlayer(player Player, weights Weights) []FactorContribution {
	fixtureSignal := 0.65
	if fixtures := s.UpcomingFixtures(player.TeamID); len(fixtures) > 0 {
		difficulty := float64(fixtures[0].HomeDifficulty)
		if fixtures[0].AwayTeam == player.TeamID {
			difficulty = float64(fixtures[0].AwayDifficulty)
		}
		fixtureSignal = math.Max(0, 1-(difficulty-2)/6)
	}
	signals := []struct {
		name           string
		signal, weight float64
	}{{"form", math.Min(1, player.Form/10), weights.Form}, {"minutes", player.ExpectedMinutes, weights.Minutes}, {"fixture", fixtureSignal, weights.Fixture}, {"recentReturns", math.Min(1, player.RecentReturns), weights.RecentReturns}, {"value", math.Min(1, player.Value/16), weights.Value}}
	result := make([]FactorContribution, 0, len(signals))
	for _, item := range signals {
		result = append(result, FactorContribution{Name: item.name, Signal: round(item.signal), Weight: item.weight, Contribution: round(item.signal * item.weight * 100)})
	}
	return result
}
func explain(player Player, score float64) string {
	return fmt.Sprintf("%s ranks at %.1f from form, minutes, fixture, recent returns, and value signals.", player.WebName, score)
}
