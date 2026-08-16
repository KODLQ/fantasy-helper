## Context

The workbench consumes public warehouse facts, authenticated user ownership, manager/league snapshots, and the common response contract. Its previous design mixed shared semantics with seven domain implementations. This umbrella now coordinates those domains without becoming a second source of requirements.

## Roadmap boundaries

The umbrella owns only:

- dependency order and release gates;
- cross-child navigation and handoff behavior;
- common response/freshness/provenance compatibility checks;
- shared authentication and ownership smoke checks;
- cross-workbench performance, accessibility, and Playwright verification.

The child changes own all domain-specific formulas, source inputs, persistence, route details, UI behavior, and domain test fixtures.

## Delivery graph

```text
fpl-public-data-warehouse
        │
        ├──────────────▶ local-user-authentication
        │                         │
        └──────────────▶ fpl-manager-league-sync
                                  │
                                  ▼
                    fpl-analysis-workbench children
                      │        │          │
                      │        │          └── fpl-live-alerts-storyline
                      │        └───────────── fpl-recommendation-evaluation
                      └────────────────────── fpl-league-gameweek-insights
                                             fpl-transfer-fixture-research
                                  │
                                  ▼
                    fpl-analysis-workbench integration gate
                                  │
                                  ▼
                       fpl-optimal-team-learning
```

The four children may be implemented in parallel after the warehouse, authentication, and manager prerequisites are complete. The umbrella's remaining implementation is a post-child integration and release gate, not a prerequisite for starting a child. The optimal-team change consumes the finalized warehouse, manager, child-analysis, and umbrella integration contracts but remains a separate retrospective engine.

## Shared integration contract

All children SHALL use the warehouse `common-response-contract` and SHALL include explicit season/gameweek/horizon scope, request ID, snapshot provenance, algorithm/ruleset versions where applicable, coverage, missing inputs, warnings, and the state values `actual`, `provisional`, `estimated`, `partial`, `stale`, or `unavailable`. No child may introduce a competing top-level response envelope.

Cross-child links SHALL be explicit and user-owned: transfer simulations may hand off to planning only after confirmation; league/autopsy pages link to manager snapshots; recommendation evaluations link to their algorithm version; live/storyline pages link to source intervals; optimal-team comparisons link to manager history.

## Release gates

1. Warehouse, response, source, and authentication contracts pass strict validation and integration tests.
2. Manager/league sync and ownership isolation pass API and Playwright tests.
3. League/gameweek and transfer/fixture children pass formula, scope, partial-data, and browser tests.
4. Recommendation evaluation and live/storyline children pass cutoff, freshness, alert, and browser tests.
5. The umbrella cross-workbench suite verifies navigation, auth scope, handoffs, common loading/error/partial states, accessibility, and no private-data leakage.

## Non-goals

This change does not implement a shared calculation service, analysis database tables, domain APIs, or pages. Those belong in child changes.
