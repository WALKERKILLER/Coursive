# Plugin Fabric core: capabilities, lifecycle, and executors

Artifact-Version: 1
Status: Approved
Approved-By: maintainer, explicit request on 2026-11-09 to adopt the Plugin Fabric RFC

## Intent

Coursive needs plugin-provided pages, UI extensions, services and agent tools without rewriting the existing Flutter, Go daemon and Node Pi stack. This spec owns the runtime-independent core: plugin identity and state, capabilities and dependencies, lifecycle and effects, permissions, events, and the executor boundary. The UI contract is [0032](0032-ui-extension-protocol.md); the design rationale and target architecture are in [plugin-fabric.md](../plugin-fabric.md); the durable choice is [decision 0033](../decisions/0033-plugin-fabric-and-ui-extension-protocol.md).

Non-goals: injecting arbitrary Dart or Flutter source into a release app; letting a plugin access the main database, credential vault, arbitrary files or arbitrary HTTP routes; automatic installs, silent permission escalation or unapproved system commands; replacing the Pi agent host or the existing source-provider protocol; enabling MoeJS by default.

## Contract

### Plugin identity and state

- One logical plugin has an `id`, `version`, publisher, compatible host/API range and declared permissions. It may contribute one or more runtime components; a plugin is not forced into a single engine.
- The registry state is one of `installed`, `enabled`, `resolving`, `ready`, `degraded`, `failed` or `disabled`. Installation and activation are separate transactions, so an installed plugin can be incompatible or degraded without breaking the app.
- The daemon is the single owner of install, enable and granted-permission state. The app and the agent host read that state through typed APIs; neither keeps a second writable registry.
- Version negotiation rejects unknown or out-of-range versions with a readable diagnostic. A permission increase requires a new consent step. A failed upgrade rolls back to the previous verified install. Persistent learning objects, notes and user data do not follow a temporary plugin effect or an uninstall.

### Manifest and contributions

- The manifest declares `id`, `version`, `api`, `runtime` entries, `requires`, `provides`, `permissions` and `contributes`. Contribution groups include navigation, pages, slots, tools, views, actions and settings.
- A versioned JSON Schema and shared Go/Node/Flutter fixtures define the accepted shape. Unknown schema versions, unknown runtime kinds, duplicate contribution keys, undeclared permissions and paths that escape the plugin package are rejected.
- A plugin activation is atomic: every declaration passes schema, version, permission and resource checks before Flutter or the agent sees any of it. A partial registration is never published.

### Capabilities and service dependencies

- A service is identified as `domain.verb@version`, for example `transcript.read@1`, `canvas.objects.create@1` or `llm.generate@1`. Its descriptor publishes `id`, `version`, `inputSchema`, `outputSchema`, `owner`, `scope`, `requiredPermissions`, `costHint` and `availability`.
- Resolution order is: version match, permission and device capability, user preference, availability, priority, then a deterministic tie-break. Map iteration order never decides a provider.
- A missing required dependency marks the contribution unavailable and names the install, enable or authorization step; a missing optional dependency selects a documented degraded mode. Dependency cycles are rejected with the cycle path.
- A switch that changes the data boundary, such as local recognition to a cloud provider, asks for consent again. Plugins do not hardcode an institution, platform or provider brand.

### Lifecycle and effects

- Every registration returns a `dispose()` handle. Tools, menus, navigation entries, slots, event subscriptions, timers, tasks and resource pools implement it.
- Disabling a plugin stops new calls, waits for or cancels in-flight work, revokes its UI contributions and then releases the runtime. A failure falls back to the last safe state.
- Persistent user objects survive uninstall and are readable through a fallback; plugin-private cache may be cleared.

### Permissions, security and executors

- Permissions are least-privilege. Capability handles are issued for a permission and context; a runtime never receives the daemon token, the main vault, a database handle or unrestricted filesystem access.
- Every write reuses the existing domain write guard, approval mode, undo record and audit trail. A plugin cannot change the approval mode or hide an action from it.
- Executors are explicit: Go built-in services, an optional bounded MoeJS runtime, process/source-provider JSON-RPC, and the Node Pi host. The Node Pi path remains the v1 baseline; existing Pi plugins are not automatically converted to MoeJS.
- MoeJS is an optional experimental executor. It has a compile-once/runtime-pool design, memory and deadline limits, a module allowlist, a host-API allowlist, cancellation and output bounds. It does not hold a daemon token, the vault or global database objects.
- The agent can inspect capabilities but cannot silently install a plugin, widen its permissions, change the approval mode or execute an untrusted page script. Tool results carry a trace id, source references, timing and a readable error.

### Events

- The event bus supports `emit`, `parallel`, `serial`, `waterfall` and `bail`. Every event carries a schema, `traceId`, timeout and scope; an observer receives only data its permission and context allow.
- A `waterfall` or `bail` hook keeps the original result for audit and defines its conflict policy. A replacement never silently discards the prior value.

## Plan

- PF-01 completes the v1/v2 contract audit and the migration ledger before implementation. PF-02 lands the canonical schemas and fixtures. PF-07 adds the service registry and resolver. PF-08 exposes the capability inspector to the agent. PF-09 prototypes MoeJS as an optional executor. PF-12 adds the SDK, validation CLI, templates and compatibility checks.
- Keep the existing Pi plugin path and source-provider protocol working throughout. A daemon route or JSON-field change updates the Flutter app and the agent host in the same change.
- Implement in dependency order from [Epic #38](https://github.com/WALKERKILLER/Coursive/issues/38); do not stack a new registry on the old PluginHost state.

## Verification

- Commands: `cd daemon && go test -count=1 -race ./... && test -z "$(gofmt -l .)" && go vet ./...`; `cd agent && npm test && npm run check`; `cd app && flutter test && flutter analyze --no-fatal-infos`; `node scripts/run-gates.mjs`.
- Contract scenarios: one authoritative install/enable state; a v1 manifest validates or gets a migration error; unknown version, duplicate key and undeclared permission are rejected; two fake `transcript.read` providers switch deterministically; a missing dependency and a cycle produce actionable diagnostics; disable revokes every contribution; an infinite MoeJS loop is cancelled; the agent cannot exceed its permissions; existing Pi, Socratic and source-provider paths pass.
- MoeJS and Web UI remain opt-in until their own performance and security gates pass. No default runtime switch may regress app startup or the local library path.

## Evidence

- Contract audit, gap list and migration map: [plugin-fabric-audit.md](../plugin-fabric-audit.md).
- Canonical manifest shape and the fixtures all three languages validate: [plugin-manifest.v3.schema.json](../schemas/plugin-manifest.v3.schema.json), [manifest_test.go](../../daemon/internal/pluginfabric/manifest_test.go), [plugin-manifest.mjs](../../agent/src/plugin-manifest.mjs) with [plugin-schema.test.mjs](../../agent/src/plugin-schema.test.mjs), [plugin_fabric_fixtures_test.dart](../../app/test/plugin_fabric_fixtures_test.dart).
- PF-07 service catalog and resolver: [service.go](../../daemon/internal/pluginfabric/service.go) and [resolve.go](../../daemon/internal/pluginfabric/resolve.go) with [resolve_test.go](../../daemon/internal/pluginfabric/resolve_test.go) — the same provider wins in every registration order, version, permission and device mismatches produce recovery hints, two `transcript.read` providers are selected by preference, a local-to-cloud switch asks for consent again, a missing required dependency is unavailable while a missing optional one selects the documented degraded mode, and a cycle is rejected with its path. [adapters.go](../../daemon/internal/pluginfabric/adapters.go) maps the daemon's own ASR, transcript-search and canvas-preview capabilities to descriptors without changing their code paths; [adapters_test.go](../../daemon/internal/pluginfabric/adapters_test.go) covers the on-device selection and the cloud fallback with consent.
- PF-08 capability inspector: [capabilities.mjs](../../agent/src/capabilities.mjs) with [capabilities.test.mjs](../../agent/src/capabilities.test.mjs) — a short summary first with service schemas only in `inspect_service`, enabled, disabled, degraded, failed and consent states, permission denial, cost, data boundary and readable reasons; plugin text is reduced to one capped line, copied field by field from a whitelist and redacted, so it cannot pose as an instruction. The inspector is the deferred `capabilities` group of bot and course conversations and reads the plugin host's state.
- PF-09 optional executor prototype: [executor.go](../../daemon/internal/pluginfabric/executor.go) with [executor_test.go](../../daemon/internal/pluginfabric/executor_test.go) — runtime factory seam, per-plugin runtime pool, concurrency bound, deadline, cancellation, output bound, host-API/module allowlists and p50/p95 metrics. The tests cover normal invoke, timeout, pre-cancelled calls, output limit, denied host API/module, dispose, pool limit and concurrency. A real MoeJS engine is injected through `RuntimeFactory`; the adapter stays optional and is not wired into the default Pi path.
- PF-12 validation CLI and template: [plugin-manifest.mjs](../../agent/src/plugin-manifest.mjs) holds the assistant host's rules and the recovery hint of each code; [validate-plugin.mjs](../../agent/scripts/validate-plugin.mjs) validates a package directory or a `package.json` and prints the JSON path, code and hint of every problem, with `--json` for CI, `--files` for declared entry, skill and source files that must exist and resolve inside the package, and `--host x.y.z` for the API range check. It exits 1 on problems and 2 on unreadable input. [validate-plugin.test.mjs](../../agent/scripts/validate-plugin.test.mjs) covers the template, a shared invalid fixture, a path escape, a symlink escape, a duplicate contribution id, a v1 field map, an unsatisfied API range, the JSON report and every exit code. The copyable package is [agent/plugins/_template](../../agent/plugins/_template/README.md): a v3 manifest, a declarative page and slot document, one action and the API Version 3.0.0 / minimum Host Version note with the coursive-plugins counterpart.
- v3 install in the assistant host ([decision 0034](../decisions/0034-v3-manifests-install-in-the-assistant-host.md)): [plugins.mjs](../../agent/src/plugins.mjs) reads a manifest with `api` through the shared rules and then the host's own (API 3.0.0, one `pi` runtime, declarative pages, the slot target catalog, declared button actions, `write` only with `library.write`), and publishes its pages, slots, navigation, `requires` and `provides`; [plugins.test.mjs](../../agent/src/plugins.test.mjs) installs the template from `path:` and runs its action, and rejects each host-specific case. The capability inspector lists declared services and slot entries ([capabilities.test.mjs](../../agent/src/capabilities.test.mjs)); provider selection stays with the daemon resolver.
- Pending for these phases: a daemon route that publishes the resolved catalog to the agent host (with the matching app client, in the same change), the daemon store cutover, a real MoeJS engine and its benchmark report, the v1 -> v3 migration writer, and the remaining PF-10/PF-11/PF-12 work.
