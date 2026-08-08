## ADDED Requirements

### Requirement: Calculate a sequential best-achievable path
The system SHALL calculate a valid sequence of squads and lineups from the season start or a selected starting squad through every completed gameweek up to an endpoint gameweek, rather than selecting disconnected weekly teams.

#### Scenario: Endpoint gameweek is recalculated
- **WHEN** a user selects a season and completed endpoint gameweek
- **THEN** the system produces a weekly path ending at that gameweek with a valid squad transition between each gameweek

#### Scenario: Later results change hindsight
- **WHEN** the endpoint advances to a later completed gameweek
- **THEN** the system may produce different earlier transfers or squads and stores the new result as a separate versioned run

The path SHALL contain one state for every gameweek in the requested range and SHALL not select disconnected weekly squads.

### Requirement: Enforce historical squad and transfer constraints
The system SHALL enforce the selected season's budget, squad size, positional composition, club limit, player availability, price-at-deadline, formation, and transfer rules for every weekly state and transition.

#### Scenario: Candidate transfer violates a constraint
- **WHEN** a proposed optimal-path transition exceeds budget, club limit, position rules, or available player state
- **THEN** the transition is rejected and cannot contribute to the result

#### Scenario: Ruleset is incomplete
- **WHEN** required historical settings are unavailable
- **THEN** the run reports the missing assumptions and marks the affected result as incomplete or assumption-based

The path SHALL use the ruleset's sell-price function and price-at-deadline values when checking bank affordability; current prices SHALL NOT be substituted for historical prices.

### Requirement: Account for free transfers and paid transfer costs
The system SHALL calculate free transfers available, carried free transfers, transfers used, paid transfers, hit cost, gross points, and net points for every weekly transition using the versioned season ruleset.

#### Scenario: Transfers exceed free allowance
- **WHEN** a weekly path uses more transfers than its available free-transfer balance
- **THEN** the system deducts the configured transfer-hit cost for each paid transfer from gross points and records the deduction

#### Scenario: Transfers remain within allowance
- **WHEN** a weekly path uses no more than its available free transfers
- **THEN** the system records zero paid transfers and carries the remaining free-transfer balance according to the ruleset

The system SHALL expose `freeBefore`, `freeUsed`, `paidTransfers`, `hitCost`, `freeAfter`, `grossPoints`, and `netPoints` for every transition.

### Requirement: Recalculate and preserve endpoint runs
The system SHALL recalculate the optimal path for each requested endpoint gameweek and SHALL preserve prior runs with source snapshot, ruleset, algorithm, objective, and search-parameter identity.

#### Scenario: Same inputs are requested again
- **WHEN** a run with identical season, endpoint, ruleset, algorithm, and input snapshots exists
- **THEN** the system reuses or deterministically reproduces the existing result

#### Scenario: Algorithm changes
- **WHEN** the optimization algorithm or candidate limits change
- **THEN** the system creates a new versioned run without overwriting the previous result

The reproducibility key SHALL include season, start/end gameweek, starting mode, entry ID when applicable, input snapshot IDs, ruleset version, algorithm version, chip policy, candidate policy, objective, and tie-breaker policy.

### Requirement: Report optimality and data quality
The system SHALL label each result as exact, bounded, incomplete, or assumption-based and SHALL report missing weeks, omitted candidates, candidate limits, and excluded rule features such as chips.

#### Scenario: Production bounded search completes
- **WHEN** a bounded search returns a best path
- **THEN** the response identifies the bounded search configuration and does not claim mathematical global optimality

#### Scenario: Required data is missing
- **WHEN** player points, prices, availability, or rules are missing for an affected week
- **THEN** the result identifies the affected week and prevents an unlabeled zero-filled result

The allowed optimality states SHALL be `exact`, `bounded_best_found`, `incomplete`, and `assumption_based`; the API SHALL return candidate omissions, search parameters, and excluded chip/ruleset features.

### Requirement: Explain and compare the optimal path
The system SHALL expose each weekly optimal squad, lineup, captain, bench, transfers, transfer cost, gross points, net points, and comparison against the user's actual synchronized team when available.

#### Scenario: User compares actual and optimal paths
- **WHEN** manager history exists for the same season and endpoint
- **THEN** the response shows cumulative and weekly differences, transfer hits, captain/bench differences, and the decisions responsible for the gap

### Requirement: Provide reproducible asynchronous run APIs
The system SHALL create or reuse optimization runs asynchronously and SHALL expose progress, cancellation, status, timeline, and comparison endpoints.

#### Scenario: Run is requested twice
- **WHEN** the same reproducibility key is submitted while the first run is queued or running
- **THEN** the API returns the existing run ID instead of creating duplicate computation

#### Scenario: Run is cancelled
- **WHEN** a user cancels a queued or running run
- **THEN** the system stops new search work, records cancellation, and does not present partial output as a completed optimal path

### Requirement: Distinguish hindsight from advice
The system SHALL label every result as retrospective and SHALL state that future completed-gameweek outcomes were used when calculating an endpoint's hindsight path.

#### Scenario: Optimal path is displayed
- **WHEN** a completed historical run is shown
- **THEN** the UI and API identify the endpoint, hindsight nature, algorithm/optimality status, and assumptions before presenting point differences
