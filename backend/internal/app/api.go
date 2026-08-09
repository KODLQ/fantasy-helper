package app

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type API struct {
	Store      *Store
	Source     *FPLSource
	DBHealthy  func(context.Context) bool
	Logger     *slog.Logger
	Repository Repository
}

func NewAPI(store *Store, source *FPLSource, dbHealthy func(context.Context) bool, logger *slog.Logger, repositories ...Repository) *API {
	var repository Repository
	if len(repositories) > 0 {
		repository = repositories[0]
	}
	api := &API{Store: store, Source: source, DBHealthy: dbHealthy, Logger: logger, Repository: repository}
	if recorder, ok := repository.(SourcePayloadRepository); ok && source != nil {
		source.OnObservation = func(observation SourceObservation) {
			if err := recorder.RecordSourceObservation(context.Background(), observation); err != nil && logger != nil {
				logger.Warn("record public source observation failed", "endpoint", observation.Endpoint, "error", err)
			}
		}
	}
	return api
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.health)
	mux.HandleFunc("/api/v1/sync/status", a.syncStatus)
	mux.HandleFunc("/api/v1/data/snapshots", a.dataSnapshots)
	mux.HandleFunc("/api/v1/sync", a.sync)
	mux.HandleFunc("/api/v1/players", a.players)
	mux.HandleFunc("/api/v1/players/", a.players)
	mux.HandleFunc("/api/v1/players/compare", a.compare)
	mux.HandleFunc("/api/v1/squad", a.squad)
	mux.HandleFunc("/api/v1/recommendations", a.recommendations)
	return withCORS(withRequestID(mux))
}
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string, details []ValidationError) {
	writeJSON(w, status, map[string]interface{}{"error": map[string]interface{}{"code": code, "message": message, "details": details}})
}
func writeEnvelope(w http.ResponseWriter, status int, requestID string, scope Scope, freshness Freshness, data interface{}) {
	writeJSON(w, status, map[string]interface{}{"data": data, "meta": ResponseMeta{RequestID: requestID, Scope: scope, Freshness: freshness}})
}
func writeContractError(w http.ResponseWriter, status int, requestID, code, message string, retryable bool, details interface{}) {
	writeJSON(w, status, map[string]interface{}{"error": ResponseError{Code: code, Message: message, Retryable: retryable, Details: details}, "meta": ResponseMeta{RequestID: requestID}})
}
func parseInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
func parseFloatParam(value string) float64 { n, _ := strconv.ParseFloat(value, 64); return n }
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	if a.DBHealthy != nil {
		dbOK = a.DBHealthy(r.Context())
	}
	status := a.Store.SyncStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "database": dbOK, "sync": status.Status, "players": len(a.Store.AllPlayers())})
}
func (a *API) syncStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.Store.SyncStatus())
}
func (a *API) dataSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, w.Header().Get("X-Request-ID"), "method_not_allowed", "Use GET for dataset snapshots.", false, nil)
		return
	}
	query := r.URL.Query()
	scope := Scope{SeasonID: parseInt(query.Get("seasonId"), 0), Gameweek: parseInt(query.Get("gameweek"), 0), Dataset: query.Get("dataset")}
	if snapshotRepository, ok := a.Repository.(DatasetSnapshotRepository); ok {
		if snapshots, err := snapshotRepository.ListDatasetSnapshots(r.Context(), scope); err == nil {
			writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, a.Store.Freshness(), map[string]interface{}{"items": snapshots})
			return
		}
	}
	status := a.Store.SyncStatus()
	state := status.Freshness.State
	if state == "" {
		state = status.Freshness.Status
	}
	if state == "fresh" {
		state = "actual"
	} else if state == "unavailable" || state == "" {
		state = "unavailable"
	} else {
		state = "partial"
	}
	updated := status.Freshness.SnapshotAt
	if updated.IsZero() {
		updated = status.FinishedAt
	}
	item := DatasetSnapshot{ID: status.Checksum, Dataset: "public-fpl", State: state, SeasonID: 1, SourceFetchedAt: status.Freshness.SourceFetchedAt, NormalizedAt: updated, NormalizerVersion: status.Freshness.NormalizerVersion, MissingInputs: append([]string{}, status.Freshness.MissingInputs...)}
	if item.NormalizerVersion == "" {
		item.NormalizerVersion = "demo-v1"
	}
	item.MissingInputs = append(item.MissingInputs, status.FailedStages...)
	if item.ID == "" {
		item.ID = "unavailable"
	}
	if scope.SeasonID == 0 {
		scope.SeasonID = 1
	}
	if scope.Dataset == "" {
		scope.Dataset = "public-fpl"
	}
	writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, status.Freshness, map[string]interface{}{"items": []DatasetSnapshot{item}})
}
func (a *API) sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST to start a sync.", nil)
		return
	}
	current := a.Store.SyncStatus()
	if current.Status == "running" {
		writeError(w, http.StatusConflict, "sync_in_progress", "A sync is already running.", nil)
		return
	}
	var request struct {
		Scope    string `json:"scope"`
		SeasonID int    `json:"seasonId"`
		Gameweek int    `json:"gameweek"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Scope == "" {
		request.Scope = "full"
	}
	validScopes := map[string]bool{"catalog": true, "fixtures": true, "live": true, "player-history": true, "full": true}
	if !validScopes[request.Scope] {
		writeError(w, http.StatusBadRequest, "invalid_sync_scope", "Scope must be catalog, fixtures, live, player-history, or full.", nil)
		return
	}
	started := time.Now().UTC()
	scope := Scope{SeasonID: request.SeasonID, Gameweek: request.Gameweek, Dataset: request.Scope}
	running := SyncStatus{Status: "running", Scope: scope, CurrentStage: "snapshot", StartedAt: started, CompletedStages: []string{}, FailedStages: []string{}, Freshness: a.Store.Freshness()}
	if syncRepository, ok := a.Repository.(SyncWorkRepository); ok {
		runID, err := syncRepository.StartSyncRun(r.Context(), scope, w.Header().Get("X-Request-ID"))
		if err != nil {
			writeError(w, http.StatusConflict, "sync_scope_unavailable", "An equivalent sync scope is already running or could not be started.", nil)
			return
		}
		running.RunID = runID
	}
	a.Store.SetSyncStatus(running)
	if a.Repository != nil && running.RunID == 0 {
		_ = a.Repository.RecordSyncStatus(r.Context(), running)
	}
	go a.runSync(scope, running.RunID)
	writeJSON(w, http.StatusAccepted, a.Store.SyncStatus())
}
func (a *API) runSync(scope Scope, runID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	status := a.Store.SyncStatus()
	status.Scope = scope
	season, weeks, teams, players, fixtures, checksum, err := a.Source.Snapshot(ctx)
	if err != nil {
		status.Status = "failed"
		status.Warning = err.Error()
		status.FailedStages = []string{"snapshot"}
		status.FinishedAt = time.Now().UTC()
		a.Store.SetSyncStatus(status)
		if a.Repository != nil {
			a.persistSyncStatus(ctx, status)
		}
		return
	}
	status.CompletedStages = append(status.CompletedStages, "snapshot")
	status.CurrentStage = "player-history"
	a.Store.SetSyncStatus(status)
	if syncRepository, ok := a.Repository.(SyncWorkRepository); ok && runID > 0 {
		items := make([]SyncWorkItem, 0, len(players))
		for _, player := range players {
			items = append(items, SyncWorkItem{Scope: "player-history", NaturalKey: fmt.Sprintf("player-history:%d:%d", season.ID, player.ID), Endpoint: fmt.Sprintf("/element-summary/%d/", player.ID), SeasonSourceID: season.ID, EntitySourceID: player.ID})
		}
		if err := syncRepository.EnqueueSyncWork(ctx, runID, items); err != nil {
			status.Status = "failed"
			status.Warning = fmt.Sprintf("sync work queue persistence failed: %v", err)
			status.FailedStages = append(status.FailedStages, "work-queue")
			status.FinishedAt = time.Now().UTC()
			a.Store.SetSyncStatus(status)
			a.persistSyncStatus(ctx, status)
			return
		}
	}
	histories, failedStages := a.syncHistories(ctx, players, runID)
	status.FailedStages = append(status.FailedStages, failedStages...)
	failures := len(failedStages)
	snapshot := Snapshot{Season: season, Gameweeks: weeks, Teams: teams, Players: players, Fixtures: fixtures, Histories: histories, Checksum: checksum}
	if a.Repository != nil {
		if err := a.Repository.UpsertSnapshot(ctx, snapshot); err != nil {
			status.Status = "failed"
			status.Warning = fmt.Sprintf("database snapshot persistence failed: %v", err)
			status.FailedStages = append(status.FailedStages, "database")
			status.FinishedAt = time.Now().UTC()
			a.Store.SetSyncStatus(status)
			a.persistSyncStatus(ctx, status)
			return
		}
	}
	snapshotID := newSnapshotID()
	if snapshotRepository, ok := a.Repository.(DatasetSnapshotRepository); ok {
		state := "actual"
		if len(failedStages) > 0 {
			state = "partial"
		}
		if err := snapshotRepository.CreateDatasetSnapshot(ctx, DatasetSnapshot{ID: snapshotID, Dataset: "public-fpl", State: state, SeasonID: season.ID, NormalizedAt: time.Now().UTC(), SourceFetchedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1", MissingInputs: append([]string{}, failedStages...)}); err != nil {
			status.Status = "failed"
			status.Warning = fmt.Sprintf("dataset snapshot persistence failed: %v", err)
			status.FailedStages = append(status.FailedStages, "dataset-snapshot")
			status.FinishedAt = time.Now().UTC()
			a.Store.SetSyncStatus(status)
			a.persistSyncStatus(ctx, status)
			return
		}
	}
	a.Store.ApplySnapshot(season, weeks, teams, players, fixtures, histories)
	status.CurrentStage = ""
	status.FinishedAt = time.Now().UTC()
	status.Checksum = checksum
	state := "actual"
	if failures > 0 {
		state = "partial"
	}
	status.Freshness = Freshness{Status: "fresh", State: state, Dataset: "public-fpl", SnapshotIDs: []string{snapshotID}, LastSuccessfulSync: status.FinishedAt, SnapshotAt: status.FinishedAt, SourceFetchedAt: status.FinishedAt, NormalizedAt: status.FinishedAt, NormalizerVersion: "fpl-public-v1", MissingInputs: append([]string{}, failedStages...)}
	if failures > 0 {
		status.Status = "partial"
		status.Warning = fmt.Sprintf("%d player history requests failed; last known good data was retained where available", failures)
	} else {
		status.Status = "success"
		status.CompletedStages = append(status.CompletedStages, "player-history")
		status.Warning = ""
	}
	_ = checksum
	a.Store.SetSyncStatus(status)
	if a.Repository != nil {
		a.persistSyncStatus(ctx, status)
	}
}

func (a *API) persistSyncStatus(ctx context.Context, status SyncStatus) {
	if a.Repository == nil {
		return
	}
	if status.RunID > 0 {
		if syncRepository, ok := a.Repository.(SyncWorkRepository); ok {
			_ = syncRepository.FinishSyncRun(ctx, status.RunID, status)
			return
		}
	}
	_ = a.Repository.RecordSyncStatus(ctx, status)
}

func newSnapshotID() string {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return fmt.Sprintf("snapshot-%d", time.Now().UnixNano())
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

type historyResult struct {
	playerID int
	history  []PlayerHistory
	err      error
}

func (a *API) syncHistories(ctx context.Context, players []Player, runID int64) (map[int][]PlayerHistory, []string) {
	if syncRepository, ok := a.Repository.(SyncWorkRepository); ok && runID > 0 {
		return a.syncQueuedHistories(ctx, syncRepository, runID)
	}
	workers := 6
	if len(players) < workers {
		workers = len(players)
	}
	if workers == 0 {
		return map[int][]PlayerHistory{}, []string{}
	}
	jobs := make(chan Player)
	results := make(chan historyResult, len(players))
	var waitGroup sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for player := range jobs {
				history, _, err := a.Source.PlayerHistory(ctx, player.ID)
				results <- historyResult{playerID: player.ID, history: history, err: err}
			}
		}()
	}
	go func() {
		for _, player := range players {
			jobs <- player
		}
		close(jobs)
		waitGroup.Wait()
		close(results)
	}()
	histories := map[int][]PlayerHistory{}
	failedStages := []string{}
	for result := range results {
		if result.err != nil {
			failedStages = append(failedStages, fmt.Sprintf("player-history:%d", result.playerID))
			continue
		}
		histories[result.playerID] = result.history
	}
	return histories, failedStages
}

func (a *API) syncQueuedHistories(ctx context.Context, repository SyncWorkRepository, runID int64) (map[int][]PlayerHistory, []string) {
	workers := 6
	results := make(chan historyResult, workers)
	var waitGroup sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for {
				item, ok, err := repository.ClaimSyncWork(ctx, runID)
				if err != nil || !ok {
					return
				}
				history, _, sourceErr := a.Source.PlayerHistory(ctx, item.EntitySourceID)
				if sourceErr != nil {
					_ = repository.FailSyncWork(ctx, item.ID, sourceErr, true)
					results <- historyResult{playerID: item.EntitySourceID, err: sourceErr}
					continue
				}
				_ = repository.CompleteSyncWork(ctx, item.ID)
				results <- historyResult{playerID: item.EntitySourceID, history: history}
			}
		}()
	}
	go func() { waitGroup.Wait(); close(results) }()
	histories := map[int][]PlayerHistory{}
	failedStages := []string{}
	for result := range results {
		if result.err != nil {
			failedStages = append(failedStages, fmt.Sprintf("player-history:%d", result.playerID))
			continue
		}
		histories[result.playerID] = result.history
	}
	return histories, failedStages
}

func (a *API) players(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET for players.", nil)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/players")
	if path != "" && path != "/" {
		id := parseInt(strings.Trim(path, "/"), 0)
		if id == 0 {
			writeError(w, http.StatusNotFound, "player_not_found", "Player not found.", nil)
			return
		}
		a.playerDetail(w, r, id)
		return
	}
	q := r.URL.Query()
	query := PlayerQuery{Search: q.Get("search"), Position: parseInt(q.Get("position"), 0), TeamID: parseInt(q.Get("teamId"), 0), MinPrice: parseFloatParam(q.Get("minPrice")), MaxPrice: parseFloatParam(q.Get("maxPrice")), MinMinutes: parseInt(q.Get("minMinutes"), 0), MinForm: parseFloatParam(q.Get("minForm")), MinPoints: parseInt(q.Get("minPoints"), 0), MinValue: parseFloatParam(q.Get("minValue")), Status: q.Get("status"), Sort: q.Get("sort"), Desc: q.Get("direction") != "asc", Page: parseInt(q.Get("page"), 1), PageSize: parseInt(q.Get("pageSize"), 25)}
	results, total := a.Store.SearchPlayers(query)
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": results, "total": total, "page": query.Page, "pageSize": query.PageSize, "freshness": a.Store.Freshness()})
}
func (a *API) playerDetail(w http.ResponseWriter, r *http.Request, id int) {
	player, ok := a.Store.Player(id)
	if !ok {
		writeError(w, http.StatusNotFound, "player_not_found", "Player not found in the active season.", nil)
		return
	}
	team, _ := a.Store.Team(player.TeamID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"player": player, "team": team, "history": a.Store.History(id), "fixtures": a.Store.UpcomingFixtures(player.TeamID), "freshness": a.Store.Freshness()})
}
func (a *API) compare(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) == 0 || len(ids) > 4 || ids[0] == "" {
		writeError(w, http.StatusBadRequest, "comparison_limit", "Compare between one and four players.", nil)
		return
	}
	items := []map[string]interface{}{}
	for _, raw := range ids {
		id := parseInt(raw, 0)
		player, ok := a.Store.Player(id)
		if !ok {
			writeError(w, http.StatusNotFound, "player_not_found", "One of the selected players was not found.", nil)
			return
		}
		items = append(items, map[string]interface{}{"player": player, "team": mustTeam(a.Store.Team(player.TeamID)), "history": a.Store.History(id)})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "freshness": a.Store.Freshness()})
}
func mustTeam(team Team, ok bool) Team { return team }
func (a *API) squad(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		squad := a.Store.EnrichSquad(a.Store.GetSquad())
		squad.Validation = a.Store.ValidatePlan(squad)
		writeJSON(w, http.StatusOK, squad)
	case http.MethodPut:
		var input Squad
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", nil)
			return
		}
		input.Budget = 100
		input = a.Store.EnrichSquad(input)
		errors := a.Store.ValidatePlan(input)
		if len(errors) > 0 {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "Squad update failed validation.", errors)
			return
		}
		if a.Repository != nil {
			if err := a.Repository.SaveSquad(r.Context(), input); err != nil {
				writeError(w, http.StatusInternalServerError, "persistence_failed", "Squad could not be saved to the database.", nil)
				return
			}
		}
		a.Store.SaveSquad(input)
		writeJSON(w, http.StatusOK, a.Store.EnrichSquad(input))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or PUT for the squad.", nil)
	}
}
func (a *API) recommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST for recommendations.", nil)
		return
	}
	var body struct {
		Weights Weights `json:"weights"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	recommendation, errors := a.Store.Recommend(a.Store.GetSquad(), body.Weights)
	if len(errors) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "recommendation_failed", "Recommendation could not be generated.", errors)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"recommendation": recommendation, "freshness": a.Store.Freshness()})
}
