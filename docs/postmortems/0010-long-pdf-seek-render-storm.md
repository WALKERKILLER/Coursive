# Long PDF seeks exhaust Android memory

Artifact-Version: 1

## Executive summary

A timestamp jump in a 220-page screenshot PDF mounted and rendered the intervening pages. Android started an independent, memory-backed PDFBox parser for each request and exhausted its 384 MiB Java heap. The missing regression was a distant seek in a long document; small-document scrolling and successful PDF generation did not exercise this path.

## Summary

The canvas PDF stayed on a loading indicator before the application crashed. Restarting into the same lecture reproduced the crash. The source PDF remained readable and unchanged.

## Timeline

2026-11-15: Device crash logs recorded repeated OutOfMemoryError failures during PDF processing. A widget regression reproduced 172 render requests when jumping to page 170. The reader supplies exact page extents; Android PDF work runs serially and PDFBox uses a disk-backed scratch buffer above 4 MiB.

## Root cause

ListView.builder deferred page layout but did not know item extents. A large scroll offset therefore built the preceding sheets to discover their heights. Each sheet immediately requested rasterization. The method-channel handler created a thread per request, and PDDocument.load used unbounded main-memory scratch buffers. With a whole document per renderer, visible-page demand was multiplied into many simultaneous full-document allocations. Tests used four-page fixtures and did not assert the work performed by a distant seek.

## Guardrails

[The PDF reader regression](../../app/test/pdf_dock_test.dart) seeks to page 170 of a 220-page document with mixed page sizes, verifies that the target is rendered, and bounds total requested pages to ten. It fails before the extent fix. [The Android channel](../../app/android/app/src/main/kotlin/io/github/walkerkiller/coursive/MainActivity.kt) serializes PDF operations, and [the parser](../../app/android/app/src/main/kotlin/io/github/walkerkiller/coursive/AndroidPdfNotes.kt) limits in-memory scratch storage. Installation acceptance uses the original document on the physical tablet and checks source-file hashes and preserved canvas content.
