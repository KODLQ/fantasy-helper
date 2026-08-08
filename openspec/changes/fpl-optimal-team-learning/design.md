## Context

The public warehouse will contain historical player-gameweek facts, prices, fixtures, availability, game settings, and source snapshots. The application can therefore calculate a retrospective best-achievable season path rather than only ranking players independently for one gameweek.

This is a hindsight learning tool. It sees historical outcomes and must make that explicit. The important distinction is between selecting the best XI each week independently and finding a valid sequential path of squads connected by transfers, budgets, free-transfer allowances, and paid transfer costs.

## Goals / Non-Goals

**Goals:**

- Recalculate the best path through every completed gameweek up to a selected endpoint.
- Model historical prices, squad constraints, free transfers, carried free transfers, and transfer hits from a versioned ruleset.
- Record weekly squads, transfers, gross points, hit costs, net points, captain, bench, and provenance.
- Compare the optimal path with the user's actual team and with alternative optimization modes.
- Make exactness, bounded-search approximations, data completeness, and algorithm versions visible.

**Non-Goals:**

- Future prediction or actionable certainty.
- Automatically executing transfers.
- Optimizing every chip strategy in the first slice; chip support must be explicit in the selected ruleset/mode and otherwise be reported as excluded.
- Claiming global mathematical optimality when a bounded search was used.

## Decisions

### 1. Use a sequential path objective

For each gameweek, a state contains squad membership, bank, team value, free-transfer balance, and chip/ruleset state. A transition represents a valid set of transfers before the next deadline. The objective maximizes cumulative net points:

```text
net points = gross lineup points - (paid transfers × hit cost)
```

Bench and captain choices are part of the weekly decision. This prevents the misleading result of selecting a disconnected “best 15” every week.

For a completed gameweek, `grossGameweekPoints` is the sum of player points for the selected XI with captain multiplier applied. Bench points are informational unless the selected chip policy enables a bench-scoring chip. The primary objective is cumulative net points; secondary tie-breakers are fewer paid hits, higher remaining bank, higher team value, and deterministic source-ID ordering.

### 2. Source rules from versioned season settings

Import free-transfer allowance, carryover cap, transfer-hit cost, squad size, budget, club limit, formation rules, and currency multiplier from the season/game settings. Store a ruleset version with each run. If a setting is absent or changes, the run reports the assumption rather than silently using a constant.

The transition contract is:

```text
freeBefore(t) = min(carryCap, freeAfter(t-1) + weeklyGrant(t))
freeUsed(t) = min(transfers(t), freeBefore(t))
paidTransfers(t) = max(0, transfers(t) - freeBefore(t))
hitCost(t) = paidTransfers(t) * hitCostPerTransfer(t)
freeAfter(t) = freeBefore(t) - freeUsed(t)
netPoints(t) = grossPoints(t) - hitCost(t)
```

The ruleset also supplies the sell-price function, buy-price-at-deadline, maximum squad/club constraints, player eligibility, and chip policy. No transition may spend more bank plus valid sell proceeds than is available.

### 3. Recalculate at every endpoint gameweek

Each endpoint gameweek gets a separate run keyed by season, endpoint gameweek, starting mode, ruleset version, and algorithm version. Prior runs remain immutable. A later endpoint may choose different earlier transfers because later observed outcomes change the hindsight-optimal path.

The endpoint run includes every gameweek from the configured start through the endpoint. Blank gameweeks remain transitions; double gameweeks aggregate all fixtures for a player in that gameweek.

### 4. Use exact search for correctness fixtures and bounded optimization for production

Small synthetic seasons use exhaustive dynamic programming to prove constraints and transfer economics. Real seasons use a bounded beam search or branch-and-bound strategy with deterministic ordering, pruning, and a configurable candidate pool. Results expose `exact` or `bounded` optimality status and search parameters.

The production default is bounded beam search with configurable beam width and candidate pool. A full-player run is asynchronous. Every bounded result includes omitted candidates, beam width, maximum transfers per transition, and a reproducibility key. UI copy uses `optimal` only for exact runs and `best found` for bounded runs.

### 5. Support two analysis modes

- **Absolute best path:** choose the starting squad under the initial budget and rules.
- **From actual squad:** start from the user's synchronized first-gameweek squad and optimize only subsequent decisions.

This distinguishes “how good was theoretically possible?” from “how much did later decisions cost me?”

The absolute-best path starts with a legal 15-player squad at the first selected deadline. The from-actual-squad path requires an imported manager snapshot at the selected start gameweek and preserves its bank, team value, and free-transfer state. A run fails validation if its requested starting state is missing or inconsistent.

### 6. Preserve explanation and comparison data

Store weekly state, transition transfers, transfer cost, player points, captain multiplier, bench points, formation, and constraint checks. The UI can then show where the path gained points and how many hits were paid to achieve them.

The initial API contract is:

- `POST /api/v1/analysis/optimal-runs` with season, start/end gameweek, starting mode, optional entry ID, chip policy, candidate policy, objective, and ruleset version.
- `GET /api/v1/analysis/optimal-runs/{runId}` for status, progress, optimality, completeness, and assumptions.
- `GET /api/v1/analysis/optimal-runs/{runId}/timeline` for weekly states and transitions.
- `GET /api/v1/analysis/optimal-runs/{runId}/compare?entryId=` for actual-versus-optimal deltas.

Run creation is asynchronous and idempotent by reproducibility key. Results use the common analysis freshness envelope.

## Risks / Trade-offs

- **[Search space is enormous]** → Use candidate pruning, bounded beam/branch-and-bound search, cached player scores, and exact small-fixture tests.
- **[“Optimal” is overstated]** → Display optimality status, algorithm version, candidate limits, and excluded chip/ruleset assumptions.
- **[Historical source data is incomplete]** → Stop or mark affected weeks incomplete; never fill missing points with zero without reporting it.
- **[Rules change across seasons]** → Version rulesets and import settings per season/endpoint.
- **[Hindsight is mistaken for advice]** → Label the page retrospective and show that later outcomes were used.
- **[Many weekly endpoint runs are expensive]** → Cache immutable runs by input identity and calculate asynchronously with progress.
- **[A bounded pool excludes the true best player]** → Report candidate policy and optimality status; provide exact small-scope and full-player asynchronous modes.
- **[Transfer economics are wrong]** → Keep sell-price, carryover, hit-cost, and chip rules in a versioned ruleset and validate against hand-calculated fixtures.

## Migration Plan

1. Add optimization ruleset, run, weekly-state, transition, and metric tables.
2. Implement exact solver against synthetic mini-seasons and validate squad/transfer rules.
3. Implement production bounded solver and compare it with exact results on reduced candidate pools.
4. Add endpoint-gameweek recalculation, caching, progress, and incomplete-data reporting.
5. Add UI timeline, weekly optimal squad, transfer path, net/gross point comparison, and user-team comparison.
6. Roll back by disabling runs and hiding incomplete results; retain stored runs for later algorithm versions.

The release gate requires exact synthetic-season fixtures, a hand-verified transfer-cost example, a blank/double gameweek example, a cache/reproducibility test, and an actual-versus-optimal comparison.

## Open Questions

- What is the default candidate pool: all players, top N per position, or players above a minimum minutes threshold?
- Should the first release optimize chips when chip history and rules are available, or show a transfer-only baseline first?
- Should the objective include team value/bank preservation as a secondary objective after net points?
- How should ties be broken: fewer hits, higher remaining bank, lower team value, or lexicographic player IDs?
- What candidate policy is the default for a full-season bounded run, and how is the omitted-player list presented?
- Are chips disabled, user-selected, or optimized in the first production release?
