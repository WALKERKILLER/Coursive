# Assistant confirmations and interrupted course edits

Artifact-Version: 1

## Executive summary

Review of the course workspace found that automatic and confirmed writes had different safeguards. The original tests covered successful file moves and general proposal handling. They missed several tool-specific confirmation paths and a crash followed by a sync restoring the old filename.

## Summary

A personal reminder could move immediately. Overwriting a note showed only its path. Some new commands offered no chat undo or warning. Reading a proposed batch wrote copies of note text into the library and scheduled a sync that could block approval. During recovery, a restored old path could prevent every later reindex. Invalid or stale page references surfaced as server failures.

## Timeline

On 2026-11-27 the review findings were checked against the PR head. The regression fixtures use synthetic notes, dates and file contents. The project also corrected stale plugin guidance, an unsupported model-routing claim in the PR description and incomplete JavaScript syntax checks.

## Root cause

Tool wrappers knew the user's approval preference while individual commands owned the conditions for safe reversal. The batch planner used the same disk receipt for pending and applied states. Sync scheduling treated every successful POST as a mutation. Recovery assumed a move always left a single unambiguous file, and the new classroom route handlers returned library errors directly.

## Guardrails

- [Note and schedule tests](../../agent/src/workspace-writes.test.mjs) exercise the changed lines, confirmation and old values. [Operation tests](../../agent/src/operate.test.mjs) cover workspace confirmation, card position checks and batch dispatch.
- [Course file tests](../../daemon/internal/library/course_files_test.go) check preview side effects, receipt lifetime, independence from unrelated files and undo. [Sync tests](../../daemon/internal/datasync/datasync_test.go) exercise contention and cancellation.
- [Mirror tests](../../daemon/internal/library/mirror_test.go) simulate a crash with competing file contents and verify that another journal still recovers. [HTTP tests](../../daemon/internal/server/course_files_test.go) check invalid page numbers and stale revisions through the authenticated handler.
- [Syntax checks](../../agent/scripts/check-syntax.mjs) include newly added modules automatically. Use the [workspace contract](../specs/0037-assistant-course-workspace.md) when adding another tool.
