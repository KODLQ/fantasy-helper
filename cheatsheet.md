# Cheat Sheet

One-page reference. Full rules live in `README.md`.

## Branches

| Branch | Use |
|---|---|
| `main` | integration; PR-only |
| `task/<slug>` | all normal short-lived work |
| `release/<major>.<minor>` | release stabilization, RCs, patch releases |

No other shared branch names exist.

Example: `task/improve-linux-server-detection`

## Daily flow

```sh
git switch main
git pull --ff-only
git switch -c task/improve-linux-server-detection
# work, commit, push
# open PR to main
# squash-merge after review + green CI
```

If a pushed `task/*` falls behind `main`, **merge** `main` into it. Do not
rebase or force-push a published branch.

## Commit and PR titles

Format: `<type>: <description>` — lowercase, no trailing period, and the
description starts with a present-tense verb ("add", "fix", "reject",
"remove" — not "added", "fixes", "rejected").

Example commit: `fix: reject empty macos mount snapshots`

PR title with a GitHub Issue:

```
<type>: <description> (#<issue-number>)
```

Allowed `<type>`: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`,
`perf`, `ai`.

## Merge style

| PR | Style |
|---|---|
| `task/* -> main` | squash |
| `release/* -> main` | merge commit |

Merge commit on the reconciliation is what keeps the next reconciliation
small.

## Tags

* `<major>.<minor>.<patch>-rc.<n>` — release candidate
* `<major>.<minor>.<patch>` — final release
* Tags are annotated and point at an exact commit on `release/*`
* Tags are protected and only Release Maintainers may push them

## Releasing

1. Cut `release/<major>.<minor>` from `main`
2. Stabilize on `release/*` (PRs + CI, like `main`)
3. Tag `<v>-rc.<n>` from chosen commits
4. Tag `<v>` from the chosen final commit
5. Reconcile `release/* -> main` with a **merge commit** immediately
   after each tag

## Hotfix on a current release

Branch from the **latest tag** of the release line — this is the code
running in prod — then merge into both branches via the normal flows.

```sh
git fetch --tags
git switch -c task/<slug> 2.3.4   # branch from the latest tag
```

1. Make the fix on the task branch.
2. PR to `release/<major>.<minor>`; squash-merge.
3. Tag the next patch from the new release-branch tip.
4. Reconcile `release/<major>.<minor>` → `main` (PR with merge commit).

The fix ends up on both branches at the same SHA.

## Don't

* Don't push directly to `main` or `release/*`
* Don't rebase or force-push published branches
* Don't move or re-tag a protected version tag — burn it and use the next patch
* Don't squash a `release/* -> main` reconciliation PR
* Don't tag a commit that isn't on a `release/*` branch

## When stuck

* Workflow rules: `README.md`
* Repo settings: `github-settings.md`
* Local Git defaults: `git-client-settings.md`
* Bringing an existing repo onto this workflow: `migration-guide.md`
