package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"
)

type ManagerSyncStatus struct {
	Status        string    `json:"status"`
	RunID         int64     `json:"runId,omitempty"`
	CompletedWork int       `json:"completedWork"`
	FailedWork    int       `json:"failedWork"`
	Warning       string    `json:"warning,omitempty"`
	Freshness     Freshness `json:"freshness"`
}

type ManagerService struct {
	Repository     ManagerDataRepository
	Source         *ManagerSource
	Sessions       RemoteSessionProvider
	MaxLeaguePages int
	mu             sync.RWMutex
	status         map[int64]ManagerSyncStatus
}

func NewManagerService(repository ManagerDataRepository, source *ManagerSource, sessions RemoteSessionProvider) *ManagerService {
	return &ManagerService{Repository: repository, Source: source, Sessions: sessions, MaxLeaguePages: 100, status: map[int64]ManagerSyncStatus{}}
}

func (s *ManagerService) Status(userID int64) ManagerSyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, ok := s.status[userID]
	if !ok {
		return ManagerSyncStatus{Status: "idle", Freshness: Freshness{Dataset: "manager-fpl", State: "unavailable", Status: "unavailable", MissingInputs: []string{"manager-sync"}}}
	}
	return status
}
func (s *ManagerService) setStatus(userID int64, status ManagerSyncStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[userID] = status
}

func (s *ManagerService) Sync(ctx context.Context, userID int64, seasonID, gameweek int, correlationID string) (ManagerSyncStatus, error) {
	if userID <= 0 || seasonID <= 0 || gameweek <= 0 {
		return ManagerSyncStatus{}, fmt.Errorf("owner, seasonId, and gameweek are required")
	}
	runID, err := s.Repository.StartManagerRun(ctx, userID, correlationID)
	if err != nil {
		return ManagerSyncStatus{}, err
	}
	status := ManagerSyncStatus{Status: "running", RunID: runID, Freshness: Freshness{Dataset: "manager-fpl", State: "syncing", Status: "syncing"}}
	s.setStatus(userID, status)
	scopes, err := s.Repository.ListManagerScopes(ctx, userID)
	if err != nil {
		return s.finish(ctx, userID, status, err)
	}
	if len(scopes) == 0 {
		return s.finish(ctx, userID, status, fmt.Errorf("no enabled manager or league scopes are configured"))
	}
	for _, scope := range scopes {
		if !scope.Enabled {
			continue
		}
		var syncErr error
		switch scope.Type {
		case "entry":
			syncErr = s.syncEntry(ctx, userID, seasonID, gameweek, scope.SourceID, runID)
		case "league":
			syncErr = s.syncLeague(ctx, userID, seasonID, gameweek, scope, runID)
		}
		if syncErr != nil {
			status.FailedWork++
			status.Warning = syncErr.Error()
		} else {
			status.CompletedWork++
		}
	}
	if status.CompletedWork == 0 && status.FailedWork > 0 {
		return s.finish(ctx, userID, status, errors.New(status.Warning))
	}
	return s.finish(ctx, userID, status, nil)
}

func (s *ManagerService) finish(ctx context.Context, userID int64, status ManagerSyncStatus, syncErr error) (ManagerSyncStatus, error) {
	status.Status = "success"
	status.Freshness = Freshness{Dataset: "manager-fpl", State: "fresh", Status: "fresh", LastSuccessfulSync: time.Now().UTC(), NormalizedAt: time.Now().UTC(), NormalizerVersion: ManagerNormalizationVersion}
	if syncErr != nil || status.FailedWork > 0 {
		status.Status = "partial"
		status.Freshness.State = "partial"
		status.Freshness.Status = "partial"
		if status.Warning == "" && syncErr != nil {
			status.Warning = syncErr.Error()
		}
		status.Freshness.Warning = status.Warning
	}
	_ = s.Repository.FinishManagerRun(ctx, status.RunID, status.Status, status.Warning)
	s.setStatus(userID, status)
	return status, syncErr
}

func (s *ManagerService) syncEntry(ctx context.Context, userID int64, seasonID, gameweek, entryID int, runID int64) error {
	work := []ManagerWorkItem{{NaturalKey: fmt.Sprintf("entry:%d", entryID), Stage: "entry", Endpoint: fmt.Sprintf("/entry/%d/", entryID)}, {NaturalKey: fmt.Sprintf("history:%d", entryID), Stage: "history", Endpoint: fmt.Sprintf("/entry/%d/history/", entryID)}, {NaturalKey: fmt.Sprintf("transfers:%d", entryID), Stage: "transfers", Endpoint: fmt.Sprintf("/entry/%d/transfers/", entryID)}, {NaturalKey: fmt.Sprintf("picks:%d:%d", entryID, gameweek), Stage: "picks", Endpoint: fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek), Checkpoint: map[string]any{"gameweek": gameweek}}}
	if err := s.Repository.EnqueueManagerWork(ctx, runID, work); err != nil {
		return err
	}
	entry, checksum, fetched, err := s.Source.Entry(ctx, entryID)
	if err != nil {
		return err
	}
	if err = s.Repository.PersistEntry(ctx, userID, seasonID, entry, checksum, fetched); err != nil {
		return err
	}
	history, checksum, fetched, err := s.Source.History(ctx, entryID)
	if err != nil {
		return err
	}
	if err = s.Repository.PersistHistory(ctx, userID, seasonID, entryID, history, checksum, fetched); err != nil {
		return err
	}
	transfers, checksum, _, err := s.Source.Transfers(ctx, entryID)
	if err != nil {
		return err
	}
	if err = s.Repository.PersistTransfers(ctx, userID, seasonID, entryID, transfers, checksum); err != nil {
		return err
	}
	picks, checksum, fetched, err := s.Source.Picks(ctx, entryID, gameweek)
	if err != nil {
		return err
	}
	if _, err = s.Repository.PersistPicks(ctx, userID, seasonID, entryID, gameweek, picks, checksum, fetched, fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek)); err != nil {
		return err
	}
	if s.Sessions == nil {
		return nil
	}
	session, err := s.Sessions.Get(ctx, userID, entryID)
	if errors.Is(err, ErrRemoteSessionMissing) {
		return nil
	}
	if err != nil {
		_ = s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteReauthRequired)
		return nil
	}
	private, privateChecksum, privateFetched, err := s.Source.MyTeam(ctx, entryID, session)
	if err != nil {
		var sourceErr ManagerSourceError
		if errors.As(err, &sourceErr) {
			state := RemoteReauthRequired
			if sourceErr.Code == "permission_denied" {
				state = RemotePermissionDenied
			}
			_ = s.Repository.SetManagerConnectionState(ctx, userID, entryID, state)
			return nil
		}
		return err
	}
	combined := sha256.Sum256([]byte(privateChecksum + checksum))
	if _, err = s.Repository.PersistActiveTeam(ctx, userID, seasonID, entryID, gameweek, private, picks, fmt.Sprintf("%x", combined), privateFetched); err != nil {
		return err
	}
	return s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteConnected)
}

func (s *ManagerService) syncLeague(ctx context.Context, userID int64, seasonID, gameweek int, scope ManagerScope, runID int64) error {
	max := s.MaxLeaguePages
	if max < 1 {
		max = 100
	}
	for page := 1; page <= max; page++ {
		endpoint := fmt.Sprintf("/leagues-classic/%d/standings/?page_standings=%d&phase=1", scope.SourceID, page)
		if err := s.Repository.EnqueueManagerWork(ctx, runID, []ManagerWorkItem{{NaturalKey: fmt.Sprintf("league:%d:gw:%d:page:%d", scope.SourceID, gameweek, page), Stage: "league-standings", Endpoint: endpoint, Checkpoint: map[string]any{"page": page}}}); err != nil {
			return err
		}
		value, checksum, fetched, err := s.Source.League(ctx, scope.SourceID, page, 1)
		if err != nil {
			return err
		}
		if err = s.Repository.PersistLeaguePage(ctx, userID, seasonID, gameweek, 1, page, value, checksum, fetched); err != nil {
			return err
		}
		if !value.Standings.HasNext {
			return nil
		}
	}
	return fmt.Errorf("league page limit reached")
}

func (s *ManagerService) Connect(ctx context.Context, userID int64, entryID int, cookie string) error {
	if s.Sessions == nil {
		return ErrRemoteSessionMissing
	}
	if err := s.Sessions.Put(ctx, userID, entryID, RemoteSession{Cookie: cookie}); err != nil {
		return err
	}
	if _, _, _, err := s.Source.Me(ctx, RemoteSession{Cookie: cookie}); err != nil {
		_ = s.Sessions.Revoke(ctx, userID, entryID)
		return err
	}
	return s.Repository.UpsertManagerConnection(ctx, userID, entryID, "memory", RemoteConnected)
}
func (s *ManagerService) Disconnect(ctx context.Context, userID int64, entryID int) error {
	if s.Sessions != nil {
		_ = s.Sessions.Revoke(ctx, userID, entryID)
	}
	return s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteRevoked)
}
