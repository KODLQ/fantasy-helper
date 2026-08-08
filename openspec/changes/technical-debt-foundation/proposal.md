## Why

The foundation is functional, but persistence, synchronization, frontend state, and deployment currently rely on duplicated or implicit behavior. Those shortcuts will make future FPL seasons, larger snapshots, and new research features increasingly risky to ship.

## What Changes

- Make versioned SQL migrations the sole database schema owner.
- Give every synchronization a single persisted run with stage updates, source checksums, cancellation, and graceful shutdown.
- Preserve last-known-good player history during partial refreshes and derive the active season from explicit source configuration.
- Batch snapshot persistence and make PostgreSQL/cache ownership explicit.
- Split the frontend by feature, harden API requests, and add formatting/linting guardrails.
- Improve deployment migration checks, dependency pinning, generated-artifact hygiene, observability, and integration-test safety.

## Impact

This is a behavior-preserving maintenance change. It affects database startup/migration flow, sync persistence, source configuration, repository performance, frontend module boundaries, developer tooling, and verification scripts. Existing API routes remain compatible unless they expose previously incorrect sync metadata.
