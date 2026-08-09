## ADDED Requirements

### Requirement: Compare a scoped league snapshot

The system SHALL compare only selected members from one league, season, gameweek, and compatible pick snapshot state, SHALL use canonical `team-overlap-v1`, `differential-contribution-v1`, and `team-points-difference-v1` formulas, and SHALL report selected and omitted members.

#### Scenario: Deterministic member limit
- **WHEN** a league comparison has no explicit entry IDs and a member limit is supplied
- **THEN** members are selected by rank then entry ID and omitted IDs/count are returned

#### Scenario: Compare teams
- **WHEN** two selected members have valid picks for the requested gameweek
- **THEN** the response includes roster overlap, starting-XI overlap when requested, differential contribution, net point difference, and provenance

### Requirement: Explain incomplete league coverage

The system SHALL report missing members, unavailable picks, stale/provisional data, and snapshot mismatch without inventing zero-valued comparison data.

#### Scenario: Rival pick snapshot is missing
- **WHEN** a selected rival has no valid pick snapshot
- **THEN** the response identifies that rival in coverage/missing inputs and does not include it in complete aggregate metrics
