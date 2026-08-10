package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func (a *API) EnableManager(service *ManagerService) { a.Manager = service }

func (a *API) requireManager(w http.ResponseWriter, r *http.Request) (*ManagerService, AuthSessionResult, bool) {
	session, ok := a.requireAuth(w, r)
	if !ok {
		return nil, session, false
	}
	if a.Manager == nil {
		writeContractError(w, http.StatusServiceUnavailable, requestIDFrom(w), "manager_unavailable", "Manager synchronization is unavailable.", true, nil)
		return nil, session, false
	}
	return a.Manager, session, true
}

func (a *API) managerEntryData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET for manager data.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/manager/entries/"), "/")
	parts := strings.Split(tail, "/")
	entryID := 0
	if len(parts) > 0 {
		entryID, _ = strconv.Atoi(parts[0])
	}
	seasonID := parseInt(r.URL.Query().Get("seasonId"), 0)
	if entryID <= 0 || seasonID <= 0 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "entry and seasonId are required.", false, nil)
		return
	}
	kind := "summary"
	if len(parts) > 1 {
		kind = parts[1]
	}
	scope := Scope{SeasonID: seasonID, Dataset: "manager-fpl"}
	fresh := service.Status(session.User.ID).Freshness
	switch kind {
	case "summary":
		item, found, err := service.Repository.LoadManagerSummary(r.Context(), session.User.ID, seasonID, entryID)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager summary is unavailable.", true, nil)
			return
		}
		if !found {
			writeContractError(w, 404, requestIDFrom(w), "manager_not_found", "Manager data was not found.", false, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, item)
	case "history":
		items, err := service.Repository.LoadManagerHistory(r.Context(), session.User.ID, seasonID, entryID)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager history is unavailable.", true, nil)
			return
		}
		from, to := 0, 0
		if len(items) > 0 {
			from, to = items[0].Gameweek, items[len(items)-1].Gameweek
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, map[string]any{"items": items, "range": map[string]any{"gameweekFrom": from, "gameweekTo": to, "count": len(items)}})
	case "transfers":
		items, err := service.Repository.LoadManagerTransfers(r.Context(), session.User.ID, seasonID, entryID)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager transfers are unavailable.", true, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, map[string]any{"items": items})
	case "picks":
		gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
		if gameweek <= 0 {
			writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "gameweek is required for picks.", false, nil)
			return
		}
		scope.Gameweek = gameweek
		items, err := service.Repository.LoadManagerPicks(r.Context(), session.User.ID, seasonID, entryID, gameweek)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager picks are unavailable.", true, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, map[string]any{"items": items})
	case "analysis":
		gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
		if gameweek <= 0 {
			writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "gameweek is required for analysis.", false, nil)
			return
		}
		scope.Gameweek = gameweek
		item, err := service.Repository.LoadManagerDecisionAnalysis(r.Context(), session.User.ID, seasonID, entryID, gameweek)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager analysis is unavailable.", true, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, item)
	case "active-team":
		gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
		if gameweek <= 0 {
			writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "gameweek is required for active-team snapshots.", false, nil)
			return
		}
		scope.Gameweek = gameweek
		item, found, err := service.Repository.LoadActiveTeam(r.Context(), session.User.ID, seasonID, entryID, gameweek)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Active-team snapshot is unavailable.", true, nil)
			return
		}
		if !found {
			writeContractError(w, 404, requestIDFrom(w), "active_team_not_found", "No synchronized active team was found.", false, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, item)
	default:
		writeContractError(w, 404, requestIDFrom(w), "manager_route_not_found", "Manager dataset was not found.", false, nil)
	}
}

func (a *API) managerLeagueData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET for league data.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/manager/leagues/"), "/")
	parts := strings.Split(tail, "/")
	if len(parts) < 2 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "league and dataset are required.", false, nil)
		return
	}
	leagueID, _ := strconv.Atoi(parts[0])
	seasonID := parseInt(r.URL.Query().Get("seasonId"), 0)
	gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
	if leagueID <= 0 || seasonID <= 0 || gameweek <= 0 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "league, seasonId, and gameweek are required.", false, nil)
		return
	}
	scope := Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "manager-fpl"}
	fresh := service.Status(session.User.ID).Freshness
	if parts[1] == "standings" {
		page := parseInt(r.URL.Query().Get("page"), 1)
		item, found, err := service.Repository.LoadLeagueStandings(r.Context(), session.User.ID, seasonID, leagueID, gameweek, page)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "league_unavailable", "League standings are unavailable.", true, nil)
			return
		}
		if !found {
			writeContractError(w, 404, requestIDFrom(w), "league_not_found", "League standings were not found.", false, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), scope, fresh, item)
		return
	}
	if parts[1] == "comparison" {
		ids := parseCSVInts(r.URL.Query().Get("entryIds"))
		result, err := service.CompareLeague(r.Context(), session.User.ID, seasonID, leagueID, gameweek, ids, parseInt(r.URL.Query().Get("rankFrom"), 0), parseInt(r.URL.Query().Get("rankTo"), 0), parseInt(r.URL.Query().Get("limit"), 50))
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "comparison_unavailable", "League comparison is unavailable.", true, nil)
			return
		}
		writeEnvelopeWithWarnings(w, 200, requestIDFrom(w), scope, fresh, result.MissingInputs, result)
		return
	}
	writeContractError(w, 404, requestIDFrom(w), "league_route_not_found", "League dataset was not found.", false, nil)
}

func (a *API) squadImportPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET for an import preview.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	seasonID := parseInt(r.URL.Query().Get("seasonId"), 0)
	entryID := parseInt(r.URL.Query().Get("entryId"), 0)
	gameweek := parseInt(r.URL.Query().Get("gameweek"), 0)
	if seasonID <= 0 || entryID <= 0 || gameweek <= 0 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "entryId, seasonId, and gameweek are required.", false, nil)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), seasonID, session.User.ID)
	if err != nil {
		writeContractError(w, 503, requestIDFrom(w), "planning_unavailable", "Planning data is unavailable.", true, nil)
		return
	}
	preview, found, err := service.PreviewImport(r.Context(), session.User.ID, seasonID, entryID, gameweek, domain)
	if err != nil {
		writeContractError(w, 503, requestIDFrom(w), "import_preview_unavailable", "Import preview is unavailable.", true, nil)
		return
	}
	if !found {
		writeContractError(w, 404, requestIDFrom(w), "active_team_not_found", "No synchronized active team was found.", false, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{SeasonID: seasonID, Gameweek: gameweek, Dataset: "manager-fpl"}, service.Status(session.User.ID).Freshness, preview)
}

func (a *API) squadImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use POST to import an active team.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	if _, ok = a.requireMutationAuth(w, r); !ok {
		return
	}
	var input struct {
		SeasonID       int    `json:"seasonId"`
		EntryID        int    `json:"entryId"`
		Gameweek       int    `json:"gameweek"`
		SnapshotID     int64  `json:"snapshotId"`
		Mode           string `json:"mode"`
		ConfirmReplace bool   `json:"confirmReplace"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.SeasonID <= 0 || input.EntryID <= 0 || input.Gameweek <= 0 || input.SnapshotID <= 0 || (input.Mode != "draft" && input.Mode != "replace") {
		writeContractError(w, 422, requestIDFrom(w), "invalid_import", "Season, entry, gameweek, snapshot, and mode are required.", false, nil)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), input.SeasonID, session.User.ID)
	if err != nil {
		writeContractError(w, 503, requestIDFrom(w), "planning_unavailable", "Planning data is unavailable.", true, nil)
		return
	}
	result, err := service.Import(r.Context(), session.User.ID, input.SeasonID, input.EntryID, input.Gameweek, input.SnapshotID, input.Mode, input.ConfirmReplace, domain)
	if err != nil {
		writeContractError(w, 422, requestIDFrom(w), "import_validation_failed", err.Error(), false, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{SeasonID: input.SeasonID, Gameweek: input.Gameweek, Dataset: "manager-fpl"}, service.Status(session.User.ID).Freshness, result)
}

func parseCSVInts(value string) []int {
	items := []int{}
	for _, part := range strings.Split(value, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && id > 0 {
			items = append(items, id)
		}
	}
	return items
}

func (a *API) managerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET for manager status.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	status := service.Status(session.User.ID)
	writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, status.Freshness, status)
}

func (a *API) managerScopes(w http.ResponseWriter, r *http.Request) {
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := service.Repository.ListManagerScopes(r.Context(), session.User.ID)
		if err != nil {
			writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Manager scopes are temporarily unavailable.", true, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, service.Status(session.User.ID).Freshness, map[string]any{"items": items})
	case http.MethodPut:
		if _, ok := a.requireMutationAuth(w, r); !ok {
			return
		}
		var input ManagerScope
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeContractError(w, 400, requestIDFrom(w), "invalid_json", "Request body is not valid JSON.", false, nil)
			return
		}
		item, err := service.Repository.UpsertManagerScope(r.Context(), session.User.ID, input)
		if err != nil {
			writeContractError(w, 422, requestIDFrom(w), "invalid_manager_scope", err.Error(), false, nil)
			return
		}
		writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, service.Status(session.User.ID).Freshness, item)
	default:
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET or PUT for manager scopes.", false, nil)
	}
}

func (a *API) managerConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use POST to connect an FPL session.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	if _, ok = a.requireMutationAuth(w, r); !ok {
		return
	}
	var input struct {
		EntryID int    `json:"entryId"`
		Session string `json:"session"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.EntryID <= 0 || input.Session == "" {
		writeContractError(w, 422, requestIDFrom(w), "invalid_connection", "Entry ID and FPL session are required.", false, nil)
		return
	}
	if err := service.Connect(r.Context(), session.User.ID, input.EntryID, input.Session); err != nil {
		writeContractError(w, 401, requestIDFrom(w), "remote_authentication_failed", "The FPL session could not be validated.", false, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, Freshness{Dataset: "manager-fpl", State: "fresh", Status: "fresh"}, map[string]any{"entryId": input.EntryID, "state": RemoteConnected, "providerType": "memory"})
}

func (a *API) managerDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use POST to disconnect.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	if _, ok = a.requireMutationAuth(w, r); !ok {
		return
	}
	var input struct {
		EntryID int `json:"entryId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.EntryID <= 0 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "entryId is required.", false, nil)
		return
	}
	if err := service.Disconnect(r.Context(), session.User.ID, input.EntryID); err != nil {
		writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Disconnect failed.", true, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, Freshness{}, map[string]any{"entryId": input.EntryID, "state": RemoteRevoked})
}

func (a *API) managerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use POST to synchronize manager data.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	if _, ok = a.requireMutationAuth(w, r); !ok {
		return
	}
	var input struct {
		SeasonID int `json:"seasonId"`
		Gameweek int `json:"gameweek"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.SeasonID <= 0 || input.Gameweek <= 0 {
		writeContractError(w, 422, requestIDFrom(w), "manager_scope_required", "seasonId and gameweek are required.", false, nil)
		return
	}
	status, err := service.Sync(r.Context(), session.User.ID, input.SeasonID, input.Gameweek, requestIDFrom(w))
	if err != nil && status.RunID == 0 {
		writeContractError(w, 502, requestIDFrom(w), "manager_sync_failed", "Manager synchronization failed.", true, map[string]any{"status": status})
		return
	}
	warnings := append([]string(nil), status.Freshness.Warnings...)
	if status.Warning != "" {
		warnings = append(warnings, status.Warning)
	}
	writeEnvelopeWithWarnings(w, 200, requestIDFrom(w), Scope{SeasonID: input.SeasonID, Gameweek: input.Gameweek, Dataset: "manager-fpl"}, status.Freshness, warnings, status)
}

func (a *API) managerExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use GET to export manager data.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	data, err := service.Repository.ExportManagerData(r.Context(), session.User.ID)
	if err != nil {
		writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Private data export failed.", true, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, service.Status(session.User.ID).Freshness, data)
}

func (a *API) managerData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeContractError(w, 405, requestIDFrom(w), "method_not_allowed", "Use DELETE to remove manager data.", false, nil)
		return
	}
	service, session, ok := a.requireManager(w, r)
	if !ok {
		return
	}
	if _, ok = a.requireMutationAuth(w, r); !ok {
		return
	}
	scopes, err := service.Repository.ListManagerScopes(r.Context(), session.User.ID)
	if err != nil {
		writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Private data deletion failed.", true, nil)
		return
	}
	for _, scope := range scopes {
		if scope.Type == "entry" && service.Sessions != nil {
			_ = service.Sessions.Revoke(r.Context(), session.User.ID, scope.SourceID)
		}
	}
	if err = service.Repository.DeleteManagerData(r.Context(), session.User.ID); err != nil {
		writeContractError(w, 503, requestIDFrom(w), "manager_unavailable", "Private data deletion failed.", true, nil)
		return
	}
	writeEnvelope(w, 200, requestIDFrom(w), Scope{Dataset: "manager-fpl"}, Freshness{}, map[string]bool{"deleted": true})
}

func managerSourceState(err error) RemoteSessionState {
	var source ManagerSourceError
	if errors.As(err, &source) && source.Code == "permission_denied" {
		return RemotePermissionDenied
	}
	return RemoteReauthRequired
}
