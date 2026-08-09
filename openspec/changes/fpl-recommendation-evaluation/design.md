## Decisions

### Replay boundary

`POST /api/v1/analysis/recommendations/backtests` accepts season, start/end gameweek, algorithm version, weight/config version, population scope, lineup/captain mode, and optional entry ID. For each gameweek, every feature input must have an observation timestamp at or before the deadline. Actual points after the deadline are evaluation labels only. The job records cutoff, input snapshot IDs, algorithm/weight versions, and missing features.

### Metrics and output

Return per-gameweek and aggregate coverage, top-k hit rate, rank correlation where applicable, points of recommended XI/captain, regret against hindsight best available choice, calibration/error buckets for estimated signals, and actual/provisional/estimated state. Metrics are not blended across incompatible seasons/rulesets. A version comparison uses identical population, cutoffs, and labels.

The default metric contract is `recommendation-evaluation-v1`:

```text
recommendedPoints(gw) = sum(actualPoints(player) × lineupMultiplier)
captainPoints(gw) = actualPoints(selectedCaptain) × captainMultiplier
oraclePoints(gw) = best legal XI/captain score under the declared hindsight oracle
regret(gw) = oraclePoints(gw) - recommendedPoints(gw)
topKHitRate = count(recommendedPlayerIds ∩ outcomeTopKPlayerIds) / min(K, evaluatedRecommendedPlayers)
coverage = validEvaluatedGameweeks / requestedGameweeks
```

`topKHitRate` uses the same position/population, outcome top-K definition, and average ranks for ties. Spearman rank correlation is calculated only over players with both recommendation and outcome ranks; fewer than three comparable players yields `unavailable`. Aggregate point/regret metrics are arithmetic means over valid gameweeks, never zero-filled omissions. Calibration is available only when an algorithm emits probabilities; otherwise it is `not_applicable`. Every metric includes its denominator, excluded weeks, and oracle version.

### Execution and UI

Backtests are asynchronous, idempotent by input identity, cancelable, and immutable by algorithm version. `GET /api/v1/analysis/recommendations/backtests/{id}` returns progress/result; `GET .../timeline` returns gameweek rows. The UI explains that this is retrospective evaluation, shows excluded weeks/features, and never calls historical success a future guarantee.

### Verification

Synthetic fixtures include a feature that changes after the deadline and must be rejected, missing features, provisional labels, algorithm ties, and identical reruns. Playwright covers job creation, progress, version/filter changes, timeline, excluded-week explanations, and all empty/error/stale states.
