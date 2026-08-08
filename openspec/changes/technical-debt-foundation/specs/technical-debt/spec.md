## ADDED Requirements

### Requirement: Use one authoritative migration history

The system SHALL apply ordered files from `db/migrations` and SHALL NOT maintain a second copy of schema DDL in application code.

#### Scenario: Backend starts against a migrated database
- **WHEN** the backend starts
- **THEN** it verifies database connectivity and uses the already-applied schema without rewriting migration history

#### Scenario: Deployment applies a new migration
- **WHEN** deployment finds unapplied migration files
- **THEN** it applies them in order and fails before serving traffic if any migration fails

### Requirement: Persist one sync run lifecycle

The system SHALL associate running, stage, partial, success, and failed updates with one `sync_runs` record and SHALL persist the actual source checksum.

#### Scenario: A sync completes partially
- **WHEN** the snapshot succeeds and one history batch fails
- **THEN** one run is marked partial, its failed stage is recorded, and its checksum matches the source response checksum

#### Scenario: The service shuts down during sync
- **WHEN** the process receives a termination signal
- **THEN** the sync context is cancelled and the server waits for the coordinator before exiting

### Requirement: Retain last-known-good history

The system SHALL merge successful history results into the current snapshot and SHALL retain existing history for failed players.

#### Scenario: A player history endpoint fails
- **WHEN** a history request fails after retries
- **THEN** the player keeps its previous history in API responses and the sync status identifies the failure

### Requirement: Identify seasons explicitly

The system SHALL not hard-code the active season identity. The source configuration SHALL provide or derive an explicit source season ID and display name.

### Requirement: Scale snapshot persistence

The repository SHALL batch or bulk-write repeated snapshot entities and SHALL avoid one database round trip per player-history row.

### Requirement: Harden client and deployment boundaries

The frontend SHALL handle non-JSON failures, cancellation, timeouts, and stale responses. Deployment SHALL run migrations and health checks deterministically, and integration tests SHALL reject unsafe database targets.
