## 1. Transfer laboratory

- [x] 1.1 Define simulation input/output/error contracts and read-only transaction boundary.
- [x] 1.2 Implement legal squad/budget/club/formation/availability validation.
- [x] 1.3 Implement historical prices, free transfers, paid transfers, hit costs, and deadline cutoff.
- [x] 1.4 Add bounded horizon/player limits, deterministic IDs, and stale/partial assumptions.
- [x] 1.5 Add explicit save-to-planning handoff with user confirmation and ownership tests.

## 2. Fixture and differential research

- [x] 2.1 Implement fixture-swing read model and versioned rank calculation.
- [x] 2.2 Implement differential peer selection, ownership/minutes/rotation/availability context, and explanation fields.
- [x] 2.3 Add fixture, price, ownership, availability, blank/double, empty, and incomplete fixtures.
- [x] 2.4 Add formula tests for fixture ease, home/away weighting, availability, blank/double aggregation, normalization, missing denominators, and differential tie-breaking.

## 3. UI and verification

- [x] 3.1 Build transfer draft/simulation UI with reset, validation, result breakdown, and save confirmation.
- [x] 3.2 Build fixture horizon and differential filters with deterministic sort and provenance display.
- [x] 3.3 Add unit/integration tests for legality, hit costs, future-leakage rejection, limits, and rankings.
- [x] 3.4 Add Playwright coverage for all transfer controls, fixture/differential filters, loading/empty/error/stale/partial states, reset, and save-confirmation/cancel behavior.
