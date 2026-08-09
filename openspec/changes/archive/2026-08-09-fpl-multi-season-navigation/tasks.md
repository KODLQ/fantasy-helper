## 1. Contract and Schema Foundation

- [x] 1.1 Inventory every backend query, route, frontend request, cache key, and hard-coded label that currently relies on `is_current` or an implicit season, and record the migration matrix in the change notes.
- [x] 1.2 Define typed backend models for season catalogue items, source kinds, completeness, available gameweeks, default gameweek, and resolved season scope using the common response vocabulary.
- [x] 1.3 Add an additive migration for required season source-profile/provenance metadata and a database constraint or partial unique index enforcing at most one current season.
- [x] 1.4 Add migration up/down and PostgreSQL integration tests proving multiple historical seasons coexist and the one-current-season invariant is enforced.
- [x] 1.5 Define and validate the versioned historical archive manifest containing expected season identity, source kind, supported datasets, payload checksums, and archive version.

## 2. Season Catalogue and Resolution

- [x] 2.1 Add repository methods that list queryable seasons with ordered gameweeks, source kind, latest import time, dataset freshness, completeness, missing inputs, and warnings without requiring source availability.
- [x] 2.2 Implement the deterministic default-season and current/historical default-gameweek rules as shared backend domain functions.
- [x] 2.3 Add an explicit season resolver that distinguishes unknown season, unavailable dataset, partial dataset, and temporary omitted-scope compatibility behavior.
- [x] 2.4 Refactor current-season repository helpers so resolved season IDs are passed into player, detail, fixture, snapshot, squad, and recommendation reads.
- [x] 2.5 Include season ID in backend analytical/cache keys and verify no season-scoped entity is looked up by source ID alone.
- [x] 2.6 Add repository tests with at least two seasons containing overlapping player, team, fixture, and gameweek source IDs and assert strict isolation.

## 3. Season API Contract

- [x] 3.1 Implement `GET /api/v1/seasons` using the common collection envelope and newest-first ordering.
- [x] 3.2 Add `seasonId` parsing and validation to every existing season-dependent version-one endpoint and echo the resolved ID in `meta.scope.seasonId`.
- [x] 3.3 Return `SEASON_NOT_FOUND`, `SEASON_DATA_UNAVAILABLE`, partial freshness, and omitted-scope deprecation warnings through the shared success/error contracts exactly as specified.
- [x] 3.4 Add API contract tests for empty, current-plus-historical, partial, unknown, unavailable, omitted-scope, and overlapping-identity cases.
- [x] 3.5 Update generated or handwritten frontend API types and fixtures from the validated season catalogue and scoped response contracts.

## 4. Current and Historical Import Policy

- [x] 4.1 Replace the single global source-season configuration with explicit typed source profiles for official-current, retained-snapshot, and historical-archive sources.
- [x] 4.2 Validate configured season ID/name and manifest checksums before canonical writes and persist source kind, import version, completeness, and provenance on resulting snapshots.
- [x] 4.3 Implement deliberate retained-payload replay and versioned historical-archive import paths that normalize through the warehouse's existing canonical adapters.
- [x] 4.4 Prevent historical imports from changing `is_current`, and atomically roll the current marker forward only after a new official-current profile passes identity validation.
- [x] 4.5 Reject scheduled/live/finalization refresh scopes for historical profiles with a stable non-retryable error and no created work items.
- [x] 4.6 Report unsupported or absent historical datasets as missing and never expand aggregate prior-season summaries into fabricated detailed facts.
- [x] 4.7 Add sync unit and PostgreSQL integration tests for current refresh, season rollover, historical import, repeated import, identity mismatch, unavailable archive, unsupported dataset, and forbidden historical live refresh.

## 5. Frontend Season Context

- [x] 5.1 Add a season catalogue client/query with an independent cache key and loading, empty, partial, and error handling.
- [x] 5.2 Implement one application-shell season context whose precedence is explicit URL, valid remembered season, server current season, then newest imported season.
- [x] 5.3 Represent selected season and gameweek as `season` and `gameweek` URL query parameters and synchronize browser Back/Forward navigation without duplicate history entries.
- [x] 5.4 Remember a valid selection locally for unscoped visits, discard unavailable remembered values, and ensure local state never overrides an explicit URL.
- [x] 5.5 Replace the hard-coded shell season/gameweek display with an accessible keyboard-operable selector using server labels and textual current/historical/partial status.
- [x] 5.6 Preserve the same gameweek when valid after a season switch; otherwise select the destination season's deterministic default before issuing dependent requests.
- [x] 5.7 Clear or revalidate season-scoped player, team, fixture, squad, manager, league, comparison, and analysis selections when the season changes.
- [x] 5.8 Add season ID to every frontend API request and query key, cancel obsolete requests, and discard responses whose scope does not match active URL state.
- [x] 5.9 Implement season-not-found and season-data-unavailable views with navigation to available seasons and no silent redirect.
- [x] 5.10 Keep the selector mounted and usable when an independent content request fails.

## 6. Frontend Unit and Component Coverage

- [x] 6.1 Test catalogue mapping, sorting, completeness labels, and all deterministic season/gameweek default branches.
- [x] 6.2 Test URL-versus-local precedence, invalid remembered selection recovery, explicit unknown URL preservation, and browser history restoration.
- [x] 6.3 Test gameweek preservation/fallback and clearing or revalidation of every currently implemented season-scoped dependent control.
- [x] 6.4 Test selector accessible name, keyboard interaction, focus behavior, loading/empty/error states, and textual historical/partial indicators.
- [x] 6.5 Test request cancellation and a controlled out-of-order response race proving prior-season data cannot render after a switch.

## 7. Playwright End-to-End Coverage

- [x] 7.1 Add deterministic two-season API/database fixtures with overlapping source IDs and different names, gameweeks, players, and fixture results.
- [x] 7.2 Test default selection and verify the shell, URL, player list, player detail, fixture data, and response scope all identify the same season.
- [x] 7.3 Test switching seasons with both a preserved gameweek and an invalid gameweek fallback, asserting no mixed-season content appears.
- [x] 7.4 Test direct scoped links, reload, remembered selection, Back/Forward navigation, and two browser tabs using different seasons.
- [x] 7.5 Test unknown URL season, unavailable dataset, partial season, empty catalogue, content-request failure, and recovery through the selector.
- [x] 7.6 Test keyboard-only selection and automated accessibility assertions for selector labels, status text, focus, and error announcements.
- [x] 7.7 Test a delayed prior-season response after a rapid switch and prove it is cancelled or discarded.
- [x] 7.8 Test that selecting a historical season neither changes the database current marker nor starts a sync, and that historical live sync is rejected.

## 8. Documentation and Release Verification

- [x] 8.1 Document source-profile configuration, historical archive manifest/replay commands, official-feed limitations, URL parameters, compatibility warnings, and rollback behavior.
- [x] 8.2 Update sample environment files and operational diagnostics without committing archive payloads, credentials, or licensed third-party data.
- [x] 8.3 Run backend unit, race, PostgreSQL integration, migration, vet, and coverage checks and resolve all failures.
- [x] 8.4 Run frontend lint, unit/component coverage, production build, and the complete Playwright suite in headless and headed modes and resolve all failures.
- [x] 8.5 Run smoke tests, `openspec validate --changes --strict`, and the portfolio dependency/ownership validator; record the commands and results in the handoff.
- [x] 8.6 Confirm downstream manager, league, analysis, recommendation, live, and optimal-team implementations consume this shared season context and do not introduce duplicate selectors or implicit-current queries.
