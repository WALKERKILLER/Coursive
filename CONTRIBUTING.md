# Contributing

Thank you for considering contributing to Coursive. This repository is agent-native: human and AI contributors follow the same governance loop. Coursive is in development and migrating into this repository ([docs/migration.md](docs/migration.md)), so expect breaking changes.

## Branches

- `main` is the integration branch.
- Work on a short-lived branch from `main`, named by type: `feat/…`, `fix/…`, `docs/…`, `ci/…`, `refactor/…`, `chore/…`. Contributors without write access work on a fork.
- Every change reaches `main` through a pull request. The `main` ruleset blocks deletion and force-pushes, and requires the CI check `governance (gates)` plus an approving review from the code owner @WALKERKILLER ([CODEOWNERS](CODEOWNERS)). Each migrated module adds its CI job to the required checks; CI runs a part's job only when the change touches that part, Markdown-only changes run just the gates, and skipped jobs count as passed.
- Releases: publishing a GitHub release or pre-release tagged `vX.Y.Z` (or `vX.Y.Z-beta.N`) on `main` runs [release.yml](.github/workflows/release.yml), which builds and attaches the Linux, Windows, macOS and Android packages. Icons for every platform come from [scripts/export-brand-icons.sh](scripts/export-brand-icons.sh).

## Governance loop

1. Read [AGENTS.md](AGENTS.md) and [docs/AGENTS.md](docs/AGENTS.md) before making changes.
2. Write a decision record only when the change selects among meaningful alternatives; risk-boundary changes use an Approved spec in [docs/specs/](docs/specs/).
3. Run the gates before committing; an authorized pre-commit hook enforces the same runner:
   ```sh
   node scripts/run-gates.mjs
   ```
4. Name the regression risk and add or update the smallest test set that proves the affected contract; a fix without reproducible evidence is a rumor.
5. If the change closes a subtle or systemic escaped failure, add a postmortem in [docs/postmortems/](./docs/postmortems/) linked to the permanent guardrail.

## Local checks

Setup per platform is in [docs/development.md](docs/development.md). Before pushing, run the gates and the test and lint commands listed in [AGENTS.md](AGENTS.md#commands); CI runs the same set on Linux.

## Commits and pull requests

- Commit messages and pull request titles use `type(scope): summary`, where type is `feat`, `fix`, `docs`, `ci`, `refactor`, `test` or `chore`, and scope is `app`, `daemon`, `agent`, `plugins`, `brand`, or omitted for cross-cutting changes. Example: `fix(daemon): resume transcription chunks after restart`.
- Keep one topic per pull request and fill in the [template](.github/pull_request_template.md): what changed, why, affected parts, platforms tested, and verification.
- UI changes include a screenshot or short recording.
- Never commit credentials, local data (the course library or the app's data directory), build outputs, or code copied from GPL/AGPL projects; see the project rules in [AGENTS.md](AGENTS.md#project-rules).
- Core code, docs and assets never name a specific institution or course platform; that knowledge belongs in a source plugin.
- Pull requests are squash- or rebase-merged by the code owner after approval. Migration pull requests that import history are merge commits, so every original commit keeps its author ([docs/migration.md](docs/migration.md#history)).

## Plugins

Plugins are developed and submitted in [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins). Changes to the plugin contract itself (manifest, permissions, source-provider protocol, marketplace index) are risk-boundary changes in this repository and start from a spec.

## Decision records

Durable architecture and process decisions use MADR in `docs/decisions/`. Follow the `repo-decisions` skill and [.repo-seed/update-strategy.md](.repo-seed/update-strategy.md) for the format. A superseded record is never rewritten; a new record supersedes it.

## Reviews

Reviewers, human or AI, use the `repo-review` skill ([.agents/skills/repo-review/SKILL.md](.agents/skills/repo-review/SKILL.md)).

## Code of conduct

Be respectful and constructive. Assume good faith.
