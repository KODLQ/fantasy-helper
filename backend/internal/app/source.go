package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
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
	RetryJitter    time.Duration
	MaxConcurrent  int
	semaphore      chan struct{}
	metrics        sourceMetrics
}

type sourceMetrics struct {
	Requests            atomic.Uint64
	Retries             atomic.Uint64
	RateLimited         atomic.Uint64
	ServerErrors        atomic.Uint64
	TransportErrors     atomic.Uint64
	InFlight            atomic.Uint64
	PeakConcurrent      atomic.Uint64
	TotalDurationMillis atomic.Uint64
	LastStatus          atomic.Int64
}

type SourceMetrics struct {
	Requests        uint64 `json:"requests"`
	Retries         uint64 `json:"retries"`
	RateLimited     uint64 `json:"rateLimited"`
	ServerErrors    uint64 `json:"serverErrors"`
	TransportErrors uint64 `json:"transportErrors"`
	InFlight        uint64 `json:"inFlight"`
	PeakConcurrent  uint64 `json:"peakConcurrent"`
	TotalDurationMs uint64 `json:"totalDurationMs"`
	LastStatus      int    `json:"lastStatus"`
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
	TotalPlayers int                 `json:"total_players"`
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
	TotalPlayers int
	Events       []SourceEvent
	Phases       []SourcePhase
	Settings     json.RawMessage
	ElementTypes []SourceElementType
	Teams        []SourceTeam
	Elements     []SourceElement
}
type sourceEvent struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	DeadlineTime *time.Time `json:"deadline_time"`
	Finished     bool       `json:"finished"`
	IsCurrent    bool       `json:"is_current"`
	DataChecked  bool       `json:"data_checked"`
	AverageScore float64    `json:"average_entry_score"`
}
type SourceEvent = sourceEvent
type sourceTeam struct {
	ID           int    `json:"id"`
	Code         int    `json:"code"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	Played       int    `json:"played"`
	Win          int    `json:"win"`
	Draw         int    `json:"draw"`
	Loss         int    `json:"loss"`
	Points       int    `json:"points"`
	Position     int    `json:"position"`
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
	ID                   int    `json:"id"`
	Code                 int    `json:"code"`
	FirstName            string `json:"first_name"`
	SecondName           string `json:"second_name"`
	WebName              string `json:"web_name"`
	ElementType          int    `json:"element_type"`
	Team                 int    `json:"team"`
	NowCost              int    `json:"now_cost"`
	TotalPoints          int    `json:"total_points"`
	Form                 string `json:"form"`
	Minutes              int    `json:"minutes"`
	ValueForm            string `json:"value_form"`
	Status               string `json:"status"`
	News                 string `json:"news"`
	Chance               *int   `json:"chance_of_playing_next_round"`
	Goals                int    `json:"goals_scored"`
	Assists              int    `json:"assists"`
	CleanSheets          int    `json:"clean_sheets"`
	Bonus                int    `json:"bonus"`
	Saves                int    `json:"saves"`
	SelectedByPercent    string `json:"selected_by_percent"`
	YellowCards          int    `json:"yellow_cards"`
	RedCards             int    `json:"red_cards"`
	OwnGoals             int    `json:"own_goals"`
	PenaltiesSaved       int    `json:"penalties_saved"`
	PenaltiesMissed      int    `json:"penalties_missed"`
	ExpectedGoals        string `json:"expected_goals"`
	ExpectedAssists      string `json:"expected_assists"`
	ExpectedGoalsPer90   string `json:"expected_goal_per_90"`
	ExpectedAssistsPer90 string `json:"expected_assists_per_90"`
	Influence            string `json:"influence"`
	Creativity           string `json:"creativity"`
	Threat               string `json:"threat"`
	ICTIndex             string `json:"ict_index"`
	PointsPerGame        string `json:"points_per_game"`
	EpThis               string `json:"ep_this"`
	EpNext               string `json:"ep_next"`
	ValueSeason          string `json:"value_season"`
	CostChangeStart      int    `json:"cost_change_start"`
	CostChangeEvent      int    `json:"cost_change_event"`
	TransfersIn          int    `json:"transfers_in"`
	TransfersOut         int    `json:"transfers_out"`
	TransfersInEvent     int    `json:"transfers_in_event"`
	TransfersOutEvent    int    `json:"transfers_out_event"`
	Starts               int    `json:"starts"`
	DreamteamCount       int    `json:"dreamteam_count"`
	InDreamteam          bool   `json:"in_dreamteam"`
}
type SourceElement = sourceElement
type sourceFixture struct {
	ID          int                 `json:"id"`
	Code        int                 `json:"code"`
	Event       *int                `json:"event"`
	Kickoff     *time.Time          `json:"kickoff_time"`
	Finished    bool                `json:"finished"`
	Provisional bool                `json:"provisional_start_time"`
	Started     *bool               `json:"started"`
	TeamH       int                 `json:"team_h"`
	TeamA       int                 `json:"team_a"`
	HDiff       int                 `json:"team_h_difficulty"`
	ADiff       int                 `json:"team_a_difficulty"`
	HScore      *int                `json:"team_h_score"`
	AScore      *int                `json:"team_a_score"`
	Stats       []SourceFixtureStat `json:"stats"`
}
type SourceFixture = sourceFixture
type sourceFixtureStat struct {
	Identifier string            `json:"identifier"`
	Home       []SourceStatValue `json:"h"`
	Away       []SourceStatValue `json:"a"`
}
type SourceFixtureStat = sourceFixtureStat
type sourceStatValue struct {
	Element int `json:"element"`
	Value   int `json:"value"`
}
type SourceStatValue = sourceStatValue
type playerSummary struct {
	History []sourceHistory `json:"history"`
}
type sourceHistory struct {
	Element      int        `json:"element"`
	Round        int        `json:"round"`
	Fixture      int        `json:"fixture"`
	OpponentTeam int        `json:"opponent_team"`
	IsHome       bool       `json:"was_home"`
	KickoffTime  *time.Time `json:"kickoff_time"`
	Minutes      int        `json:"minutes"`
	Points       int        `json:"total_points"`
	Goals        int        `json:"goals_scored"`
	Assists      int        `json:"assists"`
	CleanSheets  int        `json:"clean_sheets"`
	Bonus        int        `json:"bonus"`
	Value        int        `json:"value"`
}
type SourceHistory = sourceHistory

type LivePlayerStats struct {
	PlayerID         int    `json:"element"`
	Minutes          int    `json:"minutes"`
	Points           int    `json:"total_points"`
	Goals            int    `json:"goals_scored"`
	Assists          int    `json:"assists"`
	CleanSheets      int    `json:"clean_sheets"`
	GoalsConceded    int    `json:"goals_conceded"`
	Bonus            int    `json:"bonus"`
	BPS              int    `json:"bps"`
	Saves            int    `json:"saves"`
	YellowCards      int    `json:"yellow_cards"`
	RedCards         int    `json:"red_cards"`
	OwnGoals         int    `json:"own_goals"`
	PenaltiesSaved   int    `json:"penalties_saved"`
	PenaltiesMissed  int    `json:"penalties_missed"`
	TransfersBalance int    `json:"transfers_balance"`
	Selected         int    `json:"selected"`
	TransfersIn      int    `json:"transfers_in"`
	TransfersOut     int    `json:"transfers_out"`
	InDreamteam      bool   `json:"in_dreamteam"`
	Influence        string `json:"influence"`
	Creativity       string `json:"creativity"`
	Threat           string `json:"threat"`
	ICTIndex         string `json:"ict_index"`
	ExpectedGoals    string `json:"expected_goals"`
	ExpectedAssists  string `json:"expected_assists"`
}
type EventLive struct {
	Elements  []LivePlayerStats `json:"elements"`
	Finalized *bool             `json:"finished,omitempty"`
}

// UnmarshalJSON accepts the official nested event-live shape and the flat
// shape used by older source fixtures. Keeping this compatibility here makes
// the canonical contract independent of a source fixture's representation.
func (e *EventLive) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Elements []json.RawMessage `json:"elements"`
		Finished *bool             `json:"finished"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	e.Elements = make([]LivePlayerStats, 0, len(envelope.Elements))
	e.Finalized = envelope.Finished
	for _, raw := range envelope.Elements {
		var nested struct {
			ID    int             `json:"id"`
			Stats LivePlayerStats `json:"stats"`
		}
		if err := json.Unmarshal(raw, &nested); err != nil {
			return err
		}
		if nested.Stats.PlayerID == 0 && nested.ID != 0 {
			nested.Stats.PlayerID = nested.ID
		}
		if nested.Stats.PlayerID != 0 || nested.ID != 0 {
			e.Elements = append(e.Elements, nested.Stats)
			continue
		}
		var flat LivePlayerStats
		if err := json.Unmarshal(raw, &flat); err != nil {
			return err
		}
		e.Elements = append(e.Elements, flat)
	}
	return nil
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
	History     []SourceHistory     `json:"history"`
	HistoryPast []PastSeasonHistory `json:"history_past"`
	Fixtures    []FutureFixture     `json:"fixtures"`
}
type PastSeasonHistory struct {
	SeasonName  string `json:"season_name"`
	TotalPoints int    `json:"total_points"`
	Minutes     int    `json:"minutes"`
	Goals       int    `json:"goals_scored"`
	Assists     int    `json:"assists"`
	CleanSheets int    `json:"clean_sheets"`
	Bonus       int    `json:"bonus"`
	Value       int    `json:"value"`
}

func NewFPLSource(baseURL string) *FPLSource {
	source := &FPLSource{BaseURL: strings.TrimRight(baseURL, "/"), Client: &http.Client{Timeout: 20 * time.Second}, Retries: 2, AllowDiscovery: true, RetryJitter: 100 * time.Millisecond, MaxConcurrent: 6}
	source.configureSemaphore()
	return source
}
func NewFPLSourceWithSeason(baseURL string, seasonID int, seasonName string) *FPLSource {
	source := NewFPLSource(baseURL)
	source.SeasonID = seasonID
	source.SeasonName = seasonName
	source.AllowDiscovery = false
	return source
}
func (f *FPLSource) configureSemaphore() {
	if f.MaxConcurrent < 1 {
		f.MaxConcurrent = 1
	}
	if cap(f.semaphore) != f.MaxConcurrent {
		f.semaphore = make(chan struct{}, f.MaxConcurrent)
	}
}
func (f *FPLSource) SetMaxConcurrent(value int) {
	if value < 1 {
		value = 1
	}
	f.MaxConcurrent = value
	f.configureSemaphore()
}
func (f *FPLSource) Metrics() SourceMetrics {
	return SourceMetrics{Requests: f.metrics.Requests.Load(), Retries: f.metrics.Retries.Load(), RateLimited: f.metrics.RateLimited.Load(), ServerErrors: f.metrics.ServerErrors.Load(), TransportErrors: f.metrics.TransportErrors.Load(), InFlight: f.metrics.InFlight.Load(), PeakConcurrent: f.metrics.PeakConcurrent.Load(), TotalDurationMs: f.metrics.TotalDurationMillis.Load(), LastStatus: int(f.metrics.LastStatus.Load())}
}
func (f *FPLSource) acquire(ctx context.Context) error {
	select {
	case f.semaphore <- struct{}{}:
		current := f.metrics.InFlight.Add(1)
		for peak := f.metrics.PeakConcurrent.Load(); current > peak && !f.metrics.PeakConcurrent.CompareAndSwap(peak, current); peak = f.metrics.PeakConcurrent.Load() {
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (f *FPLSource) release() {
	<-f.semaphore
	f.metrics.InFlight.Add(^uint64(0))
}
func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds
	}
	if at, err := http.ParseTime(value); err == nil {
		if delay := at.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}
func (f *FPLSource) retryDelay(attempt int, response *http.Response) time.Duration {
	exponent := attempt
	if exponent > 8 {
		exponent = 8
	}
	wait := time.Duration(1<<exponent) * 150 * time.Millisecond
	if response != nil {
		if retry := retryAfter(response.Header.Get("Retry-After"), time.Now().UTC()); retry > 0 {
			wait = retry
		}
	}
	if f.RetryJitter > 0 {
		wait += time.Duration(rand.Int63n(int64(f.RetryJitter) + 1))
	}
	return wait
}
func (f *FPLSource) get(ctx context.Context, path string, target interface{}) (string, error) {
	var last error
	for attempt := 0; attempt <= f.Retries; attempt++ {
		if err := f.acquire(ctx); err != nil {
			f.metrics.TransportErrors.Add(1)
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.BaseURL+path, nil)
		if err != nil {
			f.release()
			return "", err
		}
		requestStarted := time.Now()
		response, err := f.Client.Do(req)
		f.metrics.Requests.Add(1)
		if err != nil {
			f.release()
			f.metrics.TotalDurationMillis.Add(uint64(time.Since(requestStarted).Milliseconds()))
			last = err
			f.metrics.TransportErrors.Add(1)
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if attempt < f.Retries {
				f.metrics.Retries.Add(1)
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		f.metrics.LastStatus.Store(int64(response.StatusCode))
		response.Body.Close()
		f.release()
		f.metrics.TotalDurationMillis.Add(uint64(time.Since(requestStarted).Milliseconds()))
		if readErr != nil {
			last = readErr
			continue
		}
		if response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests {
			last = fmt.Errorf("source returned %s", response.Status)
			if response.StatusCode == http.StatusTooManyRequests {
				f.metrics.RateLimited.Add(1)
			} else {
				f.metrics.ServerErrors.Add(1)
			}
			if attempt < f.Retries {
				f.metrics.Retries.Add(1)
			}
			wait := f.retryDelay(attempt, response)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			f.metrics.ServerErrors.Add(1)
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
	f.metrics.ServerErrors.Add(1)
	return "", last
}

func (f *FPLSource) Bootstrap(ctx context.Context) (BootstrapCatalog, string, error) {
	var payload bootstrapResponse
	checksum, err := f.get(ctx, "/bootstrap-static/", &payload)
	if err != nil {
		return BootstrapCatalog{}, "", err
	}
	if payload.Events == nil || payload.Phases == nil || payload.Teams == nil || payload.Elements == nil || payload.ElementTypes == nil || len(payload.Settings) == 0 {
		return BootstrapCatalog{}, checksum, f.sourceValidationError("/bootstrap-static/", checksum, fmt.Errorf("bootstrap-static response is missing required catalog fields"))
	}
	if err := validateBootstrap(payload); err != nil {
		return BootstrapCatalog{}, checksum, f.sourceValidationError("/bootstrap-static/", checksum, err)
	}
	return BootstrapCatalog{SeasonID: payload.SeasonID, SeasonName: payload.SeasonName, TotalPlayers: payload.TotalPlayers, Events: payload.Events, Phases: payload.Phases, Settings: payload.Settings, ElementTypes: payload.ElementTypes, Teams: payload.Teams, Elements: payload.Elements}, checksum, nil
}

func validateBootstrap(payload bootstrapResponse) error {
	seen := map[int]struct{}{}
	for _, phase := range payload.Phases {
		if phase.ID <= 0 || strings.TrimSpace(phase.Name) == "" {
			return fmt.Errorf("bootstrap phase has invalid source identity")
		}
	}
	seen = map[int]struct{}{}
	for _, elementType := range payload.ElementTypes {
		if elementType.ID <= 0 || strings.TrimSpace(elementType.SingularName) == "" {
			return fmt.Errorf("bootstrap element type has invalid source identity")
		}
		if _, ok := seen[elementType.ID]; ok {
			return fmt.Errorf("bootstrap contains duplicate element type source ID %d", elementType.ID)
		}
		seen[elementType.ID] = struct{}{}
	}
	seen = map[int]struct{}{}
	for _, event := range payload.Events {
		if event.ID <= 0 || strings.TrimSpace(event.Name) == "" {
			return fmt.Errorf("bootstrap event has invalid source identity")
		}
		if _, ok := seen[event.ID]; ok {
			return fmt.Errorf("bootstrap contains duplicate event source ID %d", event.ID)
		}
		seen[event.ID] = struct{}{}
	}
	seen = map[int]struct{}{}
	for _, team := range payload.Teams {
		if team.ID <= 0 || strings.TrimSpace(team.Name) == "" {
			return fmt.Errorf("bootstrap team has invalid source identity")
		}
		if _, ok := seen[team.ID]; ok {
			return fmt.Errorf("bootstrap contains duplicate team source ID %d", team.ID)
		}
		seen[team.ID] = struct{}{}
	}
	seen = map[int]struct{}{}
	for _, player := range payload.Elements {
		if player.ID <= 0 || player.Team <= 0 || player.ElementType <= 0 {
			return fmt.Errorf("bootstrap player has invalid source identity: %d", player.ID)
		}
		if _, ok := seen[player.ID]; ok {
			return fmt.Errorf("bootstrap contains duplicate player source ID %d", player.ID)
		}
		seen[player.ID] = struct{}{}
	}
	return nil
}

type FixtureFeed struct {
	Fixtures []SourceFixture
}

func (f *FPLSource) Fixtures(ctx context.Context, gameweek int) (FixtureFeed, string, error) {
	path := "/fixtures/"
	if gameweek > 0 {
		path += "?" + url.Values{"event": []string{fmt.Sprintf("%d", gameweek)}}.Encode()
	}
	var payload []sourceFixture
	checksum, err := f.get(ctx, path, &payload)
	if err != nil {
		return FixtureFeed{}, checksum, err
	}
	if payload == nil {
		return FixtureFeed{}, checksum, f.sourceValidationError(path, checksum, fmt.Errorf("fixtures response must be an array"))
	}
	seen := map[int]struct{}{}
	for _, fixture := range payload {
		if fixture.ID <= 0 || fixture.TeamH <= 0 || fixture.TeamA <= 0 {
			return FixtureFeed{}, checksum, f.sourceValidationError(path, checksum, fmt.Errorf("fixture has invalid source identity: %d", fixture.ID))
		}
		if _, ok := seen[fixture.ID]; ok {
			return FixtureFeed{}, checksum, f.sourceValidationError(path, checksum, fmt.Errorf("fixtures contains duplicate source ID %d", fixture.ID))
		}
		seen[fixture.ID] = struct{}{}
		for _, stats := range fixture.Stats {
			if strings.TrimSpace(stats.Identifier) == "" {
				return FixtureFeed{}, checksum, f.sourceValidationError(path, checksum, fmt.Errorf("fixture %d has a statistic without an identifier", fixture.ID))
			}
		}
	}
	return FixtureFeed{Fixtures: payload}, checksum, nil
}

func (f *FPLSource) EventLive(ctx context.Context, gameweek int) (EventLive, string, error) {
	var payload EventLive
	checksum, err := f.get(ctx, fmt.Sprintf("/event/%d/live/", gameweek), &payload)
	if err == nil && payload.Elements == nil {
		return EventLive{}, checksum, f.sourceValidationError(fmt.Sprintf("/event/%d/live/", gameweek), checksum, fmt.Errorf("event-live response is missing elements"))
	}
	if err == nil {
		for _, player := range payload.Elements {
			if player.PlayerID <= 0 {
				return EventLive{}, checksum, f.sourceValidationError(fmt.Sprintf("/event/%d/live/", gameweek), checksum, fmt.Errorf("event-live element has invalid player source identity"))
			}
		}
	}
	return payload, checksum, err
}

func (f *FPLSource) ElementSummary(ctx context.Context, playerID int) (ElementSummary, string, error) {
	var payload ElementSummary
	checksum, err := f.get(ctx, fmt.Sprintf("/element-summary/%d/", playerID), &payload)
	if err == nil && payload.History == nil && payload.HistoryPast == nil && payload.Fixtures == nil {
		return ElementSummary{}, checksum, f.sourceValidationError(fmt.Sprintf("/element-summary/%d/", playerID), checksum, fmt.Errorf("element-summary response is missing history and fixtures"))
	}
	if err == nil {
		for _, row := range payload.History {
			if row.Element != playerID || row.Round <= 0 || row.Fixture <= 0 || row.OpponentTeam <= 0 {
				return ElementSummary{}, checksum, f.sourceValidationError(fmt.Sprintf("/element-summary/%d/", playerID), checksum, fmt.Errorf("element-summary history has invalid scope for player %d", playerID))
			}
		}
		for _, fixture := range payload.Fixtures {
			if fixture.ID <= 0 || fixture.TeamH <= 0 || fixture.TeamA <= 0 {
				return ElementSummary{}, checksum, f.sourceValidationError(fmt.Sprintf("/element-summary/%d/", playerID), checksum, fmt.Errorf("element-summary fixture has invalid source identity"))
			}
		}
	}
	return payload, checksum, err
}

func (f *FPLSource) sourceValidationError(endpoint, checksum string, err error) error {
	if f.OnObservation != nil {
		f.OnObservation(SourceObservation{Endpoint: endpoint, FetchedAt: time.Now().UTC(), Checksum: checksum, ValidationState: "invalid", SchemaVersion: "fpl-public-v1", Diagnostic: err.Error()})
	}
	return err
}

func (f *FPLSource) Snapshot(ctx context.Context) (Season, []Gameweek, []Team, []Player, []Fixture, string, error) {
	catalog, checksum, err := f.Bootstrap(ctx)
	if err != nil {
		return Season{}, nil, nil, nil, nil, "", err
	}
	fixtureFeed, fixtureChecksum, err := f.Fixtures(ctx, 0)
	if err != nil {
		return Season{}, nil, nil, nil, nil, checksum, err
	}
	fixtures := fixtureFeed.Fixtures
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
		gameweek := 0
		if fixture.Event != nil {
			gameweek = *fixture.Event
		}
		normalizedFixtures = append(normalizedFixtures, Fixture{ID: fixture.ID, Gameweek: gameweek, KickoffTime: fixture.Kickoff, Finished: fixture.Finished, HomeTeam: fixture.TeamH, AwayTeam: fixture.TeamA, HomeDifficulty: fixture.HDiff, AwayDifficulty: fixture.ADiff, HomeScore: fixture.HScore, AwayScore: fixture.AScore})
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
		history = append(history, PlayerHistory{Gameweek: row.Round, FixtureID: row.Fixture, OpponentTeam: row.OpponentTeam, IsHome: row.IsHome, KickoffTime: row.KickoffTime, Minutes: row.Minutes, TotalPoints: row.Points, Goals: row.Goals, Assists: row.Assists, CleanSheets: row.CleanSheets, Bonus: row.Bonus, Value: float64(row.Value) / 10})
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
