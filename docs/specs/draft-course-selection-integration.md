# Course selection integrated with grounded answers

Artifact-Version: 1
Status: Draft

## Intent

[0029](0029-course-selection-conversations.md) (pull request 31) and [0027](0027-grounded-canvas-answers.md) / [0028](0028-reviewed-outlines-and-paged-export.md) (epic 18) were built in parallel for the same canvas conversation. They disagree on four points: how a selection is asked about, whether a conversation reads the whole course by default, how citations are written, and how a learner gets back from a followed source. This draft keeps one of each, following the settled epic 18 route, and keeps everything else from 0029. It needs a maintainer's approval because it changes the default reading scope that 0029 approved.

Non-goals: new model capabilities, new storage formats beyond 0029's version 2 conversation, removing the legacy `visuals` path.

## Contract

Asking. 「问 AI」 on a selection opens the in-place composer of 0027; the sidebar stays the place for long conversations. 「看图」 is off by default and nothing is captured while it is off. Turned on, it takes 0029's immutable selection screenshot (drawn bounds, lasso mask, frozen local video frame, at most 3 MiB, two per turn) and previews exactly that image before sending. The selection bar's 「附上截图」 button, or dragging it into the conversation, attaches the same screenshot explicitly; that gesture is consent for those pixels and does not depend on 「看图」. 0029's rectangle default, edge and corner handles and footer fallback are kept; the bar and composer sit off the selection's top-right corner, avoiding the selection, docks and sidebar, and only the bar moves to the footer.

Scope. Every conversation starts with `context_scope: selected`. The composer's 「全课程」 chip switches the conversation to `course`, letting the model call `search_course_materials` and `read_course_source` (0029's bounds, isolation and 409 behaviour unchanged). Attaching a screenshot never changes scope. Each turn records the scope it was sent with; a selected turn is the history boundary for course replay, as in 0029.

Citations. One protocol for all material: schema 2 evidence and `[[e:ID]]` markers (ADR 0029). Each course read is split into sentence passages; each passage gets its own host-issued evidence ID with `kind` `exact_text` or `timestamped_transcript`, an anchor (`id`, `source_lecture_id`, `original_path`, `page` when verified, rune `start`/`end`, transcript interval, `fingerprint`, `quote`) and `course_source`: 0029's source identity narrowed to that passage, without text or snippet. The daemon validates `course_source` like a 0029 citation source and rejects saved text. Following one re-reads it through `POST /sources/resolve`; a changed file (409) shows 「来源已失效」. Answers saved with `[N](coursive://citation/eN)` and a `citations` table load as the same evidence; the daemon keeps accepting `citations`. A quoted sentence (“…” directly before a marker, at least 12 characters) must occur in the text read for that evidence, for bound and course material alike, ignoring whitespace and punctuation and skipping code and image observations; a mismatch allows two corrections, then the run fails.

Navigation. One way back: 0027's 「返回原处」 stack of 10 views. Following course evidence reuses or imports the material card as 0029 does (PDF at the verified page, the passage lit where the page text holds it verbatim; video paused at the interval start once the player is ready; otherwise an excerpt text card), then lands like any citation. Returning also restores the conversation's scroll position and the region selection. Pills of bound material land the same way.

History. Selected scope replays the last eight completed turns with the same context key; a turn's screenshots are not part of that key, so a text-only follow-up keeps the turn that attached one. In both scopes the saved screenshots of replayed turns stay readable, and a text-only follow-up rereads the latest two. Outline drafts (0028) never replay and run without course tools; their `turn_id` is the saved message ID.

## Plan

Node host: course reads issue evidence through `CanvasToolScope.issue`; the quote check moves into the host for all sources; scope, history and attachment handling as above. Daemon: optional `course_source` on `StudyEvidence`, sharing 0029's source validation. App: snapshot capture replaces the per-source crop helper; scope chip, screenshot strip and attach button; course evidence routed through `_followEvidence`; legacy citation upgrade on load. Docs: the multimodal canvas contract describes the result.

## Verification

`node --test` and `npm run check` in `agent/`; `go test -race ./...`, `go vet` and `gofmt -l` in `daemon/`; `flutter test` and `flutter analyze` in `app/`; repository gates. Scenarios: 「看图」 off sends no pixels; on, one screenshot with the bound text; attach without 「看图」; 「全课程」 off by default and per conversation; course passages cited and followed, legacy citations followed, failed resolution keeps the view, duplicate clicks import once; quote mismatch repaired; returns restore view, scroll and drafts.

## Evidence

Pending approval. Deterministic tests: [course evidence and history](../../agent/src/canvas-course-sources.test.mjs), [daemon validation](../../daemon/internal/library/study_conversation_test.go), [asking and screenshots](../../app/test/study_quick_question_test.dart), [course and legacy navigation](../../app/test/study_source_navigation_test.dart), [region editing](../../app/test/study_region_resize_test.dart).
