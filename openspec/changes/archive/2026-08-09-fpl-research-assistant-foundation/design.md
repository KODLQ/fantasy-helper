## Context

The repository is empty and needs a small monorepo that can be run locally as a database, backend API, and frontend application. The product is a local FPL research workspace with authenticated local users rather than a multi-tenant social product. Its first useful outcome is trustworthy, fresh source data plus explainable decisions for a user's squad.

The external source is the official Fantasy Premier League web API. Source payloads can change shape and can be temporarily unavailable, so ingestion must be isolated behind an adapter, validated before writes, and observable through sync records. The application must remain useful when the most recent sync is not available.

## Goals / Non-Goals

**Goals:**

- Establish a PostgreSQL-backed data model for one current FPL season and its historical player/fixture data.
- Provide a typed HTTP API that separates source ingestion, normalized query models, planning validation, and recommendation scoring.
- Provide a simple TypeScript frontend for research and squad planning, with freshness and error states visible.
- Make recommendations deterministic, configurable, constraint-safe, and explainable.
- Keep local setup straightforward with Docker Compose and repeatable database migrations.

**Non-Goals:**

- User authentication, multi-user permissions, or cloud deployment.
- Automated transfer execution, FPL account integration, or scraping private league data.
- Guaranteed point projections, betting advice, or a black-box machine-learning model.
- Real-time updates; sync freshness is measured in minutes/hours and can be triggered manually or by a scheduler.

## Decisions

### 1. Monorepo with PostgreSQL, Go API, and React/TypeScript UI

Use `db/` for SQL migrations, `backend/` for a Go HTTP service, and `frontend/` for a Vite-powered React/TypeScript application. PostgreSQL is appropriate for relational constraints and historical rows; Go keeps the sync worker and API deployable as one small binary; React gives the research UI enough statefulness without requiring a larger framework.

Alternative considered: a JavaScript-only stack. It would reduce language count but would make the data and worker boundaries less explicit for this data-heavy service.

### 2. Normalize source data and retain sync metadata

Store season, gameweek, team, player, fixture, player gameweek history, player season history, and sync-run records in relational tables. Add source IDs and unique constraints so rerunning a sync upserts rather than duplicates. Store the last successful payload checksum and sync timestamps in `sync_runs`; raw payload retention is limited to a JSON document per run or endpoint for diagnostics.

Alternative considered: query the FPL API directly from the frontend. That would expose source instability to the UI, make filtering slow, and prevent reproducible research after a source update.

### 3. Adapter-based, bounded ingestion

Implement an FPL source client with configurable base URL, request timeout, retries for transient failures, and bounded concurrency for player history requests. A sync run first imports the season snapshot and fixtures, then refreshes player summaries in batches. Each stage records its own result; a failed history batch does not erase the last known good normalized data.

Alternative considered: one all-or-nothing transaction for every endpoint. That would make a large sync fragile and would leave the application stale when only one endpoint fails.

### 4. Typed API contracts with explicit freshness

Expose JSON endpoints under `/api/v1`. Every response uses the shared warehouse `common-response-contract`: successful data uses `{data,meta}` with request scope/freshness/provenance, and failures use `{error,meta}` with a stable code and request ID. The frontend never depends on raw source field names. The existing `dataFreshness` field is migrated through a compatibility adapter and is not used by new endpoints.

### 5. Deterministic recommendation score

Calculate a baseline score from normalized signals: recent form, expected minutes proxy, fixture difficulty, recent attacking/defensive returns, and value. Normalize each signal to a documented range, apply user-configurable weights, and return the factor contributions alongside the result. Use a constraint-aware selection algorithm over the user's 15-player squad and deterministic tie-breakers.

Alternative considered: machine learning or opaque projected points. It would require a labeled historical evaluation pipeline and would be difficult for the user to audit in the first release.

### 6. Single local planning workspace

Persist one default squad plan in PostgreSQL without authentication. The frontend can edit it through the API and cache the last view for resilience. This keeps the data model ready for later user accounts without forcing identity and authorization into the first slice.

### 7. Compose overlays for environment-specific runtime behavior

Use one base `docker-compose.yml` that always defines exactly three services—PostgreSQL, backend, and frontend—and three small overlays: local, dev, and prod. The local overlay mounts source code and runs the Vite development server for hot frontend refresh; dev runs production-like containers on non-default host ports; prod builds the backend binary and serves the frontend through Nginx. A Makefile selects the environment file and Compose overlay so the documented commands are consistent across machines.

Alternative considered: separate full Compose files per environment. That duplicates service definitions and makes drift between local, staging, and production more likely.

### 8. Headed Playwright acceptance tests

Use `@playwright/test` from the frontend workspace with Chromium as the first browser target. The default Playwright configuration is headed (`headless: false`) so local runs visibly show the browser; a `--headless` command remains available for CI. Tests target the running local frontend through `E2E_BASE_URL`, wait on user-visible state rather than implementation details, and reset the in-memory backend by restarting the local stack when a clean state is needed.

Cover the research table, player detail drawer, four-player comparison, squad planner controls, recommendation output, sync/freshness state, and a smoke path that proves the frontend is reachable from the three-container Compose stack.

Maintain a button-coverage matrix in the browser suite. Every actionable button must be exercised through its user-visible result: workspace navigation, primary calls to action, table/card view toggles, advanced-filter open/clear actions, player inspect/compare/add-to-squad/close actions, comparison back/remove/empty-state actions, squad loading/saving/optimization actions, recommendation execution, and sync. Pagination controls that are not implemented yet must be explicitly disabled and covered as such.

## Risks / Trade-offs

- **[Source API shape or availability changes]** → Keep endpoint paths and field mappings inside the adapter, validate payloads, record sync failures, and show the last good snapshot instead of clearing data.
- **[Bulk player history sync is slow or rate-limited]** → Use bounded concurrency, retries with backoff, resumable batches, and a separate sync status endpoint; allow baseline research from the latest successful data.
- **[Baseline score can be mistaken for a prediction]** → Label it as a heuristic, expose weights and factor contributions, and state that it is not a guarantee.
- **[Combinatorial lineup selection becomes complex]** → Keep the first optimizer limited to a 15-player squad, enforce formation constraints in a dedicated validator, and add exhaustive/branch-and-bound tests before optimizing implementation.
- **[No authentication is unsafe for internet exposure]** → Document local-only scope, bind development services safely, and require authentication as a prerequisite for any hosted deployment.

## Migration Plan

1. Start PostgreSQL and apply versioned migrations from `db/migrations`.
2. Start the backend with the FPL source base URL and database connection configured.
3. Run the initial sync; it creates the current season snapshot and marks the sync outcome.
4. Start the frontend and let it display the research workspace only after API health is available.
5. For rollback, stop the backend/frontend and deploy the previous binary/UI; preserve normalized data and sync records. Only roll back a migration with its explicit down migration after confirming no newer data depends on it.
6. Run `make deploy ENV=local` for hot-refresh development, `make deploy ENV=dev` for staging-like containers, or `make deploy ENV=prod` for the production image set. Use the matching `make down ENV=...` command to stop and remove the environment's containers.

## Finalized boundaries

- The warehouse retains multiple explicitly configured seasons for comparison/backtesting; the first UI may default to one active season and expose prior seasons through analysis selectors.
- The first scheduler is in-process with durable work items; external jobs remain a compatible later trigger.
- Transfer recommendations and the optimal-team change own free transfers, hits, chips, and wildcard/bench-boost rules. The foundation only supplies versioned rules and validated facts.
