# On-device PDF export with an embedded font

## Status

Accepted
Class: feature

## Context and Problem Statement

Reviewed outlines need a paged export that keeps Chinese text, math and code readable and selectable, places page breaks sensibly and matches what the learner previewed. Coursive runs on Linux, Windows, macOS and Android with no guaranteed system PDF tool or CJK font, and a desktop print dialog differs on every platform.

## Decision Drivers

Same output on every platform, a real text layer, no network service, a permissive licence, a preview that shows the saved bytes, and a modest app size.

## Considered Options

- Print HTML through a platform web view or the system print dialog.
- Render on the daemon with an external tool such as a headless browser or Pandoc and a TeX engine.
- Build the PDF in Dart with the `pdf` package and an embedded subset Noto Sans SC, and preview it through the daemon's existing Poppler/PDFKit renderer.

## Decision Outcome

The app builds the PDF with `pdf` 3.13 (Apache-2.0) and two static instances of Noto Sans SC 2.004 (OFL 1.1) subset to GB 2312 plus common symbols, about 2.4 MB each, reproducible with `scripts/subset-export-font.py`. Layout is explicit: keep-with-next headings through `NewPage(freeSpace:)`, estimated block heights, code split into slices and math and ink rasterized on the device. The preview posts the built bytes to `POST /api/export/pdf-preview`, which renders one page with the renderer already used for study PDFs and keeps nothing ([spec 0028](../specs/0028-reviewed-outlines-and-paged-export.md)).

## Pros and Cons of the Options

Printing HTML gives CSS fragmentation rules (`break-after: avoid`) for free, but output, fonts and dialogs vary by platform and Android has no headless path; the preview would not be the saved file. A daemon tool chain gives the best typography but adds large dependencies, Pandoc is GPL-licensed and needs a TeX engine, and Android's proot workspace would need it too. Dart `pdf` is pure Dart and identical everywhere; it cannot split a non-spanning widget across pages, so layout keeps blocks small, and it has no hyphenation. The embedded font adds about 4.8 MB to the app; characters outside the subset are reported before saving.

## Links

- [Reviewed outlines and paged export](../specs/0028-reviewed-outlines-and-paged-export.md)
- [Host-issued evidence markers](0029-host-issued-evidence-markers.md)
- [dart_pdf source](https://github.com/DavBfr/dart_pdf) (`MultiPage`, `TableOfContent`), [Noto Sans SC](https://github.com/google/fonts/tree/main/ofl/notosanssc)
- Pagination and review references, read 2026-11-07: [page-break patterns](https://vocab.design/page-break), [CSS printing](https://flaviocopes.com/css-printing/), [human-in-the-loop AI editing patterns](https://ckeditor.com/blog/human-in-the-loop-ai-editing-patterns/), [partial acceptance in HITL review](https://www.orbital.co.nz/post/human-in-the-loop-ux-patterns), [HITL UX patterns](https://www.tcules.com/insights/human-in-the-loop-ux-patterns)
