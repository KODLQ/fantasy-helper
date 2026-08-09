## Context

The foundation application has a large frontend shell and request code that needs stronger boundaries as new warehouse, authentication, manager, and analysis flows arrive. The warehouse and auth changes are separate owners for database/sync and identity/ownership behavior.

## Decisions

### 1. Feature-oriented frontend modules

Keep a small application shell and move research, comparison, squad planning, recommendations, authentication, manager sync, and analysis workbench behavior into feature modules with shared hooks. Components consume typed API results and do not know raw source field names.

### 2. Typed request boundary

A shared request helper owns timeout, cancellation, safe JSON parsing, request IDs, common `{data,meta}`/`{error,meta}` mapping, and stale-response protection. Non-JSON responses and aborted requests become typed recoverable errors. A response arriving after a newer request cannot overwrite current state.

### 3. Tooling and generated artifacts

Pin dependencies, run formatter/linter/type checks in the standard verification command, and ignore generated Playwright reports/videos/traces unless explicitly requested as test artifacts.

### 4. Verification safety

Integration tests require an explicit disposable/test database identity and reject production-like or unrecognized targets. Deployment smoke checks validate frontend reachability, API health, and response-envelope compatibility. Migration execution remains owned by the warehouse change.

### 5. Observability boundary

Frontend requests and analysis actions forward the server request ID when available and record redacted operation, duration, outcome, and cancellation class. Credentials, session tokens, private payloads, and raw source bodies are never logged.

### 6. CI validates the OpenSpec portfolio

The repository runs `scripts/validate-openspec-portfolio` in CI before implementation changes are accepted. The validator parses `openspec/dependencies.yaml`, verifies every referenced change exists, rejects cycles and dependencies that are scheduled after their consumers, checks that every capability has exactly one authoritative owner, rejects parent/child duplicate specs, validates `openspec/formulas.yaml` and `openspec/fpl-rules.yaml`, and runs `openspec validate --changes --strict --no-interactive`. A failure is blocking and reports the exact registry path/change/capability that failed.

## Non-goals

- Migration DDL or migration runner behavior.
- Sync coordinator, retries, source season identity, or canonical warehouse persistence.
- User registration, password/session handling, or private-record authorization.

## Rollout

1. Add shared request/error types and tests.
2. Extract one feature at a time behind the existing routes.
3. Enable strict tooling and safe database-target checks.
4. Run the cross-workbench Playwright suite before removing the old shell/request paths.

Rollback retains the old frontend entry points and disables only the new module wiring; no database data is deleted.
