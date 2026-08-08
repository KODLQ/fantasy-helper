## ADDED Requirements

### Requirement: Maintain one planning squad
The system SHALL allow the user to create and update one default planning squad containing exactly 15 distinct players from the active season, with a saved name and last-updated timestamp.

#### Scenario: User adds a valid player to the plan
- **WHEN** the user adds an active-season player and the resulting squad remains valid
- **THEN** the system saves the player selection and returns the updated squad with constraint totals

#### Scenario: User attempts to add a duplicate player
- **WHEN** the user adds a player already in the planning squad
- **THEN** the system rejects the update with a validation error and leaves the existing squad unchanged

### Requirement: Enforce squad composition and budget rules
The system SHALL validate the official squad constraints of 2 goalkeepers, 5 defenders, 5 midfielders, and 3 forwards; no more than 3 players from one club; and a total purchase cost no greater than the configured budget of 100.0.

#### Scenario: Squad exceeds a club limit
- **WHEN** an update would produce a fourth player from the same club
- **THEN** the system rejects the update and identifies the club and the violated limit

#### Scenario: Squad exceeds the budget
- **WHEN** an update would make the total purchase cost greater than 100.0
- **THEN** the system rejects the update and reports current cost, attempted cost, and remaining budget

### Requirement: Validate lineup, bench, and captain choices
The system SHALL validate a starting XI of one goalkeeper and a legal 3-5-2, 3-4-3, 4-5-1, 4-4-2, 4-3-3, 5-4-1, 5-3-2, or 5-2-3 formation, plus four bench players, a starting captain, and a distinct starting vice-captain.

#### Scenario: User saves a legal lineup
- **WHEN** the selected starting XI, bench order, captain, and vice-captain satisfy all rules
- **THEN** the system saves the lineup configuration and returns no validation errors

#### Scenario: User selects an invalid captain
- **WHEN** the captain is not in the starting XI
- **THEN** the system rejects the lineup update and explains that captain and vice-captain must be selected from starters

### Requirement: Explain planning validation errors
The system SHALL return structured validation errors containing a stable code, affected player or rule, current value, and required value where applicable.

#### Scenario: Multiple constraints fail
- **WHEN** a submitted squad violates more than one constraint
- **THEN** the response includes all detected errors in a stable order and does not partially save the update
