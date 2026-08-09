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

For a completed gameweek, `grossGameweekPoints` is the sum of player points for the selected XI with captain multiplier applied. Bench points are informational unless the selected chip policy enables a bench-scoring chip. The primary objective is cumulative net points. Team value and remaining bank are valuable learning metrics because they preserve future buying power, but they never reduce points to win the optimization. Secondary tie-breakers are fewer paid hits, higher remaining bank, higher team value, and deterministic source-ID ordering.

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

### 2b. Canonical edge-case rules

The canonical rule details live in `openspec/fpl-rules.yaml` as `fpl-rules-v1`; the optimizer must load the season-specific values and record the ruleset checksum. It must not infer unsupported chip behavior from a modern default.

- Triple captain applies the season's configured captain multiplier for one gameweek only.
- Bench boost changes bench scoring only for its legal gameweek; it does not change the selected squad or transfer economics.
- Free hit applies legal temporary transfers for one gameweek and restores the pre-chip squad, bank, and value state according to the season ruleset.
- Wildcard applies the season's permanent unlimited-transfer behavior and its hit/free-transfer treatment.
- Chip overlap/exclusivity and chip availability are validated at the deadline from the season ruleset.
- After the gameweek, a zero-minute starter may be replaced by the first eligible bench player in stored bench order only when the resulting formation remains legal. If the captain did not play, the eligible vice-captain receives the configured captain multiplier.
- Missing required points, prices, availability, fixtures, rules, or chip data makes the affected result incomplete; values are never silently zero-filled or replaced with current values. Nullable optional fields remain null and are reported.
- Shared analytical normalization uses `normalized-feature-v1`: missing values are excluded from the denominator; a constant peer set maps every present value to 0.5; points-per-90 requires at least the configured minimum minutes.

### 2a. Fantasy semantics shown to the user

The product explains that a manager has a legal 15-player squad, selects a legal starting XI and captain for each gameweek, receives official player points for all fixtures in that gameweek, and applies the captain multiplier and any enabled official chip rules. Bench points are shown as opportunity cost and only score when the selected official ruleset says they score. A transfer happens between gameweek deadlines, uses historical buy and sell prices, consumes available free transfers, and costs the configured hit amount for each transfer beyond the free allowance. No post-deadline information may change that week's decision.

The default mode is `complete_hindsight`, which models all official rules, chips, player availability, fixtures, prices, and scoring facts available for the selected season. If any required rule or fact is unavailable, the run cannot be labeled complete; it is `incomplete` or `assumption_based` and names the missing input. `transfer_only` is an explicit educational baseline, not a complete official-rules result.

### 3. Recalculate at every endpoint gameweek

Each endpoint gameweek gets a separate run keyed by season, endpoint gameweek, starting mode, ruleset version, and algorithm version. Prior runs remain immutable. A later endpoint may choose different earlier transfers because later observed outcomes change the hindsight-optimal path.

The endpoint run includes every gameweek from the configured start through the endpoint. Blank gameweeks remain transitions; double gameweeks aggregate all fixtures for a player in that gameweek.

### 4. Use exact search for correctness fixtures and bounded optimization for production

Small synthetic seasons use exhaustive dynamic programming to prove constraints and transfer economics. Real seasons use a bounded beam search or branch-and-bound strategy with deterministic ordering, pruning, and a configurable candidate pool. Results expose `complete_exact`, `best_found_bounded`, `incomplete`, or `assumption_based` status and search parameters. Only `complete_exact` may use the headline “This is the complete optimal team that was possible.”

The production default is bounded beam search with configurable beam width and candidate pool. A full-player run is asynchronous. Every bounded result includes omitted candidates, beam width, maximum transfers per transition, and a reproducibility key. UI copy uses `optimal` only for exact runs and `best found` for bounded runs.

### 4a. Certify feasibility before using `complete_exact`

`complete_exact` is a certification state, not a solver aspiration. It may be returned only when the implementation has enumerated the full legal state space for the declared season/ruleset with no candidate pruning, and a repeatable benchmark has passed on the documented baseline profile: 4 vCPU, 8 GB RAM, local PostgreSQL, and SSD storage.

The initial practical budgets are:

| Resource | Certification target |
| --- | --- |
| Full-season endpoint run | P95 wall-clock ≤ 30 minutes |
| Peak solver memory | ≤ 4 GB resident memory |
| Temporary search storage | ≤ 2 GB per concurrent run |
| Persisted result | ≤ 100 MB per season/endpoint run, excluding shared warehouse facts |
| Concurrent exact runs | 1 per worker; additional requests queue or use bounded mode |
| Cancellation | Stop new expansions within 30 seconds and persist no completed-result claim |

The benchmark suite must include exhaustive synthetic seasons, a reduced real-season dataset whose answer is independently verified, and the full eligible-player count/ruleset shape for the supported season. Lossless dynamic-programming state merging is allowed only when its equivalence key is documented and independently verified; candidate/transfer/fixture pruning is not allowed for certification. The benchmark records input cardinalities, expansions, pruning, wall time, peak memory, scratch bytes, result bytes, and deterministic checksum. If any target fails, the run is `feasibility_unproven` and the UI must offer only `best_found_bounded` or a smaller exact scope. The certification record is versioned by algorithm, ruleset, database schema, hardware profile, and benchmark dataset.

### 5. Support two analysis modes

- **Absolute best path:** choose the starting squad under the initial budget and rules.
- **From actual squad:** start from the user's synchronized first-gameweek squad and optimize only subsequent decisions.

This distinguishes “how good was theoretically possible?” from “how much did later decisions cost me?”

The absolute-best path starts with a legal 15-player squad at the first selected deadline. The from-actual-squad path requires an imported manager snapshot at the selected start gameweek and preserves its bank, team value, and free-transfer state. A run fails validation if its requested starting state is missing or inconsistent.

### 6. Preserve explanation and comparison data

Store weekly state, transition transfers, transfer cost, player points, captain multiplier, bench points, formation, and constraint checks. The UI can then show where the path gained points and how many hits were paid to achieve them.

Each timeline row is one gameweek and contains: gameweek score, gross points, transfer hit, net points, cumulative net points, optimal starting XI, captain/vice, bench, chips, starting/ending bank, starting/ending team value, free transfers before/after, and a `changes` list. Each change names transfer in/out, price, free/paid classification, hit cost, captain/lineup change, or chip activation. The score is never averaged across a range: selecting a later endpoint recalculates the entire path and displays the exact weekly deltas that make that endpoint optimal. Blank gameweeks remain in the timeline; double gameweek points aggregate all fixtures in that gameweek.

The initial API contract is:

- `POST /api/v1/analysis/optimal-runs` with season, start/end gameweek, starting mode, optional entry ID, chip policy, candidate policy, objective, and ruleset version.
- `GET /api/v1/analysis/optimal-runs/{runId}` for status, progress, optimality, completeness, and assumptions.
- `GET /api/v1/analysis/optimal-runs/{runId}/timeline` for weekly states and transitions.
- `GET /api/v1/analysis/optimal-runs/{runId}/compare?entryId=` for actual-versus-optimal deltas.

Run creation is asynchronous and idempotent by reproducibility key. Results use the common analysis freshness envelope.

The run contract includes `mode=complete_hindsight|transfer_only`, `optimalityStatus`, `completeness`, `missingInputs`, `officialRulesIncluded`, `chipPolicy`, `primaryObjective=net_points`, `secondaryTieBreakers`, feasibility benchmark ID/budget, and per-week `changes`. The API rejects a request for `complete_hindsight` when required rules/data are known to be unavailable or feasibility is uncertified, or creates an explicitly incomplete/unproven run that cannot be promoted to the complete headline.

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

## Finalized product decisions

- The complete mode searches the full eligible player universe and official rules; a bounded candidate pool is a separate best-found mode and reports omitted candidates.
- The complete mode includes official chips when the season source provides their rules and availability. Missing chip rules prevent a complete label; transfer-only is available as a clearly labeled baseline.
- Net points are the sole primary objective. Remaining bank and team value are reported each week and are secondary tie-breakers after hit count.
- Ties resolve in this order: higher cumulative net points, fewer paid hits, higher remaining bank, higher team value, then stable player/source IDs.
- Each selected endpoint gameweek has its own immutable run and may change earlier hindsight decisions; the page says this explicitly.
