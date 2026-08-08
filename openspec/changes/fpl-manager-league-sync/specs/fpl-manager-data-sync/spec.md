## ADDED Requirements

### Requirement: Synchronize configured manager entries
The system SHALL synchronize only explicitly configured entry IDs and persist their season summaries, season history, transfers, gameweek picks, captaincy, bench, automatic substitutions, chips, transfer costs, rank, and budget observations by season and gameweek.

#### Scenario: Entry history sync succeeds
- **WHEN** a configured entry summary or history endpoint returns valid data
- **THEN** the system stores or updates the entry's season-scoped summary and history facts without duplicating prior observations

#### Scenario: Gameweek picks sync succeeds
- **WHEN** a configured entry's gameweek picks endpoint returns valid data
- **THEN** the system stores player selection, position, multiplier, captain, vice-captain, automatic substitutions, and associated gameweek summary values

### Requirement: Persist the active FPL team as an imported squad snapshot
The system SHALL synchronize the configured manager's active FPL team, including its player membership, positions, captaincy, bench state, bank/value, chips, and source gameweek, as a distinct imported snapshot that can be compared with the application's planning squad.

#### Scenario: Authenticated active-team sync succeeds
- **WHEN** the configured `/my-team/{entry_id}/` endpoint and current-gameweek picks return valid data
- **THEN** the system stores the latest active-team snapshot linked to the manager entry, season, gameweek, and public player identities

#### Scenario: Active-team sync is unavailable
- **WHEN** the private active-team endpoint is unauthorized, expired, or not configured
- **THEN** the system retains the previous imported snapshot, reports its freshness, and does not change the planning squad

### Requirement: Isolate manager synchronization from public synchronization
The system SHALL maintain independent manager sync runs, freshness, errors, and retention so a manager failure cannot invalidate or block public season/player/fixture data.

#### Scenario: Manager authentication fails
- **WHEN** a manager-scoped endpoint returns an authentication or permission error
- **THEN** the manager status reports reauthentication or permission failure, retains prior manager data, and leaves public sync status unchanged

#### Scenario: One entry fails
- **WHEN** one configured entry cannot be synchronized
- **THEN** other configured entries continue processing and the run is marked partial with the failed entry identified

### Requirement: Protect authenticated session material
The system SHALL accept authenticated session material only through an injected secret/session provider and SHALL redact it from logs, diagnostics, API responses, and persisted raw payload metadata.

#### Scenario: Authenticated private request is made
- **WHEN** `/me/` or `/my-team/{entry_id}/` is enabled with a valid session provider
- **THEN** the request uses the injected session and stores only non-secret response data and redacted request metadata

#### Scenario: No session is configured
- **WHEN** a private endpoint is enabled without a session provider
- **THEN** the system reports the scope as unavailable without attempting a request or storing placeholder credentials

### Requirement: Provide manager decision-analysis data
The system SHALL expose stable read models joining manager picks and transfers to public player-gameweek facts, including points, captain multiplier, bench points, transfer cost, chips, and freshness.

#### Scenario: Manager reviews a completed gameweek
- **WHEN** a client requests a completed gameweek for a configured entry
- **THEN** the response includes the saved picks and decision outcome metrics joined to finalized public player facts

#### Scenario: Public facts are not finalized
- **WHEN** manager picks exist but the corresponding public gameweek facts are live or partial
- **THEN** the response marks the analysis as provisional and includes the public-data freshness warning

### Requirement: Expose scoped manager API contracts
The system SHALL expose manager summaries, histories, picks, transfers, active-team snapshots, and status through stable versioned endpoints that require explicit entry, season, and gameweek scope where applicable.

#### Scenario: Client requests a manager history
- **WHEN** a configured entry and season are requested
- **THEN** the response includes ordered gameweek summaries, snapshot identity, pagination or range metadata, and freshness state

#### Scenario: Client requests an unscoped manager dataset
- **WHEN** a request omits a required entry, season, or gameweek scope
- **THEN** the API rejects it with a structured validation error rather than returning an ambiguous latest dataset

### Requirement: Preserve manager source provenance
The system SHALL retain source endpoint, request scope, response checksum, fetched time, normalization version, and source conflict state for every manager snapshot used by analysis or squad import.

#### Scenario: Two source endpoints disagree
- **WHEN** authenticated active-team data and public picks differ for the same entry/gameweek
- **THEN** both observations remain available and the derived snapshot reports a conflict requiring reconciliation
