package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu           sync.RWMutex
	season       Season
	gameweeks    map[int]Gameweek
	teams        map[int]Team
	players      map[int]Player
	fixtures     []Fixture
	history      map[int][]PlayerHistory
	squad        Squad
	sync         SyncStatus
	lastSnapshot time.Time
}

func NewStore() *Store {
	s := &Store{
		gameweeks: map[int]Gameweek{}, teams: map[int]Team{}, players: map[int]Player{}, history: map[int][]PlayerHistory{},
		sync: SyncStatus{Status: "empty", CompletedStages: []string{}, FailedStages: []string{}, Freshness: Freshness{Status: "unavailable", State: "unavailable", Dataset: "public-fpl"}},
	}
	s.seedDemoData()
	return s
}

func (s *Store) seedDemoData() {
	now := time.Now().UTC()
	s.season = Season{ID: 1, Name: "2025/26 Demo", IsCurrent: true, UpdatedAt: now}
	s.gameweeks[1] = Gameweek{ID: 1, Name: "Gameweek 1", IsCurrent: true, AverageScore: 55}
	teams := []Team{{ID: 1, Name: "Northbridge", ShortName: "NOR"}, {ID: 2, Name: "Riverside", ShortName: "RIV"}, {ID: 3, Name: "Kingsport", ShortName: "KIN"}, {ID: 4, Name: "Harbour City", ShortName: "HAR"}, {ID: 5, Name: "Old Town", ShortName: "OLD"}}
	for _, team := range teams {
		s.teams[team.ID] = team
	}
	players := []Player{
		{ID: 1, FirstName: "Alex", SecondName: "Stone", WebName: "Stone", Position: Goalkeeper, TeamID: 1, Price: 5.0, TotalPoints: 62, Form: 5.2, Minutes: 900, Value: 12.4, Status: "a", ExpectedMinutes: 0.96, RecentReturns: 0.42, Saves: 34},
		{ID: 2, FirstName: "Milo", SecondName: "Grant", WebName: "Grant", Position: Goalkeeper, TeamID: 2, Price: 4.5, TotalPoints: 48, Form: 4.1, Minutes: 810, Value: 10.7, Status: "a", ExpectedMinutes: 0.91, RecentReturns: 0.32, Saves: 28},
		{ID: 3, FirstName: "Theo", SecondName: "Ward", WebName: "Ward", Position: Defender, TeamID: 1, Price: 5.5, TotalPoints: 74, Form: 6.1, Minutes: 900, Value: 13.5, Status: "a", ExpectedMinutes: 0.97, RecentReturns: 0.56, CleanSheets: 5},
		{ID: 4, FirstName: "Jon", SecondName: "Bell", WebName: "Bell", Position: Defender, TeamID: 2, Price: 5.0, TotalPoints: 68, Form: 5.8, Minutes: 870, Value: 13.6, Status: "a", ExpectedMinutes: 0.95, RecentReturns: 0.51, CleanSheets: 4},
		{ID: 5, FirstName: "Finn", SecondName: "Cole", WebName: "Cole", Position: Defender, TeamID: 3, Price: 4.5, TotalPoints: 55, Form: 4.8, Minutes: 810, Value: 12.2, Status: "a", ExpectedMinutes: 0.9, RecentReturns: 0.38, CleanSheets: 3},
		{ID: 6, FirstName: "Sam", SecondName: "Price", WebName: "Price", Position: Defender, TeamID: 4, Price: 4.5, TotalPoints: 51, Form: 4.4, Minutes: 765, Value: 11.3, Status: "a", ExpectedMinutes: 0.86, RecentReturns: 0.35, CleanSheets: 3},
		{ID: 7, FirstName: "Noah", SecondName: "Lane", WebName: "Lane", Position: Defender, TeamID: 5, Price: 4.0, TotalPoints: 44, Form: 4.0, Minutes: 720, Value: 11.0, Status: "a", ExpectedMinutes: 0.82, RecentReturns: 0.31, CleanSheets: 2},
		{ID: 8, FirstName: "Leo", SecondName: "Mason", WebName: "Mason", Position: Midfielder, TeamID: 1, Price: 8.0, TotalPoints: 112, Form: 8.7, Minutes: 900, Value: 14.0, Status: "a", ExpectedMinutes: 0.98, RecentReturns: 0.82, GoalsScored: 6, Assists: 5},
		{ID: 9, FirstName: "Owen", SecondName: "Reed", WebName: "Reed", Position: Midfielder, TeamID: 2, Price: 7.5, TotalPoints: 98, Form: 7.9, Minutes: 870, Value: 13.1, Status: "a", ExpectedMinutes: 0.96, RecentReturns: 0.70, GoalsScored: 5, Assists: 4},
		{ID: 10, FirstName: "Eli", SecondName: "Fox", WebName: "Fox", Position: Midfielder, TeamID: 3, Price: 6.5, TotalPoints: 83, Form: 6.8, Minutes: 810, Value: 12.8, Status: "a", ExpectedMinutes: 0.90, RecentReturns: 0.59, GoalsScored: 4, Assists: 4},
		{ID: 11, FirstName: "Adam", SecondName: "King", WebName: "King", Position: Midfielder, TeamID: 4, Price: 6.0, TotalPoints: 72, Form: 5.9, Minutes: 765, Value: 12.0, Status: "a", ExpectedMinutes: 0.88, RecentReturns: 0.49, GoalsScored: 3, Assists: 3},
		{ID: 12, FirstName: "Max", SecondName: "Hart", WebName: "Hart", Position: Midfielder, TeamID: 5, Price: 5.5, TotalPoints: 65, Form: 5.5, Minutes: 720, Value: 11.8, Status: "a", ExpectedMinutes: 0.83, RecentReturns: 0.42, GoalsScored: 2, Assists: 4},
		{ID: 13, FirstName: "Kai", SecondName: "Young", WebName: "Young", Position: Forward, TeamID: 1, Price: 7.5, TotalPoints: 105, Form: 8.2, Minutes: 870, Value: 14.0, Status: "a", ExpectedMinutes: 0.96, RecentReturns: 0.78, GoalsScored: 8, Assists: 2},
		{ID: 14, FirstName: "Luca", SecondName: "Miles", WebName: "Miles", Position: Forward, TeamID: 3, Price: 6.5, TotalPoints: 88, Form: 7.0, Minutes: 810, Value: 13.5, Status: "a", ExpectedMinutes: 0.91, RecentReturns: 0.63, GoalsScored: 6, Assists: 3},
		{ID: 15, FirstName: "Rory", SecondName: "West", WebName: "West", Position: Forward, TeamID: 4, Price: 5.5, TotalPoints: 63, Form: 5.6, Minutes: 720, Value: 11.5, Status: "a", ExpectedMinutes: 0.82, RecentReturns: 0.45, GoalsScored: 4, Assists: 2},
		{ID: 16, FirstName: "Ivo", SecondName: "Nash", WebName: "Nash", Position: Midfielder, TeamID: 2, Price: 5.0, TotalPoints: 45, Form: 4.1, Minutes: 500, Value: 9.0, Status: "a", ExpectedMinutes: 0.62, RecentReturns: 0.26},
		{ID: 17, FirstName: "Ben", SecondName: "Oak", WebName: "Oak", Position: Defender, TeamID: 3, Price: 4.0, TotalPoints: 37, Form: 3.5, Minutes: 540, Value: 9.2, Status: "a", ExpectedMinutes: 0.61, RecentReturns: 0.24},
		{ID: 18, FirstName: "Tom", SecondName: "Park", WebName: "Park", Position: Defender, TeamID: 4, Price: 4.0, TotalPoints: 33, Form: 3.2, Minutes: 450, Value: 8.3, Status: "a", ExpectedMinutes: 0.56, RecentReturns: 0.20},
		{ID: 22, FirstName: "Isaac", SecondName: "Vale", WebName: "Vale", Position: Defender, TeamID: 5, Price: 4.0, TotalPoints: 31, Form: 3.1, Minutes: 480, Value: 7.8, Status: "a", ExpectedMinutes: 0.58, RecentReturns: 0.18},
	}
	for _, player := range players {
		s.players[player.ID] = player
		s.history[player.ID] = []PlayerHistory{{Gameweek: 1, Minutes: player.Minutes / 20, TotalPoints: int(player.Form), Goals: player.GoalsScored / 4, Assists: player.Assists / 4, CleanSheets: player.CleanSheets / 2, Bonus: player.Bonus, Value: player.Price}}
	}
	s.squad = Squad{Name: "My FPL squad", Budget: 100, PurchasePrices: map[int]float64{}, StartingPlayerIDs: []int{}, BenchPlayerIDs: []int{}, Validation: []ValidationError{}}
}

func (s *Store) Freshness() Freshness            { s.mu.RLock(); defer s.mu.RUnlock(); return s.sync.Freshness }
func (s *Store) SyncStatus() SyncStatus          { s.mu.RLock(); defer s.mu.RUnlock(); return s.sync }
func (s *Store) SetSyncStatus(status SyncStatus) { s.mu.Lock(); defer s.mu.Unlock(); s.sync = status }
func (s *Store) Snapshot() (Season, Gameweek, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.season, s.gameweeks[1], s.lastSnapshot
}

func (s *Store) ApplySnapshot(season Season, weeks []Gameweek, teams []Team, players []Player, fixtures []Fixture, histories map[int][]PlayerHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.season = season
	s.gameweeks = map[int]Gameweek{}
	s.teams = map[int]Team{}
	s.players = map[int]Player{}
	s.fixtures = fixtures
	for playerID, rows := range histories {
		s.history[playerID] = append([]PlayerHistory{}, rows...)
	}
	for _, w := range weeks {
		s.gameweeks[w.ID] = w
	}
	for _, team := range teams {
		s.teams[team.ID] = team
	}
	for _, player := range players {
		s.players[player.ID] = player
	}
	s.lastSnapshot = time.Now().UTC()
}

type PlayerQuery struct {
	Search     string
	Position   int
	TeamID     int
	MinPrice   float64
	MaxPrice   float64
	MinMinutes int
	MinForm    float64
	MinPoints  int
	MinValue   float64
	Status     string
	Sort       string
	Desc       bool
	Page       int
	PageSize   int
}

func (s *Store) SearchPlayers(q PlayerQuery) ([]Player, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Player, 0)
	term := strings.ToLower(strings.TrimSpace(q.Search))
	for _, player := range s.players {
		if term != "" && !strings.Contains(strings.ToLower(player.WebName+" "+player.FirstName+" "+player.SecondName), term) {
			continue
		}
		if q.Position > 0 && player.Position != q.Position {
			continue
		}
		if q.TeamID > 0 && player.TeamID != q.TeamID {
			continue
		}
		if q.MinPrice > 0 && player.Price < q.MinPrice {
			continue
		}
		if q.MaxPrice > 0 && player.Price > q.MaxPrice {
			continue
		}
		if q.MinMinutes > 0 && player.Minutes < q.MinMinutes {
			continue
		}
		if q.MinForm > 0 && player.Form < q.MinForm {
			continue
		}
		if q.MinPoints > 0 && player.TotalPoints < q.MinPoints {
			continue
		}
		if q.MinValue > 0 && player.Value < q.MinValue {
			continue
		}
		if q.Status != "" && player.Status != q.Status {
			continue
		}
		items = append(items, player)
	}
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		var less bool
		switch q.Sort {
		case "price":
			less = left.Price < right.Price
		case "form":
			less = left.Form < right.Form
		case "points":
			less = left.TotalPoints < right.TotalPoints
		case "minutes":
			less = left.Minutes < right.Minutes
		case "value":
			less = left.Value < right.Value
		default:
			less = strings.ToLower(left.WebName) < strings.ToLower(right.WebName)
		}
		if q.Desc {
			less = !less
		}
		if left.WebName == right.WebName {
			return left.ID < right.ID
		}
		return less
	})
	total := len(items)
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size < 1 || size > 100 {
		size = 25
	}
	start := (page - 1) * size
	if start >= len(items) {
		return []Player{}, total
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total
}

func (s *Store) Player(id int) (Player, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	player, ok := s.players[id]
	return player, ok
}
func (s *Store) Team(id int) (Team, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	team, ok := s.teams[id]
	return team, ok
}
func (s *Store) History(id int) []PlayerHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]PlayerHistory{}, s.history[id]...)
}
func (s *Store) UpcomingFixtures(teamID int) []Fixture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Fixture{}
	for _, fixture := range s.fixtures {
		if !fixture.Finished && (fixture.HomeTeam == teamID || fixture.AwayTeam == teamID) {
			result = append(result, fixture)
		}
	}
	return result
}
func (s *Store) AllPlayers() []Player {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Player, 0, len(s.players))
	for _, player := range s.players {
		result = append(result, player)
	}
	return result
}

func (s *Store) ExportSnapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	weeks := make([]Gameweek, 0, len(s.gameweeks))
	for _, week := range s.gameweeks {
		weeks = append(weeks, week)
	}
	teams := make([]Team, 0, len(s.teams))
	for _, team := range s.teams {
		teams = append(teams, team)
	}
	players := make([]Player, 0, len(s.players))
	for _, player := range s.players {
		players = append(players, player)
	}
	histories := map[int][]PlayerHistory{}
	for id, items := range s.history {
		histories[id] = append([]PlayerHistory{}, items...)
	}
	return Snapshot{Season: s.season, Gameweeks: weeks, Teams: teams, Players: players, Fixtures: append([]Fixture{}, s.fixtures...), Histories: histories}
}
func (s *Store) GetSquad() Squad       { s.mu.RLock(); defer s.mu.RUnlock(); return cloneSquad(s.squad) }
func (s *Store) SaveSquad(squad Squad) { s.mu.Lock(); defer s.mu.Unlock(); s.squad = cloneSquad(squad) }

func cloneSquad(input Squad) Squad {
	output := input
	output.Players = append([]Player{}, input.Players...)
	output.PurchasePrices = map[int]float64{}
	for id, price := range input.PurchasePrices {
		output.PurchasePrices[id] = price
	}
	output.StartingPlayerIDs = append([]int{}, input.StartingPlayerIDs...)
	output.BenchPlayerIDs = append([]int{}, input.BenchPlayerIDs...)
	output.Validation = append([]ValidationError{}, input.Validation...)
	return output
}
func (s *Store) EnrichSquad(squad Squad) Squad {
	squad.Players = []Player{}
	for id := range squad.PurchasePrices {
		if player, ok := s.Player(id); ok {
			squad.Players = append(squad.Players, player)
		}
	}
	sort.Slice(squad.Players, func(i, j int) bool { return squad.Players[i].ID < squad.Players[j].ID })
	squad.TotalCost = 0
	for _, p := range squad.Players {
		price := squad.PurchasePrices[p.ID]
		if price == 0 {
			price = p.Price
		}
		squad.TotalCost += price
	}
	squad.RemainingBudget = squad.Budget - squad.TotalCost
	return squad
}
func positionName(position int) string {
	switch position {
	case Goalkeeper:
		return "Goalkeeper"
	case Defender:
		return "Defender"
	case Midfielder:
		return "Midfielder"
	case Forward:
		return "Forward"
	default:
		return fmt.Sprintf("Position %d", position)
	}
}
