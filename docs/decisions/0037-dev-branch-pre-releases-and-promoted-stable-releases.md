# Pre-releases from dev, stable releases by promotion to main

## Status

Accepted
Class: process

## Context and Problem Statement

Development, pre-releases and releases all happened on `main`: a release or pre-release published on GitHub built its packages from whatever `main` held, and was public before any package was attached. A stable release would have been built from a commit nobody had installed as a pre-release, and a failed package left a published release with missing downloads. Which branches and steps should a change pass before users receive it as a stable release?

## Decision Drivers

- Users of stable releases get only what was published and installed as a pre-release first.
- A release is public only with all of its packages.
- Daily development is not slowed by the release steps.
- Android version codes keep increasing from one release to the next.
- The process stays small enough for one maintainer: the repository has no separate release team, store submission or deployment.

## Considered Options

- A. `dev` for development and pre-releases; `main` changes only by promoting a published pre-release, which publishes the stable release (the branch model of [YourTJ Hub](https://github.com/YourTongji/YourTJ-Hub), without its store and deployment controllers).
- B. Keep developing on `main` and release from tags on it, with a manual checklist before a stable release.
- C. Develop on `main` and cut a `release/X.Y` branch for each stable version.
- D. Build and publish a pre-release automatically on every merge into `dev`.

## Decision Outcome

Chosen option: A. `dev` is the default branch. **Release / Prerelease** runs on `dev`, **Release / Promotion** is a required check on pull requests into `main`, and **Release / Publish** runs on every push to `main`; all three build through one reusable workflow into a draft and publish it last. The rules (next version, promotion, build number) live in [scripts/release.mjs](../../scripts/release.mjs) with tests. [docs/release-policy.md](../release-policy.md) is the procedure.

- Good: a stable release's files are exactly a published pre-release's, checked by `git merge-tree` before the merge and on the merge commit after it.
- Good: `main` always equals the latest stable release, and `dev` keeps moving while a release is built.
- Good: drafts mean no release or tag is visible until every package is attached.
- Bad: stable packages are rebuilt from the same files, because the version is compiled in; they are not the pre-release's bytes.
- Bad: a merge into `dev` after a pre-release forces a new pre-release before promotion.

## Pros and Cons of the Options

### A. Pre-releases from dev, promotion to main

- Good: the promotion gate is mechanical, not a checklist.
- Bad: two branches and three workflows to keep in step.

### B. Everything on main

- Good: no branch to maintain.
- Bad: a stable release cannot be tied to a tested pre-release without discipline alone, and `main` never shows what users run.

### C. Release branches

- Good: several stable lines can be patched at once.
- Bad: cherry-picks between branches, and Coursive maintains one stable line.

### D. A pre-release for every dev merge

- Good: the newest code is always downloadable.
- Bad: every merge runs six package builds, and the release list fills with pre-releases nobody chose to test; beta users would be offered each one.

## Links

- [Release policy](../release-policy.md)
- YourTJ Hub continuous dev delivery: https://github.com/YourTongji/YourTJ-Hub/blob/4ecd1ded63bab3b52082c829ac4e0ff416383c02/docs/operations/release-cycles.md
- YourTJ Hub RC and confirmed releases: https://github.com/YourTongji/YourTJ-Hub/blob/4ecd1ded63bab3b52082c829ac4e0ff416383c02/docs/operations/releases.md
- Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
