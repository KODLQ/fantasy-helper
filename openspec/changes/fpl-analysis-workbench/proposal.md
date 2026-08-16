## Why

The analysis workbench spans several domains that need independent delivery, ownership, and browser coverage. Keeping all implementation requirements in one change would create duplicate specifications and make it unclear which team owns a route or formula.

## What Changes

This change becomes a roadmap-only umbrella. It defines the shared analytical gates, dependency order, cross-workbench integration checks, and delivery ownership. It does not define child-domain requirements, APIs, formulas, database tables, or UI acceptance scenarios.

The authoritative delivery changes are:

1. `fpl-league-gameweek-insights` — league intelligence and gameweek autopsy.
2. `fpl-transfer-fixture-research` — transfer laboratory and fixture/differential research.
3. `fpl-recommendation-evaluation` — historical recommendation replay and evaluation.
4. `fpl-live-alerts-storyline` — live center, alerts, and season storyline.

Each child owns its proposal, design, specs, tasks, APIs, formulas, UI, and Playwright tests. Removed parent capability specs are intentionally not duplicated here.

## Capabilities

### New Capabilities

- `analysis-workbench-roadmap`: Coordinate delivery gates and cross-child integration only.

### Modified Capabilities

- None. Child changes own their domain capabilities.

## Impact

- The four child changes depend directly on `fpl-public-data-warehouse`, `local-user-authentication`, and `fpl-manager-league-sync` and may be implemented in parallel after those foundations pass their release gates.
- Final umbrella integration depends on all four child changes; `fpl-optimal-team-learning` follows the completed umbrella release gate.
- Depends on the shared response contract and versioned snapshot/provenance semantics owned by the warehouse.
- Adds no production endpoint or persisted domain model by itself.
- Completion means all child changes pass their own strict specs/tests plus the cross-workbench browser smoke suite.
