# AGENTS.md

## Project

Fantasy Helper is a local, single-user FPL research desk. It has a Go backend,
React/Vite frontend, PostgreSQL migrations, Docker Compose environments, and
OpenSpec change artifacts.

The repository is hosted on the personal GitHub account `KODLQ` at
`git@github.com:KODLQ/fantasy-helper.git`. Do not configure or use company
Gitea, ClickUp, or other company services for this project.

## Before changing files

1. Read this file and the relevant `README.md` sections.
2. Run `git status --short --branch`.
3. Preserve existing user changes. Do not reset, checkout, stash-drop, or
   overwrite unrelated work.
4. Work on a `task/*` branch for implementation. Do not push directly to
   `main` or `release/*`.

## Branches and GitHub Issues

Use only these shared branch forms:

* `main` — protected integration branch.
* `task/<slug>` — every normal short-lived task branch.
* `release/<major>.<minor>` — release stabilization and maintenance.

GitHub Issue numbers do not belong in branch names. Link an Issue from the PR
title or body instead, for example:

```text
feat: add transfer laboratory (#123)
```

Normal task PRs target `main` and use squash merging. Release stabilization
PRs may target `release/<major>.<minor>`. Reconciliation PRs from a release
branch back to `main` use a regular merge commit.

For a version 1.0 release, use branch `release/1.0` and annotated tag `1.0.0`.
Do not use `release/1.0.0`.

Published task branches must not be rebased or force-pushed. Merge `main` into
an outdated published task branch. Never move or recreate a protected release
tag; use the next patch version if a tag is wrong.

## OpenSpec workflow

Each directory under `openspec/changes/` is an independent change unless its
design explicitly declares a dependency. Develop independent changes on
separate `task/<slug>` branches and in separate sessions.

Before implementation, review that change's:

* `proposal.md`
* `design.md`
* `tasks.md`
* `specs/**/spec.md`

Implement the change's tasks, keep its OpenSpec artifacts accurate, and avoid
mixing unrelated change directories in the same branch or PR. If changes are
already mixed in the working tree, preserve them in a temporary local
checkpoint branch or commit before splitting them.

## Commits and pull requests

Use lowercase, imperative Conventional Commit subjects with no scope or
trailing period:

```text
feat: add player comparison filters
fix: preserve the last known good sync snapshot
docs: clarify release branch workflow
test: cover captain selection edge cases
ci: update github actions validation
```

Allowed prefixes: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`,
`perf`, and `ai`.

PR titles use the same format and append a GitHub Issue reference when
applicable: `<type>: <description> (#123)`. PR bodies should summarize the
change, list verification, and use `Fixes #123` when the PR closes an Issue.

## Verification

Run the checks relevant to the change. The standard application checks are:

```sh
cd backend && GOCACHE=/tmp/fantasy-helper-gocache go test ./...
cd frontend && npm run lint
cd frontend && npm test
cd frontend && npm run build
```

For integration verification, start the local stack with `make dev` and run:

```sh
sh scripts/smoke.sh
```

The GitHub Actions required checks are named `build`, `test`, and `lint`.
Keep those job names stable unless the repository settings and documentation
are updated together.

## Releases

1. Merge normal work into `main`.
2. Cut `release/<major>.<minor>` from `main`.
3. Stabilize through reviewed PRs and passing CI.
4. Create annotated release candidate or final tags from that release branch.
5. Push the tag and reconcile the release branch into `main` with a merge PR.

Release automation must use the exact commit referenced by the pushed tag; it
must never resolve the current tip of a release branch as a substitute.

## File and command safety

Use `apply_patch` for edits. Do not use destructive Git commands or delete
files unless the user explicitly requests it and the target is confirmed.
Do not commit generated output, secrets, `.env` files, `node_modules`, build
artifacts, Playwright reports, or test results.

When handing work back, report the files changed, verification performed, the
commit/branch state, and any unrelated uncommitted changes left untouched.

## Reference documents

* `README.md` — complete project and Git workflow.
* `cheatsheet.md` — daily Git reference.
* `github-settings.md` — GitHub repository configuration.
* `github-migration-guide.md` — migration and branch conversion guidance.
* `git-client-settings.md` — recommended local Git defaults.
