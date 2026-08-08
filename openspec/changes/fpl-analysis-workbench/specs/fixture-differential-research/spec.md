## ADDED Requirements

### Requirement: Detect fixture swings
The system SHALL rank teams and players by upcoming fixture difficulty changes over a selected horizon using home/away context, blank/double gameweek state, and data freshness.

#### Scenario: Fixture horizon is selected
- **WHEN** a user selects a future gameweek range
- **THEN** the response returns fixture-swing candidates with opponents, difficulty inputs, and explanation

The fixture-swing result SHALL include horizon, home/away fixtures, blank/double indicators, normalized difficulty calculation, source snapshot, and incomplete-fixture warnings.

### Requirement: Find differentials
The system SHALL identify players with attractive form, minutes, value, fixture, availability, and ownership characteristics relative to a selected league or comparison population.

#### Scenario: Differential candidate is available
- **WHEN** a player meets the configured differential thresholds
- **THEN** the response explains ownership gap, expected role, fixture run, value, and risk signals

Ownership population SHALL be explicit as `global`, `league`, `selected-rivals`, or `custom`; thresholds and formula version SHALL be returned.

### Requirement: Explain availability impact
The system SHALL identify squads, comparisons, and recommendations affected by player availability or expected-minutes changes.

#### Scenario: Player availability changes
- **WHEN** a synchronized player status or expected-minutes value changes materially
- **THEN** the response identifies affected teams and likely replacement candidates with freshness metadata

### Requirement: Bound research horizons
The system SHALL require a finite gameweek horizon and SHALL return a validation error or asynchronous job status when the request exceeds configured synchronous limits.

#### Scenario: Horizon is too large
- **WHEN** a client requests a horizon above the configured maximum
- **THEN** the API rejects the request or returns an explicit asynchronous job instead of timing out
