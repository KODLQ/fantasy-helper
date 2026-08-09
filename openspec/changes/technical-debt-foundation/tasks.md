## 1. Frontend maintainability

- [x] 1.1 Split `App.tsx` into feature components and shared hooks without changing route behavior.
- [x] 1.2 Add typed API errors, request IDs, timeouts, cancellation, safe response parsing, and stale-response protection.
- [ ] 1.3 Surface drawer, compare, auth, manager, and analysis errors with common loading/empty/stale/partial states.
- [x] 1.4 Add formatter, linter, type-check, dependency-pin, and standard verification scripts.

## 2. Verification and operational hygiene

- [x] 2.1 Protect PostgreSQL tests from truncating production-like or unrecognized databases.
- [x] 2.2 Ignore generated frontend/Playwright reports and configure disposable test-output locations.
- [x] 2.3 Add deployment smoke checks for frontend reachability, API health, and common response envelopes.
- [ ] 2.4 Add redacted frontend request/analysis correlation logging and cancellation metrics.
- [ ] 2.5 Run formatting, static checks, unit tests, integration tests, builds, contract tests, and headed Playwright tests.
- [x] 2.6 Implement `scripts/validate-openspec-portfolio` for registry schema, unknown references, cycles, release order, ownership uniqueness, parent/child duplicate detection, formula/rules registry integrity, and strict OpenSpec validation.

## 3. Ownership verification

- [x] 3.1 Add a review check proving migrations/sync/persistence requirements remain only in `fpl-public-data-warehouse`.
- [x] 3.2 Add a review check proving account/session/ownership requirements remain only in `local-user-authentication`.
