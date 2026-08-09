## ADDED Requirements

### Requirement: Use a safe typed frontend request boundary

The frontend SHALL map common success/error responses, non-JSON failures, timeouts, cancellations, and stale responses into typed state without exposing raw secrets or overwriting newer data.

#### Scenario: Older request completes late
- **WHEN** a newer request has already started and the older request completes afterward
- **THEN** the older result is ignored and the UI retains the newer request state

#### Scenario: Backend returns non-JSON
- **WHEN** an API request returns a non-JSON body, timeout, or cancellation
- **THEN** the client exposes a typed recoverable error with request context and does not treat the response as domain data

### Requirement: Protect verification targets and generated artifacts

Integration tests SHALL reject unsafe or unrecognized database targets, and generated browser artifacts SHALL be isolated from source-controlled application output.

#### Scenario: Unsafe database target
- **WHEN** an integration test is configured with a production-like or unrecognized database name
- **THEN** the test aborts before destructive setup

#### Scenario: Browser suite generates reports
- **WHEN** Playwright produces traces, screenshots, videos, or reports
- **THEN** artifacts use the configured test-output location and do not alter application source or fixtures

### Requirement: Verify cross-cutting frontend operations

The system SHALL run formatting, linting, type checks, API contract checks, health checks, and browser smoke tests through documented commands.

#### Scenario: Standard verification runs
- **WHEN** the standard verification command is run
- **THEN** all configured checks execute with actionable failures and no warehouse migration behavior is duplicated

### Requirement: Validate OpenSpec dependencies and ownership in CI

CI SHALL validate the canonical dependency/ownership registry, detect unknown changes, cycles, invalid release order, duplicate capability owners, parent/child specification duplication, formula/rules registry drift, and strict OpenSpec validation before accepting implementation changes.

#### Scenario: Dependency cycle is introduced
- **WHEN** a registry edit creates a dependency cycle or references an unknown change
- **THEN** CI fails with the cycle/reference path and does not accept the change

#### Scenario: Capability gets two owners
- **WHEN** two changes claim the same authoritative capability
- **THEN** CI fails with both change names and the capability name

#### Scenario: Child spec is duplicated in the roadmap
- **WHEN** a roadmap change adds a domain requirement already owned by a child change
- **THEN** CI fails with the parent/child ownership conflict
