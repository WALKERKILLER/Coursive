# Distinguish stylus touchdown holds from endpoint holds

## Status

Accepted
Class: feature

## Context and Problem Statement

A user holds a card with the pen, releases it and expects the next contact to move the selected card. Restricting pen movement to an enclosing lasso makes a visible card selection misleading. Unconditional pen dragging instead prevents annotation on course materials.

## Decision Drivers

Preserve direct handwriting, allow deliberate selection anywhere on card content, keep pressure erasing and shape recognition distinct, and retain source-linked notes and ink while moving materials.

## Considered Options

- Require an enclosing lasso before every pen-driven card move.
- Immediately move any card touched with a pen.
- Give stationary touchdown holds object ownership, endpoint holds drawing ownership, and selected objects ownership of subsequent drags.

## Decision Outcome

Choose explicit input ownership through the canvas. A stationary touchdown can transfer a provisional stroke to card selection; a stroke that leaves the touchdown tolerance permanently gives up card selection for that contact. Shape recognition operates on the resulting drawing. Pressure erasing wins over pending selection. Selected objects can be regrabbed until the user clears selection or chooses a pen to resume writing. Playback and paging controls retain separate hit regions. Exact interaction rules belong in [drawing controls](../drawing-controls.md).

The Notein investigation supports separating drawing, object selection and an active object editor. Coursive implements its own shared Flutter router and existing document transforms, without transferring Notein code or assets. The source digest, inspected entry points and evidence limits are retained in [the reference findings](../notein-input-reference.md).

## Pros and Cons of the Options

Lasso-only movement is unambiguous but adds unnecessary mode changes for a single card. Immediate movement eliminates the delay but steals handwriting. Touchdown holds preserve both actions and allow regrabbing, at the cost of an explicit stationary threshold and careful priority handling. Distinguishing a whole-card selection from an AI source-region selection prevents annotation context from becoming accidental drag permission.

## Links

- Supersedes [card input ownership](0026-canvas-card-input-ownership.md), retaining shared camera routing and control boundaries.
- [Notein APK investigation and provenance](../notein-input-reference.md).
- [Input regression tests](../../app/test/study_input_modes_test.dart), [shape regression tests](../../app/test/study_shape_test.dart), [video control boundaries](../../app/test/study_video_hold_test.dart).
