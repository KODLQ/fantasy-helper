## ADDED Requirements

### Requirement: Summarize league context
The system SHALL summarize a configured league for a selected gameweek with member rank, points, team ownership, common players, differentials, and the configured manager's relative position.

#### Scenario: League summary is complete
- **WHEN** standings and selected member picks are synchronized
- **THEN** the response returns league rank context, ownership counts, common-player rates, differentials, and freshness metadata

### Requirement: Identify rival threats
The system SHALL identify selected rivals or rank bands whose teams create meaningful threats to the configured manager's rank and SHALL explain the player, captaincy, or point differences involved.

#### Scenario: Rival has a threatening differential
- **WHEN** a rival owns a player that the configured manager does not and the player contributes to a material projected or actual difference
- **THEN** the response identifies the rival, differential player, contribution, outcome state, and supporting facts

### Requirement: Compare league members safely
The system SHALL bound league comparison by configured member and request limits and SHALL report omitted or failed members rather than treating them as complete zero-valued teams.

#### Scenario: League exceeds the comparison limit
- **WHEN** a league has more members than the configured limit
- **THEN** the response identifies the selected members and the omitted count/list

### Requirement: Use consistent league comparison metrics
The system SHALL calculate roster overlap as the Jaccard ratio of selected player IDs, differential contribution from effective points for team-unique players, and point difference from net team points for the same season, gameweek, and outcome state.

#### Scenario: Two complete league teams are compared
- **WHEN** two synchronized teams and public player-gameweek facts are complete
- **THEN** the response includes formula version, overlap, differential contribution, gross points, transfer cost, and net point difference

### Requirement: Expose league intelligence scope
The system SHALL require explicit league, season, gameweek, and comparison-member scope and SHALL return snapshot identities, coverage, missing inputs, and outcome state.

#### Scenario: League summary is requested
- **WHEN** a client supplies a league, season, gameweek, and valid member scope
- **THEN** the API returns the summary with the common analysis envelope
