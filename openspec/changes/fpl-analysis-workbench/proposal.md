## Why

The application will soon contain a durable FPL warehouse, manager sync, active-team import, and league team comparison, but users still need a coherent way to turn those facts into repeatable learning. A research workbench can explain what happened, test alternatives, identify useful differentials, and help the user make better decisions week after week.

## What Changes

- Add a league intelligence hub comparing the user's team with league averages, selected rivals, high-ranked teams, and league ownership patterns.
- Add a gameweek autopsy explaining points gained/lost through captaincy, bench decisions, transfers, differentials, and automatic substitutions.
- Add a transfer laboratory for testing transfer scenarios, hits, budgets, fixture runs, expected outcomes, and counterfactual past decisions.
- Add fixture-swing and differential research views for upcoming schedules, rotation, ownership, availability, and player alternatives.
- Add captaincy and bench review metrics measuring decision quality against available alternatives.
- Add recommendation backtesting so heuristic weights and lineup recommendations can be evaluated against historical gameweeks.
- Add a live gameweek center showing provisional points, captain multipliers, rank movement, rival progress, and likely substitutions.
- Add availability/news impact alerts for affected squads, likely replacements, and recommendation changes.
- Add a season storyline showing turning points, successful/failed transfers, rank movements, captaincy performance, and points left on the bench.
- Add explicit labels and freshness metadata distinguishing actual, provisional, and estimated values.
- Deliver the workbench in dependency order: shared metrics, league/gameweek learning, transfer/fixture research, recommendation evaluation, then live/season views.

## Capabilities

### New Capabilities

- `league-intelligence`: Analyze league members, rivals, ownership, overlap, differentials, and rank threats.
- `gameweek-autopsy`: Explain completed and live gameweek outcomes and decisions.
- `transfer-laboratory`: Simulate transfers, hits, budgets, fixture runs, and counterfactual decisions.
- `fixture-differential-research`: Research fixture swings, alternatives, ownership, rotation, and availability.
- `recommendation-evaluation`: Backtest lineup/captain heuristics and expose signal performance.
- `live-gameweek-center`: Present live points, rank movement, rival progress, captaincy, and substitution state.
- `season-storyline`: Build a chronological, explainable record of a manager's season.

### Modified Capabilities

- None.

## Impact

- Adds analytical read models, derived metrics, counterfactual calculation services, alerts, and new frontend workbench pages.
- Depends on the public warehouse and manager/league sync changes for historical player facts, manager decisions, league teams, and freshness states.
- Extends existing recommendation scoring with versioned evaluation and historical replay rather than changing its baseline semantics.
- Adds APIs for simulations, comparisons, backtests, alerts, and season summaries.
- Requires clear labeling that estimates and heuristic recommendations are not official FPL points or guarantees.
- Defines shared metric, API, cache-key, and incomplete-input contracts so each page produces compatible analysis.
