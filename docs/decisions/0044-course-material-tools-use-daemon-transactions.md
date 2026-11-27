# The daemon owns course material changes

## Status

Proposed
Class: architecture

## Context and Problem Statement

The assistant organizes files that are also indexed, linked from the canvas and synchronized across devices. Course file organization needs an owner for the working files and their references.

## Decision Drivers

Preserve existing course references and student edits, reuse the same action boundary across devices, and keep the Pi SDK independently upgradeable.

## Considered Options

- Register domain tools on the assistant host and commit changes in the library daemon.
- Give Pi general filesystem tools in a course directory.
- Store the organization as virtual tags while keeping the physical tree fixed.

## Decision Outcome

The assistant registers Coursive tools with Pi and sends course-scoped changes to the daemon. The daemon owns physical file moves, compatible references and the mirrored update.

## Pros and Cons of the Options

- Daemon-owned changes preserve references and recovery. This requires a library API for each supported class of change.
- General filesystem tools reach more file types with less tool code. The model would need to reconcile database state, references and sync itself.
- Virtual tags leave existing physical paths stable. They leave the actual course directory unchanged.

## Links

- [Assistant app operations](../specs/0016-agent-acts.md)
- [Course material tools](../specs/0036-course-material-tools.md)
