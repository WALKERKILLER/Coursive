# Docked local PDF annotations

## Status
Accepted

Class: feature

## Context and Problem Statement
A learner wants to scroll lecture slides inside a study canvas card, write in the margin and carry those marks into an ordinary PDF reader.

## Decision Drivers
Scrolling stays inside the card under the pointer. Existing courseware, bookmarks and page content survive. Reopening a working copy restores its ink.

## Considered Options
- Save standard PDF ink in a sibling working copy.
- Save strokes only in the study canvas and export page images when needed.

## Decision Outcome
The macOS and Android study canvases embed a vertically scrolling reader directly in each PDF document card, with optional paged and expanded views. Card headers remain available for moving the frame. The Mac helper and the Android PDFBox implementation share the working-copy metadata and page coordinate system. On first edit each clones the document into the lecture's courseware folder, writes ink annotations with appearance streams and grows the visible right edge in display orientation. Original files remain available. Already cropped pages can be annotated without expansion. A version check and an atomic replacement protect working copies.

## Pros and Cons of the Options
Native annotations can be read independently and preserve text and outlines. Updating a large working copy takes more disk I/O. A canvas sidecar is efficient for freely moving materials but requires a separate export before another reader can see the marks.

## Links
- [Apache PDFBox annotation API](https://pdfbox.apache.org/docs/2.0.27/javadocs/org/apache/pdfbox/pdmodel/interactive/annotation/PDAnnotationMarkup.html)
- [Canvas storage](0016-native-lecture-study-canvas.md)
- [pypdf page coordinates and boxes](https://pypdf.readthedocs.io/en/stable/user/cropping-and-transforming.html)
