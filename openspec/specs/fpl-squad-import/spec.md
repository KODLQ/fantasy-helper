# fpl-squad-import Specification

## Purpose
TBD - created by archiving change fpl-manager-league-sync. Update Purpose after archive.
## Requirements
### Requirement: Preview an active-team import
The system SHALL provide a preview of differences between an imported active FPL team and the saved planning squad before any planning-squad mutation occurs.

#### Scenario: Imported team differs from the plan
- **WHEN** a user requests an import preview
- **THEN** the response identifies additions, removals, changed purchase/current prices, lineup, captain, vice-captain, and validation issues without modifying the saved plan

#### Scenario: Imported team matches the plan
- **WHEN** the imported snapshot and planning squad contain the same valid team state
- **THEN** the preview reports no changes and does not create a new plan version

### Requirement: Explicitly import an active team into planning
The system SHALL allow a user to create a new planning draft or explicitly replace the saved planning squad from a validated imported active-team snapshot.

#### Scenario: User imports as a new draft
- **WHEN** the user confirms an import-as-draft action
- **THEN** the system creates a planning squad draft from the imported team while preserving the existing saved planning squad

#### Scenario: User replaces the planning squad
- **WHEN** the user confirms a replace action after reviewing the preview
- **THEN** the system atomically replaces the saved planning squad with the validated imported team and records the source snapshot used

#### Scenario: Import would violate planning rules
- **WHEN** the imported team fails squad, lineup, budget, or active-season validation
- **THEN** the system rejects the mutation, returns structured validation errors, and leaves the saved planning squad unchanged

### Requirement: Never silently overwrite planning state
The system SHALL keep manager synchronization updates separate from planning-squad persistence and SHALL require an explicit user action for every planning-squad import or replacement.

#### Scenario: Manager sync runs automatically
- **WHEN** a scheduled manager sync refreshes the active-team snapshot
- **THEN** only the imported snapshot changes and the saved planning squad remains unchanged

### Requirement: Preserve import provenance
The system SHALL record the imported snapshot ID, entry ID, season, gameweek, source endpoint set, confirmation action, and resulting planning version for every successful squad import.

#### Scenario: Draft import is confirmed
- **WHEN** a user confirms an import-as-draft action
- **THEN** the resulting draft references the exact imported snapshot and retains the prior planning version

#### Scenario: Import confirmation is repeated
- **WHEN** the same snapshot and mode are submitted again
- **THEN** the operation is idempotent and does not create duplicate players or ambiguous planning versions
