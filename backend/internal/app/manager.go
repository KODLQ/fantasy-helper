package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

const ManagerNormalizationVersion = "fpl-manager-v1"

var (
	ErrRemoteSessionMissing   = errors.New("remote FPL session is not configured")
	ErrRemoteReauthRequired   = errors.New("remote FPL session requires reauthentication")
	ErrRemotePermissionDenied = errors.New("remote FPL session lacks permission")
)

type RemoteSessionState string

const (
	RemoteDisconnected     RemoteSessionState = "disconnected"
	RemoteConnected        RemoteSessionState = "connected"
	RemoteReauthRequired   RemoteSessionState = "reauth_required"
	RemotePermissionDenied RemoteSessionState = "permission_denied"
	RemoteRevoked          RemoteSessionState = "revoked"
)

type RemoteSession struct {
	Cookie    string
	ExpiresAt time.Time
}

type RemoteSessionProvider interface {
	Get(context.Context, int64, int) (RemoteSession, error)
	Put(context.Context, int64, int, RemoteSession) error
	Revoke(context.Context, int64, int) error
}

// MemorySessionProvider is process-local: session material never enters the
// database, API response, browser storage, or source observation payload.
type MemorySessionProvider struct {
	mu       sync.RWMutex
	sessions map[string]RemoteSession
}

func NewMemorySessionProvider() *MemorySessionProvider {
	return &MemorySessionProvider{sessions: map[string]RemoteSession{}}
}

func remoteSessionKey(userID int64, entryID int) string { return fmt.Sprintf("%d:%d", userID, entryID) }

func (p *MemorySessionProvider) Get(_ context.Context, userID int64, entryID int) (RemoteSession, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	session, found := p.sessions[remoteSessionKey(userID, entryID)]
	if !found || session.Cookie == "" {
		return RemoteSession{}, ErrRemoteSessionMissing
	}
	if !session.ExpiresAt.IsZero() && !time.Now().UTC().Before(session.ExpiresAt) {
		return RemoteSession{}, ErrRemoteReauthRequired
	}
	return session, nil
}

func (p *MemorySessionProvider) Put(_ context.Context, userID int64, entryID int, session RemoteSession) error {
	if userID <= 0 || entryID <= 0 || session.Cookie == "" {
		return fmt.Errorf("owner, entry, and session cookie are required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[remoteSessionKey(userID, entryID)] = session
	return nil
}

func (p *MemorySessionProvider) Revoke(_ context.Context, userID int64, entryID int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sessions, remoteSessionKey(userID, entryID))
	return nil
}

// EnvironmentSessionProvider resolves a cookie only at request time. The
// variable name is safe metadata; the value is never retained by this type.
type EnvironmentSessionProvider struct{ Variable string }

func (p EnvironmentSessionProvider) Get(_ context.Context, _ int64, _ int) (RemoteSession, error) {
	value := os.Getenv(p.Variable)
	if value == "" {
		return RemoteSession{}, ErrRemoteSessionMissing
	}
	return RemoteSession{Cookie: value}, nil
}
func (EnvironmentSessionProvider) Put(context.Context, int64, int, RemoteSession) error {
	return errors.New("environment session provider is read-only")
}
func (EnvironmentSessionProvider) Revoke(context.Context, int64, int) error { return nil }

func SecretFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func ConstantSecretEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

type ManagerScope struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	SourceID    int    `json:"sourceId"`
	Enabled     bool   `json:"enabled"`
	MemberLimit int    `json:"memberLimit"`
}

type ManagerEntry struct {
	EntryID         int       `json:"entryId"`
	SeasonID        int       `json:"seasonId"`
	EntryName       string    `json:"entryName"`
	PlayerFirstName string    `json:"playerFirstName"`
	PlayerLastName  string    `json:"playerLastName"`
	OverallPoints   int       `json:"overallPoints"`
	OverallRank     int       `json:"overallRank"`
	TeamValue       int       `json:"teamValue"`
	Bank            int       `json:"bank"`
	SourceFetchedAt time.Time `json:"sourceFetchedAt"`
	NormalizedAt    time.Time `json:"normalizedAt"`
	SnapshotID      string    `json:"snapshotId"`
	State           string    `json:"state"`
	MissingInputs   []string  `json:"missingInputs"`
	Warning         string    `json:"warning,omitempty"`
}

type ManagerPick struct {
	PlayerID    int  `json:"playerId"`
	Position    int  `json:"position"`
	Multiplier  int  `json:"multiplier"`
	Captain     bool `json:"captain"`
	ViceCaptain bool `json:"viceCaptain"`
}

type ManagerGameweek struct {
	EntryID      int           `json:"entryId"`
	SeasonID     int           `json:"seasonId"`
	Gameweek     int           `json:"gameweek"`
	Points       int           `json:"points"`
	Rank         int           `json:"rank"`
	OverallRank  int           `json:"overallRank"`
	Bank         int           `json:"bank"`
	TeamValue    int           `json:"teamValue"`
	Transfers    int           `json:"transfers"`
	TransferCost int           `json:"transferCost"`
	BenchPoints  int           `json:"benchPoints"`
	ActiveChip   string        `json:"activeChip,omitempty"`
	Picks        []ManagerPick `json:"picks,omitempty"`
}

type ManagerTransfer struct {
	Gameweek      int       `json:"gameweek"`
	PlayerIn      int       `json:"playerIn"`
	PlayerOut     int       `json:"playerOut"`
	PlayerInCost  int       `json:"playerInCost"`
	PlayerOutCost int       `json:"playerOutCost"`
	MadeAt        time.Time `json:"madeAt"`
}

type LeagueMember struct {
	EntryID    int    `json:"entryId"`
	EntryName  string `json:"entryName"`
	PlayerName string `json:"playerName"`
	Rank       int    `json:"rank"`
	LastRank   int    `json:"lastRank"`
	Points     int    `json:"points"`
}

type LeagueStandings struct {
	LeagueID   int            `json:"leagueId"`
	Name       string         `json:"name"`
	Page       int            `json:"page"`
	HasNext    bool           `json:"hasNext"`
	Members    []LeagueMember `json:"members"`
	SnapshotID string         `json:"snapshotId"`
}

type ActiveTeamSnapshot struct {
	SnapshotID      int64           `json:"snapshotId"`
	EntryID         int             `json:"entryId"`
	SeasonID        int             `json:"seasonId"`
	Gameweek        int             `json:"gameweek"`
	Bank            int             `json:"bank"`
	TeamValue       int             `json:"teamValue"`
	ActiveChip      string          `json:"activeChip,omitempty"`
	Picks           []ManagerPick   `json:"picks"`
	PurchasePrices  map[int]float64 `json:"purchasePrices"`
	SourceFetchedAt time.Time       `json:"sourceFetchedAt"`
	NormalizedAt    time.Time       `json:"normalizedAt"`
	State           string          `json:"state"`
	ConflictState   string          `json:"conflictState"`
}

type SquadImportPreview struct {
	Snapshot         ActiveTeamSnapshot `json:"snapshot"`
	Proposed         Squad              `json:"proposed"`
	AddedPlayerIDs   []int              `json:"addedPlayerIds"`
	RemovedPlayerIDs []int              `json:"removedPlayerIds"`
	LineupChanged    bool               `json:"lineupChanged"`
	CaptainChanged   bool               `json:"captainChanged"`
	Validation       []ValidationError  `json:"validation"`
	HasChanges       bool               `json:"hasChanges"`
}

type LeagueComparisonResult struct {
	LeagueID         int              `json:"leagueId"`
	SeasonID         int              `json:"seasonId"`
	Gameweek         int              `json:"gameweek"`
	SelectedIDs      []int            `json:"selectedEntryIds"`
	OmittedIDs       []int            `json:"omittedEntryIds"`
	Comparisons      []TeamComparison `json:"comparisons"`
	OutcomeState     string           `json:"outcomeState"`
	AlgorithmVersion string           `json:"algorithmVersion,omitempty"`
	MissingInputs    []string         `json:"missingInputs"`
}

func SelectLeagueMembers(members []LeagueMember, explicit []int, rankFrom, rankTo, limit int) (selected, omitted []int) {
	ordered := append([]LeagueMember(nil), members...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Rank == ordered[j].Rank {
			return ordered[i].EntryID < ordered[j].EntryID
		}
		return ordered[i].Rank < ordered[j].Rank
	})
	wanted := map[int]bool{}
	if len(explicit) > 0 {
		for _, id := range explicit {
			wanted[id] = true
		}
	}
	if limit <= 0 {
		limit = 50
	}
	for _, member := range ordered {
		eligible := len(explicit) == 0
		if len(explicit) > 0 {
			eligible = wanted[member.EntryID]
		}
		if len(explicit) == 0 && rankFrom > 0 {
			eligible = member.Rank >= rankFrom && (rankTo == 0 || member.Rank <= rankTo)
		}
		if eligible && len(selected) < limit {
			selected = append(selected, member.EntryID)
		} else {
			omitted = append(omitted, member.EntryID)
		}
	}
	return selected, omitted
}

type TeamComparison struct {
	EntryID         int     `json:"entryId"`
	SharedPlayers   []int   `json:"sharedPlayers"`
	Differentials   []int   `json:"differentials"`
	Overlap         float64 `json:"overlap"`
	NetPoints       int     `json:"netPoints"`
	PointDifference int     `json:"pointDifference"`
	OutcomeState    string  `json:"outcomeState"`
}

func CompareTeams(left, right []ManagerPick, points map[int]int, transferCost int, outcome string) (TeamComparison, TeamComparison) {
	leftSet, rightSet := map[int]ManagerPick{}, map[int]ManagerPick{}
	for _, pick := range left {
		leftSet[pick.PlayerID] = pick
	}
	for _, pick := range right {
		rightSet[pick.PlayerID] = pick
	}
	shared := []int{}
	leftOnly := []int{}
	rightOnly := []int{}
	for id := range leftSet {
		if _, ok := rightSet[id]; ok {
			shared = append(shared, id)
		} else {
			leftOnly = append(leftOnly, id)
		}
	}
	for id := range rightSet {
		if _, ok := leftSet[id]; !ok {
			rightOnly = append(rightOnly, id)
		}
	}
	sort.Ints(shared)
	sort.Ints(leftOnly)
	sort.Ints(rightOnly)
	union := len(shared) + len(leftOnly) + len(rightOnly)
	overlap := 0.0
	if union > 0 {
		overlap = float64(len(shared)) / float64(union)
	}
	score := func(items map[int]ManagerPick) int {
		total := 0
		for id, p := range items {
			total += points[id] * p.Multiplier
		}
		return total - transferCost
	}
	lp, rp := score(leftSet), score(rightSet)
	return TeamComparison{SharedPlayers: shared, Differentials: leftOnly, Overlap: overlap, NetPoints: lp, PointDifference: lp - rp, OutcomeState: outcome}, TeamComparison{SharedPlayers: shared, Differentials: rightOnly, Overlap: overlap, NetPoints: rp, PointDifference: rp - lp, OutcomeState: outcome}
}
