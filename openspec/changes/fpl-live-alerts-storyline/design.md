## Decisions

### Live center

`GET /api/v1/analysis/live/{seasonId}/{gameweek}?entryId=&leagueId=` reads the latest live snapshot and returns per-player provisional points, captain multiplier, bench/likely-substitution state, rank movement, selected rival progress, unfinished fixture coverage, and source age. Polling is bounded and stops/backoffs when stale or the gameweek is finalized. No provisional rank is described as final.

### Alerts

`GET /api/v1/analysis/alerts`, `POST .../alerts/{id}/acknowledge`, and a server-side alert evaluator cover availability/status changes, price changes, fixture changes, and recommendation changes affecting a user-owned team or saved preference. Alert identity is a deterministic hash of user, type, entity, source snapshot, and meaningful change; repeated polls do not duplicate it. Each alert carries source, observed time, severity, state, and explanation.

The default threshold contract is `analysis-alerts-v1` and all thresholds are user-configurable within safe bounds:

```text
priceAlert := abs(priceAfter - priceBefore) >= max(0.1, 0.02 × priceBefore)
availabilityAlert := statusAfter ∈ {doubtful, unavailable}
fixtureAlert := abs(teamFixtureEaseAfter - teamFixtureEaseBefore) >= 0.20
                 OR blank/double status changes
recommendationAlert := entersOrLeavesTopN OR abs(scoreAfter - scoreBefore) >= 0.10
```

The evaluator emits one event when a threshold changes from false to true or when a normalized value changes materially. Repeated observations of the same normalized transition are deduplicated even when source snapshot IDs differ. Default severity is `critical` for an unavailable starting player within 24 hours of deadline, `warning` for doubtful/price/recommendation changes, and `info` for fixture context. Severity and rule version are returned. The deduplication key is `sha256(userId|alertType|entityId|normalizedBefore|normalizedAfter|ruleVersion)`; source snapshot IDs remain provenance metadata, not identity inputs.

### Storyline

`GET /api/v1/analysis/storyline/{entryId}?seasonId=` builds a deterministic event ledger from manager snapshots, transfers, captaincy, points, rank, fixture/availability events, and saved analysis decisions. Turning points are ranked by versioned contribution/threshold rules and include source references, weekly interval, actual/provisional state, and incomplete gaps. Regeneration creates a new algorithm version rather than rewriting prior results.

The default turning-point contract is `season-storyline-v1`. Normalize each available event component to [0,1] against the season population, then calculate:

```text
eventImpact = weightedMean(
  0.50 × normalized(abs(netPointsDelta)),
  0.30 × normalized(abs(rankDelta)),
  0.20 × normalized(abs(decisionOpportunityDelta))
)
```

`decisionOpportunityDelta` is captain opportunity lost, bench points left, or transfer differential contribution, depending on event type. Missing components are removed and remaining weights are renormalized; an event with no valid component is incomplete and not ranked. A turning point is included when `eventImpact >= 0.65` or a configured absolute net-point threshold is crossed. Ordering is impact descending, then gameweek, event type, and stable source/event ID. The response includes each component, denominator, threshold, and version.

### UI and verification

The live page shows freshness age and manual refresh, the alert center supports filters/acknowledgment, and the storyline supports gameweek navigation and event details. All use common envelopes and user ownership. Playwright covers live refresh/stale/finalized states, rival scope, alert deduplication/acknowledge, storyline filters/details, and every loading/empty/error/partial state.
