## 1. Replay engine

- [ ] 1.1 Define backtest input, job, timeline, metric, exclusion, and common response contracts.
- [ ] 1.2 Implement deadline cutoff validation and future-leakage rejection.
- [ ] 1.3 Implement versioned lineup/captain/transfer replay with population and ruleset scope.
- [ ] 1.4 Implement per-week/aggregate metrics, coverage, calibration/error buckets, and algorithm comparison.
- [ ] 1.4a Implement and test recommendation-evaluation-v1 formulas, denominators, tie handling, oracle version, and not-applicable calibration.
- [ ] 1.5 Add async idempotency, progress, cancellation, immutable versioned outputs, and missing-feature behavior.

## 2. UI and verification

- [ ] 2.1 Build evaluation setup, progress, version comparison, timeline, metric, and excluded-input views.
- [ ] 2.2 Add integration tests for cutoffs, version isolation, deterministic reruns, incomplete/provisional labels, and metrics.
- [ ] 2.3 Add Playwright coverage for setup controls, progress/cancel, version filters, timeline, excluded-week explanations, loading/empty/error/stale states, and retrospective disclaimers.
