## Why

League and gameweek pages are the first high-value research workflow: they connect the user's synchronized team to rivals, explain the points gap, and preserve an auditable weekly autopsy.

## What Changes

- Add scoped league summaries with selected rivals, member limits, overlap, ownership, and point-gap metrics.
- Add completed/live gameweek autopsy for captaincy, bench, transfers, hits, substitutions, differentials, and rival differences.
- Add deterministic comparison snapshots and common response/freshness contracts.
- Add user-facing explanations for omitted members, partial/stale inputs, and provisional points.

## Capabilities

### New Capabilities

- `league-intelligence`
- `gameweek-autopsy`

## Impact

Depends on `fpl-public-data-warehouse`, `fpl-manager-league-sync`, and `local-user-authentication`. Adds league/gameweek analysis APIs, read models, pages, and full Playwright acceptance coverage.
