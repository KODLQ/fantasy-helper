## Why

The warehouse can retain season-scoped FPL facts, but the application still reads and presents one globally current season. Users need to choose any imported season consistently across research views so historical analysis, comparisons, and shared links never mix data from different years.

## What Changes

- Add an API season catalogue containing every queryable imported season, its display label, current/historical state, available gameweeks, completeness, and freshness.
- Add a global season selector to the application shell, defaulting deterministically to the current or newest available season.
- Make the selected season explicit in browser URLs and all season-dependent API requests instead of relying on server-global current-season state.
- Reconcile or clear dependent gameweek, player, fixture, squad, manager, league, and analysis selections when the season changes.
- Keep historical seasons read-only and allow live scheduled synchronization only for the current source season.
- Support deliberate historical-season imports from configured archive sources or retained source snapshots; never imply that the official current-season feed can reconstruct unavailable historical detail.
- Expose clear loading, empty, partial, stale, unavailable, and season-not-found states without silently falling back to another season.
- Add repository, API, frontend, and Playwright coverage proving season isolation, URL restoration, fallback behavior, and current-only synchronization.

## Capabilities

### New Capabilities

- `multi-season-navigation`: Discover available seasons, select one globally, preserve the selection in navigation, and scope all research experiences to that season.

### Modified Capabilities

- `fpl-data-sync`: Extend deliberate backfill and source configuration with explicit current-season versus historical-season import and refresh rules.

## Impact

- Adds season catalogue and season/gameweek availability reads to the warehouse API and repositories.
- Changes season-dependent API consumers to send a stable season identifier and receive the selected season in the common response scope.
- Replaces the hard-coded season/gameweek display in the React shell with URL-backed selection state.
- Adds historical source-profile configuration and validation while preserving the public warehouse as the owner of source ingestion and canonical season identities.
- Establishes a shared season context for manager, league, analysis, recommendation, live, and optimal-team changes.
- Depends on `fpl-public-data-warehouse`; downstream manager and analysis changes consume this contract.
