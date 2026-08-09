## Why

Users need more than the best team for the current gameweek: they need to understand the best achievable season path under the rules that existed at each point in time. A weekly recalculated optimal-team view can show what a perfect manager could have achieved so far, while accounting for squad constraints, prices, free transfers, paid transfer hits, and the compounding effect of earlier decisions.

## What Changes

- Add a historical optimal-team engine that recalculates the best achievable squad and lineup path through every completed gameweek up to a selected point.
- Recalculate the optimal path after each gameweek so later performance can change which earlier transfers or players were optimal in hindsight.
- Model the official season ruleset from source data, including squad composition, budget, club limits, free-transfer allowances, carryover limits, and transfer-hit costs.
- Account for the number of transfers made each week and deduct paid transfer costs from the optimal path's net points.
- Preserve weekly decisions, transfers in/out, transfer costs, lineup, captain, bench, chips/ruleset assumptions, gross points, and net points for explanation.
- Provide a timeline comparing the optimal path with the user's actual team and alternative strategies.
- Expose algorithm version, optimization objective, ruleset version, data completeness, and tie-breakers for reproducibility.
- Add configurable approximations and bounded optimization so historical analysis remains usable as the search space grows.
- Add an explicit optimization-run contract with starting mode, endpoint gameweek, ruleset version, chip policy, candidate policy, objective, optimality status, and input snapshot identity.
- Show gross points, transfer hits, net points, remaining bank, team value, weekly transfers, and the exact reason each path transition was selected.
- Define the product as the complete hindsight-optimal legal path when an exact full-player run has all official source rules and facts; bounded or assumption-based runs must use visibly weaker labels.
- Score and explain the result at each gameweek boundary: the weekly row is the only scoring interval, and every transfer, lineup, captain, and chip change is attributed to the boundary before that gameweek.
- Treat net points as the only optimization objective. Team value and remaining bank are reported every week and used only as deterministic tie-breakers after points, never as a substitute for points.

## Capabilities

### New Capabilities

- `optimal-team-learning`: Calculate, explain, version, and display the best achievable historical team path under FPL constraints and transfer economics.

### Modified Capabilities

- None.

## Impact

- Adds a historical optimization engine, weekly path/snapshot tables, transfer-cost calculations, ruleset versioning, and analytical APIs/UI.
- Depends on complete public player-gameweek, price, fixture, availability, and game-setting data from the public warehouse.
- Integrates with manager data to compare the optimal path with actual squads, transfers, captaincy, and points.
- Requires deterministic tie-breaking, versioned algorithm output, and tests for small exhaustive seasons before using bounded optimization for real seasons.
- This is a learning and retrospective tool; it is not a guarantee of future points and does not execute transfers.
- A bounded result is labeled as “best found under configured search limits,” never silently presented as mathematically optimal.
- “This is the complete optimal team that was possible” is reserved for a `complete_exact` run; the UI explains the fantasy rules, hindsight nature, weekly scoring, transfers, hits, and team-value reporting beside the result.
- Do not promise `complete_exact` until a benchmark proves exhaustive full-player search meets the documented local runtime, memory, scratch-space, and persisted-result budgets.
