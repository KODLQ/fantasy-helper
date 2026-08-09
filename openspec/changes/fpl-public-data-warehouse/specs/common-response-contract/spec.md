## ADDED Requirements

### Requirement: Use one success envelope

Every versioned API response SHALL use `{data, meta}` on success. `meta` SHALL include `requestId`, the validated request scope, and `freshness` when the response depends on synced data.

#### Scenario: Complete data response
- **WHEN** an endpoint returns complete data for the requested scope
- **THEN** the response contains `data`, `meta.requestId`, `meta.scope`, and `meta.freshness.state=actual` or the appropriate domain state

#### Scenario: Partial data response
- **WHEN** an endpoint can answer with missing, stale, provisional, or estimated inputs
- **THEN** it returns the available `data` and machine-readable `meta.freshness.state`, `meta.coverage.missingInputs`, and `meta.warnings` instead of hiding the condition in prose

### Requirement: Use one error envelope

Every non-success API response SHALL use `{error, meta}` where `error` contains a stable `code`, safe user-facing `message`, `retryable`, and optional structured `details`; `meta.requestId` SHALL be present.

#### Scenario: Source or validation error
- **WHEN** a request cannot be fulfilled because a source, validation, or scope condition fails
- **THEN** the API returns a stable error code, does not leak credentials or raw secrets, and identifies whether retrying may help

#### Scenario: Non-JSON or unexpected backend failure
- **WHEN** the client receives an unexpected response body or network failure
- **THEN** the client maps it to the same typed error model with the request ID when available and never treats the body as successful domain data

### Requirement: Make freshness and provenance interoperable

The common freshness object SHALL use `state` values `actual`, `provisional`, `estimated`, `partial`, `stale`, or `unavailable`, and SHALL carry dataset, season/gameweek scope, snapshot IDs, source fetch time, normalized time, algorithm/ruleset versions when applicable, missing inputs, and warnings.

#### Scenario: Analytical response has a reproducible source
- **WHEN** an analysis is calculated from warehouse and manager snapshots
- **THEN** `meta.freshness` and `meta.provenance` identify all input snapshot IDs and calculation versions needed to reproduce it

### Requirement: Support bounded collections consistently

Collection responses SHALL use `data.items` plus `meta.pagination` with `limit`, `offset` or cursor, `returned`, and `total` when known. Any server-side member or candidate limit SHALL be reported in `meta.coverage`.

#### Scenario: Collection is bounded
- **WHEN** an endpoint omits members because of a configured limit
- **THEN** the response identifies omitted IDs/counts and the applied limit rather than implying the collection is complete
