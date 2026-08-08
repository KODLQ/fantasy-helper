## Context

The public warehouse provides season, player, fixture, and gameweek facts but not the decisions made by a manager or the context of a league. The Postman collection documents public manager entry endpoints for summaries, history, transfers, and picks, plus cookie-authenticated `/me/` and `/my-team/{team_id}/` endpoints and paginated classic-league standings. This change depends on the public warehouse's season, gameweek, player, and team identities.

The application is currently local-first and single-user. The design must be useful without requiring an FPL login, while making authenticated data possible without storing raw session cookies in ordinary database rows or diagnostics.

## Goals / Non-Goals

**Goals:**

- Synchronize explicitly configured manager entries and classic leagues into durable, season/gameweek-scoped tables.
- Preserve historical picks, captaincy, transfers, chips, ranks, budgets, and outcomes for analysis.
- Keep the synchronized active FPL team available as an imported snapshot and make adoption into the planning workspace an explicit user action.
- Make it possible to compare the selected manager's team with other members of a configured league for a chosen gameweek.
- Isolate manager/league failures from public-data freshness.
- Provide a secure session injection boundary for endpoints that require cookies.
- Expose data and freshness through stable application APIs.

**Non-Goals:**

- Executing transfers, changing a team, or mutating any FPL account.
- Supporting arbitrary social/private-league discovery in the first release.
- Building multi-user authentication and authorization for the application.
- Storing reusable plaintext FPL passwords or raw cookies in PostgreSQL.

## Decisions

### 1. Use explicit sync scopes

Configuration contains entry IDs and league IDs selected by the operator. Public entry summaries/history/transfers/picks use entry IDs. Private `/me/` and `/my-team/{id}/` calls are enabled only when an authenticated session provider is configured. No endpoint is discovered or synchronized implicitly.

### 2. Separate manager data from public facts

Manager tables reference public season, gameweek, player, and team identities but have their own sync runs, freshness, and retention. A failed private session or deleted league must not mark the public warehouse stale.

The manager domain uses these logical records:

- `manager_entry`: configured entry identity and display metadata.
- `manager_season_summary`: season totals, ranks, value, bank, transfers, and source observation.
- `manager_gameweek_summary`: event points, rank, overall rank, bank, value, transfer count/cost, and bench points.
- `manager_picks`: one row per entry/gameweek/player with position, multiplier, captain, vice-captain, and bench state.
- `manager_transfers`: transfer in/out, prices, gameweek, timestamp, and hit cost.
- `active_team_snapshot`: latest `/my-team`/picks projection used for import preview.
- `league`, `league_standing_snapshot`, `league_member`: configured league and paginated standings observations.

Every manager response includes `entryId`, `seasonId`, `gameweek`, `snapshotId`, `sourceFetchedAt`, `normalizedAt`, `state`, `missingInputs`, and `warning`.

### 3. Store immutable observations plus current projections

Use manager season summaries, gameweek summaries, picks, transfers, chips, and league-standing snapshots keyed by entry/league, season, and gameweek or phase. Repeated fetches are idempotent by source keys while fetch metadata and checksums preserve observation history when values change during live scoring.

### 4. Separate the remote active team from the planning squad

Persist the latest active team as an imported squad snapshot containing entry ID, season, gameweek, player IDs, positions, multipliers, captain/vice-captain, bank/value, chips, and source freshness. The UI exposes preview, import-as-new-draft, and replace-planning-squad actions. Synchronization updates the imported snapshot only; it never overwrites the saved planning squad automatically.

### 5. Inject sessions through a credential boundary

The source client accepts a short-lived cookie/header provider or local secret reference. It redacts cookie values from logs, errors, raw payload metadata, and HTTP diagnostics. The first local implementation may use an environment/file secret reference; a hosted implementation must replace this with a secret manager.

### 6. Treat standings pagination as durable work

Each league, phase, and standings page becomes a work item. The sync follows `has_next`/page metadata, stores page checkpoints, and upserts members by league, season, phase, page, and entry source ID. A page failure does not erase previously synchronized pages.

### 7. Synchronize league-member team selections selectively

League standings identify the member entry IDs. For a selected gameweek, the worker creates bounded pick-fetch work items for those members and stores their team snapshots independently from the user's own active-team session. The default scope is the configured league and selected gameweek; the implementation must support a configurable member limit and pagination so a large league cannot trigger an unbounded fan-out.

Member selection is deterministic: explicit `entryIds` win; otherwise selected rank range is applied; otherwise the first configured `memberLimit` standings rows ordered by rank are used. The API returns the selected and omitted IDs. A member may appear in several leagues, but its pick snapshot is stored once per entry/season/gameweek and linked to all configured league comparisons.

### 8. Compare actual, live, and estimated outcomes separately

Team comparison returns common players, differentials, captain/vice-captain, bench, formation, and player-level contributions. For completed gameweeks, points are actual finalized FPL points. For live gameweeks, values are provisional. For future gameweeks, the comparison may use the application's documented heuristic signals, but it must label them as estimates and never present them as official FPL points.

### 9. Offer analysis-oriented APIs

Read endpoints return manager history, picks, transfers, imported active-team snapshots, league standings, and league team comparisons with freshness metadata. A derived view joins picks to public player-gameweek facts to expose points, bench points, captain multiplier, transfer cost, decision-versus-outcome inputs, and pairwise team differences without exposing raw source field names. Planning-squad import is an explicit write operation with a preview and validation step.

The initial API contract is:

- `GET /api/v1/manager/status` — scope-level sync state and freshness.
- `GET /api/v1/manager/entries/{entryId}/summary?seasonId=` — season summary.
- `GET /api/v1/manager/entries/{entryId}/history?seasonId=` — gameweek history.
- `GET /api/v1/manager/entries/{entryId}/picks?seasonId=&gameweek=` — synchronized picks.
- `GET /api/v1/manager/entries/{entryId}/transfers?seasonId=` — transfer history.
- `GET /api/v1/manager/leagues/{leagueId}/standings?seasonId=&gameweek=&page=` — standings page.
- `GET /api/v1/manager/leagues/{leagueId}/comparison?seasonId=&gameweek=&entryIds=` — comparison view.
- `GET /api/v1/squad/import/preview?entryId=&seasonId=&gameweek=` — non-mutating planning import preview.
- `POST /api/v1/squad/import` with `mode=draft|replace` and a snapshot ID — validated planning import.

All endpoints use stable application field names, pagination metadata where applicable, and the common public-warehouse freshness object.

## Risks / Trade-offs

- **[Sessions expire or are rejected]** → Classify 401/403 separately, keep public sync healthy, report reauthentication required, and retain previous manager data.
- **[Manager data is sensitive]** → Minimize configuration, redact secrets, restrict diagnostics, document local-only scope, and make retention explicit.
- **[Live picks and ranks change]** → Store fetched observations and mark finalized gameweeks; do not pretend an in-progress snapshot is final.
- **[Remote sync unexpectedly changes user planning]** → Keep imported snapshots separate and require an explicit preview/confirm action before creating or replacing a planning squad.
- **[League standings are large]** → Use paginated work items, bounded concurrency, and configurable league scope rather than syncing every possible league.
- **[Comparing every member creates too many requests]** → Use selected gameweeks, configurable member limits, bounded concurrency, cached snapshots, and explicit refresh actions.
- **[Estimated points are mistaken for actual results]** → Return an outcome status of `actual`, `provisional`, or `estimated`, with the source gameweek state and algorithm version.
- **[Entry IDs may be reused or data may span seasons]** → Use `(entry_id, season_id, gameweek/phase)` keys and retain source identity metadata.
- **[A private snapshot and public picks disagree]** → Prefer the endpoint appropriate to the requested fact, retain both source observations, and mark the derived active-team snapshot as conflicted until reconciled.
- **[Imported team no longer matches current player catalog]** → Preserve the imported snapshot, return structured validation errors, and never partially replace the planning squad.

## Migration Plan

1. Confirm the public warehouse migrations and identity keys are available.
2. Add manager/league tables, sync scope configuration, secret-provider interfaces, and redaction tests.
3. Implement unauthenticated entry and standings sync against sanitized fixtures.
4. Add picks, transfers, and derived decision-analysis views.
5. Add authenticated `/me/` and `/my-team/{id}/` support behind explicit configuration and integration tests using a fake session provider.
6. Enable scheduling per configured scope and expose independent status endpoints.
7. Roll back by disabling manager scopes and worker stages; preserve historical manager facts and public data.

The rollout gate requires sanitized fixtures for public entry endpoints, fake-session tests for private endpoints, a multi-page league fixture, and an end-to-end preview/confirm import that proves automatic sync cannot overwrite planning state.

## Open Questions

- Should an entry's current team be sourced from public picks only, or should `/my-team/{id}/` be required for exact bank/chips state?
- Should importing an active team create a new planning draft by default, or update the existing plan only after explicit confirmation?
- Should users provide a cookie manually for local use, or should the first release defer authenticated endpoints entirely?
- What default retention is appropriate for league members and historical manager snapshots?
- Should private or head-to-head league endpoints be added after classic standings?
- Should comparison default to the logged/configured manager versus the whole league, or allow arbitrary subsets of league members?
- Should future-gameweek comparison use the existing recommendation heuristic or remain limited to historical/live actuals in the first release?
- Should manager snapshots be retained indefinitely for the configured entry, or use the same raw/canonical retention classes as public data?
- What is the default maximum number of league members and historical gameweeks for automatic synchronization?
