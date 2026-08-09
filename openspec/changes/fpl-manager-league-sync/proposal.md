## Why

The public FPL warehouse will explain player and fixture performance, but it cannot analyze a user's actual decisions, transfers, captaincy, bench points, or league position. The Postman collection also documents manager-scoped and cookie-authenticated endpoints, so those records should be synchronized as an opt-in, separately secured data domain rather than mixed into the global public-data pipeline.

## What Changes

- Add configuration for one or more FPL entry IDs and explicitly selected classic leagues.
- Synchronize entry summaries, season history, transfers, gameweek picks, captaincy, bench points, automatic substitutions, chips, and transfer costs.
- Synchronize the configured manager's active FPL team and expose it as an imported squad snapshot in the application.
- Allow the user to preview the imported team and explicitly create or replace a planning-squad draft from it; remote syncs SHALL NOT silently overwrite the saved planning squad.
- Synchronize paginated classic-league standings by league, gameweek/phase, and page.
- Synchronize configured league members' gameweek team selections so managers in the same league can be compared.
- Add league team-comparison views showing shared players, differentials, captain/bench differences, formation, and actual/provisional/estimated point differences.
- Add an optional authenticated session boundary for `/me/` and `/my-team/{team_id}/`, without persisting raw cookies in ordinary source payloads or logs.
- Define the remote FPL session lifecycle, per-user connection ownership, reauthentication behavior, secret-storage boundary, retention, export/deletion, and privacy guarantees.
- Store manager and league observations by season and gameweek so decisions can be compared with public player/gameweek facts.
- Add idempotent snapshots, retryable/resumable jobs, pagination checkpoints, freshness, partial-failure, and permission-error reporting.
- Add read APIs for manager history, picks, transfers, league standings, and decision-vs-outcome analysis.
- Add explicit scope, snapshot, provenance, and completeness contracts for manager and league responses.
- Add bounded synchronization policies for selected entries, leagues, gameweeks, and member subsets rather than unbounded league fan-out.
- Keep public warehouse syncs independent so a missing or expired manager session cannot make global FPL data stale.

## Capabilities

### New Capabilities

- `fpl-manager-data-sync`: Synchronize configured manager entries, picks, history, transfers, and optional authenticated team state.
- `fpl-league-data-sync`: Synchronize configured classic-league standings with pagination, phase, and gameweek snapshots.
- `fpl-squad-import`: Preview and explicitly import a synchronized active FPL team into the local planning workspace.
- `league-team-comparison`: Compare synchronized teams within a league and explain where their gameweek outcomes differ.

### Modified Capabilities

- None.

## Impact

- Adds manager/league migrations, repositories, source adapters, sync jobs, configuration, status reporting, and analysis endpoints.
- Adds league-member pick synchronization and comparison read models/endpoints with configurable limits for large leagues.
- Requires a secure local credential/session injection mechanism for `/me/` and `/my-team/{team_id}/`; cookies must not be stored unencrypted or exposed through diagnostics.
- Depends on the public warehouse's season, gameweek, player, and team identities for foreign-keyed analysis.
- Adds privacy, retention, permission-error, and authenticated-source integration tests.
- Adds manager/league API contracts, imported-team provenance, comparison-state labels, and deterministic member-selection behavior.
- Does not execute transfers or modify an FPL account.
