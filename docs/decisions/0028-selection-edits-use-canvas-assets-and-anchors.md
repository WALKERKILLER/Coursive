# Reuse canvas assets and source anchors for selection editing

## Status

Accepted
Class: feature

## Context and Problem Statement

Captures, recognized handwriting and links need durable provenance without a parallel note store. Rotation changes geometry that older clients cannot preserve, while shape editing can reuse the existing stroke representation.

## Decision Drivers

Preserve handwriting and undo semantics, reuse asset synchronization, keep model uploads explicit, and prevent older clients from silently discarding new geometry.

## Considered Options

- Store selection features in independent sidecars and reconcile them with canvas edits.
- Reuse image/text/knowledge cards and source anchors, extending the document version only for image rotation.
- Flatten all transformed selections to images.

## Decision Outcome

Use existing cards, asset imports and source anchors for captures, recognition and reciprocal region-link endpoints. Shape handles edit recognized ink points after explicit confirmation. Image quarter-turns require document version 4, are validated by the daemon and remain opaque fields to the assistant client. Unsupported card types are visibly excluded from rotation. Ink rotates as points around the same pivot and remains editable. Geometry edits respect locked layers and form one undo step.

## Pros and Cons of the Options

Sidecars would need additional atomicity, conflict and undo rules. Flattening makes rotation easy but destroys editable handwriting and live document/video controls. Reusing cards retains existing persistence and navigation, at the cost of visible link endpoints and limiting image rotation to quarter-turns until arbitrary-angle controls and source coordinates can share a stable contract.

## Links

- [Selection editing contract](../specs/0026-selection-editing.md).
- [Independent Notein behavioral reference](../notein-input-reference.md).
- [Source-bound conversations](0024-sidebar-conversations-and-manual-pins.md).
