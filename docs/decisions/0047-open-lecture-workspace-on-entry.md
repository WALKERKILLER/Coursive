# Open the lecture workspace on entry

## Status

Proposed
Class: feature

## Context and Problem Statement

A lecture contains recordings, notes, and courseware. Students need one place to resume work and a focused view for each material.

## Decision Drivers

Keep the lecture layout and the material reading position when moving between views. Provide a way to start a separate blank board under the same course.

## Considered Options

- Launch the recording from each lecture and open the canvas as a secondary view.
- Open the lecture canvas and expand materials in the same route.
- Open each focused material in an independent reader with its own state.

## Decision Outcome

The course and recent-lecture entries open the lecture canvas. Opening a recording uses the classic player and its lecture panel with the same playback session. Markdown opens the library document reader with contents and search. PDF opens its page and annotation reader. The underlying canvas stays mounted so returning restores the workspace. Links explicitly requesting a playback time can use the recording player. Blank canvases use the independent notebook identity and course association.

## Pros and Cons of the Options

- Recording-first entry makes straight-through video watching immediate and adds a step to resume work with other materials.
- Expanding in the canvas route preserves document state and camera while requiring coordination between the focused view and the card.
- Independent readers simplify navigation when opened outside a lecture and need an explicit handoff for shared page, scroll, and playback state.

## Links

- [Native lecture canvas](0016-native-lecture-study-canvas.md)
- [Independent canvas documents](0038-independent-canvas-documents.md)
- [Desktop file drop API](https://pub.dev/packages/desktop_drop/versions/0.8.0)
