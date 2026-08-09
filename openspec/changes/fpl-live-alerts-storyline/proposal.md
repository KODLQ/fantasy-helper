## Why

The application should help users understand a live gameweek as it unfolds and later explain the season as a sequence of decisions and turning points. Live data and retrospective storyline need explicit provisional/stale semantics and deterministic event provenance.

## What Changes

- Add a live gameweek center with provisional points, captain multipliers, rank movement, rival progress, and likely substitutions.
- Add deduplicated availability, price, fixture, and recommendation-impact alerts for user-owned teams.
- Add a versioned season event ledger and storyline with turning points, transfers, captaincy, rank movement, and points left on the bench.
- Add stale polling, coverage, alert acknowledgment, and incomplete-interval behavior.

## Capabilities

### New Capabilities

- `live-gameweek-center`
- `analysis-alerts`
- `season-storyline`

## Impact

Depends on the warehouse, manager/league sync, local authentication, and common analysis contract. It adds live polling/read models, user-owned alerts, storyline APIs/pages, and full Playwright coverage.
