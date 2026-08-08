# Gitea Settings

This is the matching Gitea configuration for the workflow in `README.md`.

The point of these settings is to protect the branches and tags that matter to the workflow:

* `main` stays merge-only
* `release/*` stays merge-only, with merges restricted to Release Maintainers
* shared remote branches are limited to `main`, `task/*`, and `release/*`
* only approved version tags can trigger release automation
* release artifacts come from protected tags, not from branch pushes

## Repository
These settings have to be set up individually for each repository.

Each repository must have at least:

* one **Release Maintainer**
  * may merge PRs into `release/*`
  * may create version tags
  * a second maintainer is encouraged where practical, to avoid a single
    point of failure for shipping fixes; required minimum is one
  * Release Maintainers are assigned per-repository by listing the user(s)
    directly in the `release/*` branch protection allowlist and the
    version-tag protection allowlist

### Pull Requests

* Merge Styles: **Create squash commit**, **Create merge commit**
* Default Merge Style: **Create squash commit**
* ✓ Delete pull request branch after merge by default

Merge style by PR type (convention — not enforced by Gitea):

* normal `task/* -> main` PRs use **Create squash commit**
* `release/* -> main` reconciliation PRs use **Create merge commit**

Gitea cannot restrict merge style by source and target branch pair, and we
do not add a custom status check for this. The squash default covers the
common case (every `task/*` PR), and the merge-commit choice for
reconciliation is the responsibility of the Release Maintainer merging it.
The reason the split exists is explained in `README.md` — it is what keeps
each subsequent reconciliation PR small.

## Branches
The branch protection rules must be in the exact order shown below. Earlier rules take priority over later ones.

### `main`
Protected branch name pattern: `main`

* ✓ Disable Push
* ✓ Require Signed Commits
* ✓ Disable Force Push

#### Pull Request Approvals
* Required approvals: **1**
* ✓ Enable Status Check
  * list the exact status check names your CI emits (for example `build`,
    `test`, `lint`). The names must match the check names produced by your
    pipeline; if they do not match, the protection rule silently lets PRs
    merge without them.
* ✓ Block merge on rejected reviews
* ✓ Block merge if pull request is outdated
* ✓ Administrators must follow branch protection rules

`main` is merge-only. No direct pushes.

### `release/*`
Protected branch name pattern: `release/*`

* ✓ Disable Push
* ✓ Require Signed Commits
* ✓ Disable Force Push
* ✓ Allowlist Restricted Merge: **Release Maintainer**

#### Pull Request Approvals
* Required approvals: **1**
* ✓ Enable Status Check
  * list the same set of CI check names required on `main` (for example
    `build`, `test`, `lint`). The names must match the check names produced
    by your pipeline.
* ✓ Block merge on rejected reviews
* ✓ Block merge if pull request is outdated
* ✓ Administrators must follow branch protection rules

`release/*` is merge-only, like `main`. All changes go through a PR with
review and CI — the same gates that protect `main` (Disable Push, required
approvals, required status checks, block-on-outdated, block-on-rejected,
admins-must-follow). The only release-specific addition is the merge
allowlist: only Release Maintainers may merge into a release branch.

With the rules above, the shared branch taxonomy is:

* `main`
* `task/<slug>`
* `task/<id>-<slug>`
* `release/<major>.<minor>`

`task/*` is intentionally not protected directly. In Gitea, any branch that
matches a protected-branch rule becomes awkward to delete manually, which makes
short-lived task branches annoying to clean up.

### Catch-all
The catch-all rule blocks every branch name except `main`, `release/*`, and
single-segment `task/*`. It also blocks exact `task` (no slug) and nested
`task/foo/bar` branch names. It must come after the specific `main` and
`release/*` rules so those remain usable.

The pattern works by listing every prefix shape that is *not* the
`task/<single-segment>` namespace, alternative by alternative:

* `[!t]**` — any name whose first character is not `t`
* `t`, `t[!a]**` — bare `t`, or names starting with `t` but not `ta`
* `ta`, `ta[!s]**` — bare `ta`, or names starting with `ta` but not `tas`
* `tas`, `tas[!k]**` — bare `tas`, or names starting with `tas` but not `task`
* `task` — bare `task` with no slug
* `task/**/**` — anything under `task/` with more than one path segment

The resulting pattern is the union of those alternatives:

Protected branch name pattern: `{[!t]**,t,t[!a]**,ta,ta[!s]**,tas,tas[!k]**,task,task/**/**}`

* ✓ Disable Push
* ✓ Disable Force Push
* ✓ Enable Merge Allowlist (no users)
* ✓ Administrators must follow branch protection rules

The shared branch namespace is fixed at `main`, `release/*`, and `task/*`.
Do not loosen this rule to admit new namespaces.

## Tags
Protected tags are what enforce that release automation runs only from stable, explicit commit references.

### Release candidate and final version tags
Protected tag pattern: `/\A\d+\.\d+\.\d+(-rc\.\d+)?\z/`

* ✓ Allowed users: **Release Maintainer**

These tags identify release candidates and final releases from `release/*`
branches.

### `*`
Protected tag pattern: `*`

* no allowed users
* no allowed teams

This catch-all rule blocks every other tag name. It must come after the
release-version rule so only release candidates and final releases remain
pushable.

## Required CI trigger policy
The Gitea UI settings above are not enough on their own. The workflows must
also follow these rules:

* validation CI must cover pushes to `main`, `task/*`, and `release/*`
* normal branch CI may publish ordinary non-release artifacts if desired
* any automated release workflow must run only on version tag pushes
* the release workflow must build the commit referenced by the pushed tag
* the release workflow must not infer a build commit from the current tip of
  `main` or `release/*`

This is the mechanism that enforces the key invariant:

* branch pushes may validate code and produce ordinary CI artifacts
* only a protected version tag may produce a release artifact

## Recommended automation

### Auto-open reconciliation PR after each release tag

The hotfix flow in `README.md` requires that `release/*` is reconciled into
`main` immediately after each tag. A Gitea Action automates this by
auto-opening the reconciliation PR on every final-release tag push.

Drop the following at `.gitea/workflows/reconcile.yml`:

```yaml
name: reconcile

on:
  push:
    tags:
      - '[0-9]*.[0-9]*.[0-9]*'

env:
  CI_TAG: ${{ gitea.ref_name }}

jobs:
  open-pr:
    name: open reconciliation PR
    runs-on: linux-docker
    container:
      image: alpine:latest
    env:
      GITEA_TOKEN: ${{ secrets.RECONCILE_BOT_TOKEN }}
      GITEA_SERVER_URL: ${{ gitea.server_url }}
      GITEA_REPOSITORY: ${{ gitea.repository }}
    steps:
      - name: Install curl
        run: apk add --no-cache curl

      - name: Derive release branch name
        id: branch
        run: |
          release_branch="release/$(echo "$CI_TAG" | cut -d. -f1-2)"
          echo "name=$release_branch" >> "$GITHUB_OUTPUT"

      - name: Open reconciliation PR
        env:
          RELEASE_BRANCH: ${{ steps.branch.outputs.name }}
        run: |
          curl -fsSL -X POST \
            -H "Authorization: token $GITEA_TOKEN" \
            -H "Content-Type: application/json" \
            "${GITEA_SERVER_URL}/api/v1/repos/${GITEA_REPOSITORY}/pulls" \
            --data @- <<EOF
          {
            "base": "main",
            "head": "${RELEASE_BRANCH}",
            "title": "reconcile ${RELEASE_BRANCH} into main",
            "body": "auto-opened after tag ${CI_TAG}"
          }
          EOF
```

Setup notes:

* Create a service account (`reconcile-bot` or similar) in Gitea and
  generate a personal access token with `repo:contents` and `repo:pulls`
  scopes. Store as the `RECONCILE_BOT_TOKEN` repo secret.
* The bot only opens the PR; a human still reviews and merges. That keeps
  the reconciliation visible in normal review channels.
* If the reconciliation conflicts, the auto-opened PR cannot be merged
  until a human resolves it. That is the correct behavior — surfaces the
  conflict instead of hiding it.

## Practical effect
With this setup:

* `main` remains merge-only
* `release/*` remains merge-only, with merges restricted to Release Maintainers
* `task/*` stays the standard namespace for normal work
* no other shared remote branch names are pushable
* `task/*` PRs merge by squash
* `release/*` reconciles back into `main` through a PR with a merge commit,
  to keep the parent link that makes each subsequent reconciliation small
* release candidates and releases are always tied to an exact tagged commit
* a version is always a protected tag
* no release is ever created from "whatever was at the branch tip"
