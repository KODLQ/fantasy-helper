package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	fixtureResearchFormulaVersion = "fixture-research-v1"
	differentialFormulaVersion    = "differential-opportunity-v1"
	transferSimulationVersion     = "transfer-simulation-v1"
	maxResearchHorizon            = 8
	maxSimulationTransfers        = 5
)

type ResearchSnapshot struct {
	ID            string    `json:"snapshotId"`
	SeasonID      int       `json:"seasonId"`
	Gameweek      int       `json:"gameweek"`
	Deadline      time.Time `json:"deadline"`
	ObservedAt    time.Time `json:"observedAt"`
	State         string    `json:"state"`
	MissingInputs []string  `json:"missingInputs"`
	Snapshot      Snapshot  `json:"-"`
}

type TransferMove struct {
	PlayerOut int `json:"playerOut"`
	PlayerIn  int `json:"playerIn"`
}

type TransferSimulationInput struct {
	SeasonID          int            `json:"seasonId"`
	Gameweek          int            `json:"gameweek"`
	Horizon           int            `json:"horizon"`
	FreeTransfers     int            `json:"freeTransfers"`
	Transfers         []TransferMove `json:"transfers"`
	StartingPlayerIDs []int          `json:"startingPlayerIds,omitempty"`
	BenchPlayerIDs    []int          `json:"benchPlayerIds,omitempty"`
	CaptainID         int            `json:"captainId,omitempty"`
	ViceCaptainID     int            `json:"viceCaptainId,omitempty"`
	Formation         string         `json:"formation,omitempty"`
}

type TransferSimulation struct {
	SimulationID           string         `json:"simulationId"`
	AlgorithmVersion       string         `json:"algorithmVersion"`
	Before                 Squad          `json:"before"`
	After                  Squad          `json:"after"`
	Transfers              []TransferMove `json:"transfers"`
	FreeTransfers          int            `json:"freeTransfers"`
	FreeTransfersUsed      int            `json:"freeTransfersUsed"`
	PaidTransfers          int            `json:"paidTransfers"`
	PointsHit              int            `json:"pointsHit"`
	FixtureEaseBefore      float64        `json:"fixtureEaseBefore"`
	FixtureEaseAfter       float64        `json:"fixtureEaseAfter"`
	FixtureEaseDelta       float64        `json:"fixtureEaseDelta"`
	HistoricalPointsBefore int            `json:"historicalPointsBefore"`
	HistoricalPointsAfter  int            `json:"historicalPointsAfter"`
	State                  string         `json:"state"`
	SnapshotID             string         `json:"snapshotId"`
	Deadline               time.Time      `json:"deadline"`
	ObservedAt             time.Time      `json:"observedAt"`
	FormulaVersions        []string       `json:"formulaVersions"`
	Assumptions            []string       `json:"assumptions"`
	MissingInputs          []string       `json:"missingInputs"`
}

type PlanningScenario struct {
	ID           int64              `json:"id"`
	Name         string             `json:"name"`
	SimulationID string             `json:"simulationId"`
	SeasonID     int                `json:"seasonId"`
	Gameweek     int                `json:"gameweek"`
	Result       TransferSimulation `json:"result"`
	CreatedAt    time.Time          `json:"createdAt"`
}

type FixtureResearchRow struct {
	Team             Team                   `json:"team"`
	Ease             float64                `json:"ease"`
	FixtureCount     int                    `json:"fixtureCount"`
	GameweekCount    int                    `json:"gameweekCount"`
	BlankGameweeks   []int                  `json:"blankGameweeks"`
	DoubleGameweeks  []int                  `json:"doubleGameweeks"`
	Fixtures         []Fixture              `json:"fixtures"`
	Inputs           []FixtureResearchInput `json:"inputs"`
	Denominator      float64                `json:"denominator"`
	ExcludedFixtures []int                  `json:"excludedFixtures"`
}

type FixtureResearchInput struct {
	FixtureID          int     `json:"fixtureId"`
	Gameweek           int     `json:"gameweek"`
	Difficulty         int     `json:"difficulty"`
	FixtureEase        float64 `json:"fixtureEase"`
	Home               bool    `json:"home"`
	HomeAwayWeight     float64 `json:"homeAwayWeight"`
	AvailabilityFactor float64 `json:"availabilityFactor"`
	FixtureWeight      float64 `json:"fixtureWeight"`
}

type FixtureResearchResult struct {
	Items          []FixtureResearchRow `json:"items"`
	GameweekFrom   int                  `json:"gameweekFrom"`
	GameweekTo     int                  `json:"gameweekTo"`
	Horizon        int                  `json:"horizon"`
	State          string               `json:"state"`
	SnapshotID     string               `json:"snapshotId"`
	ObservedAt     time.Time            `json:"observedAt"`
	FormulaVersion string               `json:"formulaVersion"`
	MissingInputs  []string             `json:"missingInputs"`
	Assumptions    []string             `json:"assumptions"`
}

type DifferentialComponents struct {
	PointsPer90            float64 `json:"pointsPer90"`
	MinutesShare           float64 `json:"minutesShare"`
	FixtureEase            float64 `json:"fixtureEase"`
	OwnershipSignal        float64 `json:"ownershipSignal"`
	Availability           float64 `json:"availability"`
	NormalizedPointsPer90  float64 `json:"normalizedPointsPer90"`
	NormalizedMinutesShare float64 `json:"normalizedMinutesShare"`
}

type DifferentialRow struct {
	Player      Player                 `json:"player"`
	Team        Team                   `json:"team"`
	Score       float64                `json:"score"`
	Components  DifferentialComponents `json:"components"`
	Explanation string                 `json:"explanation"`
}

type DifferentialResult struct {
	Items          []DifferentialRow `json:"items"`
	PeerCount      int               `json:"peerCount"`
	State          string            `json:"state"`
	SnapshotID     string            `json:"snapshotId"`
	ObservedAt     time.Time         `json:"observedAt"`
	FormulaVersion string            `json:"formulaVersion"`
	ResearchNotice string            `json:"researchNotice"`
	MissingInputs  []string          `json:"missingInputs"`
}

func validateResearchRange(gameweek, horizon int) error {
	if gameweek < 1 || gameweek > 38 {
		return fmt.Errorf("gameweek must be between 1 and 38")
	}
	if horizon < 1 || horizon > maxResearchHorizon {
		return fmt.Errorf("horizon must be between 1 and %d", maxResearchHorizon)
	}
	return nil
}

func researchSnapshotFromStore(store *Store, seasonID, gameweek int) (ResearchSnapshot, error) {
	snapshot := store.ExportSnapshot()
	if snapshot.Season.ID != seasonID {
		return ResearchSnapshot{}, fmt.Errorf("season data is unavailable")
	}
	var deadline time.Time
	for _, week := range snapshot.Gameweeks {
		if week.ID == gameweek && week.DeadlineTime != nil {
			deadline = *week.DeadlineTime
			break
		}
	}
	observed := snapshot.Season.UpdatedAt
	if observed.IsZero() {
		observed = time.Now().UTC()
	}
	return ResearchSnapshot{ID: fmt.Sprintf("memory-%d-%d-%d", seasonID, gameweek, observed.Unix()), SeasonID: seasonID, Gameweek: gameweek, Deadline: deadline, ObservedAt: observed, State: "estimated", MissingInputs: []string{"deadline_snapshot_persistence"}, Snapshot: snapshot}, nil
}

func calculateFixtureResearch(snapshot ResearchSnapshot, gameweek, horizon int) FixtureResearchResult {
	to := gameweek + horizon - 1
	result := FixtureResearchResult{Items: []FixtureResearchRow{}, GameweekFrom: gameweek, GameweekTo: to, Horizon: horizon, State: snapshot.State, SnapshotID: snapshot.ID, ObservedAt: snapshot.ObservedAt, FormulaVersion: fixtureResearchFormulaVersion, MissingInputs: append([]string{}, snapshot.MissingInputs...), Assumptions: []string{"Source difficulty uses its 1-5 scale.", "Team fixture rows use availability factor 1.0; player rankings apply player availability separately.", "Blank gameweeks add no fixture; every double-gameweek fixture contributes once."}}
	for _, team := range snapshot.Snapshot.Teams {
		row := FixtureResearchRow{Team: team, GameweekCount: horizon, BlankGameweeks: []int{}, DoubleGameweeks: []int{}, Fixtures: []Fixture{}, Inputs: []FixtureResearchInput{}, ExcludedFixtures: []int{}}
		counts := map[int]int{}
		weighted, denominator := 0.0, 0.0
		for _, fixture := range snapshot.Snapshot.Fixtures {
			if fixture.Gameweek < gameweek || fixture.Gameweek > to || (fixture.HomeTeam != team.ID && fixture.AwayTeam != team.ID) {
				continue
			}
			difficulty, homeAway := fixture.AwayDifficulty, 0.95
			if fixture.HomeTeam == team.ID {
				difficulty, homeAway = fixture.HomeDifficulty, 1
			}
			availability := 1.0
			weight := homeAway * availability
			weighted += ((6 - float64(difficulty)) / 5) * weight
			denominator += weight
			row.Inputs = append(row.Inputs, FixtureResearchInput{FixtureID: fixture.ID, Gameweek: fixture.Gameweek, Difficulty: difficulty, FixtureEase: round((6 - float64(difficulty)) / 5), Home: fixture.HomeTeam == team.ID, HomeAwayWeight: homeAway, AvailabilityFactor: availability, FixtureWeight: weight})
			counts[fixture.Gameweek]++
			row.Fixtures = append(row.Fixtures, fixture)
		}
		for week := gameweek; week <= to; week++ {
			if counts[week] == 0 {
				row.BlankGameweeks = append(row.BlankGameweeks, week)
			}
			if counts[week] > 1 {
				row.DoubleGameweeks = append(row.DoubleGameweeks, week)
			}
		}
		row.FixtureCount = len(row.Fixtures)
		row.Denominator = round(denominator)
		if denominator > 0 {
			row.Ease = round(weighted / denominator)
		}
		result.Items = append(result.Items, row)
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		if result.Items[i].Ease == result.Items[j].Ease {
			return result.Items[i].Team.ID < result.Items[j].Team.ID
		}
		return result.Items[i].Ease > result.Items[j].Ease
	})
	return result
}

func availabilitySignal(player Player) float64 {
	if player.Status == "u" || player.Status == "n" {
		return 0
	}
	if player.Status != "a" || (player.ChanceOfPlaying != nil && *player.ChanceOfPlaying < 100) {
		return 0.5
	}
	return 1
}

func normalize(values []float64, index int) float64 {
	if len(values) == 0 {
		return 0
	}
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	if maximum == minimum {
		return 0.5
	}
	return (values[index] - minimum) / (maximum - minimum)
}

func calculateDifferentials(snapshot ResearchSnapshot, fixture FixtureResearchResult, position int, minPrice, maxPrice, maxOwnership float64, minMinutes, limit int) DifferentialResult {
	teams := map[int]Team{}
	fixtureEase := map[int]float64{}
	for _, team := range snapshot.Snapshot.Teams {
		teams[team.ID] = team
	}
	for _, row := range fixture.Items {
		fixtureEase[row.Team.ID] = row.Ease
	}
	players := []Player{}
	missingOwnership := false
	for _, player := range snapshot.Snapshot.Players {
		if position > 0 && player.Position != position {
			continue
		}
		ownershipKnown := player.OwnershipKnown || player.SelectedByPercent > 0
		if !ownershipKnown {
			missingOwnership = true
		}
		if player.Price < minPrice || (maxPrice > 0 && player.Price > maxPrice) || !ownershipKnown || player.SelectedByPercent > maxOwnership || player.Minutes < minMinutes {
			continue
		}
		players = append(players, player)
	}
	p90s, minuteShares := make([]float64, len(players)), make([]float64, len(players))
	elapsed := math.Max(1, float64(snapshot.Gameweek-1))
	for index, player := range players {
		if player.Minutes > 0 {
			p90s[index] = float64(player.TotalPoints) / float64(player.Minutes) * 90
		}
		minuteShares[index] = math.Min(1, float64(player.Minutes)/(elapsed*90))
	}
	result := DifferentialResult{Items: []DifferentialRow{}, PeerCount: len(players), State: snapshot.State, SnapshotID: snapshot.ID, ObservedAt: snapshot.ObservedAt, FormulaVersion: differentialFormulaVersion, ResearchNotice: "Research ranking only; this is not an official FPL prediction.", MissingInputs: append([]string{}, snapshot.MissingInputs...)}
	if missingOwnership {
		result.State = "partial"
		result.MissingInputs = append(result.MissingInputs, "selected_by_percent")
	}
	for index, player := range players {
		component := DifferentialComponents{PointsPer90: round(p90s[index]), MinutesShare: round(minuteShares[index]), FixtureEase: fixtureEase[player.TeamID], OwnershipSignal: round(1 - player.SelectedByPercent/100), Availability: availabilitySignal(player), NormalizedPointsPer90: round(normalize(p90s, index)), NormalizedMinutesShare: round(normalize(minuteShares, index))}
		score := .40*component.NormalizedPointsPer90 + .25*component.NormalizedMinutesShare + .20*component.FixtureEase + .10*component.OwnershipSignal + .05*component.Availability
		explanation := fmt.Sprintf("%.1f points/90, %.0f%% minutes share, %.1f%% ownership, %.2f availability", component.PointsPer90, component.MinutesShare*100, player.SelectedByPercent, component.Availability)
		result.Items = append(result.Items, DifferentialRow{Player: player, Team: teams[player.TeamID], Score: round(score), Components: component, Explanation: explanation})
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		if result.Items[i].Score == result.Items[j].Score {
			return result.Items[i].Player.ID < result.Items[j].Player.ID
		}
		return result.Items[i].Score > result.Items[j].Score
	})
	if limit > 0 && len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	return result
}

func simulateTransfers(domain *Store, snapshot ResearchSnapshot, input TransferSimulationInput) (TransferSimulation, []ValidationError) {
	before := domain.EnrichSquad(domain.GetSquad())
	inputJSON, _ := json.Marshal(input)
	digest := sha256.Sum256(append([]byte(snapshot.ID+":"+transferSimulationVersion+":"), inputJSON...))
	result := TransferSimulation{SimulationID: "sim-" + hex.EncodeToString(digest[:12]), AlgorithmVersion: transferSimulationVersion, Before: before, After: before, Transfers: append([]TransferMove{}, input.Transfers...), FreeTransfers: input.FreeTransfers, State: snapshot.State, SnapshotID: snapshot.ID, Deadline: snapshot.Deadline, ObservedAt: snapshot.ObservedAt, FormulaVersions: []string{fixtureResearchFormulaVersion, transferSimulationVersion}, Assumptions: []string{"Outgoing value uses the saved purchase price; incoming value uses the selected deadline snapshot price.", "Doubtful players remain selectable; players marked unavailable cannot be transferred in."}, MissingInputs: append([]string{}, snapshot.MissingInputs...)}
	errors := []ValidationError{}
	if len(input.Transfers) < 1 || len(input.Transfers) > maxSimulationTransfers {
		errors = append(errors, ValidationError{Code: "transfer_count", Rule: "transfer_bounds", Current: len(input.Transfers), Required: "1-5", Message: "Choose between one and five transfers."})
	}
	if input.FreeTransfers < 0 || input.FreeTransfers > 5 {
		errors = append(errors, ValidationError{Code: "free_transfers", Rule: "free_transfer_bounds", Current: input.FreeTransfers, Required: "0-5", Message: "Free transfers must be between zero and five."})
	}
	prices := map[int]float64{}
	for id, price := range before.PurchasePrices {
		prices[id] = price
	}
	outSeen, inSeen := map[int]bool{}, map[int]bool{}
	for _, move := range input.Transfers {
		if outSeen[move.PlayerOut] || inSeen[move.PlayerIn] || move.PlayerOut == move.PlayerIn {
			errors = append(errors, ValidationError{Code: "duplicate_transfer", Rule: "distinct_transfer_players", PlayerID: move.PlayerIn, Message: "Each transfer must use distinct outgoing and incoming players."})
			continue
		}
		outSeen[move.PlayerOut], inSeen[move.PlayerIn] = true, true
		if _, exists := prices[move.PlayerOut]; !exists {
			errors = append(errors, ValidationError{Code: "player_not_owned", Rule: "outgoing_in_squad", PlayerID: move.PlayerOut, Message: "Outgoing player is not in the saved squad."})
			continue
		}
		if _, exists := prices[move.PlayerIn]; exists {
			errors = append(errors, ValidationError{Code: "player_already_owned", Rule: "incoming_not_in_squad", PlayerID: move.PlayerIn, Message: "Incoming player is already in the saved squad."})
			continue
		}
		incoming, exists := domain.Player(move.PlayerIn)
		if !exists {
			errors = append(errors, ValidationError{Code: "unknown_player", Rule: "active_season_player", PlayerID: move.PlayerIn, Message: "Incoming player is not in the selected season snapshot."})
			continue
		}
		if incoming.Status == "u" || incoming.Status == "n" {
			errors = append(errors, ValidationError{Code: "player_unavailable", Rule: "incoming_player_available", PlayerID: move.PlayerIn, Message: "Incoming player is unavailable for selection in this snapshot."})
			continue
		}
		delete(prices, move.PlayerOut)
		prices[move.PlayerIn] = incoming.Price
	}
	after := before
	after.PurchasePrices = prices
	after.StartingPlayerIDs = replacePlayerIDs(before.StartingPlayerIDs, input.Transfers)
	after.BenchPlayerIDs = replacePlayerIDs(before.BenchPlayerIDs, input.Transfers)
	after.CaptainID = replacePlayerID(before.CaptainID, input.Transfers)
	after.ViceCaptainID = replacePlayerID(before.ViceCaptainID, input.Transfers)
	if len(input.StartingPlayerIDs) > 0 {
		after.StartingPlayerIDs = append([]int{}, input.StartingPlayerIDs...)
	}
	if len(input.BenchPlayerIDs) > 0 {
		after.BenchPlayerIDs = append([]int{}, input.BenchPlayerIDs...)
	}
	if input.CaptainID > 0 {
		after.CaptainID = input.CaptainID
	}
	if input.ViceCaptainID > 0 {
		after.ViceCaptainID = input.ViceCaptainID
	}
	if strings.TrimSpace(input.Formation) != "" {
		after.Formation = strings.TrimSpace(input.Formation)
	}
	after = domain.EnrichSquad(after)
	errors = append(errors, domain.ValidatePlan(after)...)
	result.After = after
	result.FreeTransfersUsed = min(len(input.Transfers), input.FreeTransfers)
	result.PaidTransfers = max(0, len(input.Transfers)-input.FreeTransfers)
	result.PointsHit = result.PaidTransfers * 4
	fixture := calculateFixtureResearch(snapshot, input.Gameweek, input.Horizon)
	result.FixtureEaseBefore = squadFixtureEase(before, fixture)
	result.FixtureEaseAfter = squadFixtureEase(after, fixture)
	result.FixtureEaseDelta = round(result.FixtureEaseAfter - result.FixtureEaseBefore)
	result.HistoricalPointsBefore = squadHistoricalPoints(before)
	result.HistoricalPointsAfter = squadHistoricalPoints(after)
	return result, errors
}

func squadHistoricalPoints(squad Squad) int {
	total := 0
	for _, player := range squad.Players {
		total += player.TotalPoints
	}
	return total
}

func replacePlayerIDs(ids []int, moves []TransferMove) []int {
	result := append([]int{}, ids...)
	for i, id := range result {
		result[i] = replacePlayerID(id, moves)
	}
	return result
}
func replacePlayerID(id int, moves []TransferMove) int {
	for _, move := range moves {
		if move.PlayerOut == id {
			return move.PlayerIn
		}
	}
	return id
}
func squadFixtureEase(squad Squad, fixture FixtureResearchResult) float64 {
	ease := map[int]float64{}
	for _, row := range fixture.Items {
		ease[row.Team.ID] = row.Ease
	}
	total := 0.0
	for _, player := range squad.Players {
		total += ease[player.TeamID]
	}
	if len(squad.Players) == 0 {
		return 0
	}
	return round(total / float64(len(squad.Players)))
}
