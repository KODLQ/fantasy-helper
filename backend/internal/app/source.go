package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type FPLSource struct {
	BaseURL        string
	Client         *http.Client
	Retries        int
	SeasonID       int
	SeasonName     string
	AllowDiscovery bool
	OnObservation  func(SourceObservation)
}
type SourceObservation struct {
	Endpoint        string
	FetchedAt       time.Time
	HTTPStatus      int
	Checksum        string
	ValidationState string
	SchemaVersion   string
	Payload         json.RawMessage
	Diagnostic      string
}
type bootstrapResponse struct {
	SeasonID     int                 `json:"season_id"`
	SeasonName   string              `json:"season_name"`
	Events       []sourceEvent       `json:"events"`
	Phases       []sourcePhase       `json:"phases"`
	Settings     json.RawMessage     `json:"game_settings"`
	ElementTypes []sourceElementType `json:"element_types"`
	Teams        []sourceTeam        `json:"teams"`
	Elements     []sourceElement     `json:"elements"`
}

type sourcePhase struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	StartEvent int    `json:"start_event"`
	StopEvent  int    `json:"stop_event"`
}
type SourcePhase = sourcePhase
type sourceElementType struct {
	ID           int    `json:"id"`
	SingularName string `json:"singular_name"`
	PluralName   string `json:"plural_name"`
	SquadSelect  int    `json:"squad_select"`
	SquadMin     int    `json:"squad_min_select"`
	SquadMax     int    `json:"squad_max_select"`
}
type SourceElementType = sourceElementType
type BootstrapCatalog struct {
	SeasonID     int
	SeasonName   string
	Events       []sourceEvent
	Phases       []sourcePhase
	Settings     json.RawMessage
	ElementTypes []sourceElementType
	Teams        []sourceTeam
	Elements     []sourceElement
}
type sourceEvent struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	DeadlineTime *time.Time `json:"deadline_time"`
	Finished     bool       `json:"finished"`
	IsCurrent    bool       `json:"is_current"`
	AverageScore float64    `json:"average_entry_score"`
}
type SourceEvent = sourceEvent
type sourceTeam struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	Strength     int    `json:"strength"`
	StrengthHome int    `json:"strength_overall_home"`
	StrengthAway int    `json:"strength_overall_away"`
	AttackHome   int    `json:"strength_attack_home"`
	AttackAway   int    `json:"strength_attack_away"`
	DefenceHome  int    `json:"strength_defence_home"`
	DefenceAway  int    `json:"strength_defence_away"`
}
type SourceTeam = sourceTeam
type sourceElement struct {
	ID                int    `json:"id"`
	FirstName         string `json:"first_name"`
	SecondName        string `json:"second_name"`
	WebName           string `json:"web_name"`
	ElementType       int    `json:"element_type"`
	Team              int    `json:"team"`
	NowCost           int    `json:"now_cost"`
	TotalPoints       int    `json:"total_points"`
	Form              string `json:"form"`
	Minutes           int    `json:"minutes"`
	ValueForm         string `json:"value_form"`
	Status            string `json:"status"`
	News              string `json:"news"`
	Chance            *int   `json:"chance_of_playing_next_round"`
	Goals             int    `json:"goals_scored"`
	Assists           int    `json:"assists"`
	CleanSheets       int    `json:"clean_sheets"`
	Bonus             int    `json:"bonus"`
	Saves             int    `json:"saves"`
	SelectedByPercent string `json:"selected_by_percent"`
	YellowCards       int    `json:"yellow_cards"`
	RedCards          int    `json:"red_cards"`
	OwnGoals          int    `json:"own_goals"`
	PenaltiesSaved    int    `json:"penalties_saved"`
	PenaltiesMissed   int    `json:"penalties_missed"`
	ExpectedGoals     string `json:"expected_goals"`
	ExpectedAssists   string `json:"expected_assists"`
}
type SourceElement = sourceElement
type sourceFixture struct {
	ID       int        `json:"id"`
	Event    int        `json:"event"`
	Kickoff  *time.Time `json:"kickoff_time"`
	Finished bool       `json:"finished"`
	TeamH    int        `json:"team_h"`
	TeamA    int        `json:"team_a"`
	HDiff    int        `json:"team_h_difficulty"`
	ADiff    int        `json:"team_a_difficulty"`
	HScore   *int       `json:"team_h_score"`
	AScore   *int       `json:"team_a_score"`
}
type SourceFixture = sourceFixture
type playerSummary struct {
	History []sourceHistory `json:"history"`
}
type sourceHistory struct {
	Element     int `json:"element"`
	Round       int `json:"round"`
	Minutes     int `json:"minutes"`
	Points      int `json:"total_points"`
	Goals       int `json:"goals_scored"`
	Assists     int `json:"assists"`
	CleanSheets int `json:"clean_sheets"`
	Bonus       int `json:"bonus"`
	Value       int `json:"value"`
}
type SourceHistory = sourceHistory

type LivePlayerStats struct {
	PlayerID        int    `json:"element"`
	Minutes         int    `json:"minutes"`
	Points          int    `json:"total_points"`
	Goals           int    `json:"goals_scored"`
	Assists         int    `json:"assists"`
	CleanSheets     int    `json:"clean_sheets"`
	Bonus           int    `json:"bonus"`
	BPS             int    `json:"bps"`
	Saves           int    `json:"saves"`
	YellowCards     int    `json:"yellow_cards"`
	RedCards        int    `json:"red_cards"`
	OwnGoals        int    `json:"own_goals"`
	PenaltiesSaved  int    `json:"penalties_saved"`
	PenaltiesMissed int    `json:"penalties_missed"`
	ExpectedGoals   string `json:"expected_goals"`
	ExpectedAssists string `json:"expected_assists"`
}
type EventLive struct {
	Elements []LivePlayerStats `json:"elements"`
}
type FutureFixture struct {
	ID          int        `json:"id"`
	Event       *int       `json:"event"`
	KickoffTime *time.Time `json:"kickoff_time"`
	TeamH       int        `json:"team_h"`
	TeamA       int        `json:"team_a"`
	IsHome      bool       `json:"is_home"`
	Difficulty  int        `json:"difficulty"`
}
type ElementSummary struct {
	History     []sourceHistory          `json:"history"`
	HistoryPast []map[string]interface{} `json:"history_past"`
	Fixtures    []FutureFixture          `json:"fixtures"`
}

func NewFPLSource(baseURL string) *FPLSource {
	return &FPLSource{BaseURL: strings.TrimRight(baseURL, "/"), Client: &http.Client{Timeout: 20 * time.Second}, Retries: 2, AllowDiscovery: true}
}
func NewFPLSourceWithSeason(baseURL string, seasonID int, seasonName string) *FPLSource {
	source := NewFPLSource(baseURL)
	source.SeasonID = seasonID
	source.SeasonName = seasonName
	source.AllowDiscovery = false
	return source
}
func (f *FPLSource) get(ctx context.Context, path string, target interface{}) (string, error) {
	var last error
	for attempt := 0; attempt <= f.Retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.BaseURL+path, nil)
		if err != nil {
			return "", err
		}
		response, err := f.Client.Do(req)
		if err != nil {
			last = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		response.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests {
			last = fmt.Errorf("source returned %s", response.Status)
			wait := time.Duration(attempt+1) * 150 * time.Millisecond
			if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, parseErr := time.ParseDuration(retryAfter + "s"); parseErr == nil {
					wait = seconds
				}
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", fmt.Errorf("source returned %s", response.Status)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		if err := json.Unmarshal(body, target); err != nil {
			if f.OnObservation != nil {
				f.OnObservation(SourceObservation{Endpoint: path, FetchedAt: time.Now().UTC(), HTTPStatus: response.StatusCode, Checksum: checksum, ValidationState: "invalid", SchemaVersion: "fpl-public-v1", Diagnostic: err.Error()})
			}
			return "", fmt.Errorf("decode %s: %w", path, err)
		}
		if f.OnObservation != nil {
			f.OnObservation(SourceObservation{Endpoint: path, FetchedAt: time.Now().UTC(), HTTPStatus: response.StatusCode, Checksum: checksum, ValidationState: "valid", SchemaVersion: "fpl-public-v1", Payload: append(json.RawMessage(nil), body...)})
		}
		return checksum, nil
	}
	return "", last
}

func (f *FPLSource) Bootstrap(ctx context.Context) (BootstrapCatalog, string, error) {
	var payload bootstrapResponse
	checksum, err := f.get(ctx, "/bootstrap-static/", &payload)
	if err != nil {
		return BootstrapCatalog{}, "", err
	}
	if payload.Events == nil || payload.Teams == nil || payload.Elements == nil {
		return BootstrapCatalog{}, checksum, fmt.Errorf("bootstrap-static response is missing events, teams, or elements")
	}
	return BootstrapCatalog{SeasonID: payload.SeasonID, SeasonName: payload.SeasonName, Events: payload.Events, Phases: payload.Phases, Settings: payload.Settings, ElementTypes: payload.ElementTypes, Teams: payload.Teams, Elements: payload.Elements}, checksum, nil
}

func (f *FPLSource) EventLive(ctx context.Context, gameweek int) (EventLive, string, error) {
	var payload EventLive
	checksum, err := f.get(ctx, fmt.Sprintf("/event/%d/live/", gameweek), &payload)
	if err == nil && payload.Elements == nil {
		return EventLive{}, checksum, fmt.Errorf("event-live response is missing elements")
	}
	return payload, checksum, err
}

func (f *FPLSource) ElementSummary(ctx context.Context, playerID int) (ElementSummary, string, error) {
	var payload ElementSummary
	checksum, err := f.get(ctx, fmt.Sprintf("/element-summary/%d/", playerID), &payload)
	if err == nil && payload.History == nil && payload.HistoryPast == nil && payload.Fixtures == nil {
		return ElementSummary{}, checksum, fmt.Errorf("element-summary response is missing history and fixtures")
	}
	return payload, checksum, err
}

func (f *FPLSource) Snapshot(ctx context.Context) (Season, []Gameweek, []Team, []Player, []Fixture, string, error) {
	catalog, checksum, err := f.Bootstrap(ctx)
	if err != nil {
		return Season{}, nil, nil, nil, nil, "", err
	}
	var fixtures []sourceFixture
	fixtureChecksum, err := f.get(ctx, "/fixtures/", &fixtures)
	if err != nil {
		return Season{}, nil, nil, nil, nil, checksum, err
	}
	if fixtures == nil {
		return Season{}, nil, nil, nil, nil, checksum, fmt.Errorf("fixtures response must be an array")
	}
	checksum = checksum + ":" + fixtureChecksum
	seasonID, seasonName := f.SeasonID, f.SeasonName
	if f.AllowDiscovery && seasonID == 0 && seasonName == "" {
		seasonID, seasonName = catalog.SeasonID, catalog.SeasonName
	}
	if seasonID <= 0 || strings.TrimSpace(seasonName) == "" {
		return Season{}, nil, nil, nil, nil, checksum, fmt.Errorf("source season identity is required: configure FPL_SOURCE_SEASON_ID and FPL_SOURCE_SEASON_NAME or provide bootstrap discovery metadata")
	}
	season := Season{ID: seasonID, Name: seasonName, IsCurrent: true, UpdatedAt: time.Now().UTC()}
	weeks := make([]Gameweek, 0, len(catalog.Events))
	for _, event := range catalog.Events {
		weeks = append(weeks, Gameweek{ID: event.ID, Name: event.Name, DeadlineTime: event.DeadlineTime, Finished: event.Finished, IsCurrent: event.IsCurrent, AverageScore: event.AverageScore})
	}
	teams := make([]Team, 0, len(catalog.Teams))
	for _, team := range catalog.Teams {
		teams = append(teams, Team{ID: team.ID, Name: team.Name, ShortName: team.ShortName, Strength: team.Strength, StrengthHome: team.StrengthHome, StrengthAway: team.StrengthAway, AttackHome: team.AttackHome, AttackAway: team.AttackAway, DefenceHome: team.DefenceHome, DefenceAway: team.DefenceAway})
	}
	players := make([]Player, 0, len(catalog.Elements))
	for _, player := range catalog.Elements {
		players = append(players, Player{ID: player.ID, FirstName: player.FirstName, SecondName: player.SecondName, WebName: player.WebName, Position: player.ElementType, TeamID: player.Team, Price: float64(player.NowCost) / 10, TotalPoints: player.TotalPoints, Form: parseFloat(player.Form), Minutes: player.Minutes, Value: parseFloat(player.ValueForm), Status: player.Status, News: player.News, ChanceOfPlaying: player.Chance, GoalsScored: player.Goals, Assists: player.Assists, CleanSheets: player.CleanSheets, Bonus: player.Bonus, Saves: player.Saves, SelectedByPercent: parseFloat(player.SelectedByPercent), YellowCards: player.YellowCards, RedCards: player.RedCards, OwnGoals: player.OwnGoals, PenaltiesSaved: player.PenaltiesSaved, PenaltiesMissed: player.PenaltiesMissed, ExpectedGoals: parseFloat(player.ExpectedGoals), ExpectedAssists: parseFloat(player.ExpectedAssists), ExpectedMinutes: minutesSignal(player.Minutes), RecentReturns: float64(player.Goals+player.Assists) / 10})
	}
	normalizedFixtures := make([]Fixture, 0, len(fixtures))
	for _, fixture := range fixtures {
		normalizedFixtures = append(normalizedFixtures, Fixture{ID: fixture.ID, Gameweek: fixture.Event, KickoffTime: fixture.Kickoff, Finished: fixture.Finished, HomeTeam: fixture.TeamH, AwayTeam: fixture.TeamA, HomeDifficulty: fixture.HDiff, AwayDifficulty: fixture.ADiff, HomeScore: fixture.HScore, AwayScore: fixture.AScore})
	}
	return season, weeks, teams, players, normalizedFixtures, checksum, nil
}
func (f *FPLSource) PlayerHistory(ctx context.Context, playerID int) ([]PlayerHistory, string, error) {
	var summary playerSummary
	checksum, err := f.get(ctx, fmt.Sprintf("/element-summary/%d/", playerID), &summary)
	if err != nil {
		return nil, "", err
	}
	history := make([]PlayerHistory, 0, len(summary.History))
	for _, row := range summary.History {
		history = append(history, PlayerHistory{Gameweek: row.Round, Minutes: row.Minutes, TotalPoints: row.Points, Goals: row.Goals, Assists: row.Assists, CleanSheets: row.CleanSheets, Bonus: row.Bonus, Value: float64(row.Value) / 10})
	}
	return history, checksum, nil
}
func parseFloat(value string) float64 {
	var result float64
	_, _ = fmt.Sscanf(value, "%f", &result)
	return result
}
func minutesSignal(minutes int) float64 {
	if minutes >= 900 {
		return 1
	}
	if minutes <= 0 {
		return 0
	}
	return float64(minutes) / 900
}
