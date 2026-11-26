# Verify classroom passages before linking a handout

## Status

Proposed
Class: architecture

## Context and Problem Statement

A slide can change while the teacher continues discussing the previous example. A full-course handout must also survive an interrupted model request without rewriting a student's Markdown.

## Decision Drivers

Students need to open the original evidence, distinguish a visible slide from a spoken explanation and resume preparation on a limited device.

## Considered Options

- Attach the nearest slide to each transcript match.
- Index a course in advance and assign every segment a page.
- Retrieve bounded candidates on demand and verify each modality separately.

## Decision Outcome

The host searches transcript windows and readable courseware. It checks selected quotes against the retrieved text and asks the model to assess support. A snapshot can identify a parsed PDF page through multiple matching text blocks that identify a single page. Each saved reference opens its own source. An outline and its completed chapters have separate cache entries. The course note remains the editable result.

## Pros and Cons of the Options

- Nearby slides are cheap to retrieve but can show a different topic.
- Whole-course indexing amortizes later requests but makes a first question wait on unrelated lessons.
- Independent verification supports partial libraries and incremental work. It adds initial model calls and can leave a passage without a citation when transcription is poor.

## Links

- [Lesson study contract](../specs/0035-lesson-study.md)
- [Course workspace contract](../specs/0037-assistant-course-workspace.md)
- [Course note ownership](0045-link-assistant-handouts-to-course-notes.md)
