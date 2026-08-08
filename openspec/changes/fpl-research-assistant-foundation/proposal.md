## Why

Fantasy Premier League decisions are spread across raw game data, player histories, fixtures, and squad constraints. A local research assistant can turn the official FPL data into one searchable workspace, make player comparisons faster, and surface explainable lineup and captain choices for the user's current squad.

## What Changes

- Add a backend and database foundation for storing the current FPL season, gameweeks, teams, players, fixtures, player histories, and sync metadata.
- Add an idempotent synchronization workflow for the official FPL endpoints, including retryable failures and a visible last-successful-sync status.
- Add player research APIs and frontend views for search, filtering, sorting, comparison, and player detail history.
- Add a squad planner where a user can enter or edit a squad, track budget and formation constraints, and save a planning state locally.
- Add an explainable recommendation workflow that proposes a starting XI and captain/vice-captain using configurable form, minutes, fixture, and value signals.
- Add validation, health checks, and automated tests for data normalization, FPL constraints, sync behavior, and recommendation scoring.
- Make Docker Compose the primary application entry point with three separate services (`db`, `backend`, and `frontend`) in local, dev, and production modes.
- Add a Makefile interface with `make deploy ENV=local|dev|prod`, `make down ENV=local|dev|prod`, and `make dev` as a local-mode alias.
- Add visible Playwright browser tests for each major frontend feature and the local Docker Compose workflow.
- Add Playwright coverage for every actionable application button, including navigation, view toggles, filters, player actions, comparison actions, squad actions, and recommendation actions.

## Capabilities

### New Capabilities

- `fpl-data-sync`: Import and normalize official FPL season, player, team, fixture, and historical data with freshness and failure reporting.
- `player-research`: Search, filter, compare, and inspect player performance, availability, fixtures, form, minutes, price, and value data.
- `squad-planning`: Maintain a planning squad and validate budget, position, club, formation, starting XI, bench, and captain rules.
- `lineup-optimization`: Generate transparent starting XI, bench order, captain, and vice-captain recommendations from squad data and configurable signals.

### Modified Capabilities

- None.

## Impact

- Creates the initial `db`, `backend`, and `frontend` application structure in an otherwise empty repository.
- Adds a relational schema and migrations for FPL data plus a local planning workspace.
- Adds backend HTTP APIs, a scheduled/manual sync entry point, and frontend routes/components for the research workspace.
- Introduces dependencies for database access/migrations, HTTP client and validation, frontend application state, and test tooling.
- The first release depends on the availability and rate limits of the official FPL web APIs; raw source payloads and sync metadata will be retained sufficiently to diagnose normalization issues.
