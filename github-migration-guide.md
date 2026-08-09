# Migration Guide

Use this guide to bring an existing repository onto this workflow. The end
state is what `github-settings.md` describes.

## Prerequisites

* Owner or admin rights on the repository in GitHub
* At least one person identified to be a Release Maintainer (a second is
  encouraged but not required)
* Awareness of any in-flight branches that do not yet match the new naming

## Steps

### 1. Communicate the change

Announce the cutover date in the team's normal channel. Pending in-flight PRs
should either land before cutover or be renamed to match the new naming
afterward. Do not rename branches that are open for review without telling
the reviewers.

### 2. Designate Release Maintainers

Pick at least one person to be Release Maintainer for this repository. A
second is encouraged where it is practical. Maintainers are assigned
per-repository — add them as bypass actors for the `release/*` branch ruleset
and the version-tag ruleset, where your GitHub plan supports bypass actors.

### 3. Apply GitHub settings

Apply every setting in `github-settings.md`:

1. `main` branch protection
2. `release/*` branch protection
3. version-tag protection

If you maintain multiple repos, apply the same settings to each.

### 4. Configure CI

CI must cover:

* pushes to `main`, `task/*`, and `release/*` (validation)
* pushes of version tags (release artifact build, **from the tagged commit**)

The release workflow must build the commit referenced by the pushed tag and
must not resolve "tip of `release/*`" at runtime. This is the safety
property that protects against accidental branch-tip releases.

Require the exact CI check names (`build`, `test`, and `lint`) in the `main`
and `release/*` branch rules so PRs cannot merge without them. The names must
match what the workflow emits.

### 5. Migrate the trunk(s)

This is the step that varies most across repositories. Pick the case that
matches the repo's current state.

#### Case A: single trunk named `main`

Nothing to do here.

#### Case B: single trunk named `master`

The repository has only `master`, used as the integration branch. Rename it
to `main` in GitHub (Settings → Branches (or Rules) → rename default branch). GitHub's
rename will redirect open PRs to the new name.

After the rename, every contributor must update their local checkout:

```sh
git fetch origin
git remote set-head origin -a
git branch -m master main
git branch -u origin/main main
```

Update CI references (`branches: [master]` → `branches: [main]`) and any
external integration that hard-codes the branch name.

#### Case C: dual trunks (`master` + `development`)

This is the GitFlow-ish shape used by some legacy repos (for example the
historical state of `Xearch` and `Xync`). `development` is the integration
branch and the default. `master` is a long-lived branch that tracks
production, with its own commits that may not exist on `development`.

Mapping to the new model:

* `development` becomes `main`.
* `master` is **not** the new `main`. It represents an active production
  line and should become a `release/<major>.<minor>` branch — or, if it is
  truly stale, be retired.

Steps:

1. **Inventory commits unique to `master`.**

   ```sh
   git fetch origin
   git log --oneline origin/development..origin/master
   ```

   If this is empty, `master` is fully contained in `development` and is
   only a legacy pointer. If it has commits, those are production-only
   patches that must be carried forward.

2. **Decide what `master` represents.** Look at the most recent version
   tag reachable from `master` (`git describe --tags origin/master`). The
   branch typically tracks the current shipping major.minor.

3. **Cut `release/<major>.<minor>` from `master`.**

   ```sh
   git switch -c release/<major>.<minor> origin/master
   git push -u origin release/<major>.<minor>
   ```

   This new branch inherits all of master's history and its production-only
   commits.

4. **Forward-port any production-only commits to `main`.** Cherry-pick each
   commit identified in step 1 onto a `task/*` branch off the renamed
   `development → main` and open a PR. This is a one-time recovery of
   history that was orphaned on `master`.

5. **Rename `development → main`.** In GitHub, set `main` as the default and
   rename `development`. Update CI and integrations.

6. **Retire `master`.** Once `release/<major>.<minor>` is in place and any
   forward-ports have landed on `main`, delete `origin/master`. Tags pin
   the historical release commits regardless of the branch's existence.

#### Case D: existing `release-*` branches with the wrong shape

Both `Xearch` and `Xync` carry hyphenated, per-patch release branches like
`release-2.3.0`, `release-2.3.1`, `release3-2-1-rc1`, `quick-release-3-2-1`.
These do not match the new convention (`release/<major>.<minor>`,
long-lived per major.minor line).

* For each shipped version, confirm a corresponding annotated tag exists at
  the released commit. If a tag is missing, create it from the correct
  commit before deleting the branch.
* Keep one `release/<major>.<minor>` branch for each major.minor line that
  is still receiving patches. Cut it from the latest patch tag of that
  line.
* Delete the old hyphenated `release-*` branches once the tags and the new
  `release/<major>.<minor>` branches cover the same history.

### 6. Rename in-flight branches

Rename open work to the `task/*` namespace. The legacy zoo collapses:

| Old prefix(es) | New |
|---|---|
| `feature/<slug>` | `task/<slug>` |
| `fix/<slug>`, `bug/<slug>`, `bugfix/<slug>` | `task/<slug>` |
| `chore/<slug>`, `refactor/<slug>`, `enhancement/<slug>` | `task/<slug>` |
| `internal/<slug>`, `temp/<slug>`, `experiment/<slug>` | `task/<slug>` |
| `pen-test-<slug>`, `pentest-<slug>` | `task/<slug>` |
| `QA/<slug>`, `qa/<slug>`, `qa2/<slug>`, `qa3/<slug>` | review state lives in the PR, not the branch name → `task/<slug>` |
| `merge/<slug>` | `task/<slug>` |
| `hotfix/<slug>` | follow the hotfix policy in `README.md`; there is no `hotfix/*` namespace |

For each rename:

```sh
git branch -m old-name task/new-name
git push origin :old-name task/new-name
git push origin -u task/new-name
```

If the branch has an open PR, update its source branch in GitHub after the
push.

For repositories with hundreds of stale branches (Xearch and Xync both have
\> 100 remote branches), do not try to rename everything. Rename only the
branches that have **open PRs or active commits in the last 30 days**, and
delete or leave-as-is the rest. Old merged branches do not affect the new
workflow; they only clutter the branch list.

### 7. Cut your first new release branch (if applicable)

If the repository ships versioned releases and you did not already create
`release/<major>.<minor>` in step 5 / Case C:

```sh
git switch main
git pull --ff-only
git switch -c release/<major>.<minor>
git push -u origin release/<major>.<minor>
```

Then tag according to the release flow in `README.md`.

### 8. Inform the team

Point developers at `cheatsheet.md`. Most of what they need to do day-to-day
fits on that one page.

## Rollback

If you need to back out the workflow on a repository, the only state changes
that are not trivially reversible are version tags (protected, immutable).
Branch protection rules and merge-style settings can be reverted in GitHub at
any time. Renamed branches can be renamed back. Any tags pushed during the
trial remain valid as historical records.
