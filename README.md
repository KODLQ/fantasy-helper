# Fantasy Helper

Fantasy Helper is a local FPL research desk for comparing players, planning a 15-player squad, and generating an explainable lineup/captain heuristic.

## Run with Docker Compose

Prerequisite: Docker Desktop (or Docker Engine with Compose v2).

The repository is designed around three containers: `db`, `backend`, and `frontend`. Plain `docker compose up` starts the local hot-refresh stack. The Makefile provides the explicit environment commands:

```sh
# local development with Vite HMR and backend source watching
make dev

# equivalent explicit form
make deploy ENV=local

# staging-like, production-built containers
make deploy ENV=dev

# production image set
make deploy ENV=prod

# stop a selected environment
make down ENV=local
```

Use `make logs ENV=local` to follow service output and `make config ENV=local` to inspect the resolved Compose configuration. Local opens at [http://localhost:5173](http://localhost:5173); dev at [http://localhost:5174](http://localhost:5174); prod at [http://localhost:8080](http://localhost:8080). The environment files under `deploy/env/` keep the database volumes, ports, and API URL separate. Replace the production password in `deploy/env/prod.env` before a real deployment.

The application starts with a small deterministic demo snapshot so the workspace is useful before the first official sync. Use “Sync official data” to import the configured FPL endpoints.

## API surface

- `GET /healthz` — service, database-network, and data status.
- `GET /api/v1/sync/status` — current/last sync state and freshness warning.
- `POST /api/v1/sync` — start an asynchronous official data sync.
- `GET /api/v1/players` — paginated search, filters, and deterministic sorting.
- `GET /api/v1/players/:id` — normalized profile, history, and upcoming fixture context.
- `GET /api/v1/players/compare?ids=1,2` — compare up to four players.
- `GET|PUT /api/v1/squad` — read or validate/save the local planning squad.
- `POST /api/v1/recommendations` — generate the baseline lineup, bench, captain, and vice-captain.

## Data and recommendation limits

The first release is intentionally local and single-user. It does not authenticate to an FPL account, execute transfers, or access private leagues. The source sync keeps the last known good normalized snapshot when a stage fails. The recommendation is a documented heuristic using form, expected minutes, fixture difficulty, recent returns, and value; it is not a guaranteed point projection.

Upstream-shape fixtures for adapter tests live in `backend/testdata`. Keep them sanitized and update the adapter when the official response shape changes.

## Verification

```sh
cd backend && GOCACHE=/tmp/fantasy-helper-gocache go test ./...
cd frontend && npm run build && npm test

# with the local stack already running
sh scripts/smoke.sh

# optional PostgreSQL persistence integration test
cd backend && TEST_DATABASE_URL='postgres://fantasy:fantasy@localhost:5432/fantasy_helper_test?sslmode=disable' go test ./internal/app -run TestPostgresRepositoryPersistence -v
```

### Browser acceptance tests

Playwright uses Chromium and runs headed by default so local test activity is visible in a browser window. Start the local stack first with `make dev`.

```sh
cd frontend
npm run e2e:install
npm run test:e2e              # visible local run
npm run test:e2e:headed       # explicit visible run
npm run test:e2e:headless     # CI-friendly run
npm run test:e2e:ui           # interactive Playwright UI
```

Set `E2E_BASE_URL` when testing another Compose mode, for example `E2E_BASE_URL=http://localhost:5174 npm run test:e2e:headed` for dev.

---

# Git Workflow
Task branches, release branches, and tag-driven releases.

## Related docs
* [Cheat sheet](cheatsheet.md)
* [Git client settings](git-client-settings.md)
* [GitHub settings](github-settings.md)
* [GitHub migration guide](github-migration-guide.md)

## Goals
* Keep **`main` buildable at all times**.
* Keep branch naming **simple and easy to reason about**.
* Make the safe path the **default path for every shared Git action**.
* Make every released version **reproducible from an exact commit**, by
  separating development branches from release-artifact identity.

## Invariants
* Every release candidate and final release comes from a protected version
  tag that points to a specific commit.
* Version tags point at commits on a `release/*` branch.
* All changes to `main` and `release/*` go through a PR with review and CI.
* Work done on `release/*` is merged back into `main`.
* Published branches (`main` and `release/*`) are never rebased or
  force-pushed; mistakes are fixed with `git revert`.
* Shared remote branches are limited to `main`, `task/*`, and `release/*`.

The hard safety boundary is `main`, `release/*`, and version tags. `task/*` is
treated as disposable per-developer staging and is not protected — the safe
behavior on `task/*` is convention, not enforcement.

## Git taxonomy

### Branches
These are the only shared branch names used by the workflow.

`<id>` is the ClickUp task ID (for example `865d6p9z8`).
`<slug>` is a short lowercase hyphenated summary of the work.

* `main`
  * the integration branch
  * always merged into by PR
* `task/<slug>`
  * normal short-lived work with no ClickUp task
  * PR target is `main`
* `task/<id>-<slug>`
  * normal short-lived work with a ClickUp task
  * PR target is `main`
* `release/<major>.<minor>`
  * release stabilization and maintenance line
  * release candidates and final releases are tagged from commits on this branch

Examples:

```sh
task/restore-macos-keychain-entitlements
task/865d6p9z8-improve-linux-server-detection
task/869c9re7u-oracle-link-pdbs-to-instances
release/2.3
```

> `task/*` is used for all normal short-lived work, regardless of whether the
change is a feature, a fix, a refactor, docs, CI, or other focused work.

### Tags
Release-producing tags are **annotated tags** and point at an exact commit on
the relevant `release/*` branch.

* `<major>.<minor>.<patch>-rc.<n>`
  * release candidate tag from a commit on `release/<major>.<minor>`
* `<major>.<minor>.<patch>`
  * final release tag from a commit on `release/<major>.<minor>`

Examples:

```sh
2.3.0-rc.1
2.3.0
2.3.1-rc.1
2.3.1-rc.2
2.3.1
```

## Daily development flow
1. Branch from `main`:

   ```sh
   git switch main
   git pull --ff-only
   git switch -c task/865d6p9z8-improve-linux-server-detection
   ```
2. Commit and push to the task branch.
3. Open a PR to `main`.
4. PR must pass checks and review.
5. Merge by **squash**.

Merge style by PR type:

* normal `task/* -> main` PRs use **squash**
* `release/* -> main` reconciliation PRs use a **regular merge commit** (see
  Release flow step 6 for why)

When the work does not have a ClickUp task, use `task/<slug>` instead:

```sh
git switch main
git pull --ff-only
git switch -c task/restore-macos-keychain-entitlements
```

Local unpublished work is yours: rebase, amend, and reset freely on a
`task/*` branch you have not pushed yet. Once you push the branch, treat its
history as published — if it falls behind `main`, update it by merging
`main` into it. Do not rebase or force-push a published branch.

## Commit and PR title convention
Commits and PR titles use the same Conventional Commit format:

```text
<type>: <description>
```

PR titles append the ClickUp task ID at the end when the work has one. The
suffix is required on PR titles (because the PR title becomes the squash
commit on `main`) and is not used on individual commits.

```text
<type>: <description> [<id>]
```

Example — one task branch's life from first commit to PR title:

```text
# Branch: task/865d6p9z8-improve-linux-server-detection
# Commits as you work — no ClickUp ID on individual commits:
feat: improve linux server detection
refactor: lint fixes
fix: narrow linux workstation chassis hints
fix: satisfy linux chassis clippy lint
refactor: spell out linux chassis hint values

# PR title — ClickUp ID appended; this becomes the squash commit on main:
feat: improve linux server detection [865d6p9z8]
```

When the work has no ClickUp task, the branch is `task/<slug>` and the PR
title omits the suffix:

```text
# Branch: task/software-walk-network-drive-traversal
fix: gate software walk network drive traversal   # PR title (no ID)
```

### Allowed prefixes
These prefixes apply to both commit messages and PR titles:

* `feat:` for new user-facing functionality
* `fix:` for bug fixes
* `docs:` for documentation-only changes
* `refactor:` for behavior-preserving code restructuring
* `test:` for adding or updating tests
* `chore:` for non-user-facing maintenance outside CI
* `ci:` for build, test, release, or automation wiring
* `perf:` for performance improvements
* `ai:` for agent instruction and workflow changes

### Style rules
These style rules apply to both commit messages and PR titles:

* no scope: use `fix: simplify macos receipt cleanup`, not `fix(macos): simplify macos receipt cleanup`
* imperative mood: use `fix: reject empty macos mount snapshots`, not `fixed empty macos mount snapshots`
* lowercase description
* no trailing period
* keep the subject specific enough that a reader skimming `git log` six
  months from now can tell what changed without opening the diff

Bad examples:

```text
feat(detection): Improve linux server detection
fixed empty mount snapshots
docs: update config examples.
fix: bug fix
chore: small cleanup
```

Corrected:

```text
feat: improve linux server detection
fix: reject empty macos mount snapshots
docs: add exclude_network_drives to config examples
fix: classify scan-root filesystem via statfs
chore: update rand to 0.9.4
```

## CI and artifact flow
Normal CI can run on branch pushes:

* pushes to `task/*` validate task work
* pushes to `main` validate integrated work
* pushes to `release/*` validate stabilization and patch work

Those branch workflows may produce ordinary CI artifacts for internal use if desired.

The following invariants must always be upheld:

* release candidates and final releases must come from version tags
* the release workflow must build the commit referenced by the tag itself
* the release workflow must not resolve "latest commit on `release/*`" at runtime

That is the protection against accidental branch-tip releases.

## Release flow
`main` is where normal development continues. `release/<major>.<minor>` is
where a release line is frozen, stabilized, tagged, and eventually reconciled back into `main`.

Follow this procedure:

1. Cut the release branch from `main`:

   ```sh
   git switch main
   git pull --ff-only
   git switch -c release/2.3
   git push -u origin release/2.3
   ```

2. Stabilize on `release/2.3`. All changes to `release/2.3` go through a PR
   with review and CI, the same as `main`.
3. Tag release candidates from explicit commits on that branch:

   ```sh
   git tag -a 2.3.0-rc.1 <commit> -m "2.3.0-rc.1"
   git push origin 2.3.0-rc.1
   ```

4. Tag the final release from the chosen release-branch commit:

   ```sh
   git tag -a 2.3.0 <commit> -m "2.3.0"
   git push origin 2.3.0
   ```

5. If you need a patch release on that same line, keep using `release/2.3`,
   apply the fix there, and tag the next version from that branch:

   ```sh
   git tag -a 2.3.1-rc.1 <commit> -m "2.3.1-rc.1"
   git push origin 2.3.1-rc.1

   git tag -a 2.3.1 <commit> -m "2.3.1"
   git push origin 2.3.1
   ```

6. Reconcile back to `main` immediately after each tag. Open a PR from
   `release/2.3` to `main` and merge it with a **regular merge commit**, not
   squash. Do not let a release line accumulate tagged commits that have
   never been reconciled.

   Reconciling immediately is what keeps the hotfix flow simple: the latest
   release tag stays reachable from `main`, so the next hotfix can branch
   from that tag and merge into both branches without dragging release-only
   history along.

   The merge commit is what makes this scale: it establishes the parent link
   from `main` back to `release/*`, so each subsequent reconciliation PR
   only shows the *new* commits on the release line. Without that link,
   every reconciliation re-includes previously-reconciled work.

## Hotfix flow
A fix that needs to land on a current `release/*` line and on `main` is made
on a `task/*` branch cut from the **latest production tag** of that release
line, merged into the release line, tagged, and reconciled back into `main`.

1. Branch from the latest tag of the release line:

   ```sh
   git fetch --tags
   git switch -c task/<id-or-slug> 2.3.4
   ```

2. Make the fix on the task branch.
3. Open a PR to `release/<major>.<minor>` and merge by squash. Tag the next
   patch from the new release-branch tip:

   ```sh
   git tag -a 2.3.5 <commit> -m "2.3.5"
   git push origin 2.3.5
   ```

4. Reconcile `release/<major>.<minor>` to `main` (Release flow step 6) — a
   PR with a merge commit. After this, the new patch tag is reachable from
   `main`, and the next hotfix can branch from it the same way.

The fix ends up reachable from both branches at the same SHA, so Git itself
records the equivalence.

If a fix only applies to `main` (not to a current release line), follow the
normal Daily development flow — branch from `main`, PR into `main`. No
release-branch involvement.

Apply the same flow to every active `release/*` line that needs the fix
(typically at most one or two at any given time).

## Operational policies

### Release Maintainers
Every repository has at least one Release Maintainer. Adding a second is
encouraged where the team is large enough to support it, both to remove a
single point of failure for shipping fixes and to avoid unilateral release
authority — but a single maintainer is acceptable while teams are small.

### Break-glass
If `main` or a `release/*` line is blocked and the situation cannot wait for
normal review, a repository administrator may temporarily relax branch
protection to land a fix. The expectation is:

* announce the break-glass action in the team's normal incident channel before
  acting
* restore the protection settings immediately after the fix lands
* open a follow-up PR or issue documenting what was bypassed and why

Break-glass is for incidents, not for convenience.

### Long-lived feature branches
The workflow assumes `task/*` branches are short-lived. There is no
separate long-lived feature-branch namespace.

When a piece of work is too large to land in one PR, prefer breaking it
into smaller PRs that each land on `main` independently, and gate the
in-progress feature behind a flag, a config switch, or a code path that is
not yet wired up if it cannot be exposed to users yet.

If incremental landing is genuinely not possible, keep the work on a
normal `task/*` branch and merge `main` into it regularly to keep it from
drifting. The branch is still expected to land via a normal squash PR.
Long-lived branches that exist purely to accumulate work in parallel with
`main` are not part of the workflow.

### Recovering from a botched tag
Protected version tags cannot be moved. If a tag was pushed at the wrong
commit, do not try to delete or relocate it. Treat the bad tag as burned and
ship the next patch version from the correct commit. For example, if `2.3.0`
was tagged at the wrong commit, do not republish `2.3.0`; tag `2.3.1` from
the correct commit and release that instead.

### Stale release branches
Once a release line is fully reconciled into `main` and no longer receives
patches, the `release/*` branch may be left in place as historical reference.
Version tags pin the exact release commits regardless of whether the branch
still exists, so deleting the branch later is safe if cleanup is desired.
