# Git Client Settings

These are recommended local Git defaults for people working in repositories that follow this workflow.

They do not replace branch protection, PR review, CI, or protected tags. They
just make the common local Git commands line up with the workflow defaults.

## Copy-ready defaults

Run this once on each development machine:

```sh
git config --global pull.rebase false
git config --global pull.ff true
git config --global push.default simple
git config --global fetch.prune true
git config --global init.defaultBranch main
```

If your Git version supports it, also use:

```sh
git config --global push.autoSetupRemote true
```

## Why these settings

### `pull.rebase false`

`git pull` should merge instead of rebasing.

This matches the workflow rule that published branches are not rebased or
force-pushed. If a shared branch has diverged, the safe update path is a merge,
not a local history rewrite.

### `pull.ff true`

`git pull` should fast-forward when it can.

This keeps the common case clean: when your local branch has no extra commits,
pulling just moves it forward to the remote commit.

If your local branch and the remote branch have both moved, Git may still create
a merge commit because `pull.rebase false` selects merge behavior for divergent
history.

### `push.default simple`

`git push` should push the current branch to the upstream branch with the same
name.

This avoids accidentally pushing multiple local branches or pushing to an
unexpected remote branch name.

### `fetch.prune true`

`git fetch` should remove local remote-tracking references for branches that no
longer exist on the remote.

This keeps short-lived `task/*` branches from piling up locally after PRs are
merged and deleted.

### `init.defaultBranch main`

New local repositories should default to `main`.

This matches the required integration branch name in the workflow.

### `push.autoSetupRemote true`

First push of a new local branch should automatically set the matching upstream
branch.

This makes the normal task-branch flow simpler:

```sh
git switch -c task/865d6p9z8-improve-linux-server-detection
git push
```

Without this setting, the first push usually has to be:

```sh
git push -u origin task/865d6p9z8-improve-linux-server-detection
```

## Check your current values

```sh
git config --global --get pull.rebase
git config --global --get pull.ff
git config --global --get push.default
git config --global --get fetch.prune
git config --global --get init.defaultBranch
git config --global --get push.autoSetupRemote
```
