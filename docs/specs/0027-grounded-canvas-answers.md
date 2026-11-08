# Grounded canvas answers and user-led source navigation

Artifact-Version: 1
Status: Approved
Approved-By: maintainer (WALKERKILLER) through the settled decisions of [epic 18](https://github.com/WALKERKILLER/Coursive/issues/18) and sub-issues 19–22, with the explicit request to implement them on 2026-11-07; the pull request review confirms this record.

## Intent

Turn a canvas selection into a question whose answer can be checked against the material it came from. The learner selects material, asks, and receives an answer whose claims point at the exact page passage, transcript line or inspected region that the assistant actually read. Only a click on such a citation moves the canvas, and a return control restores the previous view. Answers kept on the canvas keep a stable identity.

This contract extends [0020](0020-multimodal-canvas-agent.md) and [0021](0021-modular-canvas-conversations.md). Their read-only tool boundary, scoped history, conversation sidecars, budgets (20 tool calls, 12 model rounds, 180 seconds, 3 visual reads) and explicit pins stay in force. Non-goals: model-driven camera moves or seeks, automatic tours, periodic screenshots, whole-video upload, canvas writes by the model and the tutor mode researched in issue 27.

## Contract

### Selection and asking

- A rectangle or lasso that partly covers a PDF, image or video card records only that object's normalized region; only a fully enclosed card is selected as a whole and can be moved. Ink is bound by stroke IDs. The original card and ink are never modified by asking.
- A compact action bar sits 8 dp off the selection's top-right corner, flips below when the top is obstructed, keeps 12 dp from window edges, and offers 「问 AI」 (primary), 「摘录」 (capture the region as a sourced image card) and 「关联」 (region link), plus a menu for editing actions. Targets are at least 44 dp. Enter opens the question, Esc steps back. No radial menu.
- 「问 AI」 expands into an in-place composer about 360 dp wide (at most 420 dp and the window width minus 24 dp). Chips name each bound source with its page, time range, stroke count or region and whether readable text is present; each chip can be removed, and 「+」 adds a card. Dropping a card on an existing conversation binds it without sending.
- 「看图」 is off by default. While it is off, no selection screenshot is taken and requests carry `visuals: []`. Selections without readable text (ink, scanned pages) say so and offer to enable it; when enabled, the actual crop is previewed before sending. A send freezes the bound selection and revision. Failure keeps the draft, selection and video range and never records a duplicate successful turn; a revision conflict asks the learner to rebind. Collapsing the sidebar does not cancel a run.

### PDF region text

For a paged PDF, a region without a text range reads the page's real text boxes: words whose boxes lie at least half inside the region select a contiguous UTF-16 span of the page text, which becomes the source text, range and quote. When no box qualifies, or the card is rotated so the region is not in page orientation, the source stays `vision_required`; it never widens to the page or the course.

### Evidence and suggestions (run result schema 2)

- Every tool read registers evidence candidates in the host: document passages (`exact_text`, with page and UTF-16 range inside the read span), transcript lines (`timestamped_transcript`, with times inside the bound range) and successful visual inspections (`visual_observation`, proving only that the authorized image was read). Each has an ID (`e1`…), its `source_id`, the daemon's anchor (workspace, object, page, range, region, time, fingerprint, quote), `kind` and `status` (`verified`, `stale`, `unavailable`).
- The model cites with `[[e:ID]]`. When the run finishes, the host keeps only markers whose IDs it issued in that run, removes all others, and returns `schema_version: 2`, the cited `evidence` (at most 24) and `suggestions`. Marker-like text inside source material is neutralized before the model sees it; history replays answers without markers.
- At most two suggestions, each `open_source` (an issued evidence ID), `ask_followup` (prefill text) or `propose_outline`. They create entry points only; nothing runs until the learner clicks and confirms.
- The app stores the result in the existing turn `result`; the daemon validates its shape, kinds, statuses, anchors and sizes on save. Older turns and hosts keep source-level chips.

### Citation display and navigation

- Markers render as numbered superscripts. Hover or keyboard focus highlights the cited passage on the canvas (the actual text lines, region, stroke bounds or video card) and previews its quote; it never moves the camera or a video. Offscreen sources get an edge indicator instead of a long line. Touch shows the preview on tap with an explicit locate action.
- Activating a citation first validates workspace, object, visible layer, page text, quote, ink and time. Invalid evidence shows 「来源已失效」 or 「无法定位」 and pushes nothing. Valid evidence pushes a view bookmark (camera, zoom, selection, PDF reading page, video time, sidebar state; at most 10, oldest dropped) and flies to the target within the area left by the sidebar and safe insets, `zoom = min(availableW / targetW, availableH / targetH)` clamped to 0.05–2.0, instantly under reduced motion. Pen, touch, wheel or pinch input cancels a flight.
- PDF page changes made by navigation are reading state: they add no undo entry and no document edit, and a return restores the page. A video citation seeks to the verified time and pauses; a return restores the earlier time only if the learner has not played or moved the video since. Citations to another lecture are not followed automatically.
- 「返回原处」, Esc, Alt+← / ⌘[ and the system back action restore bookmarks in LIFO order.

### Answer provenance and pins

- An answer shows a coverage line (passages, transcript lines, inspected regions, stale citations) and its suggestions.
- 「留在画布」 creates the editable text card through the existing undo path with the turn's anchors and `generation = "canvas-turn:<conversation_id>:<turn_id>"` (a stable hash when longer than 100 characters). The pinned card is found by generation, not by text: identical answers in different turns pin separately, a second click reveals the existing card, an edited card stays linked, and a deleted card can be pinned again. Placement avoids existing cards and ink near the source.

## Plan

Daemon region text and result validation; host evidence catalog, marker assembly and suggestion parsing; app action bar and composer, evidence rendering, navigation stack and pin identity. Shared fixtures cover a PDF page with Chinese and emoji text, transcript lines and forged markers.

## Verification

Go tests import a real PDF through Poppler or PDFKit and check region text, ranges and the vision fallback; conversation saves reject malformed evidence. Node tests drive the run loop with a protocol fixture and check issued, forged and out-of-scope markers, transcript times, suggestion limits, history stripping and injected markers. Flutter tests cover the action bar, vision-off sends, draft retention, citation rendering, hover without camera change, navigation with return order, gesture cancellation, document immutability, video pause and stale sources, and pin identity. Real-model, device and recording checks are reported separately from fixtures.

## Evidence

- Go: `TestStudyPagedPDFRealSelectionAndRestore` imports a two-page PDF through Poppler (PDFKit on macOS) and reads "page learning" from a region built from the page's real word boxes, with the matching UTF-16 range; a blank region stays `vision_required`. `TestStudyRegionSpanUTF16` covers Chinese, emoji and words less than half inside. `TestStudyConversationValidatesGroundedResults` rejects unknown kinds and statuses, duplicate or malformed IDs, invalid anchors, unissued jumps, unknown or excess suggestions and a future schema, and keeps legacy results.
- Node: `canvas-agent.test.mjs` covers UTF-16 passages, transcript lines clamped to the bound range, markers injected through source text, ID reuse on re-read, issued, forged and out-of-scope markers, the 24-citation and 2-suggestion limits, history without markers and a protocol run that returns schema 2.
- Flutter: `study_grounded_answers_test.dart` covers numbered citations, hover without camera change, navigation clear of the sidebar and return by Esc, flight cancellation by pointer input, stale sources, a transient PDF page, lecture seek, pause and restore, and pin identity. `study_selection_bar_test.dart` covers placement and flip, Enter and Esc with the draft kept, and readable and vision-needed chips; the existing quick-question tests cover vision-off sends, crops after opt-in and draft retention.
- Not observed yet: citation quality with a real model, touch previews on a tablet, screen-reader output and recordings.
