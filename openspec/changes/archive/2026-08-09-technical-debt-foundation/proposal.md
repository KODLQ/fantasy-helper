## Why

The application still has cross-cutting frontend and verification debt that would make the warehouse, authentication, manager, and analysis changes harder to operate safely.

## Ownership boundary

`fpl-public-data-warehouse` is the sole owner of migration execution, sync lifecycle, source checksums, season configuration, last-known-good retention, batch persistence, cache refresh, and warehouse deployment gates. `local-user-authentication` owns user/workspace ownership. This change intentionally contains no requirements or tasks for those domains.

## What Changes

- Split the frontend shell into feature modules and shared request hooks.
- Harden frontend request parsing, timeout, cancellation, stale-response, and typed-error behavior.
- Add formatting/linting/dependency guardrails and generated-artifact hygiene.
- Protect integration tests from unsafe database targets and add frontend/deployment smoke checks.
- Add correlation and verification coverage for the frontend boundary without duplicating warehouse synchronization requirements.

## Capabilities

### New Capabilities

- `frontend-maintainability`: Feature-oriented frontend modules and a safe typed request boundary.
- `verification-operational-hygiene`: Safe integration targets, generated-artifact handling, and cross-cutting checks.

### Modified Capabilities

- None. Warehouse and authentication changes own their respective backend concerns.

## Impact

This is a behavior-preserving maintenance change affecting frontend structure, client request handling, developer tooling, and verification scripts. It does not create migrations, alter sync state, or own private-data authorization.
