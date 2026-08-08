## 1. OpenSpec and migration ownership

- [x] 1.1 Create this technical-debt change with proposal, design, specification, and implementation tasks.
- [ ] 1.2 Remove application-owned schema DDL and migration-version inserts from the repository.
- [ ] 1.3 Add migration execution/verification to the Compose deployment workflow.
- [ ] 1.4 Make migration verification derive its expected count from migration files.

## 2. Sync correctness and lifecycle

- [ ] 2.1 Add a sync coordinator with cancellation, duplicate-run protection, configurable concurrency, and shutdown wait.
- [ ] 2.2 Add repository methods for starting, updating, and finishing one sync run.
- [ ] 2.3 Persist the actual source checksum and endpoint diagnostics.
- [ ] 2.4 Merge partial history results with last-known-good in-memory history.
- [ ] 2.5 Add bounded retry backoff and context-aware waits.
- [ ] 2.6 Add explicit season source ID/name configuration and tests.

## 3. Persistence and domain boundaries

- [ ] 3.1 Batch snapshot writes and reduce source-ID lookup round trips.
- [ ] 3.2 Add explicit cache refresh behavior after durable writes.
- [ ] 3.3 Introduce season/workspace identity in squad persistence without breaking the default plan.
- [ ] 3.4 Add tests for partial history retention, sync lifecycle, and multi-season-safe lookups.

## 4. Frontend maintainability

- [ ] 4.1 Split App.tsx into feature components and shared hooks.
- [ ] 4.2 Add typed API errors, request timeouts, cancellation, and safe cache parsing.
- [ ] 4.3 Prevent stale research responses and surface drawer/compare errors.
- [ ] 4.4 Pin dependencies and add formatter/linter scripts.

## 5. Verification and operational hygiene

- [ ] 5.1 Add request correlation logging and sync metrics/log fields.
- [ ] 5.2 Protect PostgreSQL tests from truncating non-test databases.
- [ ] 5.3 Ignore generated frontend reports and add deployment smoke checks.
- [ ] 5.4 Run formatting, static checks, unit tests, integration tests, builds, migration verification, and headed Playwright tests.
