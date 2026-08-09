## ADDED Requirements

### Requirement: Replay recommendations without future leakage

The system SHALL calculate each recommendation from inputs observed no later than that gameweek's decision cutoff and SHALL store algorithm, weight, ruleset, and snapshot versions.

#### Scenario: Valid historical replay
- **WHEN** all required features are available by the cutoff
- **THEN** the replay produces a versioned recommendation and evaluates it against later official outcome labels

#### Scenario: Future feature
- **WHEN** a feature was first observed after the cutoff
- **THEN** the replay rejects the affected week or marks it excluded with the exact leaked input

### Requirement: Report reproducible evaluation metrics

The system SHALL expose per-week and aggregate metrics using the canonical `recommendation-evaluation-v1` formulas for recommended points, oracle points, regret, top-k hit rate, coverage, and rank correlation, with denominators and excluded weeks. Calibration SHALL be reported only for algorithms that emit forecast probabilities.

#### Scenario: Compare algorithm versions
- **WHEN** two versions are evaluated on the same scope
- **THEN** the response returns comparable metric rows with both version IDs and shared cutoff/population provenance

#### Scenario: Metric denominator is incomplete
- **WHEN** one or more weeks or players are excluded because of missing inputs
- **THEN** aggregate metrics use only valid denominators, list exclusions, and do not convert omitted outcomes to zero

#### Scenario: Calibration input is absent
- **WHEN** an algorithm emits rankings but no probabilities
- **THEN** calibration is `not_applicable` and the response does not fabricate a confidence score
