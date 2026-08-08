## MODIFIED Requirements

### Requirement: Import official FPL data into a normalized season snapshot
The system SHALL import the configured official FPL public endpoints into PostgreSQL, persist raw source observations, and normalize season-scoped seasons, phases, gameweeks, settings, teams, player snapshots, fixtures, and live gameweek facts keyed by stable source IDs.

#### Scenario: Initial season sync succeeds
- **WHEN** an operator starts a sync against a valid FPL source
- **THEN** the system stores the current season catalog and public performance facts, records a successful database-backed sync run, and exposes the imported season as active

#### Scenario: Repeating a sync is idempotent
- **WHEN** the same source data is synchronized more than once
- **THEN** the system updates matching season-scoped rows and records a source observation without creating duplicate entities or facts

### Requirement: Refresh player historical data safely
The system SHALL retrieve and normalize each selected player's historical seasons, gameweek history, and future fixture context through bounded, retryable, resumable source requests without deleting the last known good history when a request fails.

#### Scenario: Player history refresh succeeds
- **WHEN** a player history request returns valid source data
- **THEN** the system upserts the player's season and gameweek facts and associates them with the correct season and gameweek

#### Scenario: A history request fails transiently
- **WHEN** a player history request times out or returns a retryable server or rate-limit response
- **THEN** the system retries according to configured limits, records the failed work item, and retains previously stored history

### Requirement: Report sync freshness and partial failure
The system SHALL expose the last successful canonical sync time, current sync state, completed stages, failed stages or work items, dataset freshness, and a human-readable warning through the sync-status API.

#### Scenario: The latest sync is partially complete
- **WHEN** the season catalog succeeds but one or more public history or live-data work items fail
- **THEN** the status identifies the run as partial, names the failed stage or work item, and provides the last successful normalized timestamp for each affected dataset

#### Scenario: No sync has completed
- **WHEN** the application has no successful public canonical sync
- **THEN** the status reports that data is unavailable and the frontend can render an actionable empty state

### Requirement: Preserve source diagnostics without exposing raw source contracts to clients
The system SHALL retain endpoint-level sync metadata and bounded raw payload diagnostics for failed or sampled requests, while API responses use stable application field names and never expose authentication material.

#### Scenario: A source payload cannot be normalized
- **WHEN** a source response passes transport checks but fails schema validation
- **THEN** the system records the endpoint, parameters, validation error, response checksum, and diagnostic payload reference without committing invalid normalized rows
