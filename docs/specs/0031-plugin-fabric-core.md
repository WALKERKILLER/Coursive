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

Pending until implementation. Link the PF-01 audit, schema fixtures, service-resolver tests, agent permission tests and MoeJS benchmark report here when they exist.
