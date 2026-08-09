## Context

The manager/league sync supplies repeatable pick snapshots. This change derives comparisons without changing source facts or silently mixing gameweeks.

## Decisions

### Scope and member selection

Every request requires `leagueId`, `seasonId`, and `gameweek`. Member selection is deterministic: explicit `entryIds`, otherwise rank range, otherwise the first `memberLimit` ordered by current rank then entry ID. The response returns selected IDs, omitted IDs/count, snapshot IDs, and missing inputs. One league comparison reuses one pick snapshot per member.

### Metrics

- `teamOverlap(A,B)` is Jaccard similarity of the selected 15-player rosters; the UI also shows starting-XI overlap when requested.
- `differentialContribution(A,B)` is effective points from A-only selected players minus effective points from B-only selected players.
- `pointsDifference(A,B)` is net team points for the same season/gameweek and data state.
- `captainDelta` is captain-effective points minus the captain's base points.
- `benchPoints` is points of players with multiplier zero, with chip semantics explicit.

No metric compares different gameweeks or current prices to historical picks. Partial/provisional results retain their state and cannot be presented as final.

### API and UI

- `GET /api/v1/analysis/leagues/{leagueId}/summary?seasonId=&gameweek=&memberLimit=`
- `GET /api/v1/analysis/leagues/{leagueId}/comparison?seasonId=&gameweek=&entryIds=`
- `GET /api/v1/analysis/gameweeks/{gameweek}/autopsy?seasonId=&entryId=`

All use `{data,meta}` and common freshness/error contracts. The UI offers a league overview, rival selector, overlap/difference table, gameweek score breakdown, and an explicit “data unavailable/partial” state.

### Verification

Use exact synthetic pick snapshots for formula tests, including captain, bench, transfer hits, missing rivals, provisional live points, and duplicate member requests. Playwright covers navigation, filters, tables, empty/partial/stale/error states, and accessible labels.
