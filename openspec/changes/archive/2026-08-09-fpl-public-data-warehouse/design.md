## Context

The repository currently has PostgreSQL migrations but the running backend constructs an in-memory store and only checks whether the database is reachable. The FPL adapter imports a subset of `bootstrap-static`, the fixture feed, and selected fields from `element-summary`. The target is a local analytical warehouse that can support player research, recommendations, historical comparisons, and later backtesting without losing prior source observations.

The upstream API is a collection of independently refreshed JSON endpoints. Public data includes season metadata, gameweeks, teams, players, fixtures, live gameweek performance, player history, and upcoming fixtures. Payload shapes can evolve, requests can fail independently, and the source does not provide a warehouse-style change feed.

## Goals / Non-Goals

**Goals:**

- Make PostgreSQL the source of truth for public synced FPL data.
- Retain raw source observations sufficiently to replay/debug normalization.
- Preserve historical values that change during a season, especially price, team, availability, ownership, and performance.
- Make initial backfill, incremental refresh, retries, and recovery observable and resumable.
- Keep the existing research and recommendation response contracts stable.

**Non-Goals:**

- Manager accounts, private team state, transfers, or league standings.
- Automated transfer execution or FPL account mutation.
- A cloud-scale distributed warehouse or machine-learning prediction pipeline.
- Guaranteeing that every unknown upstream field receives a dedicated relational column; unknown fields remain available in raw JSON.

## Decisions

### 1. Use a raw, canonical, and analytical three-layer model

Every successful or diagnostically useful response is recorded in `source_payloads` with endpoint identity, normalized request parameters, fetched time, HTTP status, checksum, schema version, and JSONB payload. Canonical tables hold validated entities and facts. Views or materialized read models expose query-oriented metrics for the existing application.

This is preferred over storing only normalized rows because it permits reprocessing after adapter changes. It is preferred over querying JSONB directly because player research and historical analysis need indexed relational joins.

### 2. Separate identities from time-varying snapshots

Use season-scoped source identities for players, teams, gameweeks, and fixtures. Keep relatively stable profile data in identity tables, and store changing values in player snapshots and player-gameweek fact rows. A source ID is unique within its season scope rather than assumed to be a permanent global identity.

Promote frequently filtered metrics to typed columns. Retain the complete endpoint payload for fields that are not yet modeled.

### 2a. Use explicit point-in-time identities

Every normalized dataset row is associated with a season and, where applicable, a gameweek or observation timestamp. A `dataset_snapshot` identity records source checksums, normalization version, completed stages, and completeness. APIs return this identity in freshness metadata so historical consumers never accidentally join current player values to an older gameweek.

The canonical model distinguishes:

- `player`: season-scoped identity and stable profile attributes.
- `player_snapshot`: values observed at a refresh/deadline, including price, team, status, ownership, form, and availability.
- `player_gameweek_fact`: finalized or live player performance for one season/gameweek.
- `fixture`: season/gameweek fixture identity and schedule/result state.
- `fixture_stat`: fixture-level event/stat rows keyed by fixture, player, stat type, and source observation.
- `player_future_fixture`: player-oriented fixture context from the source, linked back to the canonical fixture when possible.

All natural keys and foreign keys must be documented in migration comments or repository contracts. Source IDs are not sufficient without season scope.

### 3. Use endpoint-specific sync stages

The initial pipeline runs in dependency order:

```text
bootstrap-static
      ↓
season/catalog upserts
      ↓
fixtures + current live gameweek
      ↓
player summaries and historical backfill
      ↓
analytical read models
```

Incremental refreshes use bootstrap and fixtures for catalog/current schedule changes, event-live for the current gameweek, and player summaries only for initial backfill or targeted reconciliation. This avoids making hundreds of player requests the critical path for every refresh.

The default cadence is:

- initial backfill: manually started, resumable, all public seasons selected by configuration;
- catalog/fixture refresh: at least hourly while a season is active;
- live gameweek refresh: configurable polling interval, default five minutes while matches are in progress;
- finalization refresh: after the final fixture in a gameweek, then once after source data is marked final;
- historical reconciliation: daily or manually triggered for failed/stale player summaries.

The scheduler must not start overlapping runs for the same dataset scope.

### 4. Persist sync work as database records

Create `sync_runs`, `sync_stages`, and `sync_work_items`. Each work item has a deterministic natural key such as endpoint plus season, gameweek, and source entity ID. Claiming, retrying, and completing work is safe across process restarts. Upserts use season-scoped unique constraints and do not delete last-known-good rows when a request fails.

The first implementation uses one backend worker with bounded concurrency. The database work queue is chosen over an in-memory queue so interrupted backfills can resume.

### 5. Treat rate limits and source shape changes as normal states

The HTTP client handles transport errors, 5xx responses, 429 responses, `Retry-After`, timeouts, and bounded exponential backoff. Payload validation occurs before canonical writes. A schema-validation failure records a diagnostic and marks that stage or work item failed without invalidating other stages.

Transport policy is configurable but has safe defaults: a 20-second request timeout, three total attempts for retryable failures, exponential backoff with jitter, no retry for normal 4xx responses, and a bounded per-host concurrency. A 429 response honors `Retry-After` when present. The client records request duration, attempt number, response status, and redacted error class.

### 6. Keep API compatibility through repositories and read models

Repositories return domain models independent of source JSON names. The current `/api/v1/players`, player detail, comparison, sync status, squad, and recommendation endpoints continue to return stable application fields. The store abstraction can remain for unit tests, while production wiring uses PostgreSQL repositories.

New warehouse-facing contracts use `/api/v1/data` and `/api/v1/sync`:

- `GET /api/v1/data/snapshots` lists dataset snapshots with season, gameweek, status, source time, normalization version, and completeness.
- `GET /api/v1/sync/status` returns run, stage, work-item counts, retryable failures, and dataset freshness.
- `POST /api/v1/sync` accepts an explicit scope (`catalog`, `fixtures`, `live`, `player-history`, or `full`) plus season/gameweek filters and returns a run ID.
- `POST /api/v1/sync/runs/{id}/retry` retries only retryable failed work items.

Every analytical response places the common `freshness` object under `meta`:

```json
{
  "data": {},
  "meta": {
    "requestId": "...",
    "scope": {"seasonId": 1, "gameweek": 12},
    "freshness": {
      "dataset": "player-gameweek",
      "snapshotIds": ["snapshot-..."],
      "state": "actual|provisional|estimated|partial|stale|unavailable",
      "sourceFetchedAt": "...",
      "normalizedAt": "...",
      "normalizerVersion": "...",
      "missingInputs": [],
      "warnings": []
    }
  }
}
```

### 7. Make retention explicit

Canonical facts are retained for all imported seasons. Raw payloads are retained for successful baseline responses and failed/invalid responses; high-frequency live observations can use configurable retention after their normalized facts are finalized. Checksums and metadata remain retained even if a raw body is purged.

Raw payload retention must be enforced by a scheduled cleanup job that never deletes canonical facts, failed diagnostics needed for the configured audit window, or payloads referenced by an active reproducibility run.

### 8. Make the warehouse source contract explicit

The adapter is a typed boundary around the public FPL source. The supported endpoint families are:

| Endpoint | Required source scope | Canonical responsibilities |
| --- | --- | --- |
| `bootstrap-static/` | active season catalog | gameweeks/events, phases, game settings, teams, players/elements, element types, total players |
| `fixtures/` and `fixtures/?event={gw}` | season and optional gameweek | fixture ID, event, kickoff, home/away teams, scores, finished/provisional state, fixture stats |
| `event/{gw}/live/` | season and gameweek | player ID plus live/final minutes, points, scoring, defensive, saves, cards, bonus/BPS, and source-provided expected metrics |
| `element-summary/{playerId}/` | season and player | future fixtures, gameweek history, and prior-season totals/history |

The adapter SHALL preserve the source payload before normalization and SHALL map source IDs with explicit season scope. Numeric strings are parsed into typed values with field-level validation; missing nullable values remain null rather than being converted to zero. Unknown fields remain in raw JSON and are exposed in diagnostics when a required field is absent or changes type. A source fixture may only be normalized as `actual` after its required identity, schedule, score state, and player-stat fields pass validation.

The source contract also defines configuration rather than assuming the current season: `sourceSeasonId`, `sourceSeasonName`, source base URL, request timeout, retry policy, and endpoint cadence are required inputs. The adapter must fail clearly when the configured season identity cannot be reconciled with the bootstrap response.

### 9. Fold technical-debt ownership into the warehouse

This change owns the following implementation boundaries:

- Migration files under `db/migrations` are the sole schema authority. Application startup verifies connectivity and schema state but does not embed DDL or invent migration versions. Deployment applies migrations and fails readiness before serving traffic when one fails; verification derives expected versions from migration files.
- A `syncCoordinator` owns one run, cancellation context, duplicate-scope lock, bounded worker pool, retry/backoff, and graceful shutdown. Repository methods persist run/stage/work-item transitions and actual source checksums.
- History and snapshot writes use batch transactions. Successful durable writes refresh caches/read models explicitly; failed player work preserves last-known-good facts.
- Source season ID/name is explicit configuration, not a hard-coded constant. Request correlation IDs and sync metrics are emitted at the adapter/coordinator boundary.

The frontend timeout/error and unsafe-database-test items remain cross-cutting verification concerns, but their warehouse-specific acceptance cases are tracked here.

### 10. Common API response contract

All warehouse-facing and downstream analytical endpoints use the response contract in `specs/common-response-contract/spec.md`. Existing endpoints may have a compatibility adapter during migration, but new routes MUST use the common envelope. The envelope carries `data`, a `meta` object containing request/scope/freshness/provenance/pagination information, and a stable `error` object for failures. A response never encodes partial or stale data only in prose or HTTP status; it reports the machine-readable state and missing inputs.

## Risks / Trade-offs

- **[Large initial backfill]** → Use bounded concurrency, resumable work items, progress reporting, and a useful partial snapshot before all player histories finish.
- **[Upstream fields or types change]** → Store raw JSON, validate payloads, version normalizers, and never replace a good canonical row with an invalid response.
- **[PostgreSQL schema grows quickly]** → Promote only analysis-critical fields and keep long-tail fields in JSONB until a query justifies a column.
- **[Live data is repeatedly rewritten]** → Treat live observations as snapshots keyed by run/fetched time and derive a finalized gameweek fact; retain only configured raw/live history.
- **[Existing demo behavior changes]** → Keep fixture-backed in-memory stores for unit tests and make the production database requirement explicit in startup health checks.
- **[A partial sync looks fresh]** → Report freshness per dataset/stage as well as at the run level, including the last successful canonical timestamp.
- **[Historical joins use mismatched snapshots]** → Require snapshot IDs or explicit season/gameweek keys in repository methods and analytical API requests.
- **[A scheduler creates duplicate work]** → Enforce scope locks and unique active-run constraints in PostgreSQL.

## Migration Plan

1. Add migration bookkeeping and PostgreSQL repository/transaction primitives.
2. Add raw payload, catalog, snapshot, fact, work-item, and analytical read-model migrations without removing existing tables.
3. Backfill the current active season through the new pipeline and compare counts/metrics with the existing fixture-backed behavior.
4. Switch production API reads and writes from the in-memory store to PostgreSQL repositories.
5. Keep the existing tables as compatibility views or migrate their consumers before removing obsolete columns.
6. Enable scheduled refresh only after initial sync, recovery, and partial-failure tests pass.
7. Roll back by stopping the new worker/API build; preserve imported data. Revert migrations only when no later migration depends on them.

The rollout gate is a successful full sync against sanitized fixtures, a restart-resume test, and a production-like query proving that current and historical player snapshots cannot be confused.

## Finalized implementation choices

- Retain successful baseline/raw payloads for 90 days by default, high-frequency live bodies for 30 days after finalization, and all checksums/diagnostics for the configured audit window; make both periods configurable.
- Use an in-process scheduler for the first local deployment, with the durable work queue allowing a later external trigger without changing sync contracts.
- Materialize rolling form and fixture difficulty first because they are shared by research, recommendations, and optimization; add price/ownership trend views after the core read models are proven.
- Discover historical seasons only through explicit configured season IDs or a deliberate backfill request; never import every season implicitly.
- A gameweek is finalized only when the source marks it finished, all fixtures in the gameweek are finished, and the finalization refresh has no changed required facts; otherwise it remains provisional.
- `actual` requires the required source-contract identity, schedule/result, player-point, and rules fields for the requested dataset. Missing optional fields yield a warning; missing required fields yield `partial`, `stale`, or `unavailable`.
