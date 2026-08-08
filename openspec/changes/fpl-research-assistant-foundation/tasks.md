## 1. Repository and local runtime

- [x] 1.1 Create the `db`, `backend`, and `frontend` directory structure with documented local prerequisites.
- [x] 1.2 Add Docker Compose for PostgreSQL, the backend, and the frontend with environment-based configuration and health checks.
- [x] 1.3 Initialize the Go backend module and React/TypeScript frontend, including formatting, linting, and test commands.
- [x] 1.4 Add a root README covering local startup, database migration, initial sync, and the local-only security scope.
- [x] 1.5 Add a base Compose file and local/dev/prod overlays with exactly three services: db, backend, and frontend.
- [x] 1.6 Add environment files and a Makefile for deploy/down commands plus the `make dev` local alias.
- [x] 1.7 Add hot-refresh local mounts, production frontend serving, service health checks, and environment-specific ports/volumes.

## 2. Database foundation

- [x] 2.1 Add the initial PostgreSQL migration for seasons, gameweeks, teams, players, fixtures, and source-ID uniqueness constraints.
- [x] 2.2 Add migrations for player season history, player gameweek history, sync runs, sync stage/batch diagnostics, and bounded raw payload diagnostics.
- [x] 2.3 Add migrations for the single planning squad, squad members, lineup selections, bench order, captain, and vice-captain.
- [x] 2.4 Add database indexes for player research filters, fixture lookups, active-season queries, and history by player/gameweek.
- [x] 2.5 Add migration verification and repository tests for idempotent upserts and planning-squad persistence.

## 3. Backend application foundation

- [x] 3.1 Implement configuration loading, structured logging, request IDs, graceful shutdown, and PostgreSQL connection management.
- [x] 3.2 Define stable `/api/v1` response types, error envelopes, freshness metadata, pagination, and validation helpers.
- [x] 3.3 Add `/healthz` and `/api/v1/sync/status` endpoints with empty, running, successful, failed, and partial-sync states.
- [x] 3.4 Add repository interfaces and transaction helpers that isolate database access from HTTP handlers and domain logic.

## 4. FPL data synchronization

- [x] 4.1 Implement the configurable FPL source adapter for the season snapshot, fixtures, and player history endpoints.
- [x] 4.2 Add payload validation and normalization for seasons, gameweeks, teams, players, fixtures, and player histories.
- [x] 4.3 Implement idempotent upserts and active-season selection from a successful season snapshot.
- [x] 4.4 Implement bounded player-history batching, retry/backoff for transient failures, timeout handling, and resumable batch status.
- [x] 4.5 Persist sync runs, stage outcomes, checksums, validation failures, and bounded diagnostics without deleting last-known-good data.
- [x] 4.6 Add a manual sync endpoint/command with duplicate-run protection and automated tests using a fake source adapter.

## 5. Player research API

- [x] 5.1 Implement paginated player search with text, position, club, price, minutes, form, points, value, and availability filters.
- [x] 5.2 Implement deterministic sorting and secondary tie-breakers for every supported research sort.
- [x] 5.3 Implement player detail responses with normalized profile data, season totals, recent history, and upcoming fixture difficulty.
- [x] 5.4 Implement a four-player comparison endpoint with consistent metric definitions and active-snapshot freshness metadata.
- [x] 5.5 Add API tests for filtering, sorting, empty results, not-found players, comparison limits, stale data, and partial sync warnings.

## 6. Squad planning and validation

- [x] 6.1 Implement the planning-squad repository and default single-workspace create/read/update endpoints.
- [x] 6.2 Implement domain validation for 15 distinct players, positional composition, 100.0 budget, three-player club limit, and active-season membership.
- [x] 6.3 Implement lineup validation for legal formations, starting XI/bench partition, captain, and vice-captain rules.
- [x] 6.4 Return stable structured validation errors and guarantee invalid updates do not partially persist.
- [x] 6.5 Add API and domain tests for valid updates, duplicates, budget/club/position failures, invalid lineups, and multi-error ordering.

## 7. Explainable lineup optimization

- [x] 7.1 Define and document normalized signal calculations, default weights, allowed weight ranges, algorithm version, and deterministic tie-breakers.
- [x] 7.2 Implement player scoring with factor contributions for form, expected minutes proxy, fixture difficulty, recent returns, and value.
- [x] 7.3 Implement constraint-aware starting XI selection, bench ordering, captain selection, and vice-captain selection for a valid 15-player squad.
- [x] 7.4 Implement the recommendation endpoint with effective weights, snapshot identity, gameweek, algorithm version, scores, fixture context, and explanations.
- [x] 7.5 Add optimizer tests for legal formations, invalid squads, custom/invalid weights, equal-score tie breaks, and reproducible responses.

## 8. Frontend research workspace

- [x] 8.1 Build the application shell with navigation for Research, Compare, Squad, and Recommendations plus a global freshness/status banner.
- [x] 8.2 Build the player research table with debounced search, filter controls, deterministic sort indicators, pagination, loading, empty, stale, and error states.
- [x] 8.3 Build the player detail view with season metrics, recent history, upcoming fixtures, and add-to-compare/add-to-squad actions.
- [x] 8.4 Build the four-player comparison view with aligned metrics and a clear comparison-limit interaction.
- [x] 8.5 Add a typed API client and frontend tests for filtering, navigation, freshness warnings, and failed requests.

## 9. Frontend squad and recommendations

- [x] 9.1 Build the squad editor with position/club/budget totals and inline structured validation errors.
- [x] 9.2 Build formation, starting XI, bench, captain, and vice-captain controls with client-side guardrails backed by server validation.
- [x] 9.3 Build the recommendation view with adjustable signal weights, lineup/bench presentation, captain/vice-captain emphasis, and factor explanations.
- [x] 9.4 Add resilient loading, invalid-squad guidance, partial-data warnings, and cached last-view behavior.
- [x] 9.5 Add frontend tests for valid/invalid squad edits, lineup changes, weight validation, and recommendation rendering.

## 10. Verification and handoff

- [x] 10.1 Add an end-to-end local smoke test covering migration, sync with fixture data, player research, squad save, and recommendation generation.
- [x] 10.2 Add representative source fixtures and document how to refresh them safely when the upstream schema changes.
- [x] 10.3 Run formatting, linting, unit tests, integration tests, and the frontend production build.
- [x] 10.4 Update the README with API routes, recommendation limitations, troubleshooting for stale/partial syncs, and next-step extension points.
- [x] 10.5 Verify each Make/Compose mode resolves correctly and document the commands as the canonical application workflow.

## 11. Browser acceptance testing

- [x] 11.1 Add Playwright/Chromium dependencies, headed local defaults, configurable base URL, and visible test scripts.
- [x] 11.2 Add browser coverage for research search/filter/sort and player detail history/fixture context.
- [x] 11.3 Add browser coverage for four-player comparison and comparison-limit behavior.
- [x] 11.4 Add browser coverage for squad planning, lineup controls, validation messaging, and recommendation output.
- [x] 11.5 Add browser coverage for sync/freshness UI and a Docker Compose reachability smoke test.
- [x] 11.6 Run the headed suite against the local three-container stack and document headed/headless commands.
- [x] 11.7 Add headed Playwright coverage for every actionable application button and fix inactive controls discovered by the button matrix.
