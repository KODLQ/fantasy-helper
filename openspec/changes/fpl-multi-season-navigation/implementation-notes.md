## Implicit season migration matrix

| Area | Previous behavior | Multi-season behavior |
| --- | --- | --- |
| `PostgresRepository.LoadSnapshot` | Loads `seasons.is_current` | Compatibility wrapper; scoped reads use `LoadSnapshotForSeason` |
| Player search | SQL predicate `s.is_current` | `PlayerQuery.SeasonID` is resolved at the API boundary |
| Player detail and compare | Source player ID in current season | Season ID plus source player ID |
| Squad and recommendations | Current snapshot and globally resolved player IDs | Season-scoped snapshot and one squad plan per season |
| Dataset freshness | Optional season filter | Resolved season is included for first-party requests |
| Frontend player cache | Filter string only | Season ID is part of URL, request, and cache key |
| Frontend detail/compare/squad/recommendation | No season parameter | Shared application season context supplies `seasonId` |
| Application shell | Hard-coded season and GW 1 | Server catalogue label and URL-backed gameweek |
| Sync configuration | One ID/name pair | Typed official, retained, or archive source profile |
| Scheduler | Assumes configured source is current | Rejects historical live/scheduled scopes |

Downstream manager, league, analysis, live, recommendation-evaluation, and optimal-team changes must consume the shared season URL/context and common API scope. They must not introduce a second season selector or query season-scoped identities without season ID.

## Downstream scope audit

The canonical dependency registry places `fpl-multi-season-navigation` before manager/league delivery. The manager, league, analysis, transfer, recommendation-evaluation, live/storyline, and optimal-team proposals all consume warehouse season identities and common response scope; none currently implements a competing selector. Their implementation gates are:

- use the shell `SeasonProvider` and `season`/`gameweek` URL parameters;
- include `seasonId` in every season-dependent request, cache key, repository query, and persisted derived-result identity;
- clear or revalidate manager, league, fixture, comparison, recommendation, live, and optimal-team state when scope changes;
- reject a response whose `meta.scope.seasonId` differs from the active URL;
- keep `seasons.is_current` exclusively as source-refresh policy, never user preference.

## Verification record — 2026-08-09

- Fresh PostgreSQL database: applied migrations `000001` through `000007`, then ran all backend tests with `TEST_DATABASE_URL` and `-count=1`; passed.
- Migration round-trip: applied `000007_multi_season_navigation.down.sql` and `.up.sql` against the disposable database, then reran all PostgreSQL integration tests with `-count=1`; passed.
- Backend: `go test -count=1 -race ./...`, `go vet ./...`, and coverage profile; passed. Total statement coverage was 42.4% (the application package was 43.9%).
- Frontend: `npm run verify`; formatting, TypeScript lint, seven unit tests, and production build passed.
- Browser: `npm run test:e2e:headless` and `npm run test:e2e:headed`; both passed 27/27 Chromium tests.
- Runtime: `sh scripts/smoke.sh`; passed against the local Compose stack.
- Real data: the local catalogue contained one official current season and at least one retained historical season simultaneously; passed.
- OpenSpec: `openspec validate --changes --strict`; 12/12 changes passed.
- Portfolio: `./scripts/validate-openspec-portfolio`; 12 changes, 25 capabilities, and 12 formulas passed dependency and ownership validation.
