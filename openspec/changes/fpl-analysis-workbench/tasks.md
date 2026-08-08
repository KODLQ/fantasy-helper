## 1. Shared analytical foundation

- [ ] 1.1 Define shared metric contracts, actual/provisional/estimated state, freshness, and algorithm-version response fields.
- [ ] 1.2 Add analytical read models for ownership, player contributions, fixture horizons, availability, and manager decision outcomes.
- [ ] 1.3 Add calculation services with centralized metric definitions and deterministic tests.
- [ ] 1.4 Add cache keys, bounded scopes, pagination/member limits, and partial-input reporting.
- [ ] 1.5 Publish shared response envelope, formula version, snapshot identity, and error contracts.
- [ ] 1.6 Add contract tests for actual/provisional/estimated/partial/stale/unavailable states.

## 2. League and gameweek learning

- [ ] 2.1 Implement league intelligence summaries, rival threats, ownership, common-player, and differential metrics.
- [ ] 2.2 Implement gameweek autopsy calculations for points, captaincy, bench, transfers, hits, and rival differences.
- [ ] 2.3 Implement live/provisional outcome handling and automatic-substitution explanations.
- [ ] 2.4 Add league and gameweek pages with omitted-member, stale-data, and incomplete-input states.
- [ ] 2.5 Add league summary/rival/autopsy API routes with explicit scope validation.
- [ ] 2.6 Add formula tests for overlap, differential contribution, captain delta, bench points, gross/net points, and rival differences.

## 3. Transfer and fixture research

- [ ] 3.1 Implement transfer-laboratory scenario validation and non-persistent calculations.
- [ ] 3.2 Add transfer cost, budget, fixture horizon, and alternative-scenario comparisons.
- [ ] 3.3 Add historical counterfactual replay with deadline information cutoffs.
- [ ] 3.4 Implement fixture-swing rankings, differential finder, and availability-impact explanations.
- [ ] 3.5 Add research pages and browser tests for scenario validation and fixture/differential filters.
- [ ] 3.6 Add transfer scenario API contracts, horizon limits, assumptions, and save-to-planning integration tests.

## 4. Recommendation evaluation and live center

- [ ] 4.1 Implement historical recommendation replay with information-boundary enforcement.
- [ ] 4.2 Add algorithm/weight comparison and aggregate evaluation metrics.
- [ ] 4.3 Implement live gameweek polling, provisional totals, rival movement, captaincy, and substitution state.
- [ ] 4.4 Add configurable alerts for availability, price, fixture, and recommendation changes.
- [ ] 4.5 Add backtest, live-center, and alert verification with representative historical fixtures.
- [ ] 4.6 Add deadline-cutoff replay fixtures proving future leakage is rejected.
- [ ] 4.7 Add live stale-refresh, rank-coverage, and alert deduplication tests.

## 5. Season storyline and handoff

- [ ] 5.1 Implement versioned season event ledger and turning-point ranking.
- [ ] 5.2 Add season storyline UI with source links, outcome labels, and regenerated-version metadata.
- [ ] 5.3 Add end-to-end coverage across sync, league comparison, autopsy, transfer simulation, backtest, live state, alerts, and storyline.
- [ ] 5.4 Document metric definitions, estimation limitations, retention, and cache behavior.
- [ ] 5.5 Add storyline thresholds, source references, incomplete intervals, and deterministic ordering tests.
