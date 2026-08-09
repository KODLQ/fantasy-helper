# fpl-data-sync Specification

## Purpose
TBD - created by archiving change fpl-research-assistant-foundation. Update Purpose after archive.
## Requirements
### Requirement: Import official FPL data into a normalized season snapshot
The system SHALL import an explicitly configured current-official or historical source profile into PostgreSQL, persist raw source observations, and normalize season-scoped seasons, phases, gameweeks, settings, teams, player snapshots, fixtures, and gameweek facts keyed by stable source IDs. The system SHALL validate the expected season ID and name before canonical writes and SHALL record source kind, checksums, normalization version, supported datasets, completeness, and import time as provenance.

#### Scenario: Initial current-season sync succeeds
- **WHEN** an operator starts a sync against a valid official-current FPL source profile
- **THEN** the system stores the reconciled current season catalogue and public performance facts, records a successful database-backed sync run, and marks only that season as current

#### Scenario: Historical archive import succeeds
- **WHEN** an operator deliberately imports a valid retained-snapshot or historical-archive profile
- **THEN** the system stores that season under its declared scope with historical state and provenance without changing the official current season

#### Scenario: Repeating an import is idempotent
- **WHEN** the same source profile and source data are imported more than once
- **THEN** the system updates matching season-scoped rows and records source observations without creating duplicate entities or facts

#### Scenario: Source identity does not match configuration
- **WHEN** payload metadata or the archive manifest cannot be reconciled with the configured season ID and expected name
- **THEN** the system fails the import before canonical writes, records diagnostics, and does not relabel the payload as another season

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

### Requirement: Enforce season-aware synchronization policy
The synchronization system SHALL permit scheduled catalog, fixture, live, and finalization refreshes only for the reconciled official-current season. Historical seasons SHALL be changed only by deliberate, idempotent backfill or reconciliation from their configured retained or archive source.

#### Scenario: Scheduler targets current season
- **WHEN** a scheduled refresh runs for the reconciled official-current profile
- **THEN** the system executes the supported refresh scopes and records the current season ID in the run scope

#### Scenario: Live refresh targets historical season
- **WHEN** a caller requests a live or scheduled refresh for a historical source profile
- **THEN** the API rejects the request with a stable non-retryable scope error and creates no live work items

#### Scenario: Operator deliberately reconciles historical archive
- **WHEN** an operator starts an allowed historical backfill or reconciliation scope
- **THEN** the system imports only datasets declared by that source profile and reports unsupported or absent datasets as missing

#### Scenario: Historical source is unavailable
- **WHEN** no retained payload or configured archive supplies the requested historical season
- **THEN** the system reports the season as unavailable for import and does not reconstruct detailed facts from aggregate prior-season records

### Requirement: Preserve one current-season invariant
The warehouse SHALL maintain at most one official-current season, and historical imports or user navigation SHALL NOT modify that designation.

#### Scenario: A new official season is reconciled
- **WHEN** an official-current profile for a later season passes identity validation
- **THEN** the warehouse atomically marks the new season current and the prior official season historical while retaining all prior canonical facts

#### Scenario: Historical data is imported after rollover
- **WHEN** a prior season archive is imported or reconciled after a newer official season became current
- **THEN** the newer season remains current and the imported prior season remains historical
