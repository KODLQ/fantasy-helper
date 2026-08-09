## ADDED Requirements

### Requirement: List queryable seasons
The system SHALL expose every imported season with a queryable normalized catalogue through `GET /api/v1/seasons` using the common collection response contract. Each season SHALL include its stable source ID, display name, current or historical state, ordered available gameweeks, deterministic default gameweek, source kind, last import time, freshness state, completeness, missing inputs, and warnings.

#### Scenario: Current and historical seasons are available
- **WHEN** the warehouse contains queryable catalogues for a current season and two historical seasons
- **THEN** the endpoint returns all three exactly once in newest-first order with distinct season scopes and accurate state metadata

#### Scenario: An imported season is partial
- **WHEN** a season has a queryable catalogue but one or more analytical datasets or gameweeks are missing
- **THEN** the endpoint includes the season with partial completeness and identifies the missing inputs without describing it as fully actual

#### Scenario: A failed import produced no queryable catalogue
- **WHEN** a configured historical import fails before any valid normalized catalogue exists
- **THEN** the season is absent from selectable items and the failure remains available through sync diagnostics

#### Scenario: No season is queryable
- **WHEN** the warehouse contains no queryable season catalogue
- **THEN** the endpoint returns an empty collection with unavailable freshness and an actionable warning

### Requirement: Choose deterministic season and gameweek defaults
The system SHALL choose a default season and gameweek deterministically from the returned catalogue without changing warehouse current-season state.

#### Scenario: A current season exists
- **WHEN** no explicit or remembered season is selected and exactly one queryable season is marked current
- **THEN** the application selects that season

#### Scenario: No current season exists
- **WHEN** no explicit or remembered season is selected and queryable historical seasons exist
- **THEN** the application selects the season with the greatest source ID

#### Scenario: Current-season gameweek is selected
- **WHEN** a current season has a source-marked current gameweek
- **THEN** that gameweek is its default, with next then latest imported gameweek used as deterministic fallbacks

#### Scenario: Historical-season gameweek is selected
- **WHEN** a historical season has imported gameweeks
- **THEN** its default is the greatest finalized gameweek, or the greatest imported gameweek when none is finalized

#### Scenario: A season has no gameweeks
- **WHEN** a queryable season catalogue contains no queryable gameweek
- **THEN** its default gameweek is null and season-level views remain selectable while gameweek-dependent views show unavailable state

### Requirement: Maintain one URL-backed season context
The application SHALL maintain one global selected-season context represented by the `season` URL query parameter and SHALL render the server-provided season name in the application shell.

#### Scenario: User selects another season
- **WHEN** the user selects a different available season from the global selector
- **THEN** the URL, selector label, shell context, and all season-dependent requests change to that season

#### Scenario: User opens a scoped link
- **WHEN** the application loads a URL containing a valid season and gameweek
- **THEN** it restores that scope before issuing season-dependent content requests

#### Scenario: User navigates browser history
- **WHEN** the user switches seasons and then uses Back or Forward
- **THEN** the selector and content restore the season and gameweek represented by the resulting URL

#### Scenario: Explicit URL conflicts with remembered selection
- **WHEN** a valid URL season differs from the locally remembered season
- **THEN** the URL season wins and becomes the newly remembered selection

#### Scenario: Remembered season is no longer available
- **WHEN** the URL omits season and the remembered season is absent from the catalogue
- **THEN** the application discards it, applies the deterministic default, and displays a non-blocking notice

### Requirement: Reconcile dependent scope on season changes
The application SHALL validate the selected gameweek and every season-scoped dependent selection before requesting data for a newly selected season.

#### Scenario: Same gameweek exists in destination season
- **WHEN** the user changes seasons and the selected gameweek exists in the destination season
- **THEN** the application preserves that gameweek and requests it within the new season scope

#### Scenario: Gameweek does not exist in destination season
- **WHEN** the user changes seasons and the selected gameweek is unavailable there
- **THEN** the application replaces it with the destination season's default gameweek before issuing dependent requests

#### Scenario: Selected entity belongs to prior season
- **WHEN** a selected player, team, fixture, squad, manager snapshot, league snapshot, comparison, or analysis identity is not valid in the destination season
- **THEN** the application clears or revalidates that identity and never reuses it solely because its numeric source ID matches

### Requirement: Scope every season-dependent API read explicitly
Every first-party season-dependent API request SHALL include `seasonId`, every service and repository read SHALL use the resolved season scope, and every successful response SHALL echo the resolved season in `meta.scope.seasonId`.

#### Scenario: Explicit season contains overlapping source IDs
- **WHEN** two seasons contain the same player or team source ID and a client requests one season explicitly
- **THEN** only rows belonging to the requested season contribute to the response

#### Scenario: Existing version-one caller omits season
- **WHEN** a compatibility-period request to an existing version-one route omits `seasonId`
- **THEN** the API resolves the deterministic current/default season and returns a machine-readable deprecation warning in `meta.warnings`

#### Scenario: Response scope does not match active browser scope
- **WHEN** an obsolete request completes after the user has selected another season
- **THEN** the frontend discards the response and does not render it in the active view

### Requirement: Never silently substitute a season
The system SHALL distinguish an unknown season from a known season with unavailable data and SHALL NOT silently query or display another season for either condition.

#### Scenario: URL names an unknown season
- **WHEN** the URL explicitly identifies a season absent from the catalogue
- **THEN** the application renders a season-not-found state with available-season navigation and retains the requested value for diagnosis

#### Scenario: API request names an unknown season
- **WHEN** a season-dependent endpoint receives an unknown `seasonId`
- **THEN** it returns the common error envelope with HTTP 404 and code `SEASON_NOT_FOUND`

#### Scenario: Requested dataset is unavailable
- **WHEN** the season exists but the requested dataset cannot produce a meaningful result
- **THEN** the API returns `SEASON_DATA_UNAVAILABLE` or an explicitly unavailable common response according to the endpoint contract and does not fall back to current data

#### Scenario: Requested dataset is partial
- **WHEN** the season exists and a meaningful partial result is available
- **THEN** the API returns only that season's available data with partial freshness, missing inputs, and warnings

### Requirement: Keep selection independent from source currency
Selecting a season SHALL NOT change which warehouse season is marked current, trigger synchronization, or mutate canonical data.

#### Scenario: User selects a historical season
- **WHEN** a user chooses a historical season
- **THEN** the application performs read-only queries and the official current-season marker and scheduler remain unchanged

#### Scenario: Two browser tabs select different seasons
- **WHEN** separate browser tabs select different seasons concurrently
- **THEN** each tab continues to query and display its URL-scoped season without changing the other tab or server-global state

### Requirement: Provide accessible and resilient season controls
The global selector SHALL have an accessible name, keyboard support, visible current value, and explicit loading, empty, partial, and error states, and SHALL remain independent from content-view failures.

#### Scenario: User operates selector by keyboard
- **WHEN** focus is on the season selector and the user chooses another season using the keyboard
- **THEN** the same URL and scope transition occurs as for pointer input and focus remains predictable

#### Scenario: Selected season is historical or partial
- **WHEN** the selected item is historical or has incomplete data
- **THEN** the shell exposes that status in text and does not depend on color alone

#### Scenario: Content request fails
- **WHEN** a player, fixture, or analysis request fails after the season catalogue loaded
- **THEN** the selector remains usable and the content view reports its own error without losing season context

