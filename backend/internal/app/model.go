package app

import "time"

const (
	Goalkeeper = 1
	Defender   = 2
	Midfielder = 3
	Forward    = 4
)

type Season struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	IsCurrent bool      `json:"isCurrent"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Gameweek struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	DeadlineTime *time.Time `json:"deadlineTime,omitempty"`
	Finished     bool       `json:"finished"`
	IsCurrent    bool       `json:"isCurrent"`
	AverageScore float64    `json:"averageScore"`
}

type Team struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ShortName    string `json:"shortName"`
	Strength     int    `json:"strength,omitempty"`
	StrengthHome int    `json:"strengthOverallHome,omitempty"`
	StrengthAway int    `json:"strengthOverallAway,omitempty"`
	AttackHome   int    `json:"strengthAttackHome,omitempty"`
	AttackAway   int    `json:"strengthAttackAway,omitempty"`
	DefenceHome  int    `json:"strengthDefenceHome,omitempty"`
	DefenceAway  int    `json:"strengthDefenceAway,omitempty"`
}

type Fixture struct {
	ID             int        `json:"id"`
	Gameweek       int        `json:"gameweek"`
	KickoffTime    *time.Time `json:"kickoffTime,omitempty"`
	Finished       bool       `json:"finished"`
	HomeTeam       int        `json:"homeTeam"`
	AwayTeam       int        `json:"awayTeam"`
	HomeDifficulty int        `json:"homeDifficulty"`
	AwayDifficulty int        `json:"awayDifficulty"`
	HomeScore      *int       `json:"homeScore,omitempty"`
	AwayScore      *int       `json:"awayScore,omitempty"`
}

type Player struct {
	ID                int     `json:"id"`
	FirstName         string  `json:"firstName"`
	SecondName        string  `json:"secondName"`
	WebName           string  `json:"webName"`
	Position          int     `json:"position"`
	TeamID            int     `json:"teamId"`
	Price             float64 `json:"price"`
	TotalPoints       int     `json:"totalPoints"`
	Form              float64 `json:"form"`
	Minutes           int     `json:"minutes"`
	Value             float64 `json:"value"`
	Status            string  `json:"status"`
	News              string  `json:"news"`
	ChanceOfPlaying   *int    `json:"chanceOfPlayingNextRound,omitempty"`
	GoalsScored       int     `json:"goalsScored"`
	Assists           int     `json:"assists"`
	CleanSheets       int     `json:"cleanSheets"`
	Bonus             int     `json:"bonus"`
	Saves             int     `json:"saves"`
	SelectedByPercent float64 `json:"selectedByPercent,omitempty"`
	YellowCards       int     `json:"yellowCards,omitempty"`
	RedCards          int     `json:"redCards,omitempty"`
	OwnGoals          int     `json:"ownGoals,omitempty"`
	PenaltiesSaved    int     `json:"penaltiesSaved,omitempty"`
	PenaltiesMissed   int     `json:"penaltiesMissed,omitempty"`
	ExpectedGoals     float64 `json:"expectedGoals,omitempty"`
	ExpectedAssists   float64 `json:"expectedAssists,omitempty"`
	ExpectedMinutes   float64 `json:"expectedMinutes"`
	RecentReturns     float64 `json:"recentReturns"`
}

type PlayerHistory struct {
	Gameweek    int     `json:"gameweek"`
	Minutes     int     `json:"minutes"`
	TotalPoints int     `json:"totalPoints"`
	Goals       int     `json:"goals"`
	Assists     int     `json:"assists"`
	CleanSheets int     `json:"cleanSheets"`
	Bonus       int     `json:"bonus"`
	Value       float64 `json:"value"`
}

type Freshness struct {
	Dataset            string    `json:"dataset,omitempty"`
	State              string    `json:"state,omitempty"`
	SnapshotIDs        []string  `json:"snapshotIds,omitempty"`
	SourceFetchedAt    time.Time `json:"sourceFetchedAt,omitempty"`
	NormalizedAt       time.Time `json:"normalizedAt,omitempty"`
	NormalizerVersion  string    `json:"normalizerVersion,omitempty"`
	MissingInputs      []string  `json:"missingInputs,omitempty"`
	Warnings           []string  `json:"warnings,omitempty"`
	Status             string    `json:"status"`
	LastSuccessfulSync time.Time `json:"lastSuccessfulSync,omitempty"`
	Warning            string    `json:"warning,omitempty"`
	SnapshotAt         time.Time `json:"snapshotAt,omitempty"`
}

type SyncStatus struct {
	Status          string    `json:"status"`
	RunID           int64     `json:"runId,omitempty"`
	Scope           Scope     `json:"scope,omitempty"`
	CurrentStage    string    `json:"currentStage,omitempty"`
	CompletedStages []string  `json:"completedStages"`
	FailedStages    []string  `json:"failedStages"`
	Warning         string    `json:"warning,omitempty"`
	StartedAt       time.Time `json:"startedAt,omitempty"`
	FinishedAt      time.Time `json:"finishedAt,omitempty"`
	Checksum        string    `json:"checksum,omitempty"`
	Freshness       Freshness `json:"freshness"`
}

type SyncWorkItem struct {
	ID               int64     `json:"id,omitempty"`
	RunID            int64     `json:"runId"`
	Scope            string    `json:"scope"`
	NaturalKey       string    `json:"naturalKey"`
	Endpoint         string    `json:"endpoint"`
	SeasonSourceID   int       `json:"seasonId,omitempty"`
	GameweekSourceID int       `json:"gameweek,omitempty"`
	EntitySourceID   int       `json:"entityId,omitempty"`
	Status           string    `json:"status"`
	Attempts         int       `json:"attempts"`
	AvailableAt      time.Time `json:"availableAt"`
	LastError        string    `json:"lastError,omitempty"`
}

type ResponseMeta struct {
	RequestID  string      `json:"requestId"`
	Scope      Scope       `json:"scope,omitempty"`
	Freshness  Freshness   `json:"freshness,omitempty"`
	Provenance []string    `json:"provenance,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Coverage   *Coverage   `json:"coverage,omitempty"`
}

type Pagination struct {
	Limit    int `json:"limit"`
	Offset   int `json:"offset"`
	Returned int `json:"returned"`
	Total    int `json:"total,omitempty"`
}

type Coverage struct {
	Complete   bool     `json:"complete"`
	MissingIDs []string `json:"missingIds,omitempty"`
	Warning    string   `json:"warning,omitempty"`
}

type ResponseError struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Retryable bool        `json:"retryable"`
	Details   interface{} `json:"details,omitempty"`
}

type Scope struct {
	SeasonID int    `json:"seasonId,omitempty"`
	Gameweek int    `json:"gameweek,omitempty"`
	Dataset  string `json:"dataset,omitempty"`
}

type DatasetSnapshot struct {
	ID                string    `json:"id"`
	Dataset           string    `json:"dataset"`
	State             string    `json:"state"`
	SeasonID          int       `json:"seasonId"`
	Gameweek          int       `json:"gameweek,omitempty"`
	SourceFetchedAt   time.Time `json:"sourceFetchedAt,omitempty"`
	NormalizedAt      time.Time `json:"normalizedAt"`
	NormalizerVersion string    `json:"normalizerVersion"`
	MissingInputs     []string  `json:"missingInputs"`
}

type PlayerDetail struct {
	Player    Player          `json:"player"`
	Team      Team            `json:"team"`
	History   []PlayerHistory `json:"history"`
	Fixtures  []Fixture       `json:"fixtures"`
	Freshness Freshness       `json:"freshness"`
}

type Squad struct {
	Name              string            `json:"name"`
	Budget            float64           `json:"budget"`
	Players           []Player          `json:"players"`
	PurchasePrices    map[int]float64   `json:"purchasePrices"`
	StartingPlayerIDs []int             `json:"startingPlayerIds"`
	BenchPlayerIDs    []int             `json:"benchPlayerIds"`
	CaptainID         int               `json:"captainId"`
	ViceCaptainID     int               `json:"viceCaptainId"`
	Formation         string            `json:"formation"`
	TotalCost         float64           `json:"totalCost"`
	RemainingBudget   float64           `json:"remainingBudget"`
	Validation        []ValidationError `json:"validation"`
}

type ValidationError struct {
	Code     string      `json:"code"`
	Rule     string      `json:"rule"`
	PlayerID int         `json:"playerId,omitempty"`
	Current  interface{} `json:"current,omitempty"`
	Required interface{} `json:"required,omitempty"`
	Message  string      `json:"message"`
}

type Weights struct {
	Form          float64 `json:"form"`
	Minutes       float64 `json:"minutes"`
	Fixture       float64 `json:"fixture"`
	RecentReturns float64 `json:"recentReturns"`
	Value         float64 `json:"value"`
}

func DefaultWeights() Weights {
	return Weights{Form: 0.28, Minutes: 0.25, Fixture: 0.20, RecentReturns: 0.17, Value: 0.10}
}

type FactorContribution struct {
	Name         string  `json:"name"`
	Signal       float64 `json:"signal"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

type RecommendationPlayer struct {
	Player      Player               `json:"player"`
	Score       float64              `json:"score"`
	Factors     []FactorContribution `json:"factors"`
	Fixture     string               `json:"fixture"`
	Explanation string               `json:"explanation"`
}

type Recommendation struct {
	Season           Season                 `json:"season"`
	Gameweek         Gameweek               `json:"gameweek"`
	SnapshotAt       time.Time              `json:"snapshotAt"`
	AlgorithmVersion string                 `json:"algorithmVersion"`
	Weights          Weights                `json:"weights"`
	StartingXI       []RecommendationPlayer `json:"startingXI"`
	Bench            []RecommendationPlayer `json:"bench"`
	Captain          RecommendationPlayer   `json:"captain"`
	ViceCaptain      RecommendationPlayer   `json:"viceCaptain"`
	HeuristicNotice  string                 `json:"heuristicNotice"`
}

type Snapshot struct {
	Season    Season
	Gameweeks []Gameweek
	Teams     []Team
	Players   []Player
	Fixtures  []Fixture
	Histories map[int][]PlayerHistory
	Checksum  string
}
