# Host-issued evidence markers for canvas answers

## Status

Accepted
Class: feature

## Context and Problem Statement

Canvas answers listed the sources a run had read, but could not show which claim came from which passage, transcript line or region. A citation the learner can click must lead to material that was actually read in that run and not to a location the model made up.

## Decision Drivers

Citations must be machine-checkable, never invent locations, survive restarts in the existing conversation sidecar, cost few extra tokens, and keep working with older turns and hosts.

## Considered Options

- Ask the model for structured JSON citations with page, time and quote fields.
- Match answer sentences to read passages after generation.
- Have the host issue evidence IDs with each tool result, let the model cite them with inline markers, and keep only markers the host issued.

## Decision Outcome

The host issues evidence IDs. Each read returns passages, transcript lines or an inspected region with short IDs; the model writes `[[e:ID]]` after a claim. When the run finishes, the host drops every marker it did not issue and returns the cited evidence with the daemon's own anchors ([spec 0027](../specs/0027-grounded-canvas-answers.md)). Locations therefore always come from daemon-resolved sources, never from model text, and the app renders the surviving markers as numbered superscripts.

## Pros and Cons of the Options

Structured citations are easy to render, but the model can still invent pages, times and quotes, so every field would need a second validation against sources. Post-hoc matching needs no prompt change, but it is fuzzy for paraphrases and can attach a citation to a passage the claim did not use. Host-issued IDs cost a little prompt space and depend on the model citing at all; an uncited claim simply has no superscript. A forged or stale ID cannot become clickable.

## Links

- [Grounded canvas answers](../specs/0027-grounded-canvas-answers.md)
- [Scoped canvas tool runs](0017-scoped-canvas-tool-runs.md)
- [Runtime behavior](../multimodal-canvas.md)
- Citation interaction references, read 2026-11-07: [a NotebookLM citation is a button, not a reference](https://dev.to/anuis258/a-notebooklm-citation-is-a-button-not-a-reference-4he1), [NotebookLM help: citations](https://support.google.com/notebooklm/answer/16179559?hl=en), [RAG citation UX patterns](https://www.aydesign.ai/blog/ai-rag-citation-ux-design-patterns-2026)
