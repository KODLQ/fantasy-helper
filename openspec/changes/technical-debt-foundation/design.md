## Context

The application currently has a PostgreSQL repository and an in-memory read model, an asynchronous sync goroutine, duplicated schema definitions, and a single large React module. The objective is to reduce coupling while retaining the single-user local workflow.

## Decisions

### 1. Versioned migrations are authoritative

The backend will not create or mutate schema objects at startup. `db/migrate.sh` owns migration ordering and records applied versions. Deployment runs migrations after database readiness and before declaring the backend ready. Existing databases can be upgraded using the existing migration files.

### 2. Sync coordinator owns one run

The coordinator creates one run record, holds its ID in memory, updates stages, and finishes the run. It has a cancellable context, a bounded worker pool, and a shutdown wait. Repository methods accept a run handle rather than synthesizing a new run for every status snapshot.

### 3. Last-known-good data is merged

A successful snapshot replaces season entities, but history rows are merged by player. A failed player-history request leaves the prior in-memory and database history available. Sync status identifies the incomplete stage.

### 4. Source and database identifiers are explicit

The source adapter receives an active season source ID/name from configuration until an authoritative seasons endpoint is available. Repository mapping helpers use named source-ID concepts and batch-resolve current-season players.

### 5. PostgreSQL is durable state; Store is a cache

Writes are committed to PostgreSQL before the cache is updated. Reads continue through the Store for this release, but cache refresh is centralized and documented. The next scale step can move read queries behind the repository without changing handlers.

### 6. Feature-oriented frontend modules

The application shell remains in `App.tsx`, while research, comparison, squad planning, recommendations, and shared request hooks move into feature modules. A typed request helper owns timeout, cancellation, safe response parsing, and API errors.

### 7. Verification is part of the workflow

Migration verification derives expected versions from files, integration tests refuse unsafe database names, generated browser artifacts are ignored, and static formatting/linting runs in the standard test command.
