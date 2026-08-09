## MODIFIED Requirements

### Requirement: Search and filter players
The system SHALL provide a player search endpoint and UI that support text search plus relevant per-column filters for position, club, price range, minutes, form, total points, value, and availability status. Filter, sort, and page-size changes SHALL reset the result page to one while preserving stable server-side pagination and freshness metadata.

#### Scenario: User filters midfielders by value
- **WHEN** the user selects midfielders and sets a minimum value from the relevant table columns
- **THEN** the system requests page one and returns only matching players with stable pagination and the active data freshness metadata

#### Scenario: No player matches the filters
- **WHEN** a valid query produces no matching players
- **THEN** the API returns an empty result set and the table explains that no players match while keeping filters available to adjust or clear

### Requirement: Sort research results deterministically
The system SHALL allow results to be sorted in either direction by activating relevant table headers for player name, price, form, total points, minutes, or value, with a deterministic secondary sort by player name and source ID.

#### Scenario: User sorts by form descending
- **WHEN** the user activates the form header and selects descending order
- **THEN** the result order is descending by normalized form, the header visibly and accessibly reports descending sort, and ties are resolved consistently by the documented secondary keys

#### Scenario: User reverses the active sort
- **WHEN** the user activates the currently sorted table header
- **THEN** the server query uses the opposite direction from page one and the table updates its sort indicator

## ADDED Requirements

### Requirement: Navigate player result pages
The player research UI SHALL provide functional server-side pagination with selectable page size, current result range, bounded page numbers, and boundary navigation.

#### Scenario: User opens the next player page
- **WHEN** more matching players exist than fit on the current page and the user activates next page
- **THEN** the UI requests the next server page with the current filters and sorting and displays its returned rows

#### Scenario: User changes player page size
- **WHEN** the user selects a different supported page size
- **THEN** the UI requests page one using that size and updates the displayed result range and page count

### Requirement: Render authoritative player and club data
The player research API SHALL pair every returned player with its selected-season team record and SHALL return the complete selected-season team catalogue used by club filters. The UI SHALL render player fields, club names, club abbreviations, player counts, and home/away fixture difficulty from authoritative responses without fixed demo mappings, generated club labels, hard-coded player totals, or fabricated fixture values.

#### Scenario: Current-season clubs are displayed
- **WHEN** the selected season contains official player and team records
- **THEN** every player row and card uses its related official team metadata and the club filter lists exactly the selected season's teams

#### Scenario: Historical-season clubs are displayed
- **WHEN** the user switches to a retained historical season
- **THEN** player-team relationships and club filter options are resolved from that historical season rather than reused from the current season

#### Scenario: Player team metadata is inconsistent
- **WHEN** a returned player does not have a related team in the selected-season catalogue
- **THEN** the API returns a data-integrity error and the UI does not invent a club name from the numeric team ID
