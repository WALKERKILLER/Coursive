# Two transports for a course exchange

## Status
Accepted
Class: architecture

## Context and Problem Statement

Some learners have a WebDAV collection. Others already share a directory across their devices, including tablets with a system directory grant. Both can carry the same immutable objects and manifests.

## Decision Drivers

Keep each working library private, let a user preview the selected courses before connecting, and make the device responsible for the chosen transport visible in settings.

## Considered Options

- Offer only direct WebDAV in settings.
- Support both direct WebDAV and a user-selected exchange directory.

## Decision Outcome

Offer both configurations. With a shared directory, the user's replication client carries files between devices. Coursive verifies and imports the received generation. With WebDAV, Coursive connects to the server directly. This supersedes the transport restriction in the amendment to [the device-manifest decision](0021-course-folder-sync-with-device-manifests.md). Its object format, merge rules and working-library boundaries continue to apply.

## Pros and Cons of the Options

WebDAV needs no second client but requires a reachable server and an account. A shared directory can use an existing peer-to-peer setup and works without a NAS. Each device needs directory access, and delivery depends on the replication client catching up.

## Links

- [Course exchange contract and verification](../specs/0023-course-folder-sync.md)
- [Syncthing folder synchronization](https://docs.syncthing.net/users/syncing.html)
