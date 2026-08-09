## Decisions

### Transfer sandbox

`POST /api/v1/analysis/transfers/simulate` accepts a user-owned starting squad/snapshot, transfers in/out, target season/gameweek deadline, captain/lineup options, and a horizon. It returns a simulation ID/result with validation errors, bank/team value before/after, free transfers used, paid transfers, hit cost, projected/historical points, fixture context, and assumptions. It is read-only and has no FPL mutation side effect. A separate explicit `POST /api/v1/planning/scenarios` saves a scenario after confirmation.

Historical replay uses a strict information cutoff: prices, ownership, availability, fixtures, and recommendation inputs must have `sourceFetchedAt <= deadline`; future facts cause a validation failure, not silent leakage. Horizons are bounded by configured maximum gameweeks/players.

### Fixture and differential research

`GET /api/v1/analysis/fixtures/swing` ranks teams/players across a selected horizon using the versioned fixture difficulty source, home/away weighting, blank/double indicators, and availability state. `GET /api/v1/analysis/differentials` compares a selected player's expected opportunity/actual historical points, ownership, minutes, rotation risk, price, and fixture run against position/price peers. Every rank exposes its inputs and does not call estimates official points.

The default fixture formula is versioned as `fixture-research-v1`:

```text
fixtureEase(fixture) = (6 - difficulty) / 5
homeAwayWeight(fixture) = 1.0 for home, 0.95 for away
fixtureWeight(fixture) = homeAwayWeight × availabilityFactor
teamFixtureEase(H) = sum(fixtureEase × fixtureWeight) / sum(fixtureWeight)
```

Difficulty is the source value in the range 1–5. Blank gameweeks contribute no fixture row; double gameweeks contribute once per fixture. `availabilityFactor` is 1.0 for available, 0.5 for doubtful/unknown, and 0 for unavailable, unless a configured ruleset supplies a more specific value. The response reports fixture count, blank/double count, excluded fixtures, and denominator. Missing coverage is never silently converted into a better score.

For a forward differential research index, normalize each peer feature to [0,1] within the requested position/price peer set:

```text
opportunity = 0.40 × normalized(pointsPer90)
            + 0.25 × normalized(minutesShare)
            + 0.20 × teamFixtureEase(horizon)
            + 0.10 × (1 - ownershipShare)
            + 0.05 × availabilityFactor
```

This is a research ranking, not an official expected-points prediction. Missing features remove the candidate from a complete ranking or produce an explicitly partial row; weights and normalization version are returned. Ties sort by player ID.

### Contract and UI

All endpoints use common `{data,meta}` and error envelopes, include snapshot IDs, and identify actual/provisional/estimated values. The UI has draft transfer controls, validation before calculation, undo/reset, fixture horizon filters, ownership/position/price filters, side-by-side alternatives, and a clear “save to planning” confirmation. Saved planning data is user-owned; simulation results expire or are immutable by input identity.

### Verification

Test illegal formations, budget overspend, duplicate players, club limits, invalid price history, hit-cost math, deadline cutoff, horizon limits, stale availability, empty fixture runs, and deterministic ranking. Playwright covers every interactive control plus loading, empty, validation, stale, unavailable, and save-confirmation states.
