## ADDED Requirements

### Requirement: Import official FPL data into a normalized season snapshot
The system SHALL import the configured official FPL season snapshot and fixture feed and persist normalized seasons, gameweeks, teams, players, and fixtures keyed by their stable source IDs.

#### Scenario: Initial season sync succeeds
- **WHEN** an operator starts a sync against a valid FPL source
- **THEN** the system stores the current season snapshot and fixtures, records a successful sync run, and exposes the imported season as the active season

#### Scenario: Repeating a sync is idempotent
- **WHEN** the same source data is synchronized more than once
- **THEN** the system updates matching source-ID rows without creating duplicate seasons, teams, players, fixtures, or gameweek records

### Requirement: Refresh player historical data safely
The system SHALL retrieve and normalize each selected player's season history and gameweek history through bounded, retryable source requests without deleting the last known good history when a request fails.

#### Scenario: Player history refresh succeeds
- **WHEN** a player history request returns valid source data
- **THEN** the system upserts the player's history rows and associates them with the current season and gameweek

#### Scenario: A history request fails transiently
- **WHEN** a history request times out or returns a retryable server response
- **THEN** the system retries according to configured limits, records the failed batch details, and retains previously stored history

### Requirement: Report sync freshness and partial failure
The system SHALL expose the last successful sync time, current sync state, completed stages, failed stages, and a human-readable warning through an API health/sync-status response.

#### Scenario: The latest sync is partially complete
- **WHEN** the season snapshot succeeds but one or more player history batches fail
- **THEN** the status response identifies the run as partial, names the failed stage or batch, and provides the last successful normalized data timestamp

#### Scenario: No sync has completed
- **WHEN** the application has no successful sync run
- **THEN** the status response reports that data is unavailable and the frontend can render an actionable empty state

### Requirement: Preserve source diagnostics without exposing raw source contracts to clients
The system SHALL retain endpoint-level sync metadata and a bounded raw payload diagnostic for failed or sampled requests, while API responses use stable application field names.

#### Scenario: A source payload cannot be normalized
- **WHEN** a source response passes transport checks but fails schema validation
- **THEN** the system records the endpoint, validation error, response checksum, and diagnostic payload reference without committing invalid normalized rows
