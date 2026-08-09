# user-workspace-ownership Specification

## Purpose
TBD - created by archiving change local-user-authentication. Update Purpose after archive.
## Requirements
### Requirement: Scope private records by user

Every persisted manager connection, active-team snapshot, planning squad, saved transfer scenario, alert, recommendation preference, and private analysis run SHALL carry a non-null owner user ID and enforce that owner in repository and API access.

#### Scenario: Private record is created
- **WHEN** an authenticated user creates a private record
- **THEN** the server derives the owner from the session and ignores any client-supplied owner ID

#### Scenario: Private record is listed
- **WHEN** a user lists private records
- **THEN** only that user’s records are returned, with common pagination and coverage metadata
