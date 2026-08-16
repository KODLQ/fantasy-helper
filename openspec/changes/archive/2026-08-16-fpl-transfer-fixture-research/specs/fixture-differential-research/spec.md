## ADDED Requirements

### Requirement: Rank fixture swings in a bounded horizon

The system SHALL rank fixture runs for a requested season/horizon using the canonical `fixture-research-v1` formula, versioned difficulty, home/away, blank/double, and availability inputs, with a maximum horizon enforced.

#### Scenario: Fixture swing result
- **WHEN** a valid horizon has fixture data
- **THEN** the result includes per-fixture inputs, `fixtureEase`, home/away weighting, availability factor, aggregate denominator, blank/double counts, freshness, and assumptions using the versioned formula

### Requirement: Explain differentials

The system SHALL compare selected players with explicit position/price/ownership peer filters and calculate the canonical `differential-opportunity-v1` forward research index from normalized points-per-90, minutes share, fixture ease, ownership, and availability. It SHALL show the component values and SHALL NOT label the index as official expected points.

#### Scenario: Differential inputs are partial
- **WHEN** ownership, minutes, or availability is missing for a candidate
- **THEN** the candidate is marked partial/estimated and is not presented as a complete factual comparison

#### Scenario: Differential ranking is tied
- **WHEN** two candidates have the same calculated index
- **THEN** the result uses stable player ID ordering and exposes the normalization/weight version
