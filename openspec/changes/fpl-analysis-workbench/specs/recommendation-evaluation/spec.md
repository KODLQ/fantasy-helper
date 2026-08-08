## ADDED Requirements

### Requirement: Backtest recommendations without future leakage
The system SHALL replay a versioned recommendation algorithm against historical gameweek snapshots using only inputs available at the selected deadline.

#### Scenario: Historical recommendation is replayed
- **WHEN** a historical gameweek and algorithm version are selected
- **THEN** the response returns the selected XI, bench, captain, vice-captain, weights, input snapshot, and achieved actual points

#### Scenario: Required historical input is missing
- **WHEN** a historical snapshot lacks a required input
- **THEN** the result is marked incomplete and does not claim a complete backtest score

The cutoff SHALL be the selected gameweek deadline by default, and fields observed after that cutoff SHALL not be available to replay.

### Requirement: Evaluate recommendation quality
The system SHALL calculate repeatable evaluation metrics including points, captain success, bench points, hit-adjusted points where applicable, and comparison with the user's actual decisions.

#### Scenario: Evaluation spans multiple gameweeks
- **WHEN** a user selects a historical range
- **THEN** the response returns per-gameweek and aggregate metrics with algorithm and data versions

The evaluation SHALL report recommendation points, actual-manager points, captain delta, bench points, net transfer cost where applicable, and the difference between recommendation and actual decisions.

### Requirement: Compare algorithm configurations
The system SHALL allow bounded comparison of configured signal weights or algorithm versions using the same historical snapshots.

#### Scenario: Two configurations are compared
- **WHEN** two valid configurations are evaluated over the same range
- **THEN** the response aligns their metrics and identifies meaningful differences without changing production recommendation settings

### Requirement: Persist backtest reproducibility
The system SHALL key a backtest by season/range, deadline cutoff policy, input snapshot IDs, algorithm version, weights, and candidate/constraint configuration.

#### Scenario: Backtest is rerun
- **WHEN** the same reproducibility key is submitted
- **THEN** the system returns the prior result or an identical result without changing production settings
