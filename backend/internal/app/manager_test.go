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
}

func TestExpiredMemorySessionRequiresReauthentication(t *testing.T) {
	p := NewMemorySessionProvider()
	_ = p.Put(context.Background(), 1, 2, RemoteSession{Cookie: "x", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, err := p.Get(context.Background(), 1, 2); !errors.Is(err, ErrRemoteReauthRequired) {
		t.Fatalf("error=%v", err)
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
