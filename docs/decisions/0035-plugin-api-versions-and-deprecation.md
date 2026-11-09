# Plugin API versions, minimum host version and deprecation

## Status

Accepted
Class: process

## Context and Problem Statement

Since [decision 0034](0034-v3-manifests-install-in-the-assistant-host.md) the assistant host installs two manifest generations: v1 (`engine`, [spec 0004](../specs/0004-plugin-platform.md)) and v3 (`api`, [spec 0031](../specs/0031-plugin-fabric-core.md)). Plugin authors and the `coursive-plugins` index need to know which range to declare, which host release a package needs, what may change inside a version and how long v1 keeps working. None of this was written down.

## Decision Drivers

- An installed plugin must not break on a host upgrade without warning.
- Authors must be able to tell ahead of time whether a host accepts a package.
- The app should not carry v1 forever, but v1 is how the only official plugin ships today.

## Considered Options

- A. Semver on the manifest API: additive changes inside a major, breaking changes in a new major, the previous major deprecated with notice before removal.
- B. Pin every package to the exact host version it was built for.
- C. No versioning promise; break and fix in the same release.

## Decision Outcome

Chosen option: A.

- **API Version** is the semver of the manifest shape (`HOST_API_VERSION`, now `3.0.0`). A package declares the range it builds against in `coursive.api`; a host refuses a package whose range it does not satisfy and says why.
- **Minimum Host Version** of a package is the first host release whose API version satisfies its range; `validate-plugin.mjs --host` checks a release ahead of time, and release notes name the API version each release carries.
- Inside a major, the API only adds optional fields, node types, slot targets, result forms or permissions. Removing a field, narrowing a value set or changing a meaning is a new major.
- The previous major is deprecated, not removed: v1 (`engine`) packages keep installing and running. Removal happens only in a major host release announced at least one minor release ahead, while the validator names the v3 home of every v1 field.

### Consequences

- Good: authors and the index can check compatibility mechanically, and upgrades inside a major never break a package.
- Bad: the host keeps two readers until v1 is removed.

## Pros and Cons of the Options

### A. Semver with deprecation notice
- Good: a familiar contract; the range check already exists.
- Bad: additive-only discipline inside a major.

### B. Exact pins
- Good: no ambiguity.
- Bad: every host release breaks every plugin until it is republished.

### C. No promise
- Good: no process.
- Bad: plugins break silently on upgrade.

## Links

- Guide: [plugin-v3-guide.md](../plugin-v3-guide.md)
- Core contract: [spec 0031](../specs/0031-plugin-fabric-core.md)
- Semantic Versioning 2.0.0: https://semver.org/spec/v2.0.0.html
