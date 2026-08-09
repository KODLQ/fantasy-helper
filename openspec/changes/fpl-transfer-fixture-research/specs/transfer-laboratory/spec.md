## ADDED Requirements

### Requirement: Simulate legal transfers without mutation

The system SHALL validate squad, budget, team-value, club-limit, formation, player-availability, free-transfer, and paid-hit rules before returning a transfer simulation, and SHALL not mutate the manager or active team.

#### Scenario: Legal transfer preview
- **WHEN** a valid transfer set is submitted for a user-owned snapshot
- **THEN** the response returns before/after squad and bank, free/paid transfers, hit cost, points/context metrics, and provenance without changing persisted team state

#### Scenario: Illegal transfer
- **WHEN** a transfer violates budget, squad, club, duplicate, availability, or deadline rules
- **THEN** the response returns field-level validation errors and no simulation result is persisted as a successful scenario

### Requirement: Enforce historical cutoff

Counterfactual replay SHALL use only inputs observed at or before the selected deadline and SHALL expose the cutoff in provenance.

#### Scenario: Future leakage
- **WHEN** a candidate calculation requires a source fact fetched after the deadline
- **THEN** the calculation fails or marks the result unavailable and identifies the leaked input
