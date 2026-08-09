## Context

PostgreSQL already stores seasons, gameweeks, teams, players, fixtures, facts, and dataset snapshots with season-scoped identities. Production research repositories and the React shell nevertheless select the row marked `is_current`, and the shell displays a hard-coded season and gameweek. This makes historical rows technically retainable but not safely browsable.

The official FPL public API presents the active season rather than a complete historical catalogue. Complete historical seasons therefore come from retained raw observations or an explicitly configured archive import. Aggregate `history_past` records are not substitutes for historical players, fixtures, prices, picks, or gameweek facts.

This change depends on `fpl-public-data-warehouse`. It owns selection and navigation behavior; the warehouse remains the authority for canonical season identities, source profiles, normalization, snapshots, and the modified `fpl-data-sync` contract. Manager, league, analysis, live, recommendation, and optimal-team changes consume the selected season as part of their scope.

## Goals / Non-Goals

**Goals:**

- Make every imported, queryable season discoverable through one stable API.
- Give the application one URL-backed selected-season context and an accessible global selector.
- Ensure every season-dependent read resolves an explicit canonical season before querying repositories.
- Prevent cross-season joins, cache collisions, stale UI selections, and silent fallback.
- Distinguish current live synchronization from deliberate, provenance-preserving historical imports.
- Define deterministic defaults and complete empty/error behavior that can be covered end to end.

**Non-Goals:**

- Reconstruct historical seasons that are not available from retained or configured source data.
- Execute transfers or mutate an official FPL account.
- Match the same real-world player across seasons with a permanent cross-season identity.
- Provide cross-season statistical normalization or comparison formulas; analysis changes own those calculations.
- Store a user's preferred season in their account in this delivery. The URL is authoritative and local storage is only a convenience fallback.

## Decisions

### 1. Use the warehouse season source ID as the public identifier

The API and URL use the stable integer `seasons.source_id`, where `2026` represents the season labelled `2026/27`. Database surrogate IDs remain private. Labels are server-provided display data and are never parsed to identify a season.

This avoids leaking database IDs and keeps imported references stable across restores. A free-form slug was rejected because it creates a second identity mapping and makes archive validation harder.

### 2. Add one season catalogue endpoint

`GET /api/v1/seasons` returns the common collection envelope. Each item contains:

- `id`, `name`, and `state` (`current` or `historical`);
- `availableGameweeks` ordered by source ID;
- `defaultGameweek`, selected using the rules below;
- catalogue and analytical `freshness`/`completeness` summaries;
- `sourceKind` (`official-current`, `retained-snapshot`, or `historical-archive`);
- `lastImportedAt` and machine-readable warnings.

The catalogue includes only seasons whose normalized catalogue is queryable. An incomplete season remains listed when its identity and at least one gameweek/queryable dataset exist, with `partial` state and missing inputs. A failed source configuration that produced no queryable catalogue appears in sync diagnostics, not as a selectable season.

The endpoint is intentionally separate from sync status: navigation should remain available when the source is offline.

### 3. Resolve defaults deterministically

The selected season precedence is:

1. a valid `season` URL parameter;
2. a valid locally remembered season when the URL omits the parameter;
3. the catalogue item marked current;
4. otherwise the imported season with the greatest source ID.

If the URL explicitly names an unknown or unavailable season, the application renders a season-not-found state and does not substitute another season. If a remembered value is no longer available, it is discarded and the normal default is used with a non-blocking notice.

For a current season, `defaultGameweek` is the source-marked current gameweek, then the next gameweek, then the latest imported gameweek. For a historical season, it is the greatest imported finalized gameweek. If none is finalized, it is the greatest imported gameweek; if no gameweek is available, it is null.

### 4. Make the URL the authoritative browser state

The application shell owns `season` and `gameweek` query parameters, for example `?season=2026&gameweek=4`. The selector updates navigation history so Back and Forward restore the complete scope. The chosen season is also remembered locally for visits without a season parameter, but local state never overrides an explicit URL.

On a season change, the application preserves the selected gameweek only when that gameweek is available in the destination season. Otherwise it replaces it with that season's deterministic default. Player, team, fixture, manager, league, squad, comparison, and analysis selections whose identities are season-scoped are cleared or revalidated before dependent requests run.

### 5. Resolve an explicit season at the API boundary

Season-dependent endpoints accept `seasonId` and pass a resolved, non-zero season scope through services and repositories. Responses echo it in `meta.scope.seasonId`. Unknown seasons return `404 SEASON_NOT_FOUND`; known seasons lacking the requested dataset return a successful partial/unavailable response or `409 SEASON_DATA_UNAVAILABLE` according to whether a meaningful result exists. Neither case may query the current season as a fallback.

During migration, omitted `seasonId` on existing version-one routes resolves to the current/default season and returns a deprecation warning in `meta.warnings`. The first-party frontend always sends `seasonId`. A later versioned API can make it mandatory without blocking this rollout.

Repository methods, analytical cache keys, query-library keys, and persisted derived results include season ID. Player and team source IDs are treated as unique only inside that scope.

### 6. Keep `is_current` separate from user selection

`seasons.is_current` describes the source's active official season and is used for refresh policy; it is not a user preference. Selecting a historical season never changes it. Importing a historical archive never demotes the official current season. At most one season may be current.

This preserves scheduler behavior while allowing many clients or browser tabs to view different seasons safely.

### 7. Model source profiles explicitly

Each import resolves an explicit source profile containing season ID, expected name, source kind, base location, supported datasets, and whether live refresh is permitted. The official profile may refresh only the one reconciled current season. Historical profiles are read-only inputs backed by retained payloads or a configured archive and run only through deliberate backfill/reconciliation commands.

The importer validates the profile identity against payload metadata or a signed/versioned archive manifest before canonical writes. It records source kind, checksums, normalizer version, completeness, and import time in snapshot provenance. Missing historical datasets remain missing; current-season payloads are never relabelled as historical data.

### 8. Treat selection changes as cancellable scope changes

Frontend requests use scope-aware query keys and cancellation. A response for a previously selected season cannot populate the new season's view. The selector remains usable when a content request fails because its catalogue request has an independent cache and error boundary.

The selector exposes an accessible label, keyboard operation, current value, loading state, and season status text. Historical seasons are labelled as historical when visual context would otherwise be ambiguous.

### 9. Preserve compatibility while migrating reads

Implementation starts with catalogue/repository primitives, then threads explicit scope through existing player and fixture reads before enabling the selector. This prevents a UI that appears season-aware while backend queries still use `is_current`. Downstream feature changes must use the same season context rather than defining local selectors.

## Risks / Trade-offs

- **[Historical detail is unavailable from the official feed]** → List only queryable imported seasons, require an explicit archive/retained source profile, and show missing coverage without fabrication.
- **[Existing callers omit season]** → Keep a warning-producing version-one compatibility resolver while making all first-party calls explicit.
- **[A late response paints the wrong season]** → Include season in request and cache keys, cancel obsolete requests, and reject mismatched response scope.
- **[A historical import marks itself current]** → Separate source profile kind from selection and enforce the one-current-season invariant transactionally.
- **[URLs become stale after data removal]** → Render an explicit not-found/unavailable state with links to available seasons; never redirect silently.
- **[Catalogue freshness hides dataset gaps]** → Return per-season completeness and missing inputs rather than a single optimistic timestamp.
- **[Every downstream feature invents selection logic]** → Keep one shell season context and register this capability as a dependency of manager/analysis delivery.

## Migration Plan

1. Add source-profile/provenance fields and indexes only if the current schema cannot represent them; apply migrations before the new backend starts.
2. Implement repository season catalogue and explicit season-resolution methods without changing existing route behavior.
3. Add `GET /api/v1/seasons` and contract/integration tests using at least two seasons with overlapping player/team source IDs.
4. Thread resolved season IDs through current player, fixture, snapshot, recommendation, and planning reads; retain temporary omitted-scope compatibility warnings.
5. Add the URL-backed frontend context and replace hard-coded shell labels, then migrate all query keys and dependent controls.
6. Add retained/archive historical import fixtures and prove that scheduled/live sync rejects historical seasons.
7. Run unit, PostgreSQL integration, API contract, and full Playwright tests, including Back/Forward and stale-response races.
8. Enable the selector only after season-isolation tests pass. Roll back the UI flag/backend version if necessary; keep imported season data and additive schema intact.

## Open Questions

- Which historical archive will be approved for the first real historical backfill, and what licensing/attribution constraints does it impose?
- Should account-level preferred season replace local storage after `local-user-authentication` ships? This does not affect URL precedence.
