package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
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
			status.Warning = fmt.Sprintf("%s:%d: %v", scope.Type, scope.SourceID, syncErr)
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
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[0].NaturalKey, "retryable", err.Error())
		return err
	}
	if err = s.Repository.PersistEntry(ctx, userID, seasonID, entry, checksum, fetched); err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[0].NaturalKey, "retryable", err.Error())
		return err
	}
	_ = s.Repository.UpdateManagerWork(ctx, runID, work[0].NaturalKey, "completed", "")
	history, checksum, fetched, err := s.Source.History(ctx, entryID)
	if err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[1].NaturalKey, "retryable", err.Error())
		return err
	}
	if err = s.Repository.PersistHistory(ctx, userID, seasonID, entryID, history, checksum, fetched); err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[1].NaturalKey, "retryable", err.Error())
		return err
	}
	_ = s.Repository.UpdateManagerWork(ctx, runID, work[1].NaturalKey, "completed", "")
	transfers, checksum, _, err := s.Source.Transfers(ctx, entryID)
	if err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[2].NaturalKey, "retryable", err.Error())
		return err
	}
	if err = s.Repository.PersistTransfers(ctx, userID, seasonID, entryID, transfers, checksum); err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[2].NaturalKey, "retryable", err.Error())
		return err
	}
	_ = s.Repository.UpdateManagerWork(ctx, runID, work[2].NaturalKey, "completed", "")
	picks, picksChecksum, fetched, picksErr := s.Source.Picks(ctx, entryID, gameweek)
	publicPicksAvailable := picksErr == nil
	if picksErr != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[3].NaturalKey, "retryable", picksErr.Error())
	} else if _, err = s.Repository.PersistPicks(ctx, userID, seasonID, entryID, gameweek, picks, picksChecksum, fetched, fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek)); err != nil {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[3].NaturalKey, "retryable", err.Error())
		return err
	} else {
		_ = s.Repository.UpdateManagerWork(ctx, runID, work[3].NaturalKey, "completed", "")
	}
	if s.Sessions == nil {
		return picksErr
	}
	session, err := s.Sessions.Get(ctx, userID, entryID)
	if errors.Is(err, ErrRemoteSessionMissing) {
		return picksErr
	}
	if err != nil {
		_ = s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteReauthRequired)
		return fmt.Errorf("private active-team session requires reauthentication")
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
			return fmt.Errorf("private active-team sync failed (%s)", sourceErr.Code)
		}
		return err
	}
	if !publicPicksAvailable {
		picks = sourcePicksFromPrivate(private, gameweek)
	}
	combined := sha256.Sum256([]byte(privateChecksum + picksChecksum))
	if _, err = s.Repository.PersistActiveTeam(ctx, userID, seasonID, entryID, gameweek, private, picks, publicPicksAvailable, fmt.Sprintf("%x", combined), privateFetched); err != nil {
		return err
	}
	if err = s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteConnected); err != nil {
		return err
	}
	return picksErr
}

func sourcePicksFromPrivate(team sourceMyTeam, gameweek int) sourcePicks {
	result := sourcePicks{}
	result.EntryHistory.Event = gameweek
	result.Picks = append(result.Picks, team.Picks...)
	for _, chip := range team.Chips {
		if chip.Status == "active" || chip.Status == "played" {
			result.ActiveChip = chip.Name
			break
		}
	}
	return result
}

func (s *ManagerService) syncLeague(ctx context.Context, userID int64, seasonID, gameweek int, scope ManagerScope, runID int64) error {
	max := s.MaxLeaguePages
	if max < 1 {
		max = 100
	}
	allMembers := []LeagueMember{}
	for page := 1; page <= max; page++ {
		key := fmt.Sprintf("league:%d:gw:%d:page:%d", scope.SourceID, gameweek, page)
		endpoint := fmt.Sprintf("/leagues-classic/%d/standings/?page_standings=%d&phase=1", scope.SourceID, page)
		if err := s.Repository.EnqueueManagerWork(ctx, runID, []ManagerWorkItem{{NaturalKey: key, Stage: "league-standings", Endpoint: endpoint, Checkpoint: map[string]any{"page": page}}}); err != nil {
			return err
		}
		value, checksum, fetched, err := s.Source.League(ctx, scope.SourceID, page, 1)
		if err != nil {
			_ = s.Repository.UpdateManagerWork(ctx, runID, key, "retryable", err.Error())
			return err
		}
		if err = s.Repository.PersistLeaguePage(ctx, userID, seasonID, gameweek, 1, page, value, checksum, fetched); err != nil {
			_ = s.Repository.UpdateManagerWork(ctx, runID, key, "retryable", err.Error())
			return err
		}
		_ = s.Repository.UpdateManagerWork(ctx, runID, key, "completed", "")
		for _, member := range value.Standings.Results {
			allMembers = append(allMembers, LeagueMember{EntryID: member.Entry, EntryName: member.EntryName, PlayerName: member.PlayerName, Rank: member.Rank, LastRank: member.LastRank, Points: member.Total})
		}
		if !value.Standings.HasNext {
			break
		}
		if page == max {
			return fmt.Errorf("league page limit reached")
		}
	}
	selected, _ := SelectLeagueMembers(allMembers, nil, 0, 0, scope.MemberLimit)
	sem := make(chan struct{}, 4)
	var wait sync.WaitGroup
	var failureMu sync.Mutex
	failures := []error{}
	recordFailure := func(err error) {
		failureMu.Lock()
		failures = append(failures, err)
		failureMu.Unlock()
	}
	for _, entryID := range selected {
		entryID := entryID
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				recordFailure(ctx.Err())
				return
			}
			key := fmt.Sprintf("league:%d:gw:%d:entry:%d:picks", scope.SourceID, gameweek, entryID)
			endpoint := fmt.Sprintf("/entry/%d/event/%d/picks/", entryID, gameweek)
			if err := s.Repository.EnqueueManagerWork(ctx, runID, []ManagerWorkItem{{NaturalKey: key, Stage: "league-member-picks", Endpoint: endpoint, Checkpoint: map[string]any{"entryId": entryID, "gameweek": gameweek}}}); err != nil {
				recordFailure(err)
				return
			}
			value, checksum, fetched, err := s.Source.Picks(ctx, entryID, gameweek)
			if err == nil {
				var snapshot int64
				snapshot, err = s.Repository.PersistPicks(ctx, userID, seasonID, entryID, gameweek, value, checksum, fetched, endpoint)
				if err == nil {
					err = s.Repository.LinkLeagueMemberPick(ctx, userID, seasonID, scope.SourceID, gameweek, entryID, snapshot, "completed", "")
				}
			}
			if err != nil {
				if linkErr := s.Repository.LinkLeagueMemberPick(ctx, userID, seasonID, scope.SourceID, gameweek, entryID, 0, "failed", err.Error()); linkErr != nil {
					err = errors.Join(err, linkErr)
				}
				_ = s.Repository.UpdateManagerWork(ctx, runID, key, "retryable", err.Error())
				recordFailure(err)
			} else {
				if err = s.Repository.UpdateManagerWork(ctx, runID, key, "completed", ""); err != nil {
					recordFailure(err)
				}
			}
		}()
	}
	wait.Wait()
	if len(failures) > 0 {
		return fmt.Errorf("%d league member pick requests failed", len(failures))
	}
	return nil
}

func (s *ManagerService) Connect(ctx context.Context, userID int64, entryID int, cookie string) error {
	if s.Sessions == nil {
		return ErrRemoteSessionMissing
	}
	if err := s.Sessions.Put(ctx, userID, entryID, RemoteSession{Cookie: cookie}); err != nil {
		return err
	}
	identity, _, _, err := s.Source.Me(ctx, RemoteSession{Cookie: cookie})
	if err != nil || sourceMeEntry(identity) != entryID {
		_ = s.Sessions.Revoke(ctx, userID, entryID)
		if err != nil {
			return err
		}
		return fmt.Errorf("remote FPL session belongs to a different entry")
	}
	return s.Repository.UpsertManagerConnection(ctx, userID, entryID, "memory", RemoteConnected)
}
func (s *ManagerService) Disconnect(ctx context.Context, userID int64, entryID int) error {
	if s.Sessions != nil {
		_ = s.Sessions.Revoke(ctx, userID, entryID)
	}
	return s.Repository.SetManagerConnectionState(ctx, userID, entryID, RemoteRevoked)
}

func (s *ManagerService) PreviewImport(ctx context.Context, userID int64, seasonID, entryID, gameweek int, domain *Store) (SquadImportPreview, bool, error) {
	snapshot, found, err := s.Repository.LoadActiveTeam(ctx, userID, seasonID, entryID, gameweek)
	if err != nil || !found {
		return SquadImportPreview{}, found, err
	}
	current := domain.GetSquad()
	preview, err := BuildImportPreview(snapshot, current, domain)
	return preview, true, err
}

func (s *ManagerService) Import(ctx context.Context, userID int64, seasonID, entryID, gameweek int, snapshotID int64, mode string, confirmed bool, domain *Store) (SquadImportResult, error) {
	preview, found, err := s.PreviewImport(ctx, userID, seasonID, entryID, gameweek, domain)
	if err != nil {
		return SquadImportResult{}, err
	}
	if !found || preview.Snapshot.SnapshotID != snapshotID {
		return SquadImportResult{}, fmt.Errorf("import snapshot is not the latest owned snapshot")
	}
	if len(preview.Validation) > 0 {
		return SquadImportResult{}, fmt.Errorf("import snapshot fails planning validation")
	}
	if mode == "replace" && !confirmed {
		return SquadImportResult{}, fmt.Errorf("replace confirmation is required")
	}
	return s.Repository.ImportActiveTeam(ctx, userID, seasonID, snapshotID, mode, preview.Proposed)
}

func (s *ManagerService) CompareLeague(ctx context.Context, userID int64, seasonID, leagueID, gameweek int, explicit []int, rankFrom, rankTo, limit int) (LeagueComparisonResult, error) {
	members, err := s.Repository.LoadLeagueMembers(ctx, userID, seasonID, leagueID, gameweek)
	if err != nil {
		return LeagueComparisonResult{}, err
	}
	selected, omitted := SelectLeagueMembers(members, explicit, rankFrom, rankTo, limit)
	result := LeagueComparisonResult{
		LeagueID:        leagueID,
		SeasonID:        seasonID,
		Gameweek:        gameweek,
		SelectedIDs:     nonNilInts(selected),
		OmittedIDs:      nonNilInts(omitted),
		Comparisons:     []TeamComparison{},
		MissingInputs:   []string{},
		FailedMembers:   []int{},
		Ownership:       []PlayerOwnership{},
		SnapshotIDs:     map[int]string{},
		FormulaVersions: []string{"team-overlap-v1", "differential-contribution-v1", "team-points-difference-v1", "captain-delta-v1", "transfer-cost-v1"},
		Warnings:        []string{},
	}
	result.Coverage = AnalysisCoverage{Requested: len(selected), Selected: len(selected), Omitted: len(omitted), MissingEntryIDs: []int{}}
	if len(explicit) > 0 {
		available := map[int]bool{}
		for _, member := range members {
			available[member.EntryID] = true
		}
		for _, entryID := range explicit {
			if !available[entryID] {
				result.Coverage.MissingEntryIDs = append(result.Coverage.MissingEntryIDs, entryID)
				result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:not-in-league-snapshot", entryID))
			}
		}
		result.Coverage.Requested = len(explicit)
	}
	result.FailedMembers, err = s.Repository.LoadLeagueMemberFailures(ctx, userID, seasonID, leagueID, gameweek)
	if err != nil {
		return result, err
	}
	for _, entryID := range result.FailedMembers {
		result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:pick-sync-failed", entryID))
	}
	if len(selected) < 2 {
		result.MissingInputs = append(result.MissingInputs, "at-least-two-member-pick-snapshots")
		result.OutcomeState = "unavailable"
		return result, nil
	}
	points, state, err := s.Repository.LoadPlayerGameweekPoints(ctx, seasonID, gameweek)
	if err != nil {
		return result, err
	}
	result.OutcomeState = state
	if state == "estimated" {
		result.AlgorithmVersion = "league-points-estimate-v1"
		result.OutcomeState = "unavailable"
		result.MissingInputs = append(result.MissingInputs, "final-or-provisional-player-gameweek-points")
		result.Warnings = append(result.Warnings, "League comparisons are unavailable until player gameweek points have been synchronized.")
		return result, nil
	}
	analyses := []ManagerDecisionAnalysis{}
	ownership := map[int]int{}
	for _, entryID := range selected {
		analysis, pickErr := s.Repository.LoadManagerDecisionAnalysis(ctx, userID, seasonID, entryID, gameweek)
		if pickErr != nil {
			return result, pickErr
		}
		if len(analysis.Picks) == 0 {
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:picks", entryID))
			result.Coverage.MissingEntryIDs = append(result.Coverage.MissingEntryIDs, entryID)
			continue
		}
		compatible := true
		for _, pick := range analysis.Picks {
			if _, found := points[pick.PlayerID]; !found {
				result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:player:%d:gameweek-points", entryID, pick.PlayerID))
				compatible = false
			}
		}
		if !compatible {
			result.Coverage.MissingEntryIDs = append(result.Coverage.MissingEntryIDs, entryID)
			continue
		}
		analyses = append(analyses, analysis)
		result.Coverage.Complete++
		result.SnapshotIDs[entryID] = analysis.SnapshotID
		if analysis.SourceFetchedAt.After(result.SourceSnapshotAt) {
			result.SourceSnapshotAt = analysis.SourceFetchedAt
		}
		for _, pick := range analysis.Picks {
			ownership[pick.PlayerID]++
		}
	}
	if result.Coverage.Complete < result.Coverage.Selected {
		result.OutcomeState = ResolveAnalysisOutcome(state, true, false)
		result.Warnings = append(result.Warnings, "Some selected teams have no compatible pick snapshot and were excluded from comparisons.")
	}
	for i := 0; i < len(analyses); i++ {
		for j := i + 1; j < len(analyses); j++ {
			left, right := CompareTeamsWithCosts(analyses[i].Picks, analyses[j].Picks, points, analyses[i].TransferCost, analyses[j].TransferCost, state)
			left.EntryID, left.OpponentEntryID = analyses[i].EntryID, analyses[j].EntryID
			right.EntryID, right.OpponentEntryID = analyses[j].EntryID, analyses[i].EntryID
			result.Comparisons = append(result.Comparisons, left, right)
		}
	}
	for playerID, count := range ownership {
		result.Ownership = append(result.Ownership, PlayerOwnership{PlayerID: playerID, Count: count, Rate: float64(count) / float64(len(analyses))})
	}
	sort.Slice(result.Ownership, func(i, j int) bool { return result.Ownership[i].PlayerID < result.Ownership[j].PlayerID })
	return result, nil
}

func (s *ManagerService) LeagueSummary(ctx context.Context, userID int64, seasonID, leagueID, gameweek, limit int) (LeagueSummary, error) {
	members, err := s.Repository.LoadLeagueMembers(ctx, userID, seasonID, leagueID, gameweek)
	if err != nil {
		return LeagueSummary{}, err
	}
	selected, omitted := SelectLeagueMembers(members, nil, 0, 0, limit)
	result := LeagueSummary{LeagueID: leagueID, SeasonID: seasonID, Gameweek: gameweek, Members: members, SelectedIDs: nonNilInts(selected), OmittedIDs: nonNilInts(omitted), SnapshotIDs: map[int]string{}, FormulaVersions: []string{"team-overlap-v1", "differential-contribution-v1", "team-points-difference-v1"}, MissingInputs: []string{}, Warnings: []string{}, OutcomeState: "actual"}
	standingsFound := false
	if standings, found, standingsErr := s.Repository.LoadLeagueStandings(ctx, userID, seasonID, leagueID, gameweek, 1); standingsErr != nil {
		return result, standingsErr
	} else if found {
		standingsFound = true
		result.Name = standings.Name
	}
	result.Coverage = AnalysisCoverage{Requested: len(selected), Selected: len(selected), Omitted: len(omitted), MissingEntryIDs: []int{}}
	if len(members) == 0 {
		if standingsFound {
			result.Warnings = append(result.Warnings, "The synchronized league snapshot has no ranked members yet.")
		} else {
			result.OutcomeState = "unavailable"
			result.MissingInputs = append(result.MissingInputs, "league-standings-snapshot")
		}
		return result, nil
	}
	for _, entryID := range selected {
		analysis, loadErr := s.Repository.LoadManagerDecisionAnalysis(ctx, userID, seasonID, entryID, gameweek)
		if loadErr != nil {
			return result, loadErr
		}
		if len(analysis.Picks) == 0 {
			result.Coverage.MissingEntryIDs = append(result.Coverage.MissingEntryIDs, entryID)
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:picks", entryID))
			continue
		}
		if analysis.OutcomeState == "estimated" {
			result.Coverage.MissingEntryIDs = append(result.Coverage.MissingEntryIDs, entryID)
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:player-gameweek-points", entryID))
			result.OutcomeState = "unavailable"
			continue
		}
		result.Coverage.Complete++
		result.SnapshotIDs[entryID] = analysis.SnapshotID
		if analysis.OutcomeState == "provisional" {
			result.OutcomeState = "provisional"
		}
	}
	if result.Coverage.Complete < result.Coverage.Selected {
		result.OutcomeState = ResolveAnalysisOutcome(result.OutcomeState, true, false)
		result.Warnings = append(result.Warnings, "Coverage excludes members without a compatible pick snapshot.")
	}
	return result, nil
}

func (s *ManagerService) GameweekAutopsy(ctx context.Context, userID int64, seasonID, entryID, rivalEntryID, gameweek int) (GameweekAutopsy, error) {
	analysis, err := s.Repository.LoadManagerDecisionAnalysis(ctx, userID, seasonID, entryID, gameweek)
	if err != nil {
		return GameweekAutopsy{}, err
	}
	result := GameweekAutopsy{EntryID: entryID, RivalEntryID: rivalEntryID, SeasonID: seasonID, Gameweek: gameweek, GrossPoints: analysis.GrossPoints, TransferCost: analysis.TransferCost, NetPoints: analysis.NetPoints, BenchPoints: analysis.BenchPoints, ActiveChip: analysis.ActiveChip, Transfers: []ManagerTransfer{}, AutomaticSubstitutions: []AutomaticSubstitution{}, Contributions: []PlayerContribution{}, OutcomeState: analysis.OutcomeState, SnapshotIDs: map[int]string{}, FormulaVersions: []string{"player-points-v1", "team-points-v1", "captain-delta-v1", "transfer-cost-v1", "automatic-substitution-impact-v1"}, MissingInputs: []string{}, Warnings: []string{}, UnfinishedFixtureIDs: []int{}}
	if len(analysis.Picks) == 0 {
		result.OutcomeState = "unavailable"
		result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:picks", entryID))
		return result, nil
	}
	result.SnapshotIDs[entryID] = analysis.SnapshotID
	points, pointsState, err := s.Repository.LoadPlayerGameweekPoints(ctx, seasonID, gameweek)
	if err != nil {
		return result, err
	}
	if pointsState == "estimated" {
		result.OutcomeState = "unavailable"
		result.GrossPoints, result.NetPoints, result.CaptainDelta = 0, 0, 0
		result.MissingInputs = append(result.MissingInputs, "final-or-provisional-player-gameweek-points")
		result.Warnings = append(result.Warnings, "The autopsy is unavailable until gameweek points have been synchronized.")
		return result, nil
	}
	result.MetricsAvailable = true
	if pointsState == "provisional" {
		result.UnfinishedFixtureIDs, err = s.Repository.LoadUnfinishedFixtureIDs(ctx, seasonID, gameweek)
		if err != nil {
			return result, err
		}
	}
	if analysis.ActiveChip != "" && analysis.ActiveChip != "bboost" && analysis.ActiveChip != "3xc" && analysis.ActiveChip != "freehit" && analysis.ActiveChip != "wildcard" {
		result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("chip:%s:unsupported", analysis.ActiveChip))
	}
	for _, pick := range analysis.Picks {
		base, found := points[pick.PlayerID]
		if !found {
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("player:%d:gameweek-points", pick.PlayerID))
			continue
		}
		result.Contributions = append(result.Contributions, PlayerContribution{PlayerID: pick.PlayerID, BasePoints: base, Multiplier: pick.Multiplier, EffectivePoints: base * pick.Multiplier})
		if pick.Captain {
			result.CaptainID = pick.PlayerID
			result.CaptainDelta = base * (pick.Multiplier - 1)
		}
	}
	transfers, err := s.Repository.LoadManagerTransfers(ctx, userID, seasonID, entryID)
	if err != nil {
		return result, err
	}
	for _, transfer := range transfers {
		if transfer.Gameweek == gameweek {
			result.Transfers = append(result.Transfers, transfer)
		}
	}
	result.AutomaticSubstitutions, err = s.Repository.LoadAutomaticSubstitutions(ctx, userID, seasonID, entryID, gameweek)
	if err != nil {
		return result, err
	}
	for index := range result.AutomaticSubstitutions {
		item := &result.AutomaticSubstitutions[index]
		impact, found := AutomaticSubstitutionImpact(*item, points)
		if found {
			item.Impact = impact
		} else {
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("automatic-substitution:%d:%d:points", item.PlayerIn, item.PlayerOut))
		}
	}
	if rivalEntryID > 0 {
		rival, rivalErr := s.Repository.LoadManagerDecisionAnalysis(ctx, userID, seasonID, rivalEntryID, gameweek)
		if rivalErr != nil {
			return result, rivalErr
		}
		if len(rival.Picks) == 0 {
			result.MissingInputs = append(result.MissingInputs, fmt.Sprintf("entry:%d:picks", rivalEntryID))
		} else {
			left, _ := CompareTeamsWithCosts(analysis.Picks, rival.Picks, points, analysis.TransferCost, rival.TransferCost, pointsState)
			left.EntryID, left.OpponentEntryID = entryID, rivalEntryID
			result.RivalComparison = &left
			result.SnapshotIDs[rivalEntryID] = rival.SnapshotID
			result.FormulaVersions = append(result.FormulaVersions, "team-overlap-v1", "differential-contribution-v1", "team-points-difference-v1")
		}
	}
	if len(result.MissingInputs) > 0 {
		result.OutcomeState = ResolveAnalysisOutcome(pointsState, true, false)
		result.Warnings = append(result.Warnings, "Some inputs are missing; displayed totals exclude unavailable facts.")
	}
	if pointsState == "provisional" {
		result.Warnings = append(result.Warnings, "The gameweek is live, so points and comparisons may change.")
	}
	return result, nil
}

func nonNilInts(values []int) []int {
	if values == nil {
		return []int{}
	}
	return values
}
