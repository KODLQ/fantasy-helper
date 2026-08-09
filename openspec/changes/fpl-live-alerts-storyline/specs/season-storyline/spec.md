## ADDED Requirements

### Requirement: Build a versioned season event ledger

The system SHALL produce a deterministic, user-scoped storyline from manager decisions and public outcomes, rank events with the canonical `season-storyline-v1` weighted-impact formula, and include gameweek interval, source references, algorithm version, and incomplete/provisional state for every event.

#### Scenario: Storyline has a turning point
- **WHEN** a transfer, captaincy choice, rank movement, or bench outcome crosses the configured contribution threshold
- **THEN** the timeline includes an explainable event with the contributing facts and threshold/version used

#### Scenario: Event components are partially missing
- **WHEN** one event lacks one or more ranking components but has at least one valid component
- **THEN** remaining weights are renormalized, the missing component is reported, and the event is not presented as fully complete

#### Scenario: Source interval is incomplete
- **WHEN** one or more gameweeks lack required manager/public facts
- **THEN** the storyline marks the interval incomplete and does not infer a definitive turning point from missing data
