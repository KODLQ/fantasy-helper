## 1. Dependency and configuration

- [x] 1.1 Verify the public warehouse provides season, gameweek, player, and team foreign-key identities.
- [x] 1.2 Add configured manager-entry and classic-league scope storage with enable/disable state.
- [x] 1.3 Add a session-provider interface and local secret injection without plaintext cookie persistence.
- [x] 1.4 Add source-client redaction and security tests for headers, logs, diagnostics, and errors.
- [x] 1.5 Define manager API envelopes, freshness fields, pagination, validation errors, and scope requirements.
- [x] 1.6 Define remote FPL session states, provider types, per-user ownership, reauthentication, revocation, retention, export, and deletion contracts.

## 2. Manager data model and adapter

- [x] 2.1 Add manager entry, season summary, gameweek summary, picks, automatic substitutions, chips, and transfer tables.
- [x] 2.1a Add active-team snapshot tables for imported player membership, lineup, captaincy, bank/value, chips, source gameweek, and freshness.
- [x] 2.2 Add source adapter methods for entry summary, history, transfers, and gameweek picks.
- [x] 2.3 Add optional authenticated adapter methods for `/me/` and `/my-team/{entry_id}/`.
- [x] 2.4 Normalize manager source payloads into season/gameweek facts with raw checksums and redacted metadata.
- [ ] 2.5 Add sanitized fixtures and tests for valid, missing, unauthorized, expired-session, and partial responses.
- [x] 2.6 Add provenance/checksum/normalization-version fields and source-conflict representation.
- [ ] 2.7 Add secret-provider tests proving no cookie/password/token enters database rows, payloads, logs, traces, browser storage, or error responses.

## 3. Manager synchronization

- [ ] 3.1 Add independent manager sync runs, stages, work items, retry policy, and freshness status.
- [x] 3.2 Implement idempotent entry summary/history and transfer synchronization.
- [x] 3.3 Implement gameweek picks synchronization with durable gameweek checkpoints.
- [x] 3.4 Implement optional private-team synchronization behind explicit session configuration.
- [x] 3.5 Ensure manager failures cannot change public warehouse sync state.
- [x] 3.6 Persist the latest active-team snapshot independently from the planning squad.
- [x] 3.7 Implement deterministic league-member selection by explicit IDs, rank range, and member limit.
- [ ] 3.8 Add synchronized member-pick work items linked to reusable entry/gameweek snapshots.
- [x] 3.9 Add remote-session expiry/permission handling that stops private retries, retains prior facts, and leaves public sync independent.

## 4. League data model and adapter

- [x] 4.1 Add league configuration, league metadata, standings snapshot, and standings-member tables.
- [x] 4.2 Add paginated classic-league standings adapter with phase and page parameters.
- [x] 4.3 Implement durable page work items, pagination checkpoints, idempotent upserts, and partial-page retention.
- [ ] 4.4 Add sanitized multi-page fixtures and tests for page errors, closed leagues, and permission failures.
- [ ] 4.5 Add league member identity/link tables and deterministic standings-to-entry reconciliation.

## 5. Analysis APIs and verification

- [x] 5.1 Add manager history, picks, transfer, and league standings read APIs with freshness metadata.
- [ ] 5.2 Add decision-analysis joins for points, captain multiplier, bench points, transfer cost, and chips.
- [x] 5.3 Add manager and league sync status/error reporting separate from public sync status.
- [x] 5.3a Add scoped manager endpoints, pagination, validation errors, and common freshness envelopes.
- [x] 5.4 Add active-team preview and imported-snapshot APIs.
- [ ] 5.5 Add explicit import-as-draft and replace-planning-squad actions with atomic validation and source provenance.
- [ ] 5.6 Add frontend controls for connect, sync, preview, import, and replace confirmation states.
- [ ] 5.7 Add league-member pick synchronization with configurable member limits, selected-gameweek scope, and bounded concurrency.
- [x] 5.8 Add team comparison read models and APIs for shared players, differentials, captain/bench/formation differences, and overlap metrics.
- [ ] 5.9 Add actual/provisional/estimated point-difference calculations joined to public player-gameweek facts.
- [ ] 5.10 Add frontend controls for league selection, gameweek selection, member subset selection, comparison, and omitted-member warnings.
- [ ] 5.11 Add integration tests for multiple entries, multiple leagues, pagination resume, member-pick failures, public-data joins, import validation, no-overwrite behavior, and point-difference labeling.
- [ ] 5.12 Document configuration, privacy/retention, session setup, comparison limits, estimate limitations, and the non-mutating scope of the feature.
- [ ] 5.13 Add contract tests for missing scope, source conflicts, deterministic member selection, and provenance.
- [ ] 5.14 Add Playwright coverage for connect/login, manager sync progress/errors, active-team preview/import, league selection, member limits, comparison metrics, omitted members, and no-overwrite confirmation states.
- [ ] 5.15 Add privacy integration tests for per-user isolation, disconnect/revocation, private export/deletion, retention cleanup, and public-fact preservation.
