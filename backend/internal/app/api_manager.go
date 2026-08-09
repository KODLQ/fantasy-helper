package app

import (
	"encoding/json"
	"errors"
	"net/http"
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
	if err != nil && status.CompletedWork == 0 {
		writeContractError(w, 502, requestIDFrom(w), "manager_sync_failed", "Manager synchronization failed.", true, map[string]any{"status": status})
		return
	}
	writeEnvelopeWithWarnings(w, 200, requestIDFrom(w), Scope{SeasonID: input.SeasonID, Gameweek: input.Gameweek, Dataset: "manager-fpl"}, status.Freshness, status.Freshness.Warnings, status)
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
