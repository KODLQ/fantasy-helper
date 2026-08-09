## 1. Rules and data model

- [ ] 1.1 Add versioned optimization ruleset storage sourced from season/game settings.
- [ ] 1.2 Add optimization run, weekly state, transition, transfer, point contribution, and comparison tables.
- [ ] 1.3 Add indexes for season, endpoint gameweek, run version, player, and manager comparisons.
- [ ] 1.4 Add data-completeness checks for prices, availability, player points, fixtures, and rules.
- [ ] 1.5 Add ruleset contracts for sell price, buy price, player eligibility, weekly grants, carry cap, hit cost, formations, blanks/doubles, and chip policy.
- [ ] 1.6 Add reproducibility-key and optimality-state storage.
- [ ] 1.7 Add official-rules/chip completeness fields, primary/secondary objective fields, and per-week change records.
- [ ] 1.8 Load and checksum `openspec/fpl-rules.yaml`/season overrides for chips, auto-substitutions, missing-data policy, and normalization policy.

## 2. Solver correctness

- [ ] 2.1 Define state, transition, objective, candidate ordering, and deterministic tie-breaker contracts.
- [ ] 2.2 Implement exact exhaustive/dynamic-programming solver for small synthetic seasons.
- [ ] 2.3 Test budget, squad composition, club limits, formations, player availability, and price-at-deadline constraints.
- [ ] 2.4 Test free-transfer balances, carryover, paid transfers, hit costs, gross points, and net points.
- [ ] 2.5 Add fixtures proving that later endpoint results can change earlier hindsight-optimal decisions.
- [ ] 2.6 Add exact fixtures for blank/double gameweeks, captain multipliers, automatic substitutions, team value, and historical price affordability.
- [ ] 2.7 Add hand-calculated transfer-cost regression cases covering zero, one, and multiple paid transfers.
- [ ] 2.8 Add exact full-player completeness fixtures proving that only an all-rules, all-required-input run can be labeled `complete_exact`.
- [ ] 2.9 Add tests proving team value/bank never beat a higher net-points path and are only tie-breakers.
- [ ] 2.10 Define benchmark dataset/cardinality, baseline hardware profile, runtime/memory/scratch/result-size/cancellation budgets, and certification record schema.
- [ ] 2.11 Add exact edge-case fixtures for each supported chip, chip exclusivity, wildcard/free-hit restoration, bench boost/triple captain scoring, captain/vice-captain, automatic substitutions, missing required facts, and constant-peer normalization.

## 3. Production optimization

- [ ] 3.1 Implement candidate-pool construction and configurable positional/player pruning.
- [ ] 3.2 Implement bounded beam search or branch-and-bound with deterministic pruning and progress reporting.
- [ ] 3.3 Add exact-versus-bounded verification on reduced real-season datasets.
- [ ] 3.4 Implement absolute-best-path and from-actual-squad starting modes.
- [ ] 3.5 Add immutable run caching keyed by input snapshots, ruleset, algorithm, endpoint, and parameters.
- [ ] 3.6 Add explicit candidate omission tracking, beam/search parameters, and exact-versus-bounded output labels.
- [ ] 3.7 Add full-player asynchronous mode and small-scope exact mode.
- [ ] 3.8 Add explicit rejection/incomplete behavior when official chip/rules/points inputs are missing.
- [ ] 3.9 Add feasibility benchmark harness for exhaustive synthetic, reduced real-season, and supported full-player inputs with deterministic checksums and zero-pruning evidence.
- [ ] 3.10 Gate `complete_exact` on benchmark certification and fall back to `feasibility_unproven`/`best_found_bounded` when any budget fails.

## 4. APIs and user experience

- [ ] 4.1 Add asynchronous run creation, status/progress, cancellation, and result APIs.
- [ ] 4.2 Add weekly timeline showing optimal squad, lineup, captain, bench, transfers, hits, gross points, and net points.
- [ ] 4.3 Add endpoint-gameweek selector and recalculation controls.
- [ ] 4.4 Add actual-versus-optimal comparison using synchronized manager history.
- [ ] 4.5 Add visible exact/bounded/incomplete/assumption-based labels and excluded-rule warnings.
- [ ] 4.6 Add reproducibility-key idempotency, cancellation, and no-partial-result APIs.
- [ ] 4.7 Add actual-versus-optimal API contract with weekly gross/net/transfer/captain/bench deltas.
- [ ] 4.8 Add weekly timeline change explanations, gameweek-only scoring, complete/best-found labels, team-value/bank reporting, and fantasy-rules help content.

## 5. Verification and documentation

- [ ] 5.1 Add integration tests for full synthetic seasons and representative historical fixtures.
- [ ] 5.2 Add regression tests for deterministic results, cache reuse, and algorithm-version isolation.
- [ ] 5.3 Add browser tests for endpoint selection, progress, timeline, transfer-cost breakdown, and actual-team comparison.
- [ ] 5.4 Document the retrospective/hindsight nature, optimization objective, transfer economics, candidate limits, and chip assumptions.
- [ ] 5.5 Add end-to-end verification for blank/double gameweeks, historical prices, transfer hits, endpoint recalculation, cancellation, and retrospective labeling.
- [ ] 5.6 Add Playwright coverage for endpoint recalculation, complete versus bounded labeling, weekly score/change breakdown, transfer-hit math, team value/bank display, missing-input warnings, and actual-team comparison.
- [ ] 5.7 Add runtime/storage/cancellation regression tests and Playwright coverage for feasibility-unproven results, certification details, and prohibition of complete-optimal wording before certification.
- [ ] 5.8 Run the feasibility benchmark and persist its certification report before enabling any UI/API copy that says `complete_exact` or “complete optimal team.”
