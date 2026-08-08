## ADDED Requirements

### Requirement: Present live gameweek state
The system SHALL present live player points, captain multipliers, bench points, current team totals, and configured rival totals from the freshest available gameweek data.

#### Scenario: Live data is available
- **WHEN** a gameweek is in progress and recent live facts exist
- **THEN** the response displays provisional totals with last-updated time and source freshness

The live endpoint SHALL return refresh interval, last successful refresh, selected manager/rival coverage, player contribution rows, and a non-final state.

### Requirement: Estimate rank movement
The system SHALL report provisional rank or point movement only when the required league and live inputs are available and SHALL label it as non-final.

#### Scenario: Rival live data is incomplete
- **WHEN** some selected rival teams or players lack current live facts
- **THEN** the response identifies the incomplete inputs and avoids presenting rank movement as final

### Requirement: Bound live polling and stale state
The system SHALL use a configured polling interval and SHALL mark the live result stale when the last successful refresh exceeds the allowed age.

#### Scenario: Live refresh is delayed
- **WHEN** no successful live refresh exists within the configured freshness window
- **THEN** the response remains available with a stale warning and does not claim current live points

### Requirement: Show likely substitutions
The system SHALL identify possible automatic substitutions and captaincy outcomes from current picks, bench order, and player availability without changing saved squad state.

#### Scenario: Starter may not play
- **WHEN** a starting player is unavailable or has not played and substitution rules permit a bench replacement
- **THEN** the response shows the likely substitution as provisional with the rule inputs used
