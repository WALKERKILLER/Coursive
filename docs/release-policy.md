# Release policy

Coursive develops on `dev`, publishes pre-releases from `dev`, and publishes a stable release only by promoting a published pre-release to `main`. [Decision 0037](decisions/0037-dev-branch-pre-releases-and-promoted-stable-releases.md) records why. The workflows are [release-prerelease.yml](../.github/workflows/release-prerelease.yml), [release-promotion.yml](../.github/workflows/release-promotion.yml) and [release-publish.yml](../.github/workflows/release-publish.yml); all three build through [build.yml](../.github/workflows/build.yml) and share their version rules in [scripts/release.mjs](../scripts/release.mjs).

## Branches

- **`dev`** is the default and only development branch. Every change reaches it through a pull request from a short-lived branch, under the `dev` ruleset: an approving code-owner review and green CI.
- **`main`** holds the latest stable release. It changes only through a promotion pull request from `dev`, merged with a merge commit, under the `main` ruleset: the same review and CI plus the `promotion` check. Nothing is committed to `main` directly, and a fix for a stable release goes through `dev` and a new pre-release like any other change.

## Versions and tags

- Versions follow [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). A pre-release is `vX.Y.Z-<pre>.N`, usually `vX.Y.Z-beta.N`; a stable release is `vX.Y.Z`. A stable version is published once, after at least one of its pre-releases.
- A tag is created only when its release is published: the release stays a draft, without a tag, until every package is attached. The `release-tags` ruleset keeps `v*` tags from being moved or deleted.
- The build number of every package is the number of `v*` tags plus one, so Android version codes keep increasing across pre-releases and stable releases.
- The app's update check offers stable releases to everyone and pre-releases only to someone already on a pre-release; drafts are never offered.

## Cutting a pre-release

Run **Release / Prerelease** on `dev`, from Actions or with:

```sh
gh workflow run release-prerelease.yml --ref dev                       # continue the newest pre-release: beta.N → beta.N+1
gh workflow run release-prerelease.yml --ref dev -f version=0.2.0-beta.1   # start a new version
gh workflow run release-prerelease.yml --ref dev -f publish=false     # rehearsal: build into a draft only
```

The workflow picks the version, opens a draft pre-release on the current `dev` commit with generated notes, builds the Linux, Windows, macOS and Android packages from that commit and publishes the draft when all of them are attached. A version whose stable release exists is refused, as is a tag that exists. A rehearsal leaves a draft for inspection; delete it afterwards with `gh release delete <tag>`.

## Promoting to a stable release

1. Choose a published pre-release to promote; normally the newest, tested one. `dev` must still point at its commit: a merge into `dev` after it needs a new pre-release first.
2. Open a pull request from `dev` to `main` titled `release: vX.Y.Z`. The `promotion` check confirms that the head carries the published pre-release, that `vX.Y.Z` is unreleased and that the merge leaves `main` with exactly the pre-released files. It runs when the pull request opens or changes; re-run it after publishing a pre-release for an open pull request.
3. After approval, merge it with **Create a merge commit**. **Release / Publish** then opens a draft `vX.Y.Z` on the merge commit, with notes since the previous stable release, builds every package from it and publishes it as the latest release.

The stable packages are rebuilt from the merge commit, whose files equal the pre-release's, because the version is compiled into each package.

## Recovery

- A failed build leaves its release a draft and creates no tag. Re-run the failed jobs of the same run: packages are re-attached with `--clobber` and the draft is published when they all succeed.
- **Release / Publish** can be run on `main` from Actions to retry the newest promotion; it reuses that promotion's draft.
- A published release is never rebuilt, re-tagged or edited to different packages. A broken release is followed by a new version.
- Without the Android signing secrets the APKs carry a debug signature; see the Android job in [build.yml](../.github/workflows/build.yml).
