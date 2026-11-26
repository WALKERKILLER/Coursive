# Lesson study and shared learning context

Artifact-Version: 1
Status: Approved

## Intent

A student opens a lecture's editable handout, asks about a passage, and resumes their outstanding exercise on another personal device.

## Contract

The host resolves course ownership from the lecture in the daemon. The selection runner receives pinned preferences and, in course scope, that course's short memory. Memory text is learning context and never a source for classroom citations. A course preference stays local until the student explicitly requests a global preference. Full memory tools remain with the resident assistant. Course sessions may remember the current student's explicit scoped request. Corrections and deletion take precedence over historic attempts. The existing automatic consolidation switches govern background learning.

The installed course tutor supplies a versioned lesson handler. The host checks its enabled state and its library-read and workspace grants. Only a direct lesson request can create a Markdown note for that lecture. A second open resumes. Regeneration creates a separate revision for comparison with student edits. Excerpts are drawn from that lecture; teacher emphasis requires an exact verified quotation. The model receives bounded excerpts and any coverage gaps. A handout includes section identifiers, a concrete example, a question and links derived from verified excerpts. The student's explicit submission is the only input to assessment. A selected passage or a hint never submits an answer.

The plugin owns immutable checkpoints. A portable course exchange carries the selected course's teaching journal and its course memory. Private global preferences use personal assistant backup/sync. Concurrent answers to one checkpoint remain available as branches. The existing Markdown reader and canvas selection tools also support older cards.

For supported plugins the host provides a short lecture overview and bounded topic lookups. Each lookup covers only the current lecture's transcript, named courseware and slide images. Courseware search results locate excerpts. The host reads the associated text again before offering it as evidence. Only delivered excerpts can become citations. Slide times suggest images to inspect and do not certify transcript alignment. Exam papers attached to the lecture may inform practice after inspection. Materials whose role is unclear are treated as courseware.

The tutor first chooses teaching questions, then reads the material that can answer them. An explanation paragraph may cite more than one excerpt. Classroom examples, supplemental examples and self-tests carry their respective labels. Source links use the original asset identity and location. A saved handout and its outstanding exercise open without new model calls. The optional lookup service preserves access for plugins with the original contract.

Versioned citations carry the quoted text and its source revision. A transcript link includes the measured utterance interval when available. A snapshot link opens the image. The lecture-scoped `courseware/displayed-page` lookup accepts visible text and returns a unique PDF page with its file digest. The reader opens `courseware/page-preview` against that digest. Ambiguous visual matches produce an empty result. Course-scope canvas reads can check an explicit classroom question against the material authorized by the current search.

A compatible tutor records the assessed skill, question, rubric, answer, quotation and hint status on submission. Historic missing hint status is unknown. Two successive correct answers to distinct questions on a skill, both independently completed, qualify the student for a next-topic recommendation. Students can request a hint or skip ahead explicitly. The reading position and side conversations have no assessment effects.

## Plan

Use the existing pi model pipeline, note library, grounded selection runner and course-tutor workspace. See [the ownership decision](../decisions/0043-lesson-study.md).

## Verification

Exercise scope isolation, source validation, repeat opening and submission, stale note edits, and restoration from a second working library.

## Evidence

[Agent tests](../../agent/src/lesson-study.test.mjs) cover source lookup, grants and memory isolation. The [lesson widget test](../../app/test/study_lesson_test.dart) covers reopening and intentional submission. The [folder exchange test](../../daemon/internal/datasync/folder_lesson_test.go) checks selected-course progress and memory.
