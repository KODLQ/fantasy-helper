## 1. Transfer laboratory

- [ ] 1.1 Define simulation input/output/error contracts and read-only transaction boundary.
- [ ] 1.2 Implement legal squad/budget/club/formation/availability validation.
- [ ] 1.3 Implement historical prices, free transfers, paid transfers, hit costs, and deadline cutoff.
- [ ] 1.4 Add bounded horizon/player limits, deterministic IDs, and stale/partial assumptions.
- [ ] 1.5 Add explicit save-to-planning handoff with user confirmation and ownership tests.

## 2. Fixture and differential research

- [ ] 2.1 Implement fixture-swing read model and versioned rank calculation.
- [ ] 2.2 Implement differential peer selection, ownership/minutes/rotation/availability context, and explanation fields.
- [ ] 2.3 Add fixture, price, ownership, availability, blank/double, empty, and incomplete fixtures.
- [ ] 2.4 Add formula tests for fixture ease, home/away weighting, availability, blank/double aggregation, normalization, missing denominators, and differential tie-breaking.

## 3. UI and verification

- [ ] 3.1 Build transfer draft/simulation UI with reset, validation, result breakdown, and save confirmation.
- [ ] 3.2 Build fixture horizon and differential filters with deterministic sort and provenance display.
- [ ] 3.3 Add unit/integration tests for legality, hit costs, future-leakage rejection, limits, and rankings.
- [ ] 3.4 Add Playwright coverage for all transfer controls, fixture/differential filters, loading/empty/error/stale/partial states, reset, and save-confirmation/cancel behavior.
