package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerLeagueSyncImportAnalysisAndPrivacyIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL manager integration tests")
	}
	if err := assertDisposableTestDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := OpenDatabase(ctx, Config{DatabaseURL: dsn, DatabaseMaxConns: 8, DatabaseMaxIdle: 4, DatabasePing: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewPostgresRepository(database, nil)
	if err = repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `TRUNCATE users,seasons RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	snapshot := store.ExportSnapshot()
	if err = repository.UpsertSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	passwordHash, err := DefaultPasswordHasher().Hash("Correct-manager-password-42")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := repository.CreateUser(ctx, User{Email: "manager-owner@example.test", DisplayName: "Owner", PasswordHash: passwordHash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := repository.CreateUser(ctx, User{Email: "manager-other@example.test", DisplayName: "Other", PasswordHash: passwordHash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}

	squad := store.EnrichSquad(demoSquad())
	starts := map[int]bool{}
	for _, id := range squad.StartingPlayerIDs {
		starts[id] = true
	}
	picks := make([]map[string]any, 0, len(squad.PurchasePrices))
	position := 1
	for id, price := range squad.PurchasePrices {
		multiplier := 0
		if starts[id] {
			multiplier = 1
		}
		if id == squad.CaptainID {
			multiplier = 2
		}
		picks = append(picks, map[string]any{"element": id, "position": position, "multiplier": multiplier, "is_captain": id == squad.CaptainID, "is_vice_captain": id == squad.ViceCaptainID, "purchase_price": int(price * 10), "selling_price": int(price * 10)})
		position++
	}
	var failPageTwo atomic.Bool
	var failMember102 atomic.Bool
	var denyPrivate atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/me/":
			if r.Header.Get("Cookie") != "sessionid=integration-secret" {
				http.Error(w, `{}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"player": map[string]any{"entry": 101}})
		case path == "/my-team/101/":
			if denyPrivate.Load() {
				http.Error(w, `{}`, http.StatusForbidden)
				return
			}
			if r.Header.Get("Cookie") != "sessionid=integration-secret" {
				http.Error(w, `{}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"picks": picks, "transfers": map[string]any{"bank": 10, "value": 1000, "made": 2, "cost": 4}, "chips": []map[string]any{{"name": "bboost", "status_for_entry": "played"}}})
		case strings.HasPrefix(path, "/entry/") && strings.HasSuffix(path, "/history/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"current": []map[string]any{{"event": 1, "points": 70, "rank": 1, "overall_rank": 100, "bank": 10, "value": 1000, "event_transfers": 2, "event_transfers_cost": 4, "points_on_bench": 7}}, "chips": []map[string]any{{"name": "bboost", "event": 1}}})
		case strings.HasPrefix(path, "/entry/") && strings.HasSuffix(path, "/transfers/"):
			_, _ = w.Write([]byte(`[]`))
		case strings.HasPrefix(path, "/entry/") && strings.Contains(path, "/event/1/picks/"):
			if path == "/entry/102/event/1/picks/" && failMember102.Load() {
				http.Error(w, `{}`, http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active_chip": "bboost", "entry_history": map[string]any{"event": 1, "points": 70, "event_transfers_cost": 4, "points_on_bench": 7}, "picks": picks, "automatic_subs": []map[string]any{{"element_in": squad.BenchPlayerIDs[0], "element_out": squad.StartingPlayerIDs[0], "event": 1}}})
		case strings.HasPrefix(path, "/entry/"):
			var entryID int
			_, _ = fmt.Sscanf(path, "/entry/%d/", &entryID)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": entryID, "name": fmt.Sprintf("Entry %d", entryID), "player_first_name": "Test", "summary_overall_points": 100, "last_deadline_value": 1000, "last_deadline_bank": 10})
		case path == "/leagues-classic/202/standings/":
			page := r.URL.Query().Get("page_standings")
			if page == "2" && failPageTwo.Load() {
				http.Error(w, `{}`, http.StatusServiceUnavailable)
				return
			}
			if page == "2" {
				_ = json.NewEncoder(w).Encode(map[string]any{"league": map[string]any{"id": 202, "name": "Integration League", "closed": true}, "standings": map[string]any{"page": 2, "has_next": false, "results": []map[string]any{{"entry": 103, "entry_name": "Third", "player_name": "Three", "rank": 3, "last_rank": 3, "total": 90}}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"league": map[string]any{"id": 202, "name": "Integration League", "closed": true}, "standings": map[string]any{"page": 1, "has_next": true, "results": []map[string]any{{"entry": 101, "entry_name": "First", "player_name": "One", "rank": 1, "last_rank": 2, "total": 100}, {"entry": 102, "entry_name": "Second", "player_name": "Two", "rank": 2, "last_rank": 1, "total": 95}}}})
		case path == "/leagues-classic/203/standings/":
			_ = json.NewEncoder(w).Encode(map[string]any{"league": map[string]any{"id": 203, "name": "Second League", "closed": false}, "standings": map[string]any{"page": 1, "has_next": false, "results": []map[string]any{{"entry": 104, "entry_name": "Fourth", "player_name": "Four", "rank": 1, "last_rank": 1, "total": 88}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sessions := NewMemorySessionProvider()
	service := NewManagerService(repository, NewManagerSource(server.URL, server.Client()), sessions)
	if err = service.Connect(ctx, owner.ID, 101, "sessionid=integration-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "entry", SourceID: 101, Enabled: true, MemberLimit: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "league", SourceID: 202, Enabled: true, MemberLimit: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "entry", SourceID: 104, Enabled: true, MemberLimit: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "league", SourceID: 203, Enabled: true, MemberLimit: 1}); err != nil {
		t.Fatal(err)
	}
	status, err := service.Sync(ctx, owner.ID, snapshot.Season.ID, 1, "manager-integration-success")
	if err != nil || status.Status != "success" || status.CompletedWork != 4 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	var incompleteWork, linkedPicks, automaticSubs, privatePayloads int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM manager_sync_work_items WHERE run_id=$1 AND state<>'completed'`, status.RunID).Scan(&incompleteWork); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM league_member_pick_links WHERE state='completed' AND pick_snapshot_id IS NOT NULL`).Scan(&linkedPicks); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM manager_automatic_substitutions`).Scan(&automaticSubs); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM source_payloads WHERE endpoint LIKE '/me/%' OR endpoint LIKE '/my-team/%'`).Scan(&privatePayloads); err != nil {
		t.Fatal(err)
	}
	if incompleteWork != 0 || linkedPicks != 3 || automaticSubs == 0 || privatePayloads != 0 {
		t.Fatalf("work=%d links=%d substitutions=%d privatePayloads=%d", incompleteWork, linkedPicks, automaticSubs, privatePayloads)
	}
	members, err := repository.LoadLeagueMembers(ctx, owner.ID, snapshot.Season.ID, 202, 1)
	if err != nil || len(members) != 3 || members[2].EntryID != 103 {
		t.Fatalf("members=%#v err=%v", members, err)
	}
	if otherMembers, loadErr := repository.LoadLeagueMembers(ctx, other.ID, snapshot.Season.ID, 202, 1); loadErr != nil || len(otherMembers) != 0 {
		t.Fatalf("cross-owner members=%#v err=%v", otherMembers, loadErr)
	}
	active, found, err := repository.LoadActiveTeam(ctx, owner.ID, snapshot.Season.ID, 101, 1)
	if err != nil || !found || len(active.Picks) != 15 || active.ActiveChip != "bboost" {
		t.Fatalf("active=%#v found=%v err=%v", active, found, err)
	}
	if _, found, err = repository.LoadActiveTeam(ctx, other.ID, snapshot.Season.ID, 101, 1); err != nil || found {
		t.Fatalf("cross-owner active snapshot found=%v err=%v", found, err)
	}
	denyPrivate.Store(true)
	_, _ = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "league", SourceID: 202, Enabled: false, MemberLimit: 2})
	_, _ = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "entry", SourceID: 104, Enabled: false, MemberLimit: 2})
	_, _ = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "league", SourceID: 203, Enabled: false, MemberLimit: 1})
	permissionStatus, permissionErr := service.Sync(ctx, owner.ID, snapshot.Season.ID, 1, "manager-private-permission")
	if permissionErr == nil || permissionStatus.Status != "partial" || !strings.Contains(permissionStatus.Warning, "entry:101") {
		t.Fatalf("permission status=%#v err=%v", permissionStatus, permissionErr)
	}
	var connectionState, providerReference string
	if err = database.QueryRowContext(ctx, `SELECT state,COALESCE(provider_reference,'') FROM manager_connections WHERE user_id=$1 AND entry_source_id=101`, owner.ID).Scan(&connectionState, &providerReference); err != nil || connectionState != string(RemotePermissionDenied) || providerReference != "" {
		t.Fatalf("connection state=%q providerReference=%q err=%v", connectionState, providerReference, err)
	}
	if retained, retainedFound, retainedErr := repository.LoadActiveTeam(ctx, owner.ID, snapshot.Season.ID, 101, 1); retainedErr != nil || !retainedFound || retained.SnapshotID != active.SnapshotID {
		t.Fatalf("prior active team was not retained: %#v found=%v err=%v", retained, retainedFound, retainedErr)
	}
	denyPrivate.Store(false)
	if err = service.Connect(ctx, owner.ID, 101, "sessionid=integration-secret"); err != nil {
		t.Fatal(err)
	}
	_, _ = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "league", SourceID: 202, Enabled: true, MemberLimit: 2})

	publicSnapshotID := newSnapshotID()
	if err = repository.CreateDatasetSnapshot(ctx, DatasetSnapshot{ID: publicSnapshotID, Dataset: "player-gameweek", State: "actual", SeasonID: snapshot.Season.ID, Gameweek: 1, SourceFetchedAt: time.Now().UTC(), NormalizedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1"}); err != nil {
		t.Fatal(err)
	}
	live := make([]LivePlayerStats, 0, len(snapshot.Players))
	for _, player := range snapshot.Players {
		live = append(live, LivePlayerStats{PlayerID: player.ID, Minutes: 90, Points: player.ID % 10})
	}
	if err = repository.UpsertLiveGameweek(ctx, publicSnapshotID, snapshot.Season.ID, 1, true, time.Now().UTC(), live); err != nil {
		t.Fatal(err)
	}
	analysis, err := repository.LoadManagerDecisionAnalysis(ctx, owner.ID, snapshot.Season.ID, 101, 1)
	if err != nil || analysis.OutcomeState != "actual" || analysis.TransferCost != 4 || analysis.BenchPoints != 7 || analysis.ActiveChip != "bboost" || analysis.NetPoints != analysis.GrossPoints-4 {
		t.Fatalf("analysis=%#v err=%v", analysis, err)
	}
	comparison, err := service.CompareLeague(ctx, owner.ID, snapshot.Season.ID, 202, 1, []int{101, 102}, 0, 0, 2)
	if err != nil || comparison.OutcomeState != "actual" || len(comparison.Comparisons) != 2 || comparison.Comparisons[0].OutcomeState != "actual" {
		t.Fatalf("comparison=%#v err=%v", comparison, err)
	}

	preview, found, err := service.PreviewImport(ctx, owner.ID, snapshot.Season.ID, 101, 1, store)
	if err != nil || !found || len(preview.Validation) != 0 {
		t.Fatalf("preview=%#v found=%v err=%v", preview, found, err)
	}
	draft, err := service.Import(ctx, owner.ID, snapshot.Season.ID, 101, 1, active.SnapshotID, "draft", false, store)
	if err != nil || draft.DraftID == 0 || draft.PlanID != 0 {
		t.Fatalf("draft=%#v err=%v", draft, err)
	}
	repeatedDraft, err := service.Import(ctx, owner.ID, snapshot.Season.ID, 101, 1, active.SnapshotID, "draft", false, store)
	if err != nil || !repeatedDraft.Idempotent || repeatedDraft.DraftID != draft.DraftID {
		t.Fatalf("repeated draft=%#v err=%v", repeatedDraft, err)
	}
	if _, err = service.Import(ctx, owner.ID, snapshot.Season.ID, 101, 1, active.SnapshotID, "replace", false, store); err == nil {
		t.Fatal("replace succeeded without explicit confirmation")
	}
	invalidSquad := preview.Proposed
	invalidSquad.PurchasePrices = map[int]float64{999999: 1}
	if _, err = repository.ImportActiveTeam(ctx, owner.ID, snapshot.Season.ID, active.SnapshotID, "replace", invalidSquad); err == nil {
		t.Fatal("invalid replacement unexpectedly succeeded")
	}
	var partialPlans, partialImports int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM squad_plans WHERE user_id=$1`, owner.ID).Scan(&partialPlans); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM squad_import_events WHERE user_id=$1 AND mode='replace'`, owner.ID).Scan(&partialImports); err != nil {
		t.Fatal(err)
	}
	if partialPlans != 0 || partialImports != 0 {
		t.Fatalf("failed import was not atomic: plans=%d imports=%d", partialPlans, partialImports)
	}
	replaced, err := service.Import(ctx, owner.ID, snapshot.Season.ID, 101, 1, active.SnapshotID, "replace", true, store)
	if err != nil || replaced.PlanID == 0 || replaced.DraftID != 0 {
		t.Fatalf("replace=%#v err=%v", replaced, err)
	}

	exported, err := repository.ExportManagerData(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	exportJSON, _ := json.Marshal(exported)
	if strings.Contains(string(exportJSON), "integration-secret") {
		t.Fatalf("secret leaked in export: %s", exportJSON)
	}
	authService, err := NewAuthService(repository, AuthRuntimeConfig{RegistrationEnabled: false, AllowedOrigin: "http://localhost:5173", IdleTimeout: time.Hour, AbsoluteTimeout: 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	ownerSession, err := authService.Login(ctx, owner.Email, "Correct-manager-password-42", "127.0.0.1", "manager-api-owner")
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := authService.Login(ctx, other.Email, "Correct-manager-password-42", "127.0.0.1", "manager-api-other")
	if err != nil {
		t.Fatal(err)
	}
	api := NewAPI(store, nil, nil, nil, repository)
	api.EnableAuth(authService)
	api.EnableManager(service)
	handler := api.Handler()
	analysisResponse := authRequest(handler, http.MethodGet, fmt.Sprintf("/api/v1/manager/entries/101/analysis?seasonId=%d&gameweek=1", snapshot.Season.ID), "", ownerSession.Token, "")
	if analysisResponse.Code != http.StatusOK || !strings.Contains(analysisResponse.Body.String(), `"outcomeState":"actual"`) || !strings.Contains(analysisResponse.Body.String(), `"snapshotId":`) || !strings.Contains(analysisResponse.Body.String(), `"formulaVersions"`) {
		t.Fatalf("analysis contract=%d %s", analysisResponse.Code, analysisResponse.Body.String())
	}
	missingScopeResponse := authRequest(handler, http.MethodGet, fmt.Sprintf("/api/v1/manager/entries/101/analysis?seasonId=%d", snapshot.Season.ID), "", ownerSession.Token, "")
	if missingScopeResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(missingScopeResponse.Body.String(), `"code":"manager_scope_required"`) {
		t.Fatalf("missing scope contract=%d %s", missingScopeResponse.Code, missingScopeResponse.Body.String())
	}
	activeResponse := authRequest(handler, http.MethodGet, fmt.Sprintf("/api/v1/manager/entries/101/active-team?seasonId=%d&gameweek=1", snapshot.Season.ID), "", ownerSession.Token, "")
	if activeResponse.Code != http.StatusOK || !strings.Contains(activeResponse.Body.String(), `"conflictState":"none"`) || !strings.Contains(activeResponse.Body.String(), `"sourceFetchedAt"`) {
		t.Fatalf("active-team contract=%d %s", activeResponse.Code, activeResponse.Body.String())
	}
	isolatedResponse := authRequest(handler, http.MethodGet, fmt.Sprintf("/api/v1/manager/entries/101/active-team?seasonId=%d&gameweek=1", snapshot.Season.ID), "", otherSession.Token, "")
	if isolatedResponse.Code != http.StatusNotFound || strings.Contains(isolatedResponse.Body.String(), "Integration League") {
		t.Fatalf("cross-owner contract=%d %s", isolatedResponse.Code, isolatedResponse.Body.String())
	}
	exportResponse := authRequest(handler, http.MethodGet, "/api/v1/manager/export", "", ownerSession.Token, "")
	if exportResponse.Code != http.StatusOK || strings.Contains(exportResponse.Body.String(), "integration-secret") || !strings.Contains(exportResponse.Body.String(), `"activeTeams"`) {
		t.Fatalf("export contract=%d %s", exportResponse.Code, exportResponse.Body.String())
	}
	if err = service.Disconnect(ctx, owner.ID, 101); err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.Get(ctx, owner.ID, 101); !errors.Is(err, ErrRemoteSessionMissing) {
		t.Fatalf("session was not revoked: %v", err)
	}

	failPageTwo.Store(true)
	_, _ = repository.UpsertManagerScope(ctx, owner.ID, ManagerScope{Type: "entry", SourceID: 101, Enabled: false, MemberLimit: 2})
	failedPageStatus, failedPageErr := service.Sync(ctx, owner.ID, snapshot.Season.ID, 1, "manager-page-resume")
	if failedPageErr == nil || failedPageStatus.Status != "partial" {
		t.Fatalf("page failure status=%#v err=%v", failedPageStatus, failedPageErr)
	}
	var retryable int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM manager_sync_work_items WHERE run_id=$1 AND state='retryable'`, failedPageStatus.RunID).Scan(&retryable); err != nil || retryable != 1 {
		t.Fatalf("retryable=%d err=%v", retryable, err)
	}
	if _, found, err = repository.LoadLeagueStandings(ctx, owner.ID, snapshot.Season.ID, 202, 1, 1); err != nil || !found {
		t.Fatalf("retained page missing found=%v err=%v", found, err)
	}

	failPageTwo.Store(false)
	failMember102.Store(true)
	memberFailureStatus, memberFailureErr := service.Sync(ctx, owner.ID, snapshot.Season.ID, 1, "manager-member-partial")
	if memberFailureErr == nil || memberFailureStatus.Status != "partial" {
		t.Fatalf("member failure status=%#v err=%v", memberFailureStatus, memberFailureErr)
	}
	var failedLinks int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM league_member_pick_links WHERE state='failed' AND last_error IS NOT NULL`).Scan(&failedLinks); err != nil || failedLinks != 1 {
		t.Fatalf("failed links=%d err=%v", failedLinks, err)
	}

	var publicPlayersBefore, publicPlayersAfter int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&publicPlayersBefore); err != nil {
		t.Fatal(err)
	}
	deleteResponse := authRequest(handler, http.MethodDelete, "/api/v1/manager/data", "", ownerSession.Token, ownerSession.CSRFToken)
	if deleteResponse.Code != http.StatusOK || !strings.Contains(deleteResponse.Body.String(), `"deleted":true`) {
		t.Fatalf("delete contract=%d %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&publicPlayersAfter); err != nil {
		t.Fatal(err)
	}
	if publicPlayersAfter != publicPlayersBefore {
		t.Fatalf("public facts changed during private deletion: before=%d after=%d", publicPlayersBefore, publicPlayersAfter)
	}
	if _, found, err = repository.LoadActiveTeam(ctx, owner.ID, snapshot.Season.ID, 101, 1); err != nil || found {
		t.Fatalf("private active team survived deletion found=%v err=%v", found, err)
	}
}
