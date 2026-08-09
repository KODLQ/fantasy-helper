## ADDED Requirements

### Requirement: Present live points with explicit provisional state

The system SHALL show live player/team points, captain multipliers, likely substitutions, rank movement, and rival progress only for a requested season/gameweek/snapshot scope and SHALL expose source age and unfinished coverage.

#### Scenario: Live snapshot is current
- **WHEN** a current live snapshot exists
- **THEN** the response returns provisional totals and player contributions with freshness metadata and does not claim final rank

#### Scenario: Snapshot is stale or finalized
- **WHEN** the source is stale or the gameweek is finalized
- **THEN** the response shows stale/finalized state, stops or slows polling as configured, and explains the transition
