# Snapshot omitted from the model's tool context

Artifact-Version: 1

## Executive summary

A saved selection screenshot reached the model's visual input, but the assistant reported that it could not see it. The attachment was absent from the separate tool context, while the system prompt required another visual tool call. Input delivery alone did not prove a coherent tool conversation.

## Summary

The user supplied a real-window answer containing a screenshot thumbnail and an erroneous authorization explanation. The saved turn had a persistent attachment and completed successfully; the answer said the bound-source list was empty and visual inspection had rejected its source. The older failed messages displayed above it were separate runs. The defect belongs to [issue 28](https://github.com/WALKERKILLER/Coursive/issues/28).

## Timeline

- 2026-11-08: snapshot implementation passed deterministic byte-delivery tests; native window automation blocked full real-model acceptance.
- The user supplied the first real answer showing that the assistant disclaimed image access despite receiving an attachment.
- Review found that the run injected snapshot content but built tool authorization exclusively from legacy mutable object bindings.
- Regressions exercised the complete model/tool loop with empty object bindings for both direct-image and vision-observation inputs.

## Root cause

The implementation treated the visual model input and the tool's source registry as independent successful steps. The prompt still required `inspect_canvas_region` for images, while `get_canvas_context` exposed only object-bound sources. A source-free snapshot therefore produced contradictory instructions and an unauthorized second read. The initial tests asserted that bytes reached the vision adapter but stopped before the model enumerated and reinspected that attachment.

## Guardrails

- [Agent regression](../../agent/src/canvas-course-sources.test.mjs) traverses context enumeration, attachment reinspection and answer generation with no object bindings, for both image and described modes. It checks that the same cached evidence returns without another vision call or a fabricated object anchor.
- [Run scope](../../agent/src/canvas-agent.mjs) lists verified attachments separately from mutable sources and permits reinspection only by their run-scoped identifiers.
- [Selection contract](../specs/0027-course-selection-conversations.md) distinguishes visual input, tool discoverability and real-model acceptance. Passing fixtures does not close the real-flow acceptance item.

## Follow-up: the next turn lost both history and attachments

A subsequent real user follow-up reported empty attachments and no preceding context. The previous repair covered same-run tool enumeration, but not the transition from a sent attachment to a new text-only draft. The client correctly moved sent attachments out of the draft; the runner read only the new draft and filtered saved text by an exact input hash that included attachments. A no-image follow-up therefore lost both the old pixels and the old text.

The permanent guardrail now runs an image question, a text-only follow-up, and another follow-up through a newly constructed runner reading persisted turns. It asserts that text and freshly validated saved pixels reach the model in each follow-up. Separate tests cover scope boundaries, retry cutoffs and rejection of old citation IDs without a new evidence read. The [history decision](../decisions/0030-course-conversation-history.md) defines the bounded continuity contract. This closes a protocol gap; fixture success alone does not establish real-model answer quality.

History cutoff uses a stable saved message ID, distinct from the asynchronous model job ID. The client lifecycle regression checks this identity after start and retry, preventing a race where the runner reads a conversation after the client has replaced the cutoff ID with a job ID.
