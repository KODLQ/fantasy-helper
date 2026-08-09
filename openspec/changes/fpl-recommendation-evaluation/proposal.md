## Why

Recommendations are useful only if the application can show how a scoring heuristic performed on historical information available at the time. A reproducible evaluation change separates retrospective evidence from future-looking advice.

## What Changes

- Add versioned historical replay for lineup, captain, transfer, and player-ranking algorithms.
- Enforce per-gameweek information cutoffs and reject future leakage.
- Add aggregate metrics, confidence/coverage, algorithm comparisons, and failure explanations.
- Add evaluation pages and Playwright coverage for filters, versions, and incomplete data.

## Capabilities

### New Capabilities

- `recommendation-evaluation`

## Impact

Depends on the public warehouse, manager sync, local auth, and common response contract. It adds replay jobs/read models and does not change live recommendation semantics or execute transfers.
