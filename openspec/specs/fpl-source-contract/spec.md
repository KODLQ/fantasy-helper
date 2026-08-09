# fpl-source-contract Specification

## Purpose
TBD - created by archiving change fpl-public-data-warehouse. Update Purpose after archive.
## Requirements
### Requirement: Implement the supported FPL endpoint contract

The source adapter SHALL explicitly support `bootstrap-static/`, `fixtures/`, `fixtures/?event={gameweek}`, `event/{gameweek}/live/`, and `element-summary/{playerId}/` with season/gameweek/player scope recorded for every observation.

#### Scenario: Bootstrap is normalized
- **WHEN** a valid bootstrap payload is received
- **THEN** the adapter captures events/gameweeks, phases, game settings, teams, players/elements, element types, and total-player metadata with source IDs and raw payload provenance

#### Scenario: Fixture and live feeds are normalized
- **WHEN** fixture or event-live data is received
- **THEN** the adapter preserves fixture identity, event, kickoff, teams, scores, finished/provisional state, fixture stats, player IDs, minutes, points, scoring/defensive stats, bonus/BPS, and source-provided expected metrics when present

#### Scenario: Player summary is normalized
- **WHEN** an element-summary payload is received
- **THEN** the adapter stores future fixtures, gameweek history, and prior-season history with player, season, round, fixture, opponent, home/away, kickoff, and difficulty scope

### Requirement: Validate source types and preserve unknown fields

The adapter SHALL parse numeric strings into typed values, preserve nullable values as null, reject invalid required types before canonical writes, and retain unknown source fields in raw JSON.

#### Scenario: Source field changes type
- **WHEN** a required source field is absent or has an incompatible type
- **THEN** the response is stored diagnostically, the affected work item is marked invalid/partial, and last-known-good canonical facts remain available

### Requirement: Require explicit season identity

The source configuration SHALL provide `sourceSeasonId` and `sourceSeasonName` or an explicit discovery mode. The adapter SHALL reconcile configured identity with bootstrap metadata and SHALL never silently use a hard-coded season.

#### Scenario: Source season mismatches configuration
- **WHEN** bootstrap identifies a different season than the configured source season
- **THEN** the sync fails the catalog stage with a stable diagnostic and does not mix rows from the two seasons
