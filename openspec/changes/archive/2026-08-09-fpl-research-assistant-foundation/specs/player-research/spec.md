## ADDED Requirements

### Requirement: Search and filter players
The system SHALL provide a player search endpoint and UI that support text search plus filters for position, club, price range, minutes, form, total points, value, and availability status.

#### Scenario: User filters midfielders by value
- **WHEN** the user selects midfielders, sets a minimum value, and submits the research query
- **THEN** the system returns only matching players with stable pagination and the active data freshness metadata

#### Scenario: No player matches the filters
- **WHEN** a valid query produces no matching players
- **THEN** the API returns an empty result set and the UI explains that the filters can be adjusted

### Requirement: Sort research results deterministically
The system SHALL allow results to be sorted by price, form, total points, minutes, value, or selected recent performance metric, with a deterministic secondary sort by player name and source ID.

#### Scenario: User sorts by form descending
- **WHEN** the user selects form descending
- **THEN** the result order is descending by normalized form and ties are resolved consistently by the documented secondary keys

### Requirement: Inspect a player profile
The system SHALL provide a player detail view containing identity, position, club, price, availability, season totals, recent gameweek performance, historical points/minutes, and upcoming fixtures with difficulty indicators.

#### Scenario: User opens a player detail page
- **WHEN** a player exists in the active season
- **THEN** the API returns the player's normalized profile, history, upcoming fixture context, and freshness metadata

#### Scenario: User requests an unknown player
- **WHEN** the requested player ID does not exist in the active season
- **THEN** the API returns a not-found response and the UI provides a link back to research results

### Requirement: Compare a small set of players
The system SHALL allow the user to compare up to four players side by side using the same normalized fields and time window.

#### Scenario: User compares four players
- **WHEN** the user selects four valid players from the same active season
- **THEN** the UI displays aligned price, minutes, form, points, value, availability, and fixture context without mixing metric definitions

#### Scenario: User selects more than four players
- **WHEN** the user attempts to add a fifth player to comparison
- **THEN** the UI prevents the selection and explains the comparison limit
