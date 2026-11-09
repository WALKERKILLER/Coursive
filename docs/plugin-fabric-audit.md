# Plugin fabric contract audit

Reference for the migration from the shipped plugin contract to the Plugin Fabric. It records who owns each piece of plugin state today, what the canonical manifest adds, and which defects the migration must close before a second registry exists. Contracts stay in their own homes: [plugin fabric core](specs/0031-plugin-fabric-core.md), [UI extension protocol](specs/0032-ui-extension-protocol.md), [plugin platform](specs/0004-plugin-platform.md), [source-provider plugins](specs/0024-source-provider-plugins.md), [plugin marketplace](specs/0025-plugin-marketplace.md), [decision 0033](decisions/0033-plugin-fabric-and-ui-extension-protocol.md). The manifest shape is [plugin-manifest.v3.schema.json](schemas/plugin-manifest.v3.schema.json).

## Contract matrix

"v1" is the contract the agent host implements, "v2" is the store spec 0025 describes, "v3" is the canonical manifest this audit introduces. Each cell names the owner, with the live code beside it.

| Concern | v1 (spec 0004) | v2 (spec 0025) | v3 (spec 0031/0032) | Code |
|---|---|---|---|---|
| Manifest location | `package.json` `coursive` key | same key, plus `contributes.sources` | same key, `api` replaces `engine`; adds `runtime`, `requires`, `provides` | [plugins.mjs:215](../agent/src/plugins.mjs), [registry.go:34](../daemon/internal/sources/registry.go) |
| Identity | `coursive.id`, `[a-z][a-z0-9-]{1,31}`, reserved route names | same | same | [plugins.mjs:218](../agent/src/plugins.mjs), [plugins.mjs:34](../agent/src/plugins.mjs), [registry.go:72](../daemon/internal/sources/registry.go) |
| API version | `coursive.engine` against `PLUGIN_API_VERSION` `1.0.0` | `engine` against `2.0.0` in the index | `coursive.api` range, host evaluates | [plugins.mjs:14](../agent/src/plugins.mjs), [plugins.mjs:136](../agent/src/plugins.mjs), [market.go:34](../daemon/internal/sources/market.go) |
| Install and enable state | agent host `plugins.json` 0600 | daemon `plugins.json` 0600, one owner | daemon, read by app and agent through typed routes | [plugins.mjs:329](../agent/src/plugins.mjs), [plugins.mjs:360](../agent/src/plugins.mjs), [registry.go:130](../daemon/internal/sources/registry.go) |
| Granted permissions | `granted` list written at confirm | daemon records consent; a new permission needs consent again | same, plus a permission increase is a new consent step | [plugins.mjs:549](../agent/src/plugins.mjs), [plugins.mjs:524](../agent/src/plugins.mjs) |
| Permission names | eight names in one table | adds `source` | the nine names the schema closes | [plugins.mjs:16](../agent/src/plugins.mjs), [plugins.dart:261](../app/lib/core/plugins.dart) |
| Contributions | `contributes.{noteViews, actions, exporters, pages, blocks}` | adds `contributes.sources` | `contributes.{navigation, pages, slots, tools, views, actions, settings, sources}` | [plugins.mjs:240](../agent/src/plugins.mjs), [registry.go:45](../daemon/internal/sources/registry.go) |
| Runtime | the `agent` module as a Pi extension per generation; provider processes in the daemon | same split, no declaration | `runtime[]` entries: `pi`, `moejs`, `process`, `web-ui`, `go` | [plugins.mjs:632](../agent/src/plugins.mjs), [plugins.mjs:666](../agent/src/plugins.mjs), [registry.go:181](../daemon/internal/sources/registry.go) |
| Marketplace | agent catalogs and the bundled official catalog | signed index, mirrors, daemon-owned installs | daemon store for every contribution kind | [plugins.mjs:464](../agent/src/plugins.mjs), [market.go:38](../daemon/internal/sources/market.go), [market.go:226](../daemon/internal/sources/market.go) |
| Source-provider boundary | n/a | `contributes.sources` + `source` permission, JSON-RPC over stdio | unchanged contract, declared under the same key | [protocol.go:13](../daemon/internal/sources/protocol.go), [registry.go:90](../daemon/internal/sources/registry.go) |
| App surface | reads `/agent/plugins`, renders declared views | reads daemon routes | reads daemon routes only | [plugins.dart:94](../app/lib/core/plugins.dart), [plugins.dart:108](../app/lib/core/plugins.dart), [plugin_views.dart:21](../app/lib/features/plugins/plugin_views.dart) |
| Path safety | `relPath` for the agent module and skills; `filepath.Clean` for provider executables; tar entries refused on escape | same, before unpack | one rule, implemented by the schema and both validators | [plugins.mjs:157](../agent/src/plugins.mjs), [registry.go:96](../daemon/internal/sources/registry.go), [market.go:534](../daemon/internal/sources/market.go) |

## Gaps

Each gap is a defect of the current contract, not a missing feature.

| # | Gap | Evidence | Effect |
|---|---|---|---|
| G1 | Two install and enable owners, and they disagree on the API version: the agent host owns agent plugins at `1.0.0`, the daemon owns source plugins at `2.0.0` | [plugins.mjs:14](../agent/src/plugins.mjs), [plugins.mjs:329](../agent/src/plugins.mjs), [market.go:34](../daemon/internal/sources/market.go), [registry.go:130](../daemon/internal/sources/registry.go) | A plugin's installed state, version and consent exist in two files; nothing can answer "is this plugin installed" once |
| G2 | No canonical manifest: three readers accept three field sets, and the app validates nothing | [plugins.mjs:215](../agent/src/plugins.mjs) requires `id`/`version`/`engine`; [registry.go:61](../daemon/internal/sources/registry.go) requires `id`/`title`/`contributes.sources`/`source`; [plugins.dart:94](../app/lib/core/plugins.dart) trusts the agent | A package can be installable by one owner and invalid to the other; the schema in [plugin-manifest.v3.schema.json](schemas/plugin-manifest.v3.schema.json) replaces the three lists with one |
| G3 | The daemon market only lists index entries whose `kinds` contain `sources` | [market.go:353](../daemon/internal/sources/market.go), [market.go:377](../daemon/internal/sources/market.go) | Without the Node runtime, no agent, view or action plugin can be installed, which is the outcome spec 0025 exists for |
| G4 | The daemon ignores `engine` when it reads a source plugin | [registry.go:43](../daemon/internal/sources/registry.go) decodes it, [registry.go:61](../daemon/internal/sources/registry.go) never compares it | An incompatible provider installs and fails at first call instead of being shown as incompatible |
| G5 | Duplicate contribution ids are accepted | [plugins.mjs:240](../agent/src/plugins.mjs) checks each entry's shape but not uniqueness | Two entries share a key; the app's `entries(kind)` renders both and the winner depends on map order |
| G6 | `contributes.blocks` is silently filtered to declared views | [plugins.mjs:255](../agent/src/plugins.mjs) | A typo drops a contribution with no diagnostic, against the spec 0031 rule that a partial registration is never published |
| G7 | Nothing declares service dependencies or provisions | no `requires`/`provides` anywhere in [plugins.mjs](../agent/src/plugins.mjs) or [registry.go](../daemon/internal/sources/registry.go) | The resolver of spec 0031 has no input; a plugin that needs `transcript.read` cannot say so |
| G8 | Permission names have three sources: the agent table, the daemon's literal `source` check, and the app's icon map | [plugins.mjs:16](../agent/src/plugins.mjs), [registry.go:84](../daemon/internal/sources/registry.go), [plugins.dart:261](../app/lib/core/plugins.dart) | `source` has no app icon or label; a name added to one home is unknown to the other two |
| G9 | Contribution groups and runtime are implied by which manifest fields are set | [plugins.mjs:240](../agent/src/plugins.mjs), [registry.go:45](../daemon/internal/sources/registry.go) | A plugin cannot declare two runtimes or a runtime that is optional on this platform; the host infers the engine from the field name |

## Compatibility

A v1 package stays loadable by the v1 reader while the canonical reader is additive. The v3 reader accepts exactly one shape and names the replacement for a v1 field it does not know, so an unmigrated package fails with an instruction rather than an unknown-field message.

| v1 field | v3 home | Note |
|---|---|---|
| `engine` | `coursive.api` | Range syntax is unchanged; the host that evaluates it changes with the store |
| `agent` | `runtime[{kind: "pi", entry}]` | One runtime component instead of an implied engine |
| `skills` | `runtime[{kind: "pi", skills}]` | Pi-only; a non-`pi` runtime cannot declare skills |
| `views` | `contributes.views` | v1 strings become entries with an `id` |
| `contributes.noteViews` | `contributes.actions` with the note surface, plus `contributes.views` | The surface becomes a 0032 slot target when PF-04 lands |
| `contributes.actions` + `surface` | `contributes.actions` | `surface` keeps its v1 values until the slot catalog replaces them |
| `contributes.exporters` + `scope` | `contributes.actions` with `scope` | Same migration as the surfaces |
| `contributes.pages` | `contributes.pages` | Unchanged identity; renderer and fallback come from 0032 |
| `contributes.blocks` | `contributes.views` | A block is a view a note may embed |
| `contributes.sources` | `contributes.sources` | Unchanged; `executable` is the platform map spec 0024 defines |
| `settings`, `exec`, `hosts`, `categories`, `examples`, `icon` | unchanged | The settings form stays the JSON Schema subset of spec 0004 |

Rollback:

- The v3 reader is a new package and nothing in the shipped path calls it, so reverting this change restores the previous behaviour exactly. [manifest.go](../daemon/internal/pluginfabric/manifest.go) imports only the standard library and holds no state.
- The store migration keeps one state file per owner until cutover. The daemon store writes its own file with a `schema` marker of its own, the agent host keeps `coursive/plugins@1` ([plugins.mjs:360](../agent/src/plugins.mjs)), and neither writes the other's file. Cutover removes the agent's write path in the same change that teaches the daemon to serve the routes, so a rollback drops the store and re-reads the agent's file.
- A rejected manifest never reaches an install: the reader runs before the preview, so a rollback cannot leave a half-installed plugin.
- Existing library entries, provider vault namespaces and workspaces are untouched by any of this: they key off `id`, which v3 does not change ([registry.go:181](../daemon/internal/sources/registry.go)).

## Regression and test plan

Risk shapes follow [testing.md](testing.md): pure rules in unit tests, the store and routes in integration tests, the shipped entry points last.

| Risk | Primary evidence |
|---|---|
| A manifest the daemon accepts and the agent rejects (or the reverse) | Shared fixtures [cases.json](../daemon/internal/pluginfabric/testdata/cases.json), validated by [manifest_test.go](../daemon/internal/pluginfabric/manifest_test.go), [plugin-manifest.mjs](../agent/src/plugin-manifest.mjs) with [plugin-schema.test.mjs](../agent/src/plugin-schema.test.mjs) and [plugin_fabric_fixtures_test.dart](../app/test/plugin_fabric_fixtures_test.dart); plugin authors run the same rules through [validate-plugin.mjs](../agent/scripts/validate-plugin.mjs), covered by [validate-plugin.test.mjs](../agent/scripts/validate-plugin.test.mjs) |
| A rule lands in the validator but not in the schema | `TestSchemaAgreesWithValidator` and the schema-agreement test in the agent compare the closed sets |
| An unmigrated v1 package is silently accepted or rejected without an instruction | `invalid-v1-manifest.json` expects `api-missing`; the `field-unknown` rule names the v3 home |
| A path escapes the package | `runtime-path` and `source-path` cases, plus the existing escape tests in [sources_test.go](../daemon/internal/sources/sources_test.go) and [market_test.go](../daemon/internal/sources/market_test.go) |
| The shipped plugin path regresses while the fabric is added | [plugins.test.mjs](../agent/src/plugins.test.mjs) unchanged: install, consent, tool prefixing, deferred exposure, effects on disable |

Scenario coverage PF-01 and PF-02 cannot prove yet, owned by the issue that lands each mechanism: one authoritative store (PF-03), two `transcript.read` providers switching deterministically and a dependency cycle naming its path (PF-07), disable revoking every contribution (PF-04), an MoeJS loop cancelled (PF-09).

## Remaining work

- PF-01: this audit and the field map. Still open: the ADR revision that records the store cutover, and the decision that the v3 reader, not the v1 readers, decides installability.
- PF-02: the schema, the shared fixtures and the three validators. Still open: a validator call in the install pipeline and the migration writer that rewrites a v1 package to v3.
- PF-12: the validation CLI, the copyable template and the compatibility note ([agent/plugins/_template](../agent/plugins/_template/README.md)). Still open: the store cutover that makes a v3 manifest installable, and the matching `coursive-plugins` templates, which still write v2 `engine`.
