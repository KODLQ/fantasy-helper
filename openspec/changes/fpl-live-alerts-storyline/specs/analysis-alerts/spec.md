## ADDED Requirements

### Requirement: Create deduplicated user-owned alerts

The system SHALL create alerts only for changes affecting the authenticated user's configured scope, SHALL evaluate the canonical `analysis-alerts-v1` price/availability/fixture/recommendation thresholds, SHALL use deterministic identity for deduplication, and SHALL preserve source/time/severity/explanation.

#### Scenario: Repeated live poll
- **WHEN** the same source change is observed more than once
- **THEN** one alert remains and its delivery/acknowledgment state is stable

#### Scenario: Alert acknowledged
- **WHEN** the owner acknowledges an alert
- **THEN** only that user's alert state changes and the action is idempotent

#### Scenario: Threshold is not crossed
- **WHEN** a source snapshot changes but no configured alert threshold changes from false to true
- **THEN** no alert is created

#### Scenario: Starting player becomes unavailable near deadline
- **WHEN** a player in the user's starting XI becomes unavailable within 24 hours of the configured deadline
- **THEN** one critical alert is created with the threshold rule, before/after snapshots, and affected user scope
