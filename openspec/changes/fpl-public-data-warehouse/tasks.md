## 1. Database and repository foundation

- [ ] 1.1 Add migration tracking and PostgreSQL connection/pool configuration.
- [ ] 1.2 Add `source_payloads`, `sync_runs`, `sync_stages`, and resumable `sync_work_items` tables.
- [ ] 1.3 Add season catalog tables for phases, game settings, element types, and season-scoped source identities.
- [ ] 1.4 Add player snapshot, player gameweek fact, fixture-stat, and player-future-fixture tables.
- [ ] 1.5 Add constraints and indexes for season/gameweek/player/fixture natural keys and analysis queries.
- [ ] 1.6 Add repository interfaces, transaction helpers, upsert helpers, and integration tests against PostgreSQL.
- [ ] 1.7 Add dataset snapshot identity, completeness state, normalizer version, and source-time columns/contracts.
- [ ] 1.8 Add migration comments and repository documentation for every natural key and foreign-key relationship.
- [ ] 1.9 Add migration-runner ownership, readiness gating, file-derived verification, and unsafe-database-target protection.
- [ ] 1.10 Add shared response-envelope, error, freshness, provenance, pagination, and compatibility-contract tests.

## 2. Source adapter coverage

- [ ] 2.1 Expand bootstrap normalization to retain phases, settings, element types, and all analysis-relevant player/team fields.
- [ ] 2.2 Expand fixture normalization to retain full fixture metadata and fixture-level statistics.
- [ ] 2.3 Add event-live normalization for all player gameweek statistics and finalization state.
- [ ] 2.4 Expand element-summary normalization to include history, history-past, and upcoming fixtures.
- [ ] 2.5 Store source payloads and checksums for every stage with bounded diagnostics.
- [ ] 2.6 Add adapter fixtures and schema-validation tests for complete, partial, malformed, 429, and 5xx responses.
- [ ] 2.7 Publish typed source contracts for bootstrap, fixtures, event-live, and element-summary; retain unknown fields and test numeric/null/type changes.
- [ ] 2.8 Add explicit season ID/name configuration and tests proving no active season is hard-coded.

## 3. Sync orchestration and recovery

- [ ] 3.1 Implement dependency-ordered initial catalog, fixture, live-data, and player-history stages.
- [ ] 3.2 Implement durable work-item claiming, retry counts, backoff, `Retry-After`, and restart recovery.
- [ ] 3.3 Add bounded concurrency and configurable request timeouts without blocking unrelated stages.
- [ ] 3.4 Implement idempotent canonical upserts and last-known-good retention for failed work.
- [ ] 3.5 Add current-gameweek incremental refresh and completed-gameweek finalization.
- [ ] 3.6 Add manual sync controls and an in-process scheduler configuration.
- [ ] 3.7 Add scope locks and duplicate-run prevention for equivalent season/gameweek datasets.
- [ ] 3.8 Add configurable timeout, retry, jitter, `Retry-After`, and per-host concurrency behavior with metrics.
- [ ] 3.9 Add catalog, fixture, live, finalization, and reconciliation cadence policies with scheduler tests.
- [ ] 3.10 Add coordinator cancellation, shutdown wait, correlation IDs, sync metrics, and duplicate-scope locking.
- [ ] 3.11 Batch snapshot/history writes and test last-known-good merge plus explicit cache refresh after commit.

## 4. API and analytical read models

- [ ] 4.1 Replace production in-memory reads with PostgreSQL repositories while preserving existing API response contracts.
- [ ] 4.2 Add dataset-level freshness and partial-sync metadata to research and recommendation responses.
- [ ] 4.3 Add read models for rolling form, price/value changes, availability, fixture context, and historical gameweek analysis.
- [ ] 4.4 Add database-backed sync status, stage progress, diagnostics summary, and retry endpoints.
- [ ] 4.5 Keep the in-memory store as a fixture-backed unit-test implementation only.
- [ ] 4.6 Add `/api/v1/data/snapshots`, scoped sync request, retry-run, and common freshness response contracts.
- [ ] 4.7 Add repository query methods that require explicit season/gameweek/snapshot scope for historical reads.

## 5. Verification and rollout

- [ ] 5.1 Add migration verification, repository idempotency, transaction rollback, and restart-resume tests.
- [ ] 5.2 Add full-sync integration tests using sanitized source fixtures and a fake source server.
- [ ] 5.3 Compare player counts, fixture counts, gameweek facts, and selected metrics before/after the new pipeline.
- [ ] 5.4 Update README/configuration documentation for PostgreSQL-backed sync, retention, scheduling, and troubleshooting.
- [ ] 5.5 Run unit, integration, frontend, and browser smoke tests against the database-backed stack.
- [ ] 5.6 Add tests proving historical/current snapshot isolation and actual/partial/stale/unavailable state labels.
- [ ] 5.7 Add retention cleanup tests that preserve canonical facts and active reproducibility references.
- [ ] 5.8 Add Playwright coverage for manual sync, progress, partial/failure states, stale/unavailable freshness, retry controls, and response errors.
