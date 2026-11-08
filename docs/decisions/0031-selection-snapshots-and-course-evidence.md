# Selection snapshots and course evidence

## Status

Accepted
Class: feature

## Context and Problem Statement
A temporary visual binding can be reconstructed from a changed page instead of the screenshot the user selected. Separate quick and sidebar composers split drafts. Course questions also need sources that have not been placed on the current canvas.

## Decision Drivers
Preserve exactly reviewed pixels, keep one draft owner, reuse local course materials, and make navigation reliable without model control of the camera.

## Considered Options
- Keep temporary per-object crops and separate composers.
- Send full screenshots without provenance through general assistant tools.
- Persist selection snapshots in one composer and add course-scoped read evidence.

## Decision Outcome
Choose persistent selection snapshots and one sidebar composer. Course-scoped tools resolve candidates through the daemon and create citations only after reading. The user clicks citations to add or locate materials and can return to the prior view. Attachment assets are independent of visible canvas objects and stored with the existing course files.

## Pros and Cons of the Options
Temporary crops minimize storage but cannot preserve historical pixels and fragment draft state. Unscoped screenshots are simple but lose provenance and expose unrelated tools. Persistent snapshots and scoped evidence retain source identity and user control, at the cost of attachment lifecycle handling and a conversation format upgrade.

## Links
- [Course selection contract](../specs/0029-course-selection-conversations.md)
- [Sidebar conversations](0024-sidebar-conversations-and-manual-pins.md)
- [Selection asset reuse](0028-selection-edits-use-canvas-assets-and-anchors.md)
