# Migration

Coursive's code comes from a predecessor project and moves here as it is refactored. This document is the reference for what moves, in which order, how authorship is kept, and which gates each step passes. It changes as steps complete; the current step is marked.

## Principles

- **Main stays releasable.** Code reaches `main` only after the maintainer approves the step; until then it lives on an `import/*` branch in this repository.
- **Authorship is preserved.** Every imported commit keeps its original author, committer and date. Imports land on `main` as a merge commit, never squashed.
- **Nothing institution-specific enters the core.** Platform sign-in, course listing and media resolution leave the core and become source plugins before the code reaches `main`.
- **One import, then ordinary pull requests.** History is imported once into `import/app`; every later change is a normal pull request, so no commit appears twice.

## Steps

1. **Foundation** (done): governance, documentation, CI, brand assets, the plugin repository, and the two Draft specs for [source-provider plugins](specs/draft-source-provider-plugins.md) and the [plugin marketplace](specs/draft-plugin-marketplace.md).
2. **Approve the contracts** (done 2026-11-06): the maintainer approved both specs and resolved their questions; they take the next spec numbers when the history lands, after the imported specs.
3. **Import history** (current) into `import/app` with the procedure in [History](#history); the import passes the [de-identification checklist](#de-identification-checklist).
4. **Decouple on `import/app`**, one pull request each:
   - start on the local library: remove the splash and sign-in gates from the app root;
   - move platform sign-in, course listing and media resolution behind the source-provider protocol, and move the platform code into a source plugin outside this repository;
   - move plugin install state and consent into the daemon and add the marketplace, mirrors and speed test;
   - apply the [renames](#renames).
5. **Merge into `main`** after maintainer approval: `git merge --allow-unrelated-histories import/app` as one merge commit; add each module's CI job and required check.
6. **First release**: the release workflow, signing keys and the first `v0.x` pre-release.

## History

The import rewrites the predecessor's history with [git filter-repo](https://github.com/newren/git-filter-repo) in a scratch clone:

- `--invert-paths --paths-from-file` removes platform-specific packages, their specs, decisions, tests, fixtures and screenshots from every commit.
- `--replace-text` and `--replace-message` rewrite remaining names in file contents and commit messages.
- Authors, committers and dates stay untouched; commits that become empty are dropped.

The path and replacement lists name what they remove, so they stay outside this repository with the maintainer. Rewriting changes every commit hash; the maintainer keeps filter-repo's `commit-map` to translate old references. Co-author trailers and pull request numbers in messages refer to the predecessor project and are kept as written.

## De-identification checklist

An import or a pull request on `import/app` passes only when all of these hold, checked with a case-insensitive search over the tree and over `git log --format=%B`:

- No institution, platform or service-domain name in code, strings, localization files, docs, tests, fixtures, screenshots or commit messages.
- No sign-in, single-sign-on or institutional-network code in the core; the app builds and starts with no source plugin installed.
- No default institution, platform or account list shipped with the core.
- Application and package identifiers, release asset names and data directories use the Coursive names below.

## Renames

Proposed; each becomes final when its pull request merges.

| Old concept | Coursive name |
|---|---|
| Daemon directory and binary | `daemon/`, `coursive-daemon` |
| App and window title | Coursive |
| Plugin manifest key | `coursive` in `package.json` |
| Internal link scheme | `coursive://` |
| Deep-link scheme for exported Markdown | `coursive-app://` |
| Library folder | `~/Coursive` |
| App data directory | `coursive` under the platform data directory |
| Application ID | `io.github.walkerkiller.coursive` |
| Release tags | `vX.Y.Z`, `vX.Y.Z-beta.N` |
