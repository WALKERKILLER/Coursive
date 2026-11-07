# Selection editing and study regions

Artifact-Version: 1
Status: Approved

## Intent
Implement the seven selection improvements authorized by the user and tracked in [issue 12](https://github.com/WALKERKILLER/Coursive/issues/12). Preserve existing courses and handwriting and reuse canvas undo, asset and synchronization paths. Reference behavior is documented in [Notein input evidence](../notein-input-reference.md); implementations are independent.

## Contract
Object filters distinguish ink, text, images/PDFs, video and other cards. Excluded objects cannot enter a new lasso selection. Ink selection accounts for the rendered pressure-dependent outline. Capture creates a local image card with source references; cancellation or failed capture does not alter original objects. Selection operations respect hidden/locked layers and keep one undo entry per completed edit.

Rotation applies around a shared selection pivot to supported objects and ink, with unsupported selections explicitly disabled. Shape editing exposes control points for recognized geometric ink, previews the result with guides and commits only on confirmation. Handwriting recognition explicitly sends the selected ink image to the configured model, allows correction before inserting editable text, and retains original ink. Region links connect two reviewed selections within a canvas, retain page/time/ink references and navigate both ways; missing targets produce an explicit message.

Existing serialized fields and asset imports are preferred. Additional persistent fields must be validated by the daemon and preserved by both clients; unsupported versions must not silently overwrite them. Synchronization transports assets and canvas records through existing paths, never credentials.

## Plan
Implement selection filters and ink geometry first, then capture and references, transforms and shape editing, and model-assisted recognition. Add behavioral tests at document, widget and daemon boundaries as applicable, then update existing Mac and Android applications in place.

## Verification
Check filter combinations, boundary-width selection, cancelled operations, source-page/time preservation, locked layers, undo/redo and serialization round trips. Exercise real selection actions in widget tests and run repository gates. Device acceptance verifies launch, saved data, selection actions and model response separately from build success.

## Evidence
The [selection geometry tests](../../app/test/study_selection_geometry_test.dart) cover pressure outlines, atomic rotation and legacy PDF anchor normalization. The [selection tools widget tests](../../app/test/study_selection_tools_test.dart) exercise filtering, capture, reviewed recognition, reciprocal links and shape edits through the UI. The [daemon selection tests](../../daemon/internal/library/study_selection_test.go) cover format validation and migration; the [real PDF test](../../daemon/internal/library/study_pdf_test.go) checks that page-derived source anchors can be saved. Existing [selection question tests](../../app/test/study_quick_question_test.dart) cover the related sending regression.

Local build 2026110711 is installed in place on macOS and Android. PDF selection capture and source preservation were verified on both devices; Mac capture persistence across application restart and Android source navigation were verified. Both installed model pipelines read a captured PDF table successfully. A zoomed-out Android capture exposed inadequate logical-pixel resolution; bounded higher-resolution rasterization corrected the tested reading. Real handwriting recognition quality, full cross-device transfer of the new operations and physical stylus ergonomics remain device acceptance work; deterministic tests do not establish those outcomes.

The selection-to-question audit adds [visual restoration tests](../../app/test/study_visual_restore_test.dart) for reopened crops, image-permission changes, source replacement and cancellation; [query tests](../../app/test/study_query_test.dart) cover daemon defaults as well as removed references. Android UI acceptance exercised a reopened cropped PDF conversation with vision disabled then re-enabled, and a fresh rectangular selection with an attached image; both model responses used the selected exercise. Physical pen/palm sensing remains outside injected-input acceptance.
