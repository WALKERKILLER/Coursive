# Independent canvas documents and PDF ink

Artifact-Version: 1
Status: Approved
Approved-By: local deployment owner, explicit notebook and PDF editing requests; upstream maintainer review pending

## Intent

Allow a learner to create a canvas without a recording and annotate a PDF inside a material card. Preserve source PDFs, existing lecture workspaces, plugin objects and handwriting. This contract extends [lecture canvases](0019-lecture-study-canvas.md) and does not expand assistant write permissions.

## Contract

An independent notebook has an immutable random ID, a bounded nonempty title and an optional existing course ID. Its metadata, vector canvas and imported assets live under `notebooks/{id}/` in the library. Authenticated `GET/POST /api/notebooks`, `GET/PUT /api/notebooks/{id}/canvas` and `POST /api/notebooks/{id}/assets?name=…` use the same revision, size, rooted-path and atomic-save guards as lecture canvases. Both Flutter and Node consumers carry these routes. A standalone canvas contains no artificial lecture card; lecture canvases still require their lecture identity.

The [uniform transform contract](0034-canvas-gesture-and-uniform-scale.md) owns format 7 and content scaling. Canvas format 6 adds finite free rotation (−π to π radians) and quarter turns on every built-in card. Formats 1–5 remain readable, including version-5 plugin objects; documents needing the new transforms serialize as version 6 so older clients reject them instead of silently dropping orientation. Locked and hidden objects remain protected. Existing material bounds and attached ink transform together; selection hit testing uses displayed geometry.

PDF cards offer vertical scrolling, horizontal paging or expanded sheets. Only vertical reading exposes right-margin growth. Touch navigation and pen input are distinct; controls consume contact instead of drawing through to the canvas. Completed ink and its live tail repaint independently, and erasing changes vector fragments before a coalesced save. Large seeks mount only visible pages, with an exact-extent regression guarding against a full-document render storm.

The first edit creates a sibling “批注版.pdf” working copy in `courseware/`; the source is unchanged. macOS uses a packaged pypdf helper and PDFKit renderer, and Android uses PDFBox through a serial native executor with bounded scratch memory. Standard Ink annotations include appearance streams. Source hash, page count and current working-copy hash guard saves, which replace files atomically. Only Coursive-tagged annotations are replaced; other annotations remain intact. Right margins are 0%, 25% or 50%, cannot shrink, and are unavailable for already cropped pages. Reopening restores supported ink styles and coordinates. A changed working copy reports a conflict without overwriting it.

The macOS annotation helper requires `/usr/bin/python3` (Python 3.9 or later); pinned pure-Python dependencies and their licenses ship inside its executable zip. Systems without that interpreter must install it before using embedded PDF annotation. The PDFKit renderer is a universal binary. Linux and Windows retain their existing PDF import/reading paths; the native working-copy editor is limited to macOS and Android.

Independent notebooks are local documents. The course-folder exchange protocol does not yet publish their directory; synchronizing lecture canvases does not imply that independent notebooks are synchronized.

## Plan

Reuse the vector editor, rooted library boundary and existing settings for independent documents. Keep the native PDF working-copy model behind one Flutter adapter, retain upstream plugin rendering, and package desktop dependencies through the release build. Record independent identity and PDF storage alternatives in [ADR 0038](../decisions/0038-independent-canvas-documents.md) and [ADR 0039](../decisions/0039-docked-pdf-working-copy.md).

## Verification

Use authenticated server tests for independent identity, traversal rejection, stale revisions and lecture compatibility; model and widget tests for transforms, Markdown, selection, pointer isolation and PDF distant seeks; Go race tests for persistence; native helper tests for source preservation, other-reader annotations, rotated margin coordinates, reread and revision conflicts. Check the macOS helper package and Android compile path separately from Flutter widgets. No live account or private course material belongs in a test fixture.

## Evidence

[Notebook and canvas routes](../../daemon/internal/server/study_canvas_test.go), [rotation persistence](../../daemon/internal/library/study_selection_test.go), [notebook widgets](../../app/test/notebook_canvas_test.dart), [PDF gestures and bounded seeks](../../app/test/pdf_dock_test.dart) and [selection transforms](../../app/test/study_selection_geometry_test.dart) exercise these contracts. The [render-storm postmortem](../postmortems/0010-long-pdf-seek-render-storm.md) records the escaped Android memory failure. [Native PDF helper checks](../../scripts/desktop/test_pdf_notes.py) cover the packaged entry, revision conflicts, rotated margin geometry and preservation of other-reader annotations.
