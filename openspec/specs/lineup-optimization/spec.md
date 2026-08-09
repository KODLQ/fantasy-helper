# lineup-optimization Specification

## Purpose
TBD - created by archiving change fpl-research-assistant-foundation. Update Purpose after archive.
## Requirements
### Requirement: Generate a constraint-safe lineup recommendation
The system SHALL generate a recommended starting XI, bench order, captain, and vice-captain from the saved 15-player planning squad while satisfying every squad-planning constraint.

#### Scenario: A valid squad is optimized
- **WHEN** the user requests recommendations for a valid planning squad
- **THEN** the system returns a legal lineup, bench order, captain, and vice-captain for the active gameweek

#### Scenario: The planning squad is invalid
- **WHEN** the user requests recommendations for a squad with validation errors
- **THEN** the system returns the validation errors and does not return a misleading lineup recommendation

### Requirement: Use transparent configurable signals
The system SHALL score eligible players using documented normalized signals for recent form, expected minutes proxy, fixture difficulty, recent returns, and value, with request-level weights that default to the documented baseline.

#### Scenario: User changes recommendation weights
- **WHEN** the user submits valid custom weights
- **THEN** the recommendation reflects those weights and returns the effective weights used for the calculation

#### Scenario: User submits invalid weights
- **WHEN** a weight is negative, non-numeric, or outside the allowed range
- **THEN** the API rejects the request with a structured validation error and does not generate a recommendation

### Requirement: Explain every recommendation
The system SHALL return each selected player's overall score, factor contributions, relevant fixture context, and a concise explanation of why the player was selected or benched.

#### Scenario: User reviews an optimized captain
- **WHEN** the recommendation response is displayed
- **THEN** the captain and vice-captain include their scores, contributing factors, and the reason they ranked above other eligible starters

### Requirement: Make recommendation results reproducible
The system SHALL use deterministic tie-breakers and include the active season, gameweek, data snapshot timestamp, and algorithm version in every recommendation response.

#### Scenario: Two players have equal scores
- **WHEN** eligible players receive equal calculated scores
- **THEN** the system resolves the tie using the documented deterministic order and returns the same result for identical inputs and snapshot data
