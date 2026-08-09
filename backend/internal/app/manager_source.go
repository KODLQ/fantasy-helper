package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ManagerSource struct {
	BaseURL string
	Client  *http.Client
}

func NewManagerSource(baseURL string, client *http.Client) *ManagerSource {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &ManagerSource{BaseURL: strings.TrimRight(baseURL, "/"), Client: client}
}

type ManagerSourceError struct {
	Status int
	Code   string
}

func (e ManagerSourceError) Error() string {
	return fmt.Sprintf("FPL manager source request failed (%s)", e.Code)
}

type sourceEntry struct {
	ID                   int    `json:"id"`
	PlayerFirstName      string `json:"player_first_name"`
	PlayerLastName       string `json:"player_last_name"`
	Name                 string `json:"name"`
	StartedEvent         int    `json:"started_event"`
	SummaryOverallPoints int    `json:"summary_overall_points"`
	SummaryOverallRank   int    `json:"summary_overall_rank"`
	LastDeadlineValue    int    `json:"last_deadline_value"`
	LastDeadlineBank     int    `json:"last_deadline_bank"`
}

type sourceEntryHistory struct {
	Current []struct {
		Event              int `json:"event"`
		Points             int `json:"points"`
		Rank               int `json:"rank"`
		OverallRank        int `json:"overall_rank"`
		Bank               int `json:"bank"`
		Value              int `json:"value"`
		EventTransfers     int `json:"event_transfers"`
		EventTransfersCost int `json:"event_transfers_cost"`
		PointsOnBench      int `json:"points_on_bench"`
	} `json:"current"`
	Chips []struct {
		Name  string     `json:"name"`
		Event int        `json:"event"`
		Time  *time.Time `json:"time"`
	} `json:"chips"`
}

type sourceTransfer struct {
	Event          int       `json:"event"`
	ElementIn      int       `json:"element_in"`
	ElementOut     int       `json:"element_out"`
	ElementInCost  int       `json:"element_in_cost"`
	ElementOutCost int       `json:"element_out_cost"`
	Entry          int       `json:"entry"`
	Time           time.Time `json:"time"`
}

type sourcePicks struct {
	ActiveChip   string `json:"active_chip"`
	EntryHistory struct {
		Event              int `json:"event"`
		Points             int `json:"points"`
		Rank               int `json:"rank"`
		OverallRank        int `json:"overall_rank"`
		Bank               int `json:"bank"`
		Value              int `json:"value"`
		EventTransfers     int `json:"event_transfers"`
		EventTransfersCost int `json:"event_transfers_cost"`
		PointsOnBench      int `json:"points_on_bench"`
	} `json:"entry_history"`
	Picks []struct {
		Element       int  `json:"element"`
		Position      int  `json:"position"`
		Multiplier    int  `json:"multiplier"`
		IsCaptain     bool `json:"is_captain"`
		IsViceCaptain bool `json:"is_vice_captain"`
		PurchasePrice int  `json:"purchase_price"`
		SellingPrice  int  `json:"selling_price"`
	} `json:"picks"`
	AutomaticSubs []struct {
		ElementIn  int `json:"element_in"`
		ElementOut int `json:"element_out"`
		Event      int `json:"event"`
	} `json:"automatic_subs"`
}

type sourceMyTeam struct {
	Picks []struct {
		Element       int  `json:"element"`
		Position      int  `json:"position"`
		Multiplier    int  `json:"multiplier"`
		IsCaptain     bool `json:"is_captain"`
		IsViceCaptain bool `json:"is_vice_captain"`
		PurchasePrice int  `json:"purchase_price"`
		SellingPrice  int  `json:"selling_price"`
	} `json:"picks"`
	Transfers struct {
		Bank  int  `json:"bank"`
		Value int  `json:"value"`
		Limit *int `json:"limit"`
		Made  int  `json:"made"`
		Cost  int  `json:"cost"`
	} `json:"transfers"`
	Chips []struct {
		Name   string `json:"name"`
		Status string `json:"status_for_entry"`
	} `json:"chips"`
}

type sourceLeagueStandings struct {
	League struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Closed bool   `json:"closed"`
	} `json:"league"`
	Standings struct {
		HasNext bool `json:"has_next"`
		Page    int  `json:"page"`
		Results []struct {
			Entry      int    `json:"entry"`
			EntryName  string `json:"entry_name"`
			PlayerName string `json:"player_name"`
			Rank       int    `json:"rank"`
			LastRank   int    `json:"last_rank"`
			Total      int    `json:"total"`
		} `json:"results"`
	} `json:"standings"`
}

func (s *ManagerSource) get(ctx context.Context, path, cookie string, target any) (string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+path, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	response, err := s.Client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("FPL manager source transport failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("FPL manager source response could not be read")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := "source_unavailable"
		if response.StatusCode == http.StatusUnauthorized {
			code = "reauth_required"
		}
		if response.StatusCode == http.StatusForbidden {
			code = "permission_denied"
		}
		if response.StatusCode == http.StatusNotFound {
			code = "not_found"
		}
		return "", time.Time{}, ManagerSourceError{Status: response.StatusCode, Code: code}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return "", time.Time{}, fmt.Errorf("FPL manager source response is invalid")
	}
	return fmt.Sprintf("%x", sha256.Sum256(body)), time.Now().UTC(), nil
}

func (s *ManagerSource) Entry(ctx context.Context, entryID int) (sourceEntry, string, time.Time, error) {
	var v sourceEntry
	c, t, e := s.get(ctx, fmt.Sprintf("/entry/%d/", entryID), "", &v)
	return v, c, t, e
}
func (s *ManagerSource) History(ctx context.Context, entryID int) (sourceEntryHistory, string, time.Time, error) {
	var v sourceEntryHistory
	c, t, e := s.get(ctx, fmt.Sprintf("/entry/%d/history/", entryID), "", &v)
	return v, c, t, e
}
func (s *ManagerSource) Transfers(ctx context.Context, entryID int) ([]sourceTransfer, string, time.Time, error) {
	var v []sourceTransfer
	c, t, e := s.get(ctx, fmt.Sprintf("/entry/%d/transfers/", entryID), "", &v)
	return v, c, t, e
}
func (s *ManagerSource) Picks(ctx context.Context, entryID, gameweek int) (sourcePicks, string, time.Time, error) {
	var v sourcePicks
	c, t, e := s.get(ctx, fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek), "", &v)
	return v, c, t, e
}
func (s *ManagerSource) MyTeam(ctx context.Context, entryID int, session RemoteSession) (sourceMyTeam, string, time.Time, error) {
	var v sourceMyTeam
	c, t, e := s.get(ctx, fmt.Sprintf("/my-team/%d/", entryID), session.Cookie, &v)
	return v, c, t, e
}
func (s *ManagerSource) Me(ctx context.Context, session RemoteSession) (map[string]any, string, time.Time, error) {
	var v map[string]any
	c, t, e := s.get(ctx, "/me/", session.Cookie, &v)
	return v, c, t, e
}
func (s *ManagerSource) League(ctx context.Context, leagueID, page, phase int) (sourceLeagueStandings, string, time.Time, error) {
	if page < 1 {
		page = 1
	}
	if phase < 1 {
		phase = 1
	}
	query := url.Values{"page_standings": {strconv.Itoa(page)}, "phase": {strconv.Itoa(phase)}}
	var v sourceLeagueStandings
	c, t, e := s.get(ctx, fmt.Sprintf("/leagues-classic/%d/standings/?%s", leagueID, query.Encode()), "", &v)
	return v, c, t, e
}
