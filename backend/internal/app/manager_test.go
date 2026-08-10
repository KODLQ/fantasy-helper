package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMemorySessionProviderLifecycleAndIsolation(t *testing.T) {
	provider := NewMemorySessionProvider()
	secret := "sessionid=private-cookie-value"
	if err := provider.Put(context.Background(), 1, 42, RemoteSession{Cookie: secret}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Get(context.Background(), 2, 42); !errors.Is(err, ErrRemoteSessionMissing) {
		t.Fatalf("cross-owner read=%v", err)
	}
	session, err := provider.Get(context.Background(), 1, 42)
	if err != nil || session.Cookie != secret {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if err := provider.Revoke(context.Background(), 1, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Get(context.Background(), 1, 42); !errors.Is(err, ErrRemoteSessionMissing) {
		t.Fatalf("revoked read=%v", err)
	}
}

func TestEnvironmentSessionProviderDoesNotRetainSecret(t *testing.T) {
	const variable = "TEST_FPL_MANAGER_SESSION"
	t.Setenv(variable, "sessionid=environment-secret")
	provider := EnvironmentSessionProvider{Variable: variable}
	session, err := provider.Get(context.Background(), 1, 1)
	if err != nil || session.Cookie == "" {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	if err := os.Unsetenv(variable); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Get(context.Background(), 1, 1); !errors.Is(err, ErrRemoteSessionMissing) {
		t.Fatalf("missing=%v", err)
	}
}

func TestManagerSourceUsesSessionOnlyAtTransportBoundaryAndRedactsErrors(t *testing.T) {
	secret := "sessionid=do-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/my-team/91/" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Cookie") != secret {
			t.Errorf("cookie was not injected")
		}
		http.Error(w, `{"detail":"`+secret+`"}`, http.StatusUnauthorized)
	}))
	defer server.Close()
	source := NewManagerSource(server.URL, server.Client())
	_, _, _, err := source.MyTeam(context.Background(), 91, RemoteSession{Cookie: secret})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error=%v", err)
	}
	var sourceErr ManagerSourceError
	if !errors.As(err, &sourceErr) || sourceErr.Code != "reauth_required" {
		t.Fatalf("classified error=%v", err)
	}
}

func TestManagerSourceUsesBearerTokensAndRejectsUnauthenticatedMeResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") == "Bearer header.payload.signature" {
			_, _ = w.Write([]byte(`{"player":{"entry":91}}`))
			return
		}
		_, _ = w.Write([]byte(`{"player":null}`))
	}))
	defer server.Close()
	source := NewManagerSource(server.URL, server.Client())
	identity, _, _, err := source.Me(context.Background(), RemoteSession{Cookie: "header.payload.signature"})
	if err != nil || sourceMeEntry(identity) != 91 {
		t.Fatalf("identity=%#v err=%v", identity, err)
	}
	if _, _, _, err = source.Me(context.Background(), RemoteSession{Cookie: "sessionid=invalid"}); err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("unauthenticated response error=%v", err)
	}
}

func TestManagerSourcePublicAdaptersAndLeaguePagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/entry/7/":
			json.NewEncoder(w).Encode(map[string]any{"id": 7, "name": "Desk XI", "summary_overall_points": 123})
		case "/entry/7/history/":
			json.NewEncoder(w).Encode(map[string]any{"current": []map[string]any{{"event": 1, "points": 55}}})
		case "/entry/7/transfers/":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/entry/7/event/1/picks/":
			json.NewEncoder(w).Encode(map[string]any{"entry_history": map[string]any{"event": 1}, "picks": []map[string]any{{"element": 10, "position": 1, "multiplier": 1}}})
		case "/leagues-classic/8/standings/":
			if r.URL.Query().Get("page_standings") != "2" || r.URL.Query().Get("phase") != "3" {
				t.Errorf("query=%s", r.URL.RawQuery)
			}
			json.NewEncoder(w).Encode(map[string]any{"league": map[string]any{"id": 8, "name": "Test League"}, "standings": map[string]any{"page": 2, "has_next": true, "results": []map[string]any{{"entry": 7, "rank": 1, "total": 123}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	source := NewManagerSource(server.URL, server.Client())
	entry, checksum, _, err := source.Entry(context.Background(), 7)
	if err != nil || entry.Name != "Desk XI" || len(checksum) != 64 {
		t.Fatalf("entry=%#v checksum=%s err=%v", entry, checksum, err)
	}
	if _, _, _, err = source.History(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = source.Transfers(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = source.Picks(context.Background(), 7, 1); err != nil {
		t.Fatal(err)
	}
	league, _, _, err := source.League(context.Background(), 8, 2, 3)
	if err != nil || league.Standings.Page != 2 || !league.Standings.HasNext {
		t.Fatalf("league=%#v err=%v", league, err)
	}
}

func TestManagerSourceClassifiesSanitizedFailureFixtures(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantSource bool
	}{
		{name: "missing", status: http.StatusNotFound, body: `{"detail":"not found"}`, wantCode: "not_found", wantSource: true},
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"detail":"session expired"}`, wantCode: "reauth_required", wantSource: true},
		{name: "permission", status: http.StatusForbidden, body: `{"detail":"private league"}`, wantCode: "permission_denied", wantSource: true},
		{name: "invalid", status: http.StatusOK, body: `{`, wantCode: "invalid"},
		{name: "partial", status: http.StatusOK, body: `{"id":7}`, wantCode: "incomplete"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, _, _, err := NewManagerSource(server.URL, server.Client()).Entry(context.Background(), 7)
			if err == nil || strings.Contains(err.Error(), test.body) {
				t.Fatalf("unsafe or missing error: %v", err)
			}
			if test.wantSource {
				var sourceErr ManagerSourceError
				if !errors.As(err, &sourceErr) || sourceErr.Code != test.wantCode {
					t.Fatalf("classified error=%v", err)
				}
			} else if !strings.Contains(err.Error(), test.wantCode) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestManagerSourceSupportsClosedMultiPageLeagueFixtures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page_standings")
		if page == "2" {
			_, _ = w.Write([]byte(`{"league":{"id":8,"name":"Closed research","closed":true},"standings":{"page":2,"has_next":false,"results":[{"entry":3,"entry_name":"C","rank":3,"total":90}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"league":{"id":8,"name":"Closed research","closed":true},"standings":{"page":1,"has_next":true,"results":[{"entry":1,"entry_name":"A","rank":1,"total":100},{"entry":2,"entry_name":"B","rank":2,"total":95}]}}`))
	}))
	defer server.Close()
	source := NewManagerSource(server.URL, server.Client())
	first, _, _, err := source.League(context.Background(), 8, 1, 1)
	if err != nil || !first.League.Closed || !first.Standings.HasNext || len(first.Standings.Results) != 2 {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, _, _, err := source.League(context.Background(), 8, 2, 1)
	if err != nil || second.Standings.HasNext || len(second.Standings.Results) != 1 {
		t.Fatalf("second=%#v err=%v", second, err)
	}
}

func TestDeterministicMemberSelectionAndComparisonFormulas(t *testing.T) {
	members := []LeagueMember{{EntryID: 3, Rank: 2}, {EntryID: 2, Rank: 1}, {EntryID: 1, Rank: 1}}
	selected, omitted := SelectLeagueMembers(members, nil, 0, 0, 2)
	if strings.Trim(strings.Join([]string{itoa(selected[0]), itoa(selected[1])}, ","), ",") != "1,2" || len(omitted) != 1 || omitted[0] != 3 {
		t.Fatalf("selected=%v omitted=%v", selected, omitted)
	}
	left, right := CompareTeams([]ManagerPick{{PlayerID: 1, Multiplier: 2}, {PlayerID: 2, Multiplier: 1}}, []ManagerPick{{PlayerID: 1, Multiplier: 1}, {PlayerID: 3, Multiplier: 1}}, map[int]int{1: 5, 2: 4, 3: 8}, 4, "actual")
	if left.Overlap != 1.0/3.0 || left.NetPoints != 10 || right.NetPoints != 9 || left.PointDifference != 1 || right.PointDifference != -1 {
		t.Fatalf("left=%#v right=%#v", left, right)
	}
	left, right = CompareTeamsWithCosts([]ManagerPick{{PlayerID: 1, Multiplier: 2}}, []ManagerPick{{PlayerID: 1, Multiplier: 1}}, map[int]int{1: 5}, 4, 0, "provisional")
	if left.NetPoints != 6 || right.NetPoints != 5 || left.PointDifference != 1 || left.OutcomeState != "provisional" {
		t.Fatalf("cost comparison left=%#v right=%#v", left, right)
	}
}

func TestExpiredMemorySessionRequiresReauthentication(t *testing.T) {
	p := NewMemorySessionProvider()
	_ = p.Put(context.Background(), 1, 2, RemoteSession{Cookie: "x", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, err := p.Get(context.Background(), 1, 2); !errors.Is(err, ErrRemoteReauthRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestActiveTeamConflictDetectionPreservesSourceDisagreement(t *testing.T) {
	var team sourceMyTeam
	team.Picks = append(team.Picks, struct {
		Element       int  `json:"element"`
		Position      int  `json:"position"`
		Multiplier    int  `json:"multiplier"`
		IsCaptain     bool `json:"is_captain"`
		IsViceCaptain bool `json:"is_vice_captain"`
		PurchasePrice int  `json:"purchase_price"`
		SellingPrice  int  `json:"selling_price"`
	}{Element: 1, Position: 1, Multiplier: 2, IsCaptain: true})
	var picks sourcePicks
	picks.Picks = append(picks.Picks, struct {
		Element       int  `json:"element"`
		Position      int  `json:"position"`
		Multiplier    int  `json:"multiplier"`
		IsCaptain     bool `json:"is_captain"`
		IsViceCaptain bool `json:"is_vice_captain"`
		PurchasePrice int  `json:"purchase_price"`
		SellingPrice  int  `json:"selling_price"`
	}{Element: 1, Position: 1, Multiplier: 1})
	if sourceTeamMatchesPicks(team, picks) {
		t.Fatal("captaincy disagreement was treated as matching")
	}
	picks.Picks[0].Multiplier = 2
	picks.Picks[0].IsCaptain = true
	if !sourceTeamMatchesPicks(team, picks) {
		t.Fatal("identical source selections were treated as conflicted")
	}
}

func TestActiveTeamImportPreviewIsNonMutatingAndExplainsChanges(t *testing.T) {
	domain := NewStore()
	current := demoSquad()
	current = domain.EnrichSquad(current)
	domain.SaveSquad(current)
	snapshot := ActiveTeamSnapshot{SnapshotID: 44, EntryID: 9, SeasonID: domain.ExportSnapshot().Season.ID, Gameweek: 1, TeamValue: 1000, PurchasePrices: map[int]float64{}}
	starts := map[int]bool{}
	for _, id := range current.StartingPlayerIDs {
		starts[id] = true
	}
	for id, price := range current.PurchasePrices {
		snapshot.PurchasePrices[id] = price
		snapshot.Picks = append(snapshot.Picks, ManagerPick{PlayerID: id, Position: 1, Multiplier: map[bool]int{true: 1, false: 0}[starts[id]], Captain: id == current.CaptainID, ViceCaptain: id == current.ViceCaptainID})
	}
	preview, err := BuildImportPreview(snapshot, current, domain)
	if err != nil {
		t.Fatal(err)
	}
	if preview.HasChanges || len(preview.Validation) > 0 {
		t.Fatalf("unchanged preview=%#v", preview)
	}
	if domain.GetSquad().CaptainID != current.CaptainID {
		t.Fatal("preview mutated planning squad")
	}
	for index := range snapshot.Picks {
		snapshot.Picks[index].Captain = false
	}
	for index := range snapshot.Picks {
		if snapshot.Picks[index].PlayerID != current.CaptainID && snapshot.Picks[index].Multiplier > 0 {
			snapshot.Picks[index].Captain = true
			break
		}
	}
	preview, err = BuildImportPreview(snapshot, current, domain)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.HasChanges || !preview.CaptainChanged {
		t.Fatalf("changed preview=%#v", preview)
	}
}

func itoa(value int) string { b, _ := json.Marshal(value); return string(b) }
