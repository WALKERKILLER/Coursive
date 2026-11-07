# Course conversation history and saved screenshots

## Status

Accepted
Class: feature

## Context and Problem Statement

Exact attachment hashes identify one run's input, but using that identity to filter course conversation history erases both the previous question and its image when the next question has no new attachment. The user explicitly requested continuous follow-up in the same conversation on 2026-11-08.

## Decision Drivers

Preserve conversational continuity and saved pixels while retaining conversation isolation, bounded input and fresh citation validation.

## Considered Options

- Require users to attach the same screenshot every turn.
- Load all images from every saved message on each request.
- Keep bounded course conversation text and make its saved screenshots readable by identity.

## Decision Outcome

Use the last eight completed course turns before the current saved turn. A selected-scope turn is a boundary. Retain exact input hashes for run identity; selected-mode legacy history still requires an exact match. The current turn ID prevents retry from reading later messages. New screenshots enter model input directly; a text-only follow-up rereads the two most recent distinct saved screenshots. Other screenshots in the bounded history are available through the visual tool. Every read validates the saved attachment against the current lecture endpoint and its hash. Historical answers never supply new citation authority.

## Pros and Cons of the Options

Repeated attachment loses expected continuity. Loading every image wastes model input and time. Bounded text plus individually readable immutable images preserves continuity and allows explicit rereading, while limiting the directly retained window to eight completed turns.

## Links

- [Selection conversation contract](../specs/0027-course-selection-conversations.md)
- [Escaped attachment failures](../postmortems/0009-snapshot-missing-from-tool-context.md)
