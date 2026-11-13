# AGENTS.md

Coursive is a local-first study workspace for recorded lectures: a Flutter app, a Go daemon that owns the library, credentials and downloads, and a Node.js assistant host. The app lives in `app/`, the daemon in `daemon/` and the assistant host in `agent/`, imported with their commit history ([docs/migration.md](docs/migration.md)); [docs/architecture.md](docs/architecture.md) maps them and [docs/specs/](docs/specs/) holds their contracts. Plugins, the plugin index and the mirror list live in [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins).

## Repository governance

This repository's governance layer is managed by repo-seed; product code remains repository-owned. [`.repo-seed/manifest.json`](.repo-seed/manifest.json) is authoritative for managed files, capability state, governance paths, external sources of truth, and the installed repo-seed version.

Route governance work explicitly:

- Change or pull-request review must use `repo-review`.
- A durable choice with meaningful alternatives must use `repo-decisions`.
- New complexity, delivery, ownership, security, release, or incident signals must use `repo-governance`.
- Seeding, adoption, governance-layer upgrades, or upstream repair must use the global `repo-seed` skill. If it is unavailable, stop and ask rather than hand-copying upstream governance.
- Ordinary implementation follows this file and the linked project documents without invoking repo-seed again.

If automatic skill activation is unavailable, read the linked resident `SKILL.md` directly and follow it.

## Repository layout

See [docs/architecture.md](docs/architecture.md) for the module map and seams. Directories named there are the canonical layout; anything else is user code.

## Code readability

Write code for human readers. Keep the primary logic path visible; introduce an abstraction only when it hides more complexity than its indirection adds; choose names that reveal domain meaning, effects, and cost; use one repository term per concept; and keep behavior within its owning boundary.

## Commands

- Gates: `node scripts/run-gates.mjs`
- Brand assets: `scripts/export-brand-icons.sh` regenerates every icon from `assets/brand/source/coursive-logo-4096.png`; commit the master and the exports together.
- Test: `(cd daemon && go test -count=1 -race ./...) && (cd agent && npm test) && (cd app && flutter test)`
- Lint: `(cd daemon && test -z "$(gofmt -l .)" && go vet ./...) && (cd agent && npm run check) && (cd app && flutter analyze --no-fatal-infos)`
- Interface text: edit `app/lib/l10n/app_zh.arb`, then run `flutter gen-l10n` in `app/`.
- Releases: `node --test scripts/release.test.mjs` tests the version and promotion rules; cutting and promoting releases follows [docs/release-policy.md](docs/release-policy.md).

Run relevant tests while working and the shared gate runner before every commit. CI or an installed pre-commit hook may repeat the exhaustive matrix.

## Governance loop (hard rules)

1. Run the gates before every commit; install the optional pre-commit capability only with user authorization.
2. Record a decision in [docs/decisions/](docs/decisions/) when a change chooses among meaningful alternatives whose rationale may be revisited; decisions are not a change log.
3. Risk-boundary changes start from an Approved spec in [docs/specs/](docs/specs/); routine changes are exempt.
4. A subtle, systemic, or costly escaped failure earns a postmortem in [docs/postmortems/](docs/postmortems/) linked to a permanent guardrail.
5. Implementation materially derived from an external source retains provenance at the closest stable repository location; follow [docs/development.md](docs/development.md#source-attribution).
6. The only upgrade channel for the seeded governance layer is re-running the repo-seed skill. Never hand-edit seeded files to "match upstream"; re-run the skill instead.

## Project rules

- Credentials stay inside the daemon: source-plugin secrets, sync and service tokens and model-provider API keys are sealed there and never logged, committed, or returned by an API (a token at most as its last 4 characters). Source plugins receive their own secrets only through the daemon vault. User data (the course library and the app's data directory) never enters the repository.
- The core app has no sign-in and no startup gate beyond the daemon: it opens on the local library. Every connection to an external course platform is a source plugin ([spec 0024](docs/specs/0024-source-provider-plugins.md)); core code, docs and assets never name a specific institution or platform.
- Plugin downloads are verified against the signed index whatever mirror served them; mirrors receive GET requests only and never credentials ([spec 0025](docs/specs/0025-plugin-marketplace.md)).
- Assistant actions are recorded with their undo and follow the user's approval mode; deleting downloaded files always waits for confirmation, and the assistant cannot change the approval mode. The assistant's write guards (note overwrite and transcript coverage in `agent/src/tools.mjs`, the memory guard in `agent/src/memory.mjs`, operations and approval checks in `agent/src/operate.mjs`) change only through an Approved spec.
- Do not copy code from GPL/AGPL projects; reimplement from protocols and documentation with provenance, and keep third-party asset `NOTICE` files.
- A daemon route or JSON field change updates both clients (the app and the assistant host) in the same change.

## Security rules

- Never `git commit` or `git push` unless the user explicitly asks.
- Never modify files outside the seeded paths (AGENTS.md, CLAUDE.md, docs/, scripts/, .agents/skills/, .github/, CONTRIBUTING.md, LICENSE, .editorconfig, .gitattributes, .repo-seed/) and the brand assets (`assets/brand/`) without asking.
- Never read `.env` files or other secrets.

## Local updates and backup retention

- Update the existing application in place, preserving its bundle/package identifier and Android signing key. Keep one installed copy per device unless the user explicitly requests side-by-side versions; never leave runnable `.app` copies in build or backup directories where macOS can register them.
- Back up only files an update will replace or migrate. Use one rolling rollback set outside the repository, with a file/hash manifest; ordinary code updates must not duplicate the course library, recordings, models, or full application. Before retiring a rollback set, verify the new installation and any newer backup needed to recover unique user edits.
- Retain at most the latest verified installer per platform. Replace superseded installers after successful installation; remove task-generated staging directories, extracted frameworks, build caches and temporary device packages after validation. Keep small source diffs, build manifests and test reports for reproducibility.
- Course files, handwritten strokes, conversations and device-specific credentials are user data, not disposable build output. Never overwrite them with another device's older snapshot or delete their only recoverable copy during cleanup.
- Before and after deployment, verify the installed version, launch, course inventory and affected user-data hashes. Report incomplete device checks explicitly; a successful build alone is not installation acceptance.

## Documentation

Follow [docs/AGENTS.md](docs/AGENTS.md): one home per fact, tutorials vs references, hygiene checklist.

## Decisions

Durable architecture and process decisions use MADR in `docs/decisions/`. Status flows Proposed → Accepted → Superseded by NNNN. A superseded record is never rewritten into its opposite.

## Governance evolution

Use the `repo-governance` skill when complexity, delivery, ownership, security, release, or incident signals change. Read-only audits and dry-runs are autonomous. Ask before enabling a capability, installing a hook, changing policy/source-of-truth, or connecting an external system. Raise blocking recommendations before implementation and advisory recommendations in the handoff; do not repeat a declined recommendation until its assessment hash changes.

## Testing

Follow [docs/testing.md](docs/testing.md). Name the regression risk, then choose the smallest test set at the lowest sufficiently real boundary; verify observable outcomes rather than implementation details.

## Skills

- [`.agents/skills/repo-review`](.agents/skills/repo-review/SKILL.md) — semantic review policy (instantiated per project) before merging.
- [`.agents/skills/repo-decisions`](.agents/skills/repo-decisions/SKILL.md) — how to write and update decision records.
- [`.agents/skills/repo-governance`](.agents/skills/repo-governance/SKILL.md) — progressive capability assessment, authorization, and upgrades.

## Enabled optional capabilities

- CI is an existing mechanism (`ci` capability: external). [.github/workflows/ci.yml](.github/workflows/ci.yml) runs the gates and the release-rule tests on every push and pull request to `dev` and `main` and checks that the brand exports are complete and generated from the current master; the daemon, assistant-host and app jobs run only for changes that touch them, and the app's Linux build only for changes to `app/linux/`, the pubspec or the workflow and on pushes. Releases follow [docs/release-policy.md](docs/release-policy.md): **Release / Prerelease** cuts a pre-release from `dev`, the `promotion` check admits only a published pre-release into `main`, and **Release / Publish** releases it as stable; each builds the Linux, Windows, macOS and Android packages through [.github/workflows/build.yml](.github/workflows/build.yml) into a draft and publishes it last.
- Path-level owners live in [CODEOWNERS](CODEOWNERS). The `dev` and `main` branch rulesets require an approving code-owner review (@WALKERKILLER) and green CI checks on every pull request; `dev` is the default branch, and `main` also requires the `promotion` check.
- Security vulnerabilities are reported per [SECURITY.md](SECURITY.md).
