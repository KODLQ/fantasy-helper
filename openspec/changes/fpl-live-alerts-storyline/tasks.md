## 1. Live center and polling

- [ ] 1.1 Define live snapshot, rank coverage, substitution, rival, and common freshness contracts.
- [ ] 1.2 Implement bounded polling, manual refresh, finalization detection, stale backoff, and source-age metrics.
- [ ] 1.3 Implement provisional player/team points, captain, bench/substitution, rank movement, and rival progress calculations.

## 2. Alerts

- [ ] 2.1 Implement user-owned alert rules for availability, price, fixture, and recommendation changes.
- [ ] 2.2 Add deterministic deduplication, severity/state, acknowledgment, retention, and notification-preference contracts.
- [ ] 2.3 Add integration tests for repeated polls, changed source snapshots, ownership, acknowledgment, and partial input.
- [ ] 2.4 Add formula tests for price, availability, fixture, recommendation thresholds, severity mapping, and deduplication keys.

## 3. Storyline

- [ ] 3.1 Implement event ledger, thresholds, deterministic ordering, source links, incomplete intervals, and versioned regeneration.
- [ ] 3.2 Add storyline API, gameweek navigation, event detail, and provenance UI.
- [ ] 3.3 Add tests for transfer/captaincy/bench/rank turning points and missing/provisional intervals.
- [ ] 3.4 Add formula tests for normalized event impact, missing-component weight renormalization, threshold inclusion, and deterministic ordering.

## 4. Browser and release verification

- [ ] 4.1 Add Playwright coverage for live refresh, stale/finalized states, rival scope, alert filters/acknowledgment/deduplication, storyline navigation/details, loading/empty/error/partial states, and keyboard accessibility.
- [ ] 4.2 Run the full seeded browser suite with deterministic clocks/source fixtures and verify no private alert/storyline data crosses users.
