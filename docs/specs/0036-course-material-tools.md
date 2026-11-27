# Course material tools for the assistant

Artifact-Version: 1
Status: Approved
Approved-By: local deployment owner, explicit implementation request on 2026-11-26

## Intent

The student can ask the assistant to inspect and organize actual course files through the same library as the app. The student authorized the course-scoped tools, batch approval and undo in the implementation conversation.

## Contract

The resident and course assistants can list, search and read local course material. The host resolves the active course or requires a named destination. The daemon validates ownership on every path and refuses aliases that leave the course. Plotting which files to change gives the user a summary with a stable batch ID and versions. A single rename, move, archive or import follows the existing approval mode. Multiple edits and Markdown replacements always need confirmation under [0037](0037-assistant-course-workspace.md). Canvas passage questions retain their read scope.

The batch supports renaming, moving between lectures in one course, archiving courseware, importing from the course's intake directory, and exact edits to Markdown. Course recordings, generated subtitles, derived PDF assets and library metadata remain owned by their app workflows. Imported or moved files may be opened under their established references. Explicitly authorized Markdown patches keep the note ID and update its index. File contents are evidence for organization as requested by the student.

The daemon rechecks source versions and target availability at apply and undo. A change that would replace a newer file fails without committing the batch. The daemon journals file and index changes together, publishes a refresh after commit and keeps the committed operation for undo. Previews live in daemon memory for up to an hour. After a daemon restart the assistant can generate a fresh preview. Applied receipts support undo for thirty days. Recovery preserves ambiguous paths and records the conflict separately so other writes can proceed. Sync publishes complete local changes. Permanent deletion remains subject to the existing separate operation rule.

## Plan

Register a course material tool group in the Pi host, route writes through the existing proposal machinery, and reuse the daemon library catalog, mirror journal and index. See [0044](../decisions/0044-course-material-tools-use-daemon-transactions.md) for ownership.

## Verification

Exercise the authenticated tools with two courses and multiple lectures. Verify real rename and move, old source links, parse sidecars, undo, stale versions, refused out-of-course paths, and the auto and ask dispatch paths. Rebuild the index from committed files and verify source links again.

## Evidence

[Library tests](../../daemon/internal/library/course_files_test.go) exercise source references, reindex, stale versions and undo. [HTTP route tests](../../daemon/internal/server/course_files_test.go) exercise authenticated preview and the former file URL. [Assistant tests](../../agent/src/operate.test.mjs) exercise scope and the two approval modes.
