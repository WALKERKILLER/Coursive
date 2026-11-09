# Plugin Fabric extends Pi with reversible UI and capability contracts

## Status

Proposed
Class: architecture

## Context and Problem Statement

Coursive's v1 plugin platform loads plugins as Pi extensions and renders their results through a versioned native view catalog ([spec 0004](../specs/0004-plugin-platform.md), [decision 0005](0005-plugins-as-pi-extensions-with-declarative-views.md)). That decision deliberately rejected a second plugin runtime, plugin-supplied HTML and arbitrary plugin UI code. It gave the product a small, native and testable surface, but it cannot add a complete plugin page, decorate an existing Flutter surface, resolve service providers, or let an agent discover plugin capabilities without an app release.

The accepted Plugin Fabric design ([RFC](../plugin-fabric.md)) extends the v1 baseline rather than replacing it: dynamic navigation and pages, typed UI slots, a declarative renderer, a service registry, an agent capability inspector, an optional MoeJS executor and an optional isolated Web UI. The repository needs one durable record of that extension and of the parts of decision 0005 it revisits.

## Decision Drivers

- Preserve the shipped Pi extension path, source-provider protocol and native Material 3 rendering.
- Let a plugin add or remove a page and UI contributions without forking `shell.dart` or shipping Dart.
- Keep install, permission, lifecycle and user-data ownership in the daemon.
- Give the agent a bounded, permission-aware view of available, missing and degraded capabilities.
- Keep the core library usable when a plugin, executor or WebView is broken.
- Deliver incrementally: native declarative UI first, optional runtimes and Web UI only after measured security and platform gates.

## Considered Options

- A. Keep the Pi-only platform and grow only the existing view catalog.
- B. Replace Pi with a Cordis-style runtime and a React/WebView UI.
- C. Add a Plugin Fabric and UI Extension Protocol around Pi, with optional MoeJS and isolated Web UI behind separate specs.
- D. Make WebView the primary plugin UI and keep the native app as a shell.

## Decision Outcome

Chosen option: C. The daemon becomes the single owner of plugin install, enablement, permissions, capability registration and dependency resolution. Flutter owns navigation, slots, declarative rendering, canvas objects and the action surface. The Node Pi host remains the v1 agent extension path; MoeJS is an optional bounded executor; Web UI is an opt-in isolated renderer with a platform admission process. Existing Pi plugins and source providers keep working.

This record extends [decision 0005](0005-plugins-as-pi-extensions-with-declarative-views.md). It is `Proposed` until the first implementation phase ships; at that point it becomes `Accepted` and the parts of 0005 that reject a second runtime or plugin-supplied UI are marked as superseded by this record. The normative contracts are [spec 0031](../specs/0031-plugin-fabric-core.md) and [spec 0032](../specs/0032-ui-extension-protocol.md).

### Consequences

- Good: plugin pages and UI slots become possible without a shell fork or an app release for every view.
- Good: the agent can discover capabilities and dependencies while the daemon keeps the final permission decision.
- Good: native Flutter remains the default, so the local library and existing plugin views keep working when optional runtimes fail.
- Bad: two runtime families (Node Pi and optional MoeJS) and a larger protocol surface must be maintained.
- Bad: WebView adds platform-specific packaging, input and security risk; it stays off by default.
- Bad: the schema, lifecycle and permission checks add work for plugin authors.
- Mitigation: versioned schemas and fixtures, atomic activation, mandatory dispose handles, security review, and no default MoeJS or Web UI.

## Pros and Cons of the Options

### A. Pi-only with a larger view catalog
- Good: smallest change and no new protocol.
- Bad: every new page or surface still needs an app release; services and agent capability discovery remain hard-coded; the plugin cannot compose capabilities.

### B. Cordis-style runtime and React/WebView UI
- Good: a uniform service/effect model and arbitrary UI.
- Bad: duplicates Pi's registry and lifecycle, discards the native Flutter investment, and makes security and accessibility the plugin's problem. This is the option decision 0005 already rejected.

### C. Plugin Fabric around Pi
- Good: incremental and compatible with the shipped v1 path; native UI stays the default; optional runtimes are isolated behind contracts.
- Bad: more contracts and a longer migration; requires strict schema and permission discipline.

### D. WebView-first
- Good: the widest plugin UI freedom with one renderer.
- Bad: no maintained WebView on every desktop target; token, origin, CSP and bridge risk become the default; native fidelity and offline startup regress.

## Links

- Accepted design reference: [plugin-fabric.md](../plugin-fabric.md)
- Core contract: [spec 0031](../specs/0031-plugin-fabric-core.md)
- UI contract: [spec 0032](../specs/0032-ui-extension-protocol.md)
- Existing plugin platform: [spec 0004](../specs/0004-plugin-platform.md)
- Existing source-provider and marketplace boundaries: [spec 0024](../specs/0024-source-provider-plugins.md), [spec 0025](../specs/0025-plugin-marketplace.md)
- Prior decision: [0005](0005-plugins-as-pi-extensions-with-declarative-views.md)
- Tracking epic: https://github.com/WALKERKILLER/Coursive/issues/38
- Cordis services, inject, events and effects: https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/cordis-primer.md
- DSH typed UI slots: https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/slots.md
- MoeJS pure-Go JavaScript runtime: https://github.com/MoeForge/moejs
- Flutter AOT and platform constraints: https://docs.flutter.dev/resources/faq
- Flutter performance profiling: https://docs.flutter.dev/perf/ui-performance
