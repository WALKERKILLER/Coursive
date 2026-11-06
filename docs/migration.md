# Migration

Coursive's code comes from a predecessor project and moves here as it is refactored. This document is the reference for what moves, in which order, how authorship is kept, and which gates each step passes. It changes as steps complete; the current step is marked.

## Principles

- **Main stays releasable.** Code reaches `main` only after the maintainer approves the step; until then it lives on an `import/*` branch in this repository.
- **Authorship is preserved.** Every imported commit keeps its original author, committer and date. Imports land on `main` as a merge commit, never squashed.
- **Nothing institution-specific enters the core.** Platform sign-in, course listing and media resolution leave the core and become source plugins before the code reaches `main`.
- **One import, then ordinary pull requests.** History is imported once into `import/app`; every later change is a normal pull request, so no commit appears twice.

## Steps

1. **Foundation** (done): governance, documentation, CI, brand assets, the plugin repository, and the two Draft specs for [source-provider plugins](specs/0024-source-provider-plugins.md) and the [plugin marketplace](specs/0025-plugin-marketplace.md).
2. **Approve the contracts** (done 2026-11-06): the maintainer approved both specs and resolved their questions; they took the next spec numbers after the imported specs, [0024](specs/0024-source-provider-plugins.md) and [0025](specs/0025-plugin-marketplace.md).
3. **Import history** (done 2026-11-06) into `import/app` with the procedure in [History](#history), which rewrites names in every commit; the decoupling pull requests complete the [de-identification checklist](#de-identification-checklist).
4. **Decouple on `import/app`** (done 2026-11-06 in #4): the local-library start, the source-provider protocol with the platform code moved to coursive-plugins, source-plugin installs from the index, and the renames. Agent-plugin install state in the daemon, the index signature, mirrors and the speed test continue as ordinary pull requests on `main`.
   - start on the local library: remove the splash and sign-in gates from the app root;
   - move platform sign-in, course listing and media resolution behind the source-provider protocol, and move the platform code into a source plugin outside this repository;
   - move plugin install state and consent into the daemon and add the marketplace, mirrors and speed test;
   - apply the [renames](#renames).
5. **Merge into `main`** (done 2026-11-06 in #5): `git merge --allow-unrelated-histories import/app` as one merge commit; the daemon, assistant-host, app and Linux-build jobs are required checks.
6. **First release** (current): the release workflow, restored from the imported history with `vX.Y.Z` tags, and the first `v0.x` pre-release; the Android signing key and the plugin-index signing key follow.

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
