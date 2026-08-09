## Why

Users need a safe place to explore transfers and fixture swings before saving anything to their active plan. The research workflow must expose hit costs, prices, ownership, availability, and alternatives without mutating the user's squad.

## What Changes

- Add a non-persistent transfer laboratory with budget, squad, club-limit, price, free-transfer, and hit validation.
- Add fixture-swing and differential research across bounded horizons with ownership, availability, rotation, and source freshness.
- Add historical counterfactual replay using only information available before the selected deadline.
- Add optional save-to-planning handoff that requires explicit user confirmation.

## Capabilities

### New Capabilities

- `transfer-laboratory`
- `fixture-differential-research`

## Impact

Depends on the public warehouse, manager sync, local authentication, and the shared analysis contract. Adds simulation APIs, research pages, and Playwright coverage; a preview never writes manager/FPL state.
