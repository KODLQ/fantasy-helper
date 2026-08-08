## ADDED Requirements

### Requirement: Build a season event timeline
The system SHALL generate a chronological timeline of manager transfers, rank changes, captaincy outcomes, differentials, bench points, and other explainable turning points.

#### Scenario: Season has synchronized history
- **WHEN** manager history, picks, transfers, and public gameweek facts are available
- **THEN** the response returns ordered events with source references, metric values, and outcome state

### Requirement: Identify turning points
The system SHALL rank significant positive and negative events by their contribution to rank or net-point change and SHALL explain the contributing decisions.

#### Scenario: Major rank drop occurs
- **WHEN** a gameweek produces a material negative rank movement
- **THEN** the timeline identifies the likely causes, including captaincy, transfers, differentials, or bench decisions

Turning-point ranking SHALL use configured thresholds and deterministic ordering by material net-point delta, rank movement, gameweek, and source ID.

### Requirement: Regenerate storyline versions
The system SHALL retain calculation and source versions for storyline events so the timeline can be regenerated after analytical definitions change.

#### Scenario: Metric definition changes
- **WHEN** a storyline calculation version is updated
- **THEN** newly generated events use the new version while prior results remain attributable to their original version

### Requirement: Expose storyline scope
The system SHALL require an entry and season for storyline requests and SHALL return source references, calculation version, incomplete intervals, and outcome state.

#### Scenario: Storyline has missing manager data
- **WHEN** one or more gameweeks lack manager picks or transfers
- **THEN** the response marks the affected interval incomplete instead of inventing a continuous narrative
