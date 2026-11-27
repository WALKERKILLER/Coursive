# The course note owns assistant handouts

## Status

Proposed
Class: architecture

## Context and Problem Statement

The assistant must leave an editable Markdown file and a canvas card. Both entry points must survive a retry, independently edited material and device sync.

## Decision Drivers

Keep edits to a single file, reuse established course file sync, protect the user’s layout, and expose operations with the same rules as the visible UI.

## Considered Options

- Store a Markdown note and reference it by its stable ID from the canvas.
- Store a Markdown body in the note and the canvas, with a two-way merge.
- Let the assistant own the content and launch its handout viewer.

## Decision Outcome

The course owns one Markdown note for the review. The canvas keeps only the note reference and its geometry. Pi stages writes through Coursive's application commands. The daemon owns library writes; Flutter owns actions on live controls. Retry opens or links the existing note and leaves its text and the card’s geometry intact.

## Pros and Cons of the Options

- One note keeps file edits and assistant context consistent. Opening a card requires the matching note to be available in the local library.
- Copying the body lets the card load alone. Edits require a merge and can diverge when devices reconnect.
- An assistant-owned viewer can track structured content directly. It would add a separate editing and synchronization path.

## Links

- [Course workspace contract](../specs/0037-assistant-course-workspace.md)
- [Daemon material transactions](0044-course-material-tools-use-daemon-transactions.md)
