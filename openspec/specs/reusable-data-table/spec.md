# reusable-data-table Specification

## Purpose
Provide controlled, accessible tabular rendering with configurable columns, relevant filters, server-backed sorting and pagination, and explicit data states.

## Requirements

### Requirement: Configure typed table columns
The system SHALL provide a reusable typed data-table component whose column definitions specify row rendering and explicitly opt into sorting or filtering behavior.

#### Scenario: Display-only column is rendered
- **WHEN** a consumer declares a column without sort or filter configuration
- **THEN** the component renders its header and cells without irrelevant interaction controls

#### Scenario: Interactive column is rendered
- **WHEN** a consumer declares a sortable or filterable column
- **THEN** the component renders the declared accessible controls and reports changes through controlled callbacks

### Requirement: Report controlled sorting
The data table SHALL expose accessible ascending and descending sorting from declared sortable headers without reordering server-provided rows locally.

#### Scenario: User selects a new sortable column
- **WHEN** the user activates a sortable header that is not active
- **THEN** the component requests that sort key using the column's default direction and exposes the direction with `aria-sort`

#### Scenario: User reverses active sorting
- **WHEN** the user activates the currently sorted header
- **THEN** the component requests the opposite direction and updates the visible and accessible indicator

### Requirement: Filter relevant columns
The data table SHALL render text, select, minimum-number, and number-range filters only for columns that declare them and SHALL report normalized controlled values to its consumer.

#### Scenario: User changes a column filter
- **WHEN** the user enters or selects a value in a declared filter
- **THEN** the component reports the filter ID and new value while leaving fetching and domain validation to the consumer

#### Scenario: User clears table filters
- **WHEN** at least one declared filter has a value and the user activates clear filters
- **THEN** the component requests empty values for the active filters and retains the current column configuration

### Requirement: Navigate paginated results
The data table SHALL render server-controlled result counts, page size, bounded page numbers, and first, previous, next, and last navigation with correct disabled boundaries.

#### Scenario: User navigates forward
- **WHEN** another result page exists and the user activates next page
- **THEN** the component requests the next page while preserving current sort and filter controls

#### Scenario: User changes page size
- **WHEN** the user selects an allowed page size
- **THEN** the component reports the new page size and requests page one

#### Scenario: Results fit one page
- **WHEN** the total result count does not exceed page size
- **THEN** boundary navigation is disabled and the displayed result range remains accurate

### Requirement: Render complete data states
The data table SHALL render loading, error, empty, and populated states without changing the table's column structure.

#### Scenario: Query has no matches
- **WHEN** loading is complete without an error and no rows are returned
- **THEN** one full-width row displays the configured empty message

#### Scenario: Query fails
- **WHEN** the consumer supplies an error message
- **THEN** one full-width alert row displays the error instead of stale rows
