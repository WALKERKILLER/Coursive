# Pen writing and uniform card transforms

## Status
Accepted
Class: feature

## Context and Problem Statement

A selected card steals pen handwriting, touch contacts cannot reliably reach resize handles, and changing a reader's width reflows text when the learner intends to magnify its contents.

## Decision Drivers

Keep pen writing continuous, separate camera navigation from object manipulation, preserve document layout and PDF ink coordinates, and keep geometry identical across built-in card types and pointer devices.

## Considered Options

- Keep mode-specific touchdown holds and independent reader resize behavior.
- Assign pen content contacts to writing, finger title contacts to manipulation, and use shared uniform transforms.

## Decision Outcome

Use shared input ownership and snapshot-based card transforms. A stored content scale separates layout size from displayed size. Touchdown holds on content do not grant card movement; selected content remains writable. Exact routing, snap thresholds and compatibility live in [the gesture contract](../specs/0034-canvas-gesture-and-uniform-scale.md).

## Pros and Cons of the Options

Mode-specific holds preserve established gestures but require the user to remember whether selection or drawing currently owns the same contact. Separate input ownership makes daily writing predictable and reuses one transform frame, at the cost of a new document format and coordinated device upgrades. Uniform scaling preserves text layout but deliberately does not provide independent width-only reflow through corner handles.

## Links

- Supersedes [touchdown holds](0027-stylus-card-holds.md).
- [Notein static observations and provenance](../notein-input-reference.md).
- [Notein input modes](https://help.notein.ai/category/editor-tools/input-mode?lang=en).
