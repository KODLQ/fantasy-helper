## Why

The current FPL integration imports only a partial public snapshot into an in-memory store, which prevents durable historical analysis, backtesting, and reliable recovery from source changes or failed requests. The public FPL endpoints expose enough season, fixture, player, and gameweek data to support a proper local analytical warehouse, but that data needs to be captured as immutable source observations and normalized into historical relational facts.

## What Changes

- Replace the sync path's in-memory persistence with PostgreSQL-backed repositories and transactional upserts.
- Add a raw source-payload layer retaining endpoint, parameters, checksum, fetch metadata, validation outcome, and bounded JSON diagnostics.
- Import the complete public season catalog from `bootstrap-static`, including phases, gameweek settings, element types, teams, players, and changing player metrics.
- Import full fixture data, fixture statistics, current/live gameweek player data, player historical seasons, player gameweek history, and player-specific future fixtures.
- Preserve historical player price, team, availability, ownership, and performance snapshots rather than overwriting analysis inputs.
- Add resumable sync work items, bounded concurrency, retry/backoff including rate-limit handling, idempotency, and last-known-good retention.
- Add scheduled/manual sync modes with stage-level progress, freshness, partial-failure reporting, and operational diagnostics.
- Add database-backed analytical views or read models for player research, rolling form, fixture context, value, and recommendation inputs.
- Keep the existing research and recommendation APIs stable while sourcing their data from PostgreSQL.
- Add explicit point-in-time dataset identities so every analytical response can name the season, gameweek, source snapshot, normalization version, and completeness state it used.
- Add configuration for initial backfill, incremental refresh, live polling, raw-payload retention, worker concurrency, request timeout, retry policy, and rate-limit behavior.
- Fold the migration, sync-lifecycle, batching, last-known-good, season-identity, and deployment-boundary debt items into this warehouse change so there is one implementation owner.
- Define one response envelope and error contract for warehouse, manager, analysis, and optimization APIs.
- Define the upstream FPL source contract explicitly, including endpoint shapes, required fields, season scope, normalization rules, and unknown-field handling.

## Capabilities

### New Capabilities

- `fpl-public-data-warehouse`: Durable ingestion, normalization, historical retention, freshness reporting, and analytical read models for public FPL data.
- `common-response-contract`: Versioned success, error, freshness, provenance, pagination, and partial-data response semantics shared by all API changes.
- `fpl-source-contract`: Explicit source endpoint, field, typing, identity, and validation contract for the FPL public API.

### Modified Capabilities

- `fpl-data-sync`: Expand the initial snapshot/history sync into a database-backed, resumable public-data ingestion pipeline.

## Impact

- Affects `backend/internal/app` source adapters, sync orchestration, repositories, configuration, and HTTP status endpoints.
- Extends `db/migrations` with raw payload, snapshot, gameweek-statistics, fixture-statistics, and analytical read-model tables/indexes.
- Requires PostgreSQL to be the source of truth for synced data; the demo in-memory store becomes test-only or a fallback fixture store.
- Adds worker/scheduler configuration, source request metrics, and migration/integration tests.
- Defines stable dataset freshness and sync-status contracts consumed by the frontend and future analytical services.
- Becomes the owner of migration execution, sync lifecycle correctness, batch persistence, explicit season configuration, and warehouse-facing deployment checks previously listed in `technical-debt-foundation`.
- The manager, private-team, and league endpoints remain out of scope for this change and are handled separately.
