package app

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type API struct {
	Store       *Store
	Source      *FPLSource
	DBHealthy   func(context.Context) bool
	Logger      *slog.Logger
	Repository  Repository
	Auth        *AuthService
	Manager     *ManagerService
	SyncWorkers int
	startMu     sync.Mutex
	syncMu      sync.Mutex
	syncCancels map[uint64]context.CancelFunc
	syncWait    sync.WaitGroup
	nextSyncID  uint64
	metrics     SyncMetrics
}

type SyncMetrics struct {
	Started   uint64 `json:"started"`
	Completed uint64 `json:"completed"`
	Partial   uint64 `json:"partial"`
	Failed    uint64 `json:"failed"`
	Cancelled uint64 `json:"cancelled"`
}

type SyncPolicyError struct {
	Code    string
	Message string
}

func (e SyncPolicyError) Error() string { return e.Message }

func NewAPI(store *Store, source *FPLSource, dbHealthy func(context.Context) bool, logger *slog.Logger, repositories ...Repository) *API {
	var repository Repository
	if len(repositories) > 0 {
		repository = repositories[0]
	}
	api := &API{Store: store, Source: source, DBHealthy: dbHealthy, Logger: logger, Repository: repository, SyncWorkers: 6, syncCancels: map[uint64]context.CancelFunc{}}
	if recorder, ok := repository.(SourcePayloadRepository); ok && source != nil {
		source.OnObservation = func(observation SourceObservation) {
			if err := recorder.RecordSourceObservation(context.Background(), observation); err != nil && logger != nil {
				logger.Warn("record public source observation failed", "endpoint", observation.Endpoint, "error", err)
			}
		}
	}
	return api
}

func (a *API) Metrics() SyncMetrics {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	return a.metrics
}

func (a *API) startSync(scope Scope, runID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a.syncMu.Lock()
	a.nextSyncID++
	token := a.nextSyncID
	a.syncCancels[token] = cancel
	a.syncWait.Add(1)
	a.metrics.Started++
	a.syncMu.Unlock()
	go func() {
		defer func() {
			a.syncMu.Lock()
			delete(a.syncCancels, token)
			status := a.Store.SyncStatus()
			if ctx.Err() == context.Canceled {
				a.metrics.Cancelled++
			} else {
				switch status.Status {
				case "success":
					a.metrics.Completed++
				case "partial":
					a.metrics.Partial++
				default:
					a.metrics.Failed++
				}
			}
			a.syncMu.Unlock()
			a.syncWait.Done()
		}()
		a.runSync(ctx, scope, runID)
	}()
}

func (a *API) Shutdown(ctx context.Context) error {
	a.syncMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.syncCancels))
	for _, cancel := range a.syncCancels {
		cancels = append(cancels, cancel)
	}
	a.syncMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		a.syncWait.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.health)
	mux.HandleFunc("/api/v1/auth/config", a.authConfig)
	mux.HandleFunc("/api/v1/auth/register", a.authRegister)
	mux.HandleFunc("/api/v1/auth/login", a.authLogin)
	mux.HandleFunc("/api/v1/auth/logout", a.authLogout)
	mux.HandleFunc("/api/v1/auth/me", a.authMe)
	mux.HandleFunc("/api/v1/auth/password", a.authPassword)
	mux.HandleFunc("/api/v1/manager/status", a.managerStatus)
	mux.HandleFunc("/api/v1/manager/scopes", a.managerScopes)
	mux.HandleFunc("/api/v1/manager/connect", a.managerConnect)
	mux.HandleFunc("/api/v1/manager/disconnect", a.managerDisconnect)
	mux.HandleFunc("/api/v1/manager/sync", a.managerSync)
	mux.HandleFunc("/api/v1/manager/export", a.managerExport)
	mux.HandleFunc("/api/v1/manager/data", a.managerData)
	mux.HandleFunc("/api/v1/manager/entries/", a.managerEntryData)
	mux.HandleFunc("/api/v1/manager/leagues/", a.managerLeagueData)
	mux.HandleFunc("/api/v1/squad/import/preview", a.squadImportPreview)
	mux.HandleFunc("/api/v1/squad/import", a.squadImport)
	mux.HandleFunc("/api/v1/sync/status", a.syncStatus)
	mux.HandleFunc("/api/v1/sync/runs/", a.syncRun)
	mux.HandleFunc("/api/v1/data/snapshots", a.dataSnapshots)
	mux.HandleFunc("/api/v1/seasons", a.seasons)
	mux.HandleFunc("/api/v1/sync", a.sync)
	mux.HandleFunc("/api/v1/players", a.players)
	mux.HandleFunc("/api/v1/players/", a.players)
	mux.HandleFunc("/api/v1/analysis/players/", a.playerAnalysis)
	mux.HandleFunc("/api/v1/players/compare", a.compare)
	mux.HandleFunc("/api/v1/squad", a.squad)
	mux.HandleFunc("/api/v1/recommendations", a.recommendations)
	return a.withCORS(withRequestID(a.withOptionalAuth(mux)))
}

func (a *API) playerAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET for historical player analysis.", nil)
		return
	}
	repository, ok := a.Repository.(HistoricalResearchRepository)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "analysis_unavailable", "Historical analysis requires the PostgreSQL warehouse.", nil)
		return
	}
	playerID := parseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/analysis/players/"), 0)
	query := r.URL.Query()
	scope := Scope{SeasonID: parseInt(query.Get("seasonId"), 0), Gameweek: parseInt(query.Get("gameweek"), 0), Dataset: "public-fpl"}
	snapshotID := query.Get("snapshotId")
	if playerID <= 0 || scope.SeasonID <= 0 || (scope.Gameweek <= 0 && snapshotID == "") {
		writeError(w, http.StatusBadRequest, "historical_scope_required", "Player, seasonId, and gameweek or snapshotId are required.", nil)
		return
	}
	if _, _, err := a.resolveSeason(r.Context(), scope.SeasonID); err != nil {
		writeSeasonResolutionError(w, w.Header().Get("X-Request-ID"), err)
		return
	}
	item, found, err := repository.LoadPlayerAnalysis(r.Context(), scope.SeasonID, scope.Gameweek, snapshotID, playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analysis_unavailable", "Historical player analysis is temporarily unavailable.", nil)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "analysis_not_found", "No player analysis exists for the requested scope.", nil)
		return
	}
	freshness := a.requestFreshness(r.Context(), scope)
	writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, item)
}
func (a *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if a.Auth != nil && a.Auth.Config.AllowedOrigin != "" {
			origin = a.Auth.Config.AllowedOrigin
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-CSRF-Token")
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
func writeEnvelopeWithWarnings(w http.ResponseWriter, status int, requestID string, scope Scope, freshness Freshness, warnings []string, data interface{}) {
	writeJSON(w, status, map[string]interface{}{"data": data, "meta": ResponseMeta{RequestID: requestID, Scope: scope, Freshness: freshness, Warnings: warnings}})
}
func writeContractError(w http.ResponseWriter, status int, requestID, code, message string, retryable bool, details interface{}) {
	writeJSON(w, status, map[string]interface{}{"error": ResponseError{Code: code, Message: message, Retryable: retryable, Details: details}, "meta": ResponseMeta{RequestID: requestID}})
}
func (a *API) requestFreshness(ctx context.Context, scope Scope) Freshness {
	if repository, ok := a.Repository.(DatasetFreshnessRepository); ok {
		if freshness, err := repository.CurrentDatasetFreshness(ctx, scope); err == nil && freshness.State != "" {
			return freshness
		}
	}
	return a.Store.Freshness()
}
func parseInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
func parseFloatParam(value string) float64 { n, _ := strconv.ParseFloat(value, 64); return n }

type seasonResolutionError struct {
	Status  int
	Code    string
	Message string
}

func (e seasonResolutionError) Error() string { return e.Message }

func (a *API) seasonCatalogue(ctx context.Context) ([]SeasonCatalogueItem, error) {
	if repository, ok := a.Repository.(SeasonCatalogueRepository); ok {
		return repository.ListSeasons(ctx)
	}
	season, gameweek, snapshotAt := a.Store.Snapshot()
	if season.ID == 0 {
		return []SeasonCatalogueItem{}, nil
	}
	state := SeasonHistorical
	if season.IsCurrent {
		state = SeasonCurrent
	}
	weeks := []Gameweek{}
	if gameweek.ID != 0 {
		weeks = append(weeks, gameweek)
	}
	item := SeasonCatalogueItem{ID: season.ID, Name: season.Name, State: state, AvailableGameweeks: weeks, SourceKind: season.SourceKind, LastImportedAt: snapshotAt, Freshness: a.Store.Freshness(), Completeness: map[string]interface{}{"catalogue": true}, MissingInputs: []string{}, Warnings: []string{}}
	if item.SourceKind == "" {
		item.SourceKind = SourceRetainedSnapshot
	}
	item.DefaultGameweek = DefaultGameweek(state, weeks)
	return []SeasonCatalogueItem{item}, nil
}

func (a *API) resolveSeason(ctx context.Context, requested int) (SeasonCatalogueItem, []string, error) {
	items, err := a.seasonCatalogue(ctx)
	if err != nil {
		return SeasonCatalogueItem{}, nil, seasonResolutionError{Status: http.StatusServiceUnavailable, Code: "SEASON_DATA_UNAVAILABLE", Message: "Season catalogue is temporarily unavailable."}
	}
	if requested > 0 {
		for _, item := range items {
			if item.ID == requested {
				if slices.Contains(item.MissingInputs, "catalogue") {
					return SeasonCatalogueItem{}, nil, seasonResolutionError{Status: http.StatusConflict, Code: "SEASON_DATA_UNAVAILABLE", Message: "The requested season is known, but its queryable catalogue is unavailable."}
				}
				return item, nil, nil
			}
		}
		return SeasonCatalogueItem{}, nil, seasonResolutionError{Status: http.StatusNotFound, Code: "SEASON_NOT_FOUND", Message: "The requested season is not available."}
	}
	item, found := DefaultSeason(items)
	if !found {
		return SeasonCatalogueItem{}, nil, seasonResolutionError{Status: http.StatusConflict, Code: "SEASON_DATA_UNAVAILABLE", Message: "No queryable FPL season is available."}
	}
	return item, []string{"seasonId was omitted and resolved to the default season; explicit seasonId will be required by a future API version"}, nil
}

func writeSeasonResolutionError(w http.ResponseWriter, requestID string, err error) {
	resolved, ok := err.(seasonResolutionError)
	if !ok {
		writeContractError(w, http.StatusInternalServerError, requestID, "season_resolution_failed", "Season scope could not be resolved.", true, nil)
		return
	}
	writeContractError(w, resolved.Status, requestID, resolved.Code, resolved.Message, false, nil)
}

func (a *API) seasons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeContractError(w, http.StatusMethodNotAllowed, w.Header().Get("X-Request-ID"), "method_not_allowed", "Use GET for seasons.", false, nil)
		return
	}
	items, err := a.seasonCatalogue(r.Context())
	if err != nil {
		writeContractError(w, http.StatusServiceUnavailable, w.Header().Get("X-Request-ID"), "season_catalogue_unavailable", "Season catalogue is temporarily unavailable.", true, nil)
		return
	}
	if items == nil {
		items = []SeasonCatalogueItem{}
	}
	freshness := Freshness{Dataset: "season-catalogue", State: "actual", Status: "fresh"}
	warnings := []string{}
	if len(items) == 0 {
		freshness.State = "unavailable"
		freshness.Status = "unavailable"
		warnings = append(warnings, "No queryable season has been imported.")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"items": items}, "meta": ResponseMeta{RequestID: w.Header().Get("X-Request-ID"), Freshness: freshness, Pagination: &Pagination{Limit: len(items), Returned: len(items), Total: len(items)}, Warnings: warnings}})
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	if a.DBHealthy != nil {
		dbOK = a.DBHealthy(r.Context())
	}
	status := a.Store.SyncStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "database": dbOK, "sync": status.Status, "players": len(a.Store.AllPlayers())})
}
func (a *API) syncStatus(w http.ResponseWriter, r *http.Request) {
	status := a.Store.SyncStatus()
	if repository, ok := a.Repository.(SyncStatusRepository); ok {
		if loaded, err := repository.LoadLatestSyncStatus(r.Context()); err == nil {
			status = loaded
		}
	}
	writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), status.Scope, status.Freshness, status)
}
func (a *API) syncRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/retry") {
		writeContractError(w, http.StatusMethodNotAllowed, w.Header().Get("X-Request-ID"), "method_not_allowed", "Use POST /api/v1/sync/runs/{id}/retry.", false, nil)
		return
	}
	if _, ok := a.requireMutationAuth(w, r); !ok {
		return
	}
	path := strings.TrimSuffix(strings.TrimPrefix(strings.TrimRight(r.URL.Path, "/"), "/api/v1/sync/runs/"), "/retry")
	runID, err := strconv.ParseInt(path, 10, 64)
	if err != nil || runID <= 0 {
		writeContractError(w, http.StatusBadRequest, w.Header().Get("X-Request-ID"), "invalid_run_id", "The sync run ID is invalid.", false, nil)
		return
	}
	repository, ok := a.Repository.(SyncStatusRepository)
	if !ok {
		writeContractError(w, http.StatusNotImplemented, w.Header().Get("X-Request-ID"), "sync_retry_unavailable", "Sync retry is not available without PostgreSQL.", false, nil)
		return
	}
	a.startMu.Lock()
	defer a.startMu.Unlock()
	if a.Store.SyncStatus().Status == "running" {
		writeContractError(w, http.StatusConflict, w.Header().Get("X-Request-ID"), "sync_retry_failed", "Another sync is already running.", true, nil)
		return
	}
	status, err := repository.RetrySyncRun(r.Context(), runID)
	if err != nil {
		writeContractError(w, http.StatusConflict, w.Header().Get("X-Request-ID"), "sync_retry_failed", "The sync run could not be retried.", true, nil)
		return
	}
	a.Store.SetSyncStatus(status)
	a.startSync(status.Scope, runID)
	writeEnvelope(w, http.StatusAccepted, w.Header().Get("X-Request-ID"), status.Scope, status.Freshness, status)
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
			writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, a.requestFreshness(r.Context(), scope), map[string]interface{}{"items": snapshots})
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
	writeEnvelope(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, a.requestFreshness(r.Context(), scope), map[string]interface{}{"items": []DatasetSnapshot{item}})
}
func (a *API) sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST to start a sync.", nil)
		return
	}
	if _, ok := a.requireMutationAuth(w, r); !ok {
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
	scope := Scope{SeasonID: request.SeasonID, Gameweek: request.Gameweek, Dataset: request.Scope}
	running, err := a.StartScopedSync(r.Context(), scope, w.Header().Get("X-Request-ID"))
	if err != nil {
		if policy, ok := err.(SyncPolicyError); ok {
			writeContractError(w, http.StatusConflict, w.Header().Get("X-Request-ID"), policy.Code, policy.Message, false, nil)
			return
		}
		writeContractError(w, http.StatusConflict, w.Header().Get("X-Request-ID"), "sync_scope_unavailable", "An equivalent sync scope is already running or could not be started.", true, nil)
		return
	}
	writeEnvelope(w, http.StatusAccepted, w.Header().Get("X-Request-ID"), scope, a.Store.Freshness(), running)
}

func (a *API) StartScopedSync(ctx context.Context, scope Scope, correlationID string) (SyncStatus, error) {
	a.startMu.Lock()
	defer a.startMu.Unlock()
	if a.Source == nil {
		return SyncStatus{}, SyncPolicyError{Code: "SOURCE_UNAVAILABLE", Message: "No FPL source profile is configured."}
	}
	if scope.SeasonID > 0 && a.Source.SeasonID > 0 && scope.SeasonID != a.Source.SeasonID {
		return SyncStatus{}, SyncPolicyError{Code: "HISTORICAL_SOURCE_UNAVAILABLE", Message: "Historical seasons require a deliberate retained-snapshot or archive import."}
	}
	if a.Source.Kind != "" && a.Source.Kind != SourceOfficialCurrent {
		return SyncStatus{}, SyncPolicyError{Code: "HISTORICAL_LIVE_SYNC_FORBIDDEN", Message: "Scheduled and live synchronization are not allowed for historical source profiles."}
	}
	if a.Store.SyncStatus().Status == "running" {
		return SyncStatus{}, fmt.Errorf("sync already running")
	}
	started := time.Now().UTC()
	if scope.SeasonID == 0 && a.Source.SeasonID > 0 {
		scope.SeasonID = a.Source.SeasonID
	}
	running := SyncStatus{Status: "running", Scope: scope, CurrentStage: "catalog", CorrelationID: correlationID, StartedAt: started, CompletedStages: []string{}, FailedStages: []string{}, Freshness: a.Store.Freshness()}
	if syncRepository, ok := a.Repository.(SyncWorkRepository); ok {
		runID, err := syncRepository.StartSyncRun(ctx, scope, correlationID)
		if err != nil {
			return SyncStatus{}, err
		}
		running.RunID = runID
	}
	a.Store.SetSyncStatus(running)
	if a.Repository != nil && running.RunID == 0 {
		_ = a.Repository.RecordSyncStatus(ctx, running)
	}
	a.startSync(scope, running.RunID)
	return running, nil
}

func (a *API) startStage(ctx context.Context, status *SyncStatus, name string) {
	status.CurrentStage = name
	a.Store.SetSyncStatus(*status)
	if repository, ok := a.Repository.(SyncStageRepository); ok && status.RunID > 0 {
		_ = repository.RecordSyncStage(ctx, SyncStage{RunID: status.RunID, Name: name, Status: "running", StartedAt: time.Now().UTC()})
	}
}

func (a *API) completeStage(ctx context.Context, status *SyncStatus, name string, processed int) {
	if !contains(status.CompletedStages, name) {
		status.CompletedStages = append(status.CompletedStages, name)
	}
	if status.CurrentStage == name {
		status.CurrentStage = ""
	}
	a.Store.SetSyncStatus(*status)
	if repository, ok := a.Repository.(SyncStageRepository); ok && status.RunID > 0 {
		_ = repository.RecordSyncStage(ctx, SyncStage{RunID: status.RunID, Name: name, Status: "success", ProcessedCount: processed, FinishedAt: time.Now().UTC()})
	}
}

func (a *API) failStage(ctx context.Context, status *SyncStatus, name string, err error) {
	status.Status = "failed"
	status.CurrentStage = ""
	status.Warning = err.Error()
	if !contains(status.FailedStages, name) {
		status.FailedStages = append(status.FailedStages, name)
	}
	status.FinishedAt = time.Now().UTC()
	a.Store.SetSyncStatus(*status)
	if repository, ok := a.Repository.(SyncStageRepository); ok && status.RunID > 0 {
		_ = repository.RecordSyncStage(ctx, SyncStage{RunID: status.RunID, Name: name, Status: "failed", FailedCount: 1, Error: err.Error(), FinishedAt: status.FinishedAt})
	}
	a.persistSyncStatus(ctx, *status)
}

func (a *API) runSync(ctx context.Context, scope Scope, runID int64) {
	status := a.Store.SyncStatus()
	status.Scope = scope
	status.RunID = runID
	a.startStage(ctx, &status, "catalog")
	catalog, catalogChecksum, err := a.Source.Bootstrap(ctx)
	if err != nil {
		a.failStage(ctx, &status, "catalog", err)
		return
	}
	a.completeStage(ctx, &status, "catalog", len(catalog.Elements)+len(catalog.Teams)+len(catalog.Events))

	fixtureFeed := FixtureFeed{Fixtures: []SourceFixture{}}
	fixtureChecksum := ""
	needsFixtures := scope.Dataset != "catalog"
	if needsFixtures {
		a.startStage(ctx, &status, "fixtures")
		fixtureFeed, fixtureChecksum, err = a.Source.Fixtures(ctx, scope.Gameweek)
		if err != nil {
			a.failStage(ctx, &status, "fixtures", err)
			return
		}
		a.completeStage(ctx, &status, "fixtures", len(fixtureFeed.Fixtures))
	}
	season, weeks, teams, players, fixtures, err := a.Source.NormalizeSnapshot(catalog, fixtureFeed)
	if err != nil {
		a.failStage(ctx, &status, "catalog", err)
		return
	}
	checksum := catalogChecksum
	if fixtureChecksum != "" {
		checksum += ":" + fixtureChecksum
	}

	liveGameweek := scope.Gameweek
	if liveGameweek == 0 {
		for _, event := range catalog.Events {
			if event.IsCurrent {
				liveGameweek = event.ID
				break
			}
		}
	}
	var live EventLive
	needsLive := scope.Dataset == "live" || scope.Dataset == "full"
	if needsLive {
		if liveGameweek == 0 && scope.Dataset == "live" {
			a.failStage(ctx, &status, "live", fmt.Errorf("live sync requires a gameweek when no current event is available"))
			return
		}
		if liveGameweek > 0 {
			a.startStage(ctx, &status, "live")
			var liveChecksum string
			live, liveChecksum, err = a.Source.EventLive(ctx, liveGameweek)
			if err != nil {
				a.failStage(ctx, &status, "live", err)
				return
			}
			checksum += ":" + liveChecksum
			a.completeStage(ctx, &status, "live", len(live.Elements))
		}
	}

	histories := map[int][]PlayerHistory{}
	failedStages := []string{}
	needsHistories := scope.Dataset == "player-history" || scope.Dataset == "full"
	if needsHistories {
		a.startStage(ctx, &status, "player-history")
		if syncRepository, ok := a.Repository.(SyncWorkRepository); ok && runID > 0 {
			items := make([]SyncWorkItem, 0, len(players))
			for _, player := range players {
				items = append(items, SyncWorkItem{Scope: "player-history", NaturalKey: fmt.Sprintf("player-history:%d:%d", season.ID, player.ID), Endpoint: fmt.Sprintf("/element-summary/%d/", player.ID), SeasonSourceID: season.ID, EntitySourceID: player.ID})
			}
			if err := syncRepository.EnqueueSyncWork(ctx, runID, items); err != nil {
				a.failStage(ctx, &status, "player-history", fmt.Errorf("sync work queue persistence failed: %w", err))
				return
			}
		}
		histories, failedStages = a.syncHistories(ctx, players, runID)
		status.FailedStages = append(status.FailedStages, failedStages...)
		if len(failedStages) == 0 {
			a.completeStage(ctx, &status, "player-history", len(histories))
		} else {
			status.CurrentStage = ""
			if repository, ok := a.Repository.(SyncStageRepository); ok && status.RunID > 0 {
				_ = repository.RecordSyncStage(ctx, SyncStage{RunID: status.RunID, Name: "player-history", Status: "partial", ProcessedCount: len(histories), FailedCount: len(failedStages), Error: fmt.Sprintf("%d player history requests failed", len(failedStages)), FinishedAt: time.Now().UTC()})
			}
		}
	}

	failures := len(failedStages)
	existing := a.Store.ExportSnapshot()
	if !needsFixtures {
		fixtures = existing.Fixtures
	} else if scope.Gameweek > 0 && scope.Dataset != "full" {
		fixtures = mergeFixtures(existing.Fixtures, fixtures, scope.Gameweek)
	}
	if !needsHistories {
		histories = existing.Histories
	}
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
	factRepository, hasFactRepository := a.Repository.(WarehouseFactRepository)
	liveFactsUnchanged := false
	if hasFactRepository && needsLive && liveGameweek > 0 {
		liveFactsUnchanged, err = factRepository.LiveGameweekFactsUnchanged(ctx, season.ID, liveGameweek, live.Elements)
		if err != nil {
			a.failStage(ctx, &status, "live", fmt.Errorf("live finalization check failed: %w", err))
			return
		}
	}
	snapshotID := newSnapshotID()
	if snapshotRepository, ok := a.Repository.(DatasetSnapshotRepository); ok {
		state := "actual"
		if len(failedStages) > 0 {
			state = "partial"
		}
		snapshotGameweek := scope.Gameweek
		if needsLive {
			snapshotGameweek = liveGameweek
		}
		if err := snapshotRepository.CreateDatasetSnapshot(ctx, DatasetSnapshot{ID: snapshotID, Dataset: "public-fpl", State: state, SeasonID: season.ID, Gameweek: snapshotGameweek, NormalizedAt: time.Now().UTC(), SourceFetchedAt: time.Now().UTC(), NormalizerVersion: "fpl-public-v1", MissingInputs: append([]string{}, failedStages...)}); err != nil {
			status.Status = "failed"
			status.Warning = fmt.Sprintf("dataset snapshot persistence failed: %v", err)
			status.FailedStages = append(status.FailedStages, "dataset-snapshot")
			status.FinishedAt = time.Now().UTC()
			a.Store.SetSyncStatus(status)
			a.persistSyncStatus(ctx, status)
			return
		}
	}
	if hasFactRepository {
		observedAt := time.Now().UTC()
		if err := factRepository.UpsertPlayerSnapshots(ctx, snapshotID, season.ID, observedAt, players); err != nil {
			a.failStage(ctx, &status, "catalog", fmt.Errorf("player snapshot persistence failed: %w", err))
			return
		}
		if needsFixtures {
			if err := factRepository.UpsertFixtureStats(ctx, season.ID, observedAt, fixtureFeed.Fixtures); err != nil {
				a.failStage(ctx, &status, "fixtures", fmt.Errorf("fixture statistics persistence failed: %w", err))
				return
			}
		}
		if needsLive && liveGameweek > 0 {
			finalized := liveFinalized(catalog.Events, fixtureFeed.Fixtures, liveGameweek, live.Finalized, liveFactsUnchanged)
			if err := factRepository.UpsertLiveGameweek(ctx, snapshotID, season.ID, liveGameweek, finalized, observedAt, live.Elements); err != nil {
				a.failStage(ctx, &status, "live", fmt.Errorf("live gameweek persistence failed: %w", err))
				return
			}
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
		status.Warning = ""
	}
	a.Store.SetSyncStatus(status)
	if a.Repository != nil {
		a.persistSyncStatus(ctx, status)
	}
}

func mergeFixtures(existing, incoming []Fixture, gameweek int) []Fixture {
	merged := make([]Fixture, 0, len(existing)+len(incoming))
	for _, fixture := range existing {
		if fixture.Gameweek != gameweek {
			merged = append(merged, fixture)
		}
	}
	return append(merged, incoming...)
}

func liveFinalized(events []SourceEvent, fixtures []SourceFixture, gameweek int, sourceValue *bool, unchanged bool) bool {
	sourceFinished := false
	if sourceValue != nil {
		sourceFinished = *sourceValue
	}
	for _, event := range events {
		if event.ID == gameweek {
			sourceFinished = sourceFinished || (event.Finished && event.DataChecked)
			break
		}
	}
	if !sourceFinished || !unchanged || len(fixtures) == 0 {
		return false
	}
	foundGameweekFixture := false
	for _, fixture := range fixtures {
		if fixture.Event != nil && *fixture.Event == gameweek {
			foundGameweekFixture = true
			if !fixture.Finished {
				return false
			}
		}
	}
	return foundGameweekFixture
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
	workers := a.SyncWorkers
	if workers < 1 {
		workers = 1
	}
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
	workers := a.SyncWorkers
	if workers < 1 {
		workers = 1
	}
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
	season, warnings, err := a.resolveSeason(r.Context(), parseInt(r.URL.Query().Get("seasonId"), 0))
	if err != nil {
		writeSeasonResolutionError(w, w.Header().Get("X-Request-ID"), err)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/players")
	if path != "" && path != "/" {
		id := parseInt(strings.Trim(path, "/"), 0)
		if id == 0 {
			writeError(w, http.StatusNotFound, "player_not_found", "Player not found.", nil)
			return
		}
		a.playerDetail(w, r, season.ID, id, warnings)
		return
	}
	q := r.URL.Query()
	query := PlayerQuery{SeasonID: season.ID, Search: q.Get("search"), Position: parseInt(q.Get("position"), 0), TeamID: parseInt(q.Get("teamId"), 0), MinPrice: parseFloatParam(q.Get("minPrice")), MaxPrice: parseFloatParam(q.Get("maxPrice")), MinMinutes: parseInt(q.Get("minMinutes"), 0), MinForm: parseFloatParam(q.Get("minForm")), MinPoints: parseInt(q.Get("minPoints"), 0), MinValue: parseFloatParam(q.Get("minValue")), Status: q.Get("status"), Sort: q.Get("sort"), Desc: q.Get("direction") != "asc", Page: parseInt(q.Get("page"), 1), PageSize: parseInt(q.Get("pageSize"), 25)}
	var results []Player
	var total int
	var teams []Team
	if researchRepository, ok := a.Repository.(ResearchReadRepository); ok {
		loaded, loadedTotal, err := researchRepository.SearchPlayers(r.Context(), query)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "research_unavailable", "Player research is temporarily unavailable.", nil)
			return
		}
		results, total = loaded, loadedTotal
		teams, err = researchRepository.ListTeamsForSeason(r.Context(), season.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "research_unavailable", "Player research is temporarily unavailable.", nil)
			return
		}
	} else {
		results, total = a.Store.SearchPlayers(query)
		teams = a.Store.AllTeams()
	}
	items, err := relatePlayersToTeams(results, teams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "player_team_inconsistent", "Player and club data are temporarily inconsistent.", nil)
		return
	}
	scope := Scope{SeasonID: season.ID, Dataset: "public-fpl"}
	freshness := a.requestFreshness(r.Context(), scope)
	writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, map[string]interface{}{"items": items, "teams": teams, "total": total, "page": query.Page, "pageSize": query.PageSize, "freshness": freshness})
}

func relatePlayersToTeams(players []Player, teams []Team) ([]PlayerResearchItem, error) {
	byID := make(map[int]Team, len(teams))
	for _, team := range teams {
		byID[team.ID] = team
	}
	items := make([]PlayerResearchItem, 0, len(players))
	for _, player := range players {
		team, ok := byID[player.TeamID]
		if !ok {
			return nil, fmt.Errorf("player %d references missing team %d", player.ID, player.TeamID)
		}
		items = append(items, PlayerResearchItem{Player: player, Team: team})
	}
	return items, nil
}
func (a *API) playerDetail(w http.ResponseWriter, r *http.Request, seasonID, id int, warnings []string) {
	scope := Scope{SeasonID: seasonID, Dataset: "public-fpl"}
	if researchRepository, ok := a.Repository.(ResearchReadRepository); ok {
		detail, found, err := researchRepository.LoadPlayerDetail(r.Context(), seasonID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "player_unavailable", "Player research is temporarily unavailable.", nil)
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "player_not_found", "Player not found in the active season.", nil)
			return
		}
		detail.Freshness = a.requestFreshness(r.Context(), scope)
		writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, detail.Freshness, warnings, detail)
		return
	}
	player, ok := a.Store.Player(id)
	if !ok {
		writeError(w, http.StatusNotFound, "player_not_found", "Player not found in the active season.", nil)
		return
	}
	team, found := a.Store.Team(player.TeamID)
	if !found {
		writeError(w, http.StatusInternalServerError, "player_team_inconsistent", "Player and club data are temporarily inconsistent.", nil)
		return
	}
	freshness := a.Store.Freshness()
	writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, map[string]interface{}{"player": player, "team": team, "history": a.Store.History(id), "fixtures": a.Store.UpcomingFixtures(player.TeamID), "fixtureContext": a.Store.FixtureContext(player), "freshness": freshness})
}
func (a *API) compare(w http.ResponseWriter, r *http.Request) {
	season, warnings, err := a.resolveSeason(r.Context(), parseInt(r.URL.Query().Get("seasonId"), 0))
	if err != nil {
		writeSeasonResolutionError(w, w.Header().Get("X-Request-ID"), err)
		return
	}
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) == 0 || len(ids) > 4 || ids[0] == "" {
		writeError(w, http.StatusBadRequest, "comparison_limit", "Compare between one and four players.", nil)
		return
	}
	items := []map[string]interface{}{}
	for _, raw := range ids {
		id := parseInt(raw, 0)
		if repository, ok := a.Repository.(ResearchReadRepository); ok {
			detail, found, err := repository.LoadPlayerDetail(r.Context(), season.ID, id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "comparison_unavailable", "Player comparison is temporarily unavailable.", nil)
				return
			}
			if !found {
				writeError(w, http.StatusNotFound, "player_not_found", "One of the selected players was not found.", nil)
				return
			}
			items = append(items, map[string]interface{}{"player": detail.Player, "team": detail.Team, "history": detail.History, "fixtures": detail.Fixtures})
			continue
		}
		player, found := a.Store.Player(id)
		if !found {
			writeError(w, http.StatusNotFound, "player_not_found", "One of the selected players was not found.", nil)
			return
		}
		team, found := a.Store.Team(player.TeamID)
		if !found {
			writeError(w, http.StatusInternalServerError, "player_team_inconsistent", "Player and club data are temporarily inconsistent.", nil)
			return
		}
		items = append(items, map[string]interface{}{"player": player, "team": team, "history": a.Store.History(id), "fixtures": a.Store.UpcomingFixtures(player.TeamID)})
	}
	scope := Scope{SeasonID: season.ID, Dataset: "public-fpl"}
	freshness := a.requestFreshness(r.Context(), scope)
	writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, map[string]interface{}{"items": items, "freshness": freshness})
}
func (a *API) squad(w http.ResponseWriter, r *http.Request) {
	session, authenticated := a.requireAuth(w, r)
	if !authenticated {
		return
	}
	if r.Method == http.MethodPut {
		if _, ok := a.requireMutationAuth(w, r); !ok {
			return
		}
	}
	season, warnings, err := a.resolveSeason(r.Context(), parseInt(r.URL.Query().Get("seasonId"), 0))
	if err != nil {
		writeSeasonResolutionError(w, w.Header().Get("X-Request-ID"), err)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), season.ID, session.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "planning_unavailable", "Squad planning data is temporarily unavailable.", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		squad := domain.EnrichSquad(domain.GetSquad())
		squad.Validation = domain.ValidatePlan(squad)
		scope := Scope{SeasonID: season.ID, Dataset: "public-fpl"}
		freshness := a.requestFreshness(r.Context(), scope)
		writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, squad)
	case http.MethodPut:
		var input Squad
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.", nil)
			return
		}
		input.Budget = 100
		input = domain.EnrichSquad(input)
		errors := domain.ValidatePlan(input)
		if len(errors) > 0 {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "Squad update failed validation.", errors)
			return
		}
		if a.Repository != nil {
			var saveErr error
			if a.Auth != nil {
				if repository, ok := a.Repository.(UserSeasonSquadRepository); ok {
					saveErr = repository.SaveSquadForUserSeason(r.Context(), session.User.ID, season.ID, input)
				} else {
					saveErr = fmt.Errorf("authenticated squad repository is unavailable")
				}
			} else if repository, ok := a.Repository.(SeasonSquadRepository); ok {
				saveErr = repository.SaveSquadForSeason(r.Context(), season.ID, input)
			} else {
				saveErr = a.Repository.SaveSquad(r.Context(), input)
			}
			if saveErr != nil {
				writeError(w, http.StatusInternalServerError, "persistence_failed", "Squad could not be saved to the database.", nil)
				return
			}
		}
		if a.Auth == nil {
			a.Store.SaveSquad(input)
		}
		scope := Scope{SeasonID: season.ID, Dataset: "public-fpl"}
		freshness := a.requestFreshness(r.Context(), scope)
		writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, domain.EnrichSquad(input))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or PUT for the squad.", nil)
	}
}
func (a *API) recommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST for recommendations.", nil)
		return
	}
	session, authenticated := a.requireMutationAuth(w, r)
	if !authenticated {
		return
	}
	var body struct {
		Weights Weights `json:"weights"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	season, warnings, err := a.resolveSeason(r.Context(), parseInt(r.URL.Query().Get("seasonId"), 0))
	if err != nil {
		writeSeasonResolutionError(w, w.Header().Get("X-Request-ID"), err)
		return
	}
	domain, err := a.requestDomainStoreForUser(r.Context(), season.ID, session.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recommendation_unavailable", "Recommendation data is temporarily unavailable.", nil)
		return
	}
	recommendation, errors := domain.Recommend(domain.GetSquad(), body.Weights)
	if len(errors) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "recommendation_failed", "Recommendation could not be generated.", errors)
		return
	}
	scope := Scope{SeasonID: season.ID, Dataset: "public-fpl"}
	freshness := a.requestFreshness(r.Context(), scope)
	writeEnvelopeWithWarnings(w, http.StatusOK, w.Header().Get("X-Request-ID"), scope, freshness, warnings, map[string]interface{}{"recommendation": recommendation, "freshness": freshness})
}

func (a *API) requestDomainStore(ctx context.Context, seasonID int) (*Store, error) {
	return a.requestDomainStoreForOwner(ctx, seasonID, nil)
}

func (a *API) requestDomainStoreForUser(ctx context.Context, seasonID int, userID int64) (*Store, error) {
	if a.Auth == nil {
		return a.requestDomainStore(ctx, seasonID)
	}
	if userID <= 0 {
		return nil, fmt.Errorf("authenticated workspace owner is required")
	}
	return a.requestDomainStoreForOwner(ctx, seasonID, &userID)
}

func (a *API) requestDomainStoreForOwner(ctx context.Context, seasonID int, userID *int64) (*Store, error) {
	if a.Repository == nil {
		return a.Store, nil
	}
	var snapshot Snapshot
	var found bool
	var err error
	if repository, ok := a.Repository.(SeasonCatalogueRepository); ok {
		snapshot, found, err = repository.LoadSnapshotForSeason(ctx, seasonID)
	} else {
		snapshot, found, err = a.Repository.LoadSnapshot(ctx)
	}
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no persisted public snapshot is available")
	}
	domain := NewWarehouseCache()
	domain.ApplySnapshot(snapshot.Season, snapshot.Gameweeks, snapshot.Teams, snapshot.Players, snapshot.Fixtures, snapshot.Histories)
	var squad Squad
	if userID != nil {
		if repository, ok := a.Repository.(UserSeasonSquadRepository); ok {
			squad, found, err = repository.LoadSquadForUserSeason(ctx, *userID, seasonID)
		} else {
			return nil, fmt.Errorf("authenticated squad repository is unavailable")
		}
	} else if repository, ok := a.Repository.(SeasonSquadRepository); ok {
		squad, found, err = repository.LoadSquadForSeason(ctx, seasonID)
	} else {
		squad, found, err = a.Repository.LoadSquad(ctx)
	}
	if err != nil {
		return nil, err
	} else if found {
		domain.SaveSquad(squad)
	}
	return domain, nil
}
