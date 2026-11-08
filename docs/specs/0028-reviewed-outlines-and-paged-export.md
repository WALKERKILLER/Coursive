# Reviewed outlines and paged study-note export

Artifact-Version: 1
Status: Approved
Approved-By: maintainer (WALKERKILLER) through the settled decisions of [epic 18](https://github.com/WALKERKILLER/Coursive/issues/18) and sub-issues 23–25, with the explicit request to implement them on 2026-11-07; the pull request review confirms this record.

## Intent

Turn the material a learner selected on the canvas into a review outline that the learner, not the model, decides on. The assistant proposes a draft with cited facts, its own summaries, review questions and the marks it could not interpret; the learner reviews every item, saves the result as an ordinary lecture note and may export it as Markdown or a paged PDF whose citations stay readable on paper.

This extends [0027](0027-grounded-canvas-answers.md): evidence IDs, the read-only tool boundary, budgets and explicit pins are unchanged. Non-goals: the model writing notes, cards or files; automatic interpretation of ticks, arrows or circles; rebinding stale evidence to new material; cloud export services; the tutor mode of issue 27.

## Contract

### Proposal (`task: "propose_outline"`)

- A run started with `task: "propose_outline"` reads only the bound sources, ignores earlier turns of the conversation and returns `outline_proposal` beside the normal answer, evidence and trace. Outline turns are not replayed as history to later answers. Unknown tasks are rejected.
- `OutlineProposal` version 1: `proposal_id`, lecture, conversation and turn IDs, `source_revision`, `source_fingerprints`, `state` (`draft`, or `accepted` with `note_id` and `note_hash`), `title`, at most 50 `sections` (`heading`, `level` 1–3, at most 20 `items`), at most 20 `conflicts` with evidence on both sides, at most 20 `unresolved_questions` and at most 24 `missing_sources`.
- An item has `kind` `fact` (must cite issued evidence), `summary`, `question` or `unresolved_mark`, its `text`, at most 24 `evidence_ids` and a `review` of `pending`, `confirmed`, `to_check` or `needs_interpretation`. A mark carries the ink or image `source_id` and anchor it came from and a learner `choice` (`mastered`, `key`, `review`, `custom` with `choice_text`, `keep`); until chosen it is `needs_interpretation`. Selected ink the run never saw becomes such a mark instead of a guess.
- The host keeps only issued evidence (at most 120 per outline), clips text, maps unknown kinds to `summary` and trims the draft to the 300 KB result budget. A reply that is not an outline is an error with no draft. The daemon validates all of this when the conversation is saved.

### Review

- The outline opens in a review panel on the side away from the assistant dock (at most 400 dp, 42 % of the window); below 720 dp it is a bottom sheet that shrinks to a 76 dp peek while a citation is being viewed. Nothing on the canvas changes.
- Each item can be confirmed (A), flagged 待查 (F), edited (Enter, E), split at the caret, merged, moved between sections (drag or Alt+↑/↓), deleted or added; headings and levels are editable. Ctrl/⌘+Z undoes the last 60 changes. Changes are kept in the conversation after 500 ms and survive switching conversations and restarts.
- A mark shows its real ink or image thumbnail and offers 已掌握 / 重点 / 待复习 / 自定义 / 保持原样; it never becomes a fact by default. Evidence that no longer resolves shows 「依据已失效」 and can only be kept as 待查 or have the stale citation removed.
- Citations behave as in 0027: hover previews, click navigates, 「返回原处」 returns to the same item with keyboard focus.

### Saving as a note

- 「保存为笔记」 writes `PUT /api/notes` with the stable ID `outline-<proposal_id>`: it reads the note first, sends the last saved body as `base_body`, then marks the proposal `accepted` with `note_id` and `note_hash` (FNV-1a 64). A retry after a lost response updates the same note. A 409, or a note changed elsewhere, shows the conflict, keeps the other edit and offers 「另存一份」. The canvas document is never written.
- 「留在画布」 pins the outline through the 0027 pin path.

### Export

- The note body holds the pending and confirmed items, with 问题 and 未核实 tags, and the marks the learner chose. Everything unconfirmed (conflicts, uninterpreted or kept marks, 待查 items, open questions and unread sources) goes to a 「待核实」 appendix under a note that it is not a conclusion. Sources are a numbered list with a readable label, page or time, quote, 「依据已失效」 where it applies and a `coursive://` link back.
- Markdown is the note body. The PDF is built on the device from the same document (A4 or Letter, portrait or landscape, normal or compact margins). It has a text layer, a contents page with real page numbers when there are four or more headings, linked citation marks, footers with page numbers, no heading stranded at the bottom of a page, code split rather than dropped, and rendered math and ink.
- The preview renders the exact bytes that will be saved, through `POST /api/export/pdf-preview` (the PDF body, at most 40 MB; `page`); it writes nothing. Missing glyphs are listed before saving. A save failure keeps the note and offers Markdown. Exporting changes no source, note or canvas data.

## Plan

Host outline task and assembly; daemon proposal validation and the preview route (app-only, the assistant host does not call it); app review panel, note save and PDF builder with a subset Noto Sans SC. A shared fixture `daemon/internal/library/testdata/outline-turn-result.json` is written by the host test and read by Go and Flutter.

## Verification

Go validates good, resolved and accepted proposals and rejects malformed ones, saves notes idempotently with conflicts, and renders preview bytes without keeping files. Node covers issued-only evidence, unread ink, the size budget with 40 passages and 15 marks, scope and history isolation, malformed replies and the shared fixture. Flutter covers review actions, marks, stale evidence, save and conflict, close and Esc, citation follow and return with focus, the narrow peek, Markdown output and a Poppler-checked PDF.

## Evidence

- Go: [study_conversation_test.go](../../daemon/internal/library/study_conversation_test.go) (`TestStudyConversationValidatesOutlineProposals`), [outline_note_test.go](../../daemon/internal/library/outline_note_test.go), [pdf_preview_test.go](../../daemon/internal/library/pdf_preview_test.go).
- Node: [canvas-agent.test.mjs](../../agent/src/canvas-agent.test.mjs) (outline tests and the shared fixture).
- Flutter: [study_outline_review_test.dart](../../app/test/study_outline_review_test.dart) and [outline_export_test.dart](../../app/test/outline_export_test.dart), which checks a seven-section A4 outline with Chinese, math and code through Poppler: text layer, contents pages matching headings, no stranded headings, footer page numbers and different bytes for Letter.
- Not observed yet: outline quality with a real model, an Android tablet, an external PDF reader opened from the file manager, screen readers and Windows or macOS saves.
