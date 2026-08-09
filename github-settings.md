# GitHub Settings

This is the GitHub configuration for the workflow in `README.md`.

These settings keep `main` and `release/*` review-only, require the
validation checks emitted by GitHub Actions, and keep releases tied to exact
version tags.

## Repository settings

Configure these in the repository’s GitHub Settings.

### Pull requests

Enable:

* Allow squash merging
* Allow merge commits
* Disable rebase merging
* Default to **Squash and merge**
* Automatically delete head branches

Use **Squash and merge** for normal `task/* → main` pull requests. Use
**Create a merge commit** when reconciling `release/* → main` after a release
tag.

GitHub does not enforce merge style by source and target branch pair, so the
release maintainer must select the merge-commit option for reconciliation PRs.

## Branch protection

Create rulesets or classic branch protection rules for:

### `main`

Target branch pattern: `main`

Enable:

* Require a pull request before merging
* Require at least **1** approval
* Require status checks to pass before merging:
  * `build`
  * `test`
  * `lint`
* Require branches to be up to date before merging
* Dismiss stale approvals when new commits are pushed
* Require conversation resolution
* Block force pushes
* Restrict deletions
* Include administrators

`main` is merge-only. Normal changes arrive through pull requests from
`task/*`.

### `release/*`

Target branch pattern: `release/*`

Apply the same protections as `main`, plus restrict bypass or merge
permissions to the repository’s Release Maintainer(s), where your GitHub plan
and ruleset configuration support that control.

`release/*` is merge-only and must be reconciled back into `main` after
each release tag.

### Task branches

`task/*` is intentionally not protected as a long-lived shared branch.
Developers may push their own task branches, then delete them after merging.
Use the branch naming convention from the workflow:

* `task/<slug>`
* `task/<issue-number>-<slug>`

GitHub does not provide a native negative branch-pattern rule for this
namespace. Enforce it through review convention, and
optionally add a lightweight naming check if the repository needs hard
enforcement.

## Tag protection

Create a tag ruleset for release tags:

* Target tags matching `[0-9]*.[0-9]*.[0-9]*`
* Restrict creation, update, and deletion
* Bypass only for the Release Maintainer(s)

If using classic tag protection, protect the same version-tag pattern. Keep
release candidate and final version tags annotated and immutable by convention.

Create a second catch-all tag ruleset if your GitHub plan supports it, with no
bypass users except repository administrators. This prevents arbitrary tag
names
from triggering release automation.

## Actions and status checks

The validation workflow is at `.github/workflows/validate.yml`. It runs on:

* pushes to `main`, `task/*`, and `release/*`
* pull requests targeting `main` or `release/*`

Its required job names are `build`, `test`, and `lint`. Add those exact
checks to the required-status-check list for both protected branch rules.

The reconciliation workflow is at `.github/workflows/reconcile.yml`. It runs
from a pushed version tag and creates a pull request from the corresponding
`release/<major>.<minor>` branch into `main`. The workflow uses the
tag-triggered GitHub context and never resolves the current branch tip to
choose a release commit.

In repository Settings → Actions → General, allow workflows to create and
approve pull requests if you want the reconciliation workflow to open the PR
with the repository-provided `GITHUB_TOKEN`.

## Practical effect

With these settings:

* `main` and `release/*` are protected and review-gated
* `task/*` remains the normal disposable work namespace
* `task/* → main` uses squash merging
* `release/* → main` uses a merge commit
* only protected version tags can start release reconciliation
* release identity remains the exact commit referenced by the tag
