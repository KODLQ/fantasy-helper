## ADDED Requirements

### Requirement: Explain one gameweek outcome

The system SHALL calculate gross points, transfer cost, net points, captain delta, bench points, automatic substitutions, differentials, and rival point differences for one entry, season, and gameweek using the canonical shared formula registry and season ruleset.

#### Scenario: Completed gameweek autopsy
- **WHEN** finalized player picks and points exist
- **THEN** the autopsy returns weekly metrics, player-level contributions, exact transfers/hits, lineup/captain/bench decisions, and calculation version

#### Scenario: Live gameweek autopsy
- **WHEN** the gameweek is still provisional
- **THEN** the autopsy labels totals provisional, identifies unfinished fixtures, and avoids final-rank language

### Requirement: Preserve calculation scope

The system SHALL reject or clearly mark requests with missing manager/public snapshots, mismatched gameweeks, or unsupported chip/substitution inputs.

#### Scenario: Missing input
- **WHEN** a required pick or player fact is absent
- **THEN** the response uses the common partial/unavailable state and names the missing input
