# Schemas

Versioned, self-contained JSON Schemas for cross-process contracts.

| File | Contract |
|---|---|
| [plugin-manifest.v3.schema.json](plugin-manifest.v3.schema.json) | A plugin package's `coursive` manifest: identity, API range, runtime components, services, permissions and contributions |

## How a schema here is used

The schema is the authority for the fields it names; the code that must reject a bad document implements the same rules and is tested against it. For the plugin manifest, the daemon parser is [manifest.go](../../daemon/internal/pluginfabric/manifest.go), the fixtures all three languages validate are [cases.json](../../daemon/internal/pluginfabric/testdata/cases.json), the assistant host's rules are [plugin-manifest.mjs](../../agent/src/plugin-manifest.mjs) with its fixture test [plugin-schema.test.mjs](../../agent/src/plugin-schema.test.mjs), and plugin authors run the same rules through [validate-plugin.mjs](../../agent/scripts/validate-plugin.mjs). Each validator has a test that compares its closed sets (permissions, runtime kinds, categories, contribution groups) with this file, so a rule cannot land in one home only.

A new contract version adds a new `*.v<N>.schema.json` file and leaves the previous one in place: readers name the version they accept, and a manifest declares the version it was written for. The manifest's `api` range is the plugin-side counterpart of that: the host rejects a range it does not satisfy, and the plugin is expected to keep working across the range it declares.

Contract prose lives in the specs, not here: [plugin fabric core](../specs/0031-plugin-fabric-core.md), [UI extension protocol](../specs/0032-ui-extension-protocol.md), [plugin platform](../specs/0004-plugin-platform.md), [source-provider plugins](../specs/0024-source-provider-plugins.md).
