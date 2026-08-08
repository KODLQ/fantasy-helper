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
	BaseURL string
	Client  *http.Client
	Retries int
}
type bootstrapResponse struct {
	Events   []sourceEvent   `json:"events"`
	Teams    []sourceTeam    `json:"teams"`
	Elements []sourceElement `json:"elements"`
}
type sourceEvent struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	DeadlineTime *time.Time `json:"deadline_time"`
	Finished     bool       `json:"finished"`
	IsCurrent    bool       `json:"is_current"`
	AverageScore float64    `json:"average_entry_score"`
}
type sourceTeam struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}
type sourceElement struct {
	ID          int    `json:"id"`
	FirstName   string `json:"first_name"`
	SecondName  string `json:"second_name"`
	WebName     string `json:"web_name"`
	ElementType int    `json:"element_type"`
	Team        int    `json:"team"`
	NowCost     int    `json:"now_cost"`
	TotalPoints int    `json:"total_points"`
	Form        string `json:"form"`
	Minutes     int    `json:"minutes"`
	ValueForm   string `json:"value_form"`
	Status      string `json:"status"`
	News        string `json:"news"`
	Chance      *int   `json:"chance_of_playing_next_round"`
	Goals       int    `json:"goals_scored"`
	Assists     int    `json:"assists"`
	CleanSheets int    `json:"clean_sheets"`
	Bonus       int    `json:"bonus"`
	Saves       int    `json:"saves"`
}
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

func NewFPLSource(baseURL string) *FPLSource {
	return &FPLSource{BaseURL: strings.TrimRight(baseURL, "/"), Client: &http.Client{Timeout: 20 * time.Second}, Retries: 2}
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
		if response.StatusCode >= 500 {
			last = fmt.Errorf("source returned %s", response.Status)
			time.Sleep(time.Duration(attempt+1) * 150 * time.Millisecond)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", fmt.Errorf("source returned %s", response.Status)
		}
		if err := json.Unmarshal(body, target); err != nil {
			return "", fmt.Errorf("decode %s: %w", path, err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(body)), nil
	}
	return "", last
}

func (f *FPLSource) Snapshot(ctx context.Context) (Season, []Gameweek, []Team, []Player, []Fixture, string, error) {
	var bootstrap bootstrapResponse
	checksum, err := f.get(ctx, "/bootstrap-static/", &bootstrap)
	if err != nil {
		return Season{}, nil, nil, nil, nil, "", err
	}
	var fixtures []sourceFixture
	fixtureChecksum, err := f.get(ctx, "/fixtures/", &fixtures)
	if err != nil {
		return Season{}, nil, nil, nil, nil, checksum, err
	}
	checksum = checksum + ":" + fixtureChecksum
	season := Season{ID: 1, Name: fmt.Sprintf("%d/%d", time.Now().Year(), time.Now().Year()+1), IsCurrent: true, UpdatedAt: time.Now().UTC()}
	weeks := make([]Gameweek, 0, len(bootstrap.Events))
	for _, event := range bootstrap.Events {
		weeks = append(weeks, Gameweek{ID: event.ID, Name: event.Name, DeadlineTime: event.DeadlineTime, Finished: event.Finished, IsCurrent: event.IsCurrent, AverageScore: event.AverageScore})
	}
	teams := make([]Team, 0, len(bootstrap.Teams))
	for _, team := range bootstrap.Teams {
		teams = append(teams, Team{ID: team.ID, Name: team.Name, ShortName: team.ShortName})
	}
	players := make([]Player, 0, len(bootstrap.Elements))
	for _, player := range bootstrap.Elements {
		players = append(players, Player{ID: player.ID, FirstName: player.FirstName, SecondName: player.SecondName, WebName: player.WebName, Position: player.ElementType, TeamID: player.Team, Price: float64(player.NowCost) / 10, TotalPoints: player.TotalPoints, Form: parseFloat(player.Form), Minutes: player.Minutes, Value: parseFloat(player.ValueForm), Status: player.Status, News: player.News, ChanceOfPlaying: player.Chance, GoalsScored: player.Goals, Assists: player.Assists, CleanSheets: player.CleanSheets, Bonus: player.Bonus, Saves: player.Saves, ExpectedMinutes: minutesSignal(player.Minutes), RecentReturns: float64(player.Goals+player.Assists) / 10})
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
