# Personal NAS services through a local bridge

## Status
Deprecated
Class: architecture

## Context and Problem Statement
This number held how the app reached one user's NAS deployment of course services. The core no longer contains platform-specific code; connections to external platforms are source plugins ([source-provider plugins](../specs/0024-source-provider-plugins.md)).

## Decision Drivers
- The core names no institution or platform.

## Considered Options
- Keep the decision in the core.
- Move it with the platform code into its plugin.

## Decision Outcome
Chosen option: move it with the platform code into its plugin.

## Pros and Cons of the Options
### Keep the decision in the core
- Bad: the core would describe a platform it no longer contains.

### Move it with the platform code into its plugin
- Good: the decision stays next to the code it explains.

## Links
- [Source-provider plugins](../specs/0024-source-provider-plugins.md)
