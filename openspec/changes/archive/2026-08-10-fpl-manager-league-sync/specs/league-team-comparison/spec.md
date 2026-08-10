## ADDED Requirements

### Requirement: Synchronize league-member gameweek teams
The system SHALL use configured classic-league standings to identify members and SHALL synchronize their gameweek team selections for explicitly requested or scheduled gameweeks with bounded concurrency and configurable member limits.

#### Scenario: League member picks are synchronized
- **WHEN** a configured league and gameweek have valid standings and pick responses
- **THEN** the system stores each selected member's player IDs, positions, multipliers, captain, vice-captain, and bench state linked to the league, entry, season, and gameweek

#### Scenario: A member pick request fails
- **WHEN** one member's gameweek picks request exhausts its retry policy
- **THEN** other members continue processing, the failed member is reported, and previously stored picks remain available

#### Scenario: League member scope is large
- **WHEN** the configured league contains more members than the comparison limit
- **THEN** the system synchronizes only the configured or explicitly selected subset and reports which members were omitted

### Requirement: Compare league teams
The system SHALL compare two or more synchronized league teams for a selected season and gameweek, including common players, differentials, formation, captain, vice-captain, bench, and team-level overlap metrics.

#### Scenario: Two teams are compared
- **WHEN** a client requests a comparison for two synchronized teams
- **THEN** the response identifies shared players, players unique to each team, captain and bench differences, formation differences, and a similarity/overlap summary

#### Scenario: Multiple teams are compared
- **WHEN** a client requests a comparison for a selected member set
- **THEN** the response provides pairwise similarities and a league-wide ownership/differential summary without exceeding the configured comparison scope

### Requirement: Explain team point differences
The system SHALL join compared team selections to public player-gameweek facts and report team points, player-level contributions, captain multipliers, bench points, and the point difference between compared teams.

#### Scenario: Completed gameweek comparison
- **WHEN** the selected gameweek is finalized in the public warehouse
- **THEN** the response labels the outcomes `actual` and calculates point differences from finalized player and fixture facts

#### Scenario: Live gameweek comparison
- **WHEN** the selected gameweek is still in progress
- **THEN** the response labels the outcomes `provisional`, includes freshness metadata, and indicates that points may change

#### Scenario: Future gameweek comparison
- **WHEN** the selected gameweek has not started and an estimate is requested
- **THEN** the response labels results `estimated`, includes the algorithm version and factors used, and does not present estimates as official FPL points

### Requirement: Expose comparison freshness and omissions
The system SHALL report the source snapshot time, public-data finalization state, manager-pick freshness, omitted members, failed member requests, and any estimate limitations with every league comparison.

#### Scenario: Comparison data is incomplete
- **WHEN** one or more selected members or public player facts are missing or stale
- **THEN** the response identifies the incomplete inputs and prevents the comparison from being represented as complete actual data

### Requirement: Use deterministic member selection
The system SHALL select comparison members in the order of explicit entry IDs, selected rank range, or configured rank-ordered member limit, and SHALL return the selected and omitted member IDs.

#### Scenario: No explicit members are supplied
- **WHEN** a comparison request supplies only a league and configured member limit
- **THEN** the system selects rank-ordered members deterministically and reports the omitted members

#### Scenario: Explicit members are supplied
- **WHEN** a comparison request supplies entry IDs
- **THEN** the system compares only those entries, subject to the configured maximum, and reports any unavailable entries

### Requirement: Calculate comparable team metrics consistently
The system SHALL define team overlap as shared selected players divided by the union of selected players, differential ownership as each team's unique selected players, and point difference as net team points minus the comparison team's net team points for the same gameweek state.

#### Scenario: Two completed teams are compared
- **WHEN** two complete team snapshots and finalized player facts exist
- **THEN** overlap, differentials, and point differences use the documented formulas and identical player/gameweek inputs
