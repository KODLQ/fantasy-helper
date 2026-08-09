## ADDED Requirements

### Requirement: Calculate a sequential best-achievable path
The system SHALL calculate a valid sequence of squads and lineups from the season start or a selected starting squad through every completed gameweek up to an endpoint gameweek, using the canonical player-points, team-points, captain, and transfer-cost formulas rather than selecting disconnected weekly teams.

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

### Requirement: Apply canonical chip, substitution, missing-data, and normalization rules

The system SHALL use the season-specific `fpl-rules-v1` behavior from `openspec/fpl-rules.yaml` for chips, captain/vice-captain, automatic substitutions, missing inputs, and shared normalization, and SHALL record the ruleset checksum in every run.

#### Scenario: Chip behavior is modeled
- **WHEN** a run includes wildcard, free hit, bench boost, triple captain, or another season-defined chip
- **THEN** the optimizer applies the configured availability, exclusivity, scoring, transfer, squad-restoration, and permanence behavior for that season

#### Scenario: Chip behavior is unavailable
- **WHEN** the source does not provide the required chip rule or historical availability
- **THEN** the affected run is incomplete or assumption-based and cannot be labeled `complete_exact`

#### Scenario: Automatic substitution is required
- **WHEN** a starting player has zero minutes after a gameweek and the source provides valid bench order and minutes
- **THEN** the first eligible bench player that preserves a legal formation replaces the starter, and vice-captain multiplier behavior is applied when the captain did not play

#### Scenario: Required fact is missing
- **WHEN** a required player point, price, availability, fixture, rules, or chip input is missing
- **THEN** the affected gameweek is marked incomplete and no zero or current-value substitute is used

#### Scenario: Normalization has no spread
- **WHEN** a peer set has equal values for a normalized feature
- **THEN** every present value receives 0.5, missing values are excluded from the denominator, and the normalization version is returned

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
The system SHALL label each result as complete_exact, best_found_bounded, incomplete, assumption_based, or feasibility_unproven and SHALL report missing weeks, omitted candidates, candidate limits, excluded rule features such as chips, and feasibility certification where applicable.

#### Scenario: Production bounded search completes
- **WHEN** a bounded search returns a best path
- **THEN** the response identifies the bounded search configuration and does not claim mathematical global optimality

#### Scenario: Required data is missing
- **WHEN** player points, prices, availability, or rules are missing for an affected week
- **THEN** the result identifies the affected week and prevents an unlabeled zero-filled result

The allowed optimality states SHALL be `complete_exact`, `best_found_bounded`, `incomplete`, `assumption_based`, and `feasibility_unproven`; the API SHALL return candidate omissions, search parameters, excluded chip/ruleset features, and the certification record for `complete_exact`.

### Requirement: Certify exact solver feasibility

The system SHALL refuse to label a full-player result `complete_exact` until a reproducible benchmark proves the declared runtime, memory, temporary-storage, persisted-result, concurrency, and cancellation budgets for the same algorithm, ruleset, schema, and supported player cardinality.

#### Scenario: Feasibility benchmark passes
- **WHEN** exhaustive full-player search completes within all declared budgets and its checksum is reproducible
- **THEN** the run may be labeled `complete_exact` and exposes the benchmark ID, hardware profile, input cardinalities, zero-pruning evidence, and resource measurements

#### Scenario: Feasibility benchmark fails
- **WHEN** runtime, memory, scratch storage, result size, determinism, or cancellation exceeds its budget
- **THEN** the run is labeled `feasibility_unproven` or `best_found_bounded` and the UI cannot use complete-optimal wording

#### Scenario: Exact run is cancelled
- **WHEN** a full-player exact run is cancelled or exceeds its budget
- **THEN** search stops within the cancellation target, no partial result is presented as complete, and only progress/diagnostic metadata is retained

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
