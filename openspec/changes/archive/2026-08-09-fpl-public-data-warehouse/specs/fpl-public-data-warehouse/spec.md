## ADDED Requirements

### Requirement: Capture public source observations
The system SHALL persist public FPL endpoint responses with endpoint identity, request parameters, fetched timestamp, HTTP status, checksum, validation result, and bounded raw JSON diagnostics.

#### Scenario: Successful response is captured
- **WHEN** a configured public endpoint returns a valid response
- **THEN** the system stores its checksum, fetch metadata, and raw payload before or alongside normalization

#### Scenario: Invalid response is captured diagnostically
- **WHEN** a response cannot be validated against the expected source shape
- **THEN** the system stores the endpoint, checksum, validation error, and bounded payload without committing invalid canonical rows

### Requirement: Persist a complete season catalog
The system SHALL normalize season-scoped gameweeks, phases, game settings, element types, teams, players, and fixtures from the public FPL source using source IDs and foreign-key relationships.

#### Scenario: Initial catalog import succeeds
- **WHEN** the baseline public snapshot and fixture feed are valid
- **THEN** the system creates or updates the active season catalog and associates gameweeks, teams, players, and fixtures with that season

#### Scenario: Repeated catalog import is idempotent
- **WHEN** the same catalog is imported more than once
- **THEN** existing season-scoped rows are updated by natural key without duplicate entities or fixtures

### Requirement: Preserve changing player and gameweek facts
The system SHALL retain historical player snapshots and player-gameweek performance facts for price, team, availability, ownership, minutes, points, scoring, defensive, bonus, and source-provided performance metrics.

#### Scenario: Player values change between refreshes
- **WHEN** a later public snapshot changes a player's price, team, status, or performance metric
- **THEN** the later value is stored as a new time/gameweek-scoped observation and the earlier observation remains queryable

#### Scenario: Gameweek live data is finalized
- **WHEN** a gameweek is marked complete
- **THEN** the system stores the final player and fixture facts and marks the analytical gameweek dataset as finalized

### Requirement: Run resumable public synchronization
The system SHALL execute public syncs as durable stages and work items with bounded concurrency, retry/backoff, rate-limit handling, deterministic idempotency keys, and restart recovery.

#### Scenario: Worker restarts during a backfill
- **WHEN** the sync process stops after some player work items have completed
- **THEN** a subsequent run resumes incomplete or retryable work without duplicating completed facts

#### Scenario: One source request fails
- **WHEN** a public endpoint request exhausts its retry policy
- **THEN** the system records the failed work item and retains the last-known-good canonical data for that item

### Requirement: Report dataset freshness and partial completion
The system SHALL expose run-level and stage-level status including current stage, completed and failed work, last successful canonical timestamps, warnings, and whether each public dataset is fresh, stale, partial, or unavailable.

#### Scenario: Catalog succeeds but player backfill is incomplete
- **WHEN** catalog import succeeds and one or more player-history work items fail
- **THEN** the status reports a partial sync and identifies the affected stage or work items while serving the successful catalog

#### Scenario: No public data exists
- **WHEN** no successful canonical sync has completed
- **THEN** the status reports unavailable data and provides an actionable failure reason

### Requirement: Provide analytical public-data read models
The system SHALL provide indexed read models for player research, rolling gameweek form, fixture context, price/value changes, availability, and recommendation inputs keyed by season and gameweek.

#### Scenario: Research query uses a historical snapshot
- **WHEN** a client requests player metrics for a season and gameweek
- **THEN** the response uses the corresponding historical snapshot rather than only the latest player row

#### Scenario: Current research data is partial
- **WHEN** one analytical input dataset is stale or partial
- **THEN** the response includes the affected freshness warning and does not present the result as fully fresh

### Requirement: Identify point-in-time datasets
The system SHALL assign every normalized analytical dataset a stable snapshot identity containing season, gameweek or observation time, source fetch time, normalization version, and completeness state.

#### Scenario: Historical and current snapshots coexist
- **WHEN** a player is queried for a historical gameweek and for the current gameweek
- **THEN** the system returns values from the requested snapshots and does not silently substitute current values for historical values

#### Scenario: Dataset is incomplete
- **WHEN** required source stages are missing or failed for a requested snapshot
- **THEN** the response identifies missing inputs and reports `partial`, `stale`, or `unavailable` rather than claiming `actual`

### Requirement: Expose stable sync control contracts
The system SHALL expose explicit sync scopes, run IDs, stage progress, retryable failures, and dataset freshness through versioned API contracts.

#### Scenario: Operator starts a scoped sync
- **WHEN** an operator requests a valid catalog, fixture, live, player-history, or full scope
- **THEN** the API returns a run ID and the run status identifies the requested season/gameweek scope

#### Scenario: Duplicate scoped run is requested
- **WHEN** an equivalent run is already active
- **THEN** the API rejects or coalesces the request with a stable conflict/status response and does not create overlapping work
