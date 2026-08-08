## Context

The public warehouse and manager/league sync changes provide historical facts, active teams, league members, picks, transfers, and freshness metadata. The workbench turns those facts into explainable research workflows: compare against rivals, understand past decisions, simulate alternatives, evaluate recommendations, observe live gameweeks, and review the season.

The product is local-first and single-user. Calculations must remain reproducible and must distinguish finalized facts from live observations and heuristic estimates. The workbench must not mutate an FPL account or silently modify the saved planning squad.

## Goals / Non-Goals

**Goals:**

- Provide a coherent research loop from current data to explanation, simulation, decision, and retrospective review.
- Reuse common analytical facts and calculation services across pages rather than implementing separate formulas in each UI.
- Version algorithms, weights, and source snapshots so results can be reproduced.
- Expose uncertainty, omissions, and freshness in every analysis result.
- Keep expensive league, backtest, and simulation work bounded and cacheable.

**Non-Goals:**

- Social messaging, public profiles, or automatic transfer execution.
- Guaranteed future-point predictions.
- Replacing the separate optimal-team-learning engine with ad-hoc UI calculations.
- Scraping news sources outside the configured FPL data boundary in the first release.

## Decisions

### 1. Build a shared analytical service layer

Create calculation services and read models for ownership, fixture difficulty, player contributions, decision outcomes, transfer scenarios, and recommendation replay. Frontend pages consume stable APIs and do not recalculate business metrics independently.

The common analysis response envelope is:

```json
{
  "data": {},
  "state": "actual|provisional|estimated|partial|stale|unavailable",
  "snapshot": {
    "seasonId": 1,
    "gameweek": 12,
    "publicSnapshotId": "...",
    "managerSnapshotIds": [],
    "algorithmVersion": "...",
    "rulesetVersion": "..."
  },
  "coverage": {
    "requested": 20,
    "available": 19,
    "omittedIds": [],
    "missingInputs": []
  },
  "warning": ""
}
```

Every analysis endpoint accepts explicit season/gameweek/horizon scope and returns this envelope.

### 1a. Define shared metric formulas

The initial metric contract is:

- `effectivePlayerPoints = playerGameweekPoints * pickMultiplier`.
- `grossTeamPoints = sum(effectivePlayerPoints for the lineup state)`.
- `benchPoints = sum(playerGameweekPoints where pickMultiplier = 0)`.
- `transferCost = paidTransfers * ruleset.hitCost`.
- `netTeamPoints = grossTeamPoints - transferCost`.
- `captainDelta = captainEffectivePoints - captainBasePoints`.
- `teamOverlap = |playersA intersection playersB| / |playersA union playersB|`, using the 15-player roster unless an endpoint explicitly requests starting XI.
- `differentialContribution(A,B) = sum(effective points of A-only players) - sum(effective points of B-only players)`.
- `pointsDifference(A,B) = netTeamPoints(A) - netTeamPoints(B)` for the same season, gameweek, and state.

Responses include the input IDs and formula version used for derived metrics.

### 2. Make result state explicit

Every result carries `actual`, `provisional`, or `estimated` state, source snapshot timestamps, and algorithm/ruleset versions where relevant. Completed gameweek facts can be compared as actual; live facts remain provisional; future estimates are heuristic outputs.

### 3. Reuse synchronized league and manager snapshots

League intelligence and rival comparison use synchronized standings and member picks. Requests have explicit league, gameweek, and member scopes with configurable limits. Missing members or failed pick requests appear as omissions rather than being silently treated as zero data.

Initial API routes are:

- `GET /api/v1/analysis/league/{leagueId}/summary?seasonId=&gameweek=&entryIds=&memberLimit=`.
- `GET /api/v1/analysis/league/{leagueId}/rivals?seasonId=&gameweek=&entryId=&memberLimit=`.
- `GET /api/v1/analysis/gameweeks/{gameweek}/autopsy?seasonId=&entryId=&rivalEntryIds=`.
- `POST /api/v1/analysis/transfers/simulate` with squad, transfers, horizon, objective, and snapshot IDs.
- `GET /api/v1/analysis/fixtures/swing?seasonId=&fromGameweek=&toGameweek=`.
- `GET /api/v1/analysis/differentials?seasonId=&gameweek=&population=&limit=`.
- `POST /api/v1/analysis/recommendations/backtest` with algorithm, weights, range, and entry ID.
- `GET /api/v1/analysis/live?seasonId=&gameweek=&entryId=&rivalEntryIds=`.
- `GET /api/v1/analysis/season/storyline?seasonId=&entryId=`.

### 4. Treat transfer analysis as a sandbox

The transfer laboratory receives a squad state and scenario, validates FPL constraints, computes transfer count, hit cost, budget, fixtures, and projected/observed outcomes, and returns a result. It never writes to the real planning squad unless the user explicitly saves a proposed draft.

Scenario results include baseline, scenario, delta, assumptions, horizon, free transfers used, paid transfers, transfer cost, gross points, and net points. A scenario is never compared across different snapshots without an explicit warning.

### 5. Version recommendation replay

Backtesting invokes the same recommendation algorithm with a historical snapshot and records algorithm version, weights, available information boundary, selected XI, captain, bench, and achieved points. It must not use post-gameweek values when evaluating what was knowable before the deadline.

The default backtest cutoff is the gameweek deadline. Fields observed after that cutoff are excluded. Results include per-gameweek and cumulative points, captain success, bench points, and comparison with the manager's actual lineup.

### 6. Use event-driven alert inputs

Availability, price, fixture, and recommendation alerts derive from warehouse snapshot changes. Alerts are deduplicated by player/event/type and carry severity, explanation, and affected squad/rival scopes.

Alert states are `new`, `acknowledged`, `dismissed`, and `resolved`; one event cannot create duplicate active alerts for the same scope.

### 7. Store a season event ledger

The storyline is assembled from manager actions, rank changes, captaincy outcomes, transfers, differentials, and missed points. Events retain source references and calculation versions so the narrative can be regenerated after metric changes.

Turning-point ranking is deterministic: material net-point delta first, rank movement second, and gameweek/source ID tie-breakers last. Thresholds are configuration, not hidden constants.

## Risks / Trade-offs

- **[Many pages compute similar metrics differently]** → Centralize domain calculations and add metric-definition tests.
- **[Live data changes rapidly]** → Cache with short TTLs, label provisional results, and retain finalized snapshots.
- **[Backtests leak future information]** → Enforce a historical cutoff timestamp and only expose data available at that gameweek deadline.
- **[League comparisons fan out to many requests]** → Pre-synchronize selected members, bound scopes, and reuse cached snapshots.
- **[Alerts become noisy]** → Deduplicate, support severity/filter settings, and include a clear reason/action.
- **[Users treat estimates as facts]** → Use visible state labels, factor explanations, and heuristic notices everywhere.
- **[A broad workbench becomes unshippable]** → Deliver in five gates and require a domain-test, API-contract test, partial-data test, and browser workflow at each gate.

## Migration Plan

1. Add shared analytical metric definitions and read-model migrations.
2. Implement league intelligence and gameweek autopsy against existing synchronized snapshots.
3. Add transfer scenarios and fixture/differential research.
4. Add recommendation replay, live center, alerts, and season storyline incrementally.
5. Add frontend routes, loading/partial states, and browser acceptance coverage for each workflow.
6. Roll back individual workbench pages without removing source or manager facts; preserve calculation versions and cached results.

Each gate must pass its contract, calculation, partial-data, and browser tests before the next gate begins.

## Open Questions

- Should the first live center use polling only, or introduce server-sent events later?
- Which alerts are enabled by default: injury/news, price movement, fixture swing, or recommendation change?
- Should league intelligence compare against league average, rank bands, or user-selected rivals by default?
- Should counterfactual transfer analysis use only information available at the historical deadline or offer a separate hindsight mode?
- Which metrics are allowed to use heuristic estimates, and which must be actual-only?
- What is the maximum simulation horizon and member/rival count for synchronous requests before an asynchronous job is required?
- Which alert delivery is required first: in-app only, email, or an external integration?
