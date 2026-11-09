# v3 manifests install in the assistant host before the daemon store cutover

## Status

Proposed
Class: architecture

## Context and Problem Statement

[Spec 0031](../specs/0031-plugin-fabric-core.md) defines the v3 manifest, its schema and shared fixtures, and three validators read it. The only installer that ships, the assistant host's `PluginHost`, still read v1 `engine` manifests, so the copyable [template](../../agent/plugins/_template/README.md) passed the validation CLI and was refused by the installer. The same v1 reader dropped page `renderer`/`document` and every `slots` entry, so the app registries from [spec 0032](../specs/0032-ui-extension-protocol.md) had nothing to read and the capability inspector always reported no services or slots. The [audit](../plugin-fabric-audit.md) deferred installability to a daemon store cutover that has no date. Which reader decides whether a v3 package installs until then?

## Decision Drivers

- A package the validator accepts must install, or be refused for a reason the validator also states.
- Pages, slots and capabilities must be exercisable end to end before the store cutover.
- One owner of plugin state at a time: today that is `PluginHost` and `plugins.json`.
- The cutover must stay a move, not a rewrite: the same manifest and the same rules.

## Considered Options

- A. Keep v3 uninstallable until the daemon store ships.
- B. Teach the v1 reader the v3 fields it lacks.
- C. Dispatch on `api`: a v3 manifest runs the shared rules (`plugin-manifest.mjs`), then this host's own, and is normalized to the same public shape as v1.

## Decision Outcome

Chosen option: C. `validateManifest` sends a manifest with `api` to the v3 reader. It accepts API 3.0.0 and exactly one required `pi` runtime (other kinds only when optional), refuses `contributes.sources`, accepts declarative pages only, checks slot targets and modes against the host catalog, and requires every document button to name a declared action. The result carries `schema`, `requires` and `provides`; v1 manifests keep their reader unchanged. When the daemon store lands it takes over install and these host rules move with it.

### Consequences

- Good: the template and experimental plugins install from `path:`, and their pages, slots and services reach the app and the inspector.
- Good: v1 packages are untouched.
- Bad: the host has rules beyond the shared schema (runtime, renderer, slot catalog) that the daemon store must reproduce.
- Bad: the slot catalog is duplicated between the agent host and the app until the daemon publishes it.

## Pros and Cons of the Options

### A. Keep v3 uninstallable
- Good: no interim code.
- Bad: the template is a dead end, and no page, slot or service contribution can be tested on a real install.

### B. Extend the v1 reader
- Good: one code path.
- Bad: mixes `engine` and `api` semantics in one function and still skips the shared rules, so the validator and installer can disagree.

### C. Dispatch on `api`
- Good: validator and installer share the rule set; v1 is unchanged; the cutover moves code instead of redefining it.
- Bad: two readers until v1 is migrated.

## Links

- Parent decision: [0033](0033-plugin-fabric-and-ui-extension-protocol.md)
- Core contract: [spec 0031](../specs/0031-plugin-fabric-core.md)
- UI contract: [spec 0032](../specs/0032-ui-extension-protocol.md)
- Audit and field map: [plugin-fabric-audit.md](../plugin-fabric-audit.md)
- Tracking epic: https://github.com/WALKERKILLER/Coursive/issues/38
