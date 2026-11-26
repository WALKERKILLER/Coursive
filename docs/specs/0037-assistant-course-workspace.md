# Assistant course workspace

Artifact-Version: 1
Status: Approved
Approved-By: local deployment owner, explicit implementation request on 2026-11-26

## Intent

Let the student delegate everyday actions in the Coursive library and on open course pages.

## Contract

The pi host provides Coursive commands for the active course or a course the student names. Tool arguments never select an arbitrary endpoint or a machine path. The application owns permissions, plugin grants and runtime configuration. Other assistant surfaces keep their existing read scopes.

The host stages native commands and library writes in Ask mode. Editing an original, changing several files or records, permanent deletion and external sharing always require user confirmation. A pending or dismissed proposal has no library effects. The confirmation identifies the affected files or records. Applying a proposal revalidates the original version, ownership and destination. Material operations follow [0036](0036-course-material-tools.md).

Course review saves an editable Markdown note under the course’s selected lecture, then pins one reference to it on the chosen lecture canvas. The canonical note and card have stable identities. Retry repairs a missing card, preserves existing content and reports a conflict if the target identity belongs to another object. Card geometry and existing ink remain user-owned. The model organizes course material and writes the explanation using verified source IDs. The host produces the links. The user can change the teaching instructions from assistant preferences. When only part of the course is readable, the handout identifies its coverage.

The host builds a concept outline from the available lecture catalogs and checkpoints each completed chapter under the selected model and its source revisions. Reopening the saved note bypasses generation. Explicit revision presents the proposed change for confirmation against the exact body read. Changing writing preferences can reuse the evidence cache.

## Plan

Register typed pi tools, reuse daemon library commands and route active canvas commands through Flutter’s player and transform handlers.

## Verification

Verify the course review and retry, unchanged student edits, source navigation, declined proposals, unauthorized scope, ordinary settings and player/PDF actions. Compare versions on macOS, Windows and Android. Use fake course data for tests and the student's real course for installed acceptance.

## Evidence

[Agent operation tests](../../agent/src/operate.test.mjs), [source grounding tests](../../agent/src/course-review.test.mjs) and [authenticated library tests](../../daemon/internal/server/course_files_test.go) cover confirmations, course ownership and retry.
