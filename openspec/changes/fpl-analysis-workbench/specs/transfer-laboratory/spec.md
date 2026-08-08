## ADDED Requirements

### Requirement: Simulate transfer scenarios
The system SHALL allow a user to submit hypothetical transfers against a squad state and return validated resulting squad, budget, transfer count, hit cost, lineup options, fixture context, and projected or historical outcome metrics.

#### Scenario: Valid transfer scenario
- **WHEN** a submitted transfer obeys squad, budget, club, and position constraints
- **THEN** the response returns the resulting squad and its scenario metrics without modifying saved planning state

#### Scenario: Invalid transfer scenario
- **WHEN** a transfer violates an FPL constraint
- **THEN** the response returns structured validation errors and no resulting squad is persisted

### Requirement: Compare transfer alternatives
The system SHALL compare multiple transfer scenarios against the current squad using consistent metrics, assumptions, and outcome-state labels.

#### Scenario: User compares two transfer plans
- **WHEN** two valid scenarios are submitted for the same gameweek and snapshot
- **THEN** the response aligns cost, budget, fixtures, expected signals, and projected/observed point differences

Each scenario response SHALL include free transfers available, free transfers used, paid transfers, hit cost, gross points, net points, horizon, objective, and assumption identifiers.

### Requirement: Support counterfactual review
The system SHALL support replaying a historical transfer decision using a declared information cutoff and SHALL distinguish hindsight results from information available at the original deadline.

#### Scenario: Historical transfer is reviewed
- **WHEN** a user selects a historical gameweek and alternative transfer
- **THEN** the response reports the alternative's observed outcome and identifies whether it uses deadline-available information or hindsight data

### Requirement: Keep simulations non-persistent by default
The system SHALL not mutate the saved planning squad when a transfer scenario is created, compared, or replayed.

#### Scenario: User simulates a transfer
- **WHEN** a valid scenario is submitted
- **THEN** the API returns a scenario result and the saved planning squad remains unchanged

#### Scenario: User explicitly saves a scenario
- **WHEN** the user confirms a scenario import through the planning workflow
- **THEN** existing squad validation and provenance rules are applied atomically
