## ADDED Requirements

### Requirement: Explain a manager's gameweek outcome
The system SHALL explain a selected manager's gameweek points through player contributions, captaincy, bench points, transfers, hits, automatic substitutions, and comparison to the available squad alternatives.

#### Scenario: Completed gameweek is reviewed
- **WHEN** finalized player facts and manager picks are available
- **THEN** the response reports actual gross points, transfer costs, net points, captain contribution, bench points, and decision explanations

The calculation SHALL use `effectivePlayerPoints = playerGameweekPoints * pickMultiplier`, `grossTeamPoints = sum(effectivePlayerPoints)`, `benchPoints = sum(points where multiplier = 0)`, `transferCost = paidTransfers * hitCost`, and `netTeamPoints = grossTeamPoints - transferCost`.

### Requirement: Compare outcome with rivals
The system SHALL attribute the point difference between the configured manager and selected league rivals to shared players, differentials, captaincy, bench, and transfer decisions.

#### Scenario: Rival comparison has complete inputs
- **WHEN** both teams and finalized player facts are available
- **THEN** the response provides an explainable point-difference breakdown with actual outcome state

### Requirement: Label incomplete outcomes
The system SHALL mark a live review as provisional and SHALL identify missing, stale, or estimated inputs.

#### Scenario: Live gameweek is reviewed
- **WHEN** the selected gameweek is not finalized
- **THEN** the response labels points provisional and includes a freshness warning

### Requirement: Expose autopsy scope and provenance
The system SHALL require season, gameweek, manager entry, and optional rival scope and SHALL return formula version plus all public and manager snapshot IDs.

#### Scenario: Autopsy request omits scope
- **WHEN** a client requests an autopsy without a season or gameweek
- **THEN** the API returns a structured scope validation error
