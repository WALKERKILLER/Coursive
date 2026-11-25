# Editable lecture handouts and course tutor checkpoints

## Status
Proposed
Class: architecture

## Context and Problem Statement
Course chat and canvas questions use different runners. A student can ask about a handout while an exercise is pending.

## Decision Drivers
Readable library files, recoverable answers, scoped access and stable citations matter across devices.

## Considered Options
- A separate teaching agent and its own library.
- A teaching plugin with scoped host services.
- Send every attachment and transcript to the writer in one request.

## Decision Outcome
The pi host owns memory, model selection, lecture-scoped source lookup and the library write boundary. The plugin chooses teaching questions and follow-up reads, then saves paragraph citations and resumable attempts. The note is the editable handout. See [the contract](../specs/0035-lesson-study.md).

## Pros and Cons of the Options
A separate agent can specialize its interface but duplicates context, permissions and learning history. A plugin reuses the same student identity and requires an explicit contract for checkpoint recovery and note revision.

A single request has lower latency for small lectures. Bounded topic lookup gives comparable work to short and long lectures and lets the tutor request complementary media. It adds a planning call and depends on the model choosing useful search terms.

## Links
- [Course tutor memory](0042-course-tutor-memory.md)
- [Grounded canvas answers](../specs/0027-grounded-canvas-answers.md)
