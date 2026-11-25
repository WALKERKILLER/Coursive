# Independent canvas documents

## Status
Accepted

Class: architecture

## Context and Problem Statement
A learner can start a drawing before a course has any recordings and can keep several drawings for the same course.

## Decision Drivers
Both entry points need the same document identity. Existing lecture workspaces must continue to resolve their video and sources.

## Considered Options
- Give a standalone document its own ID and optional course association.
- Create an artificial lecture for each drawing.

## Decision Outcome
Use an independent ID, immutable catalog metadata and the existing vector document encoding. The course and library views open that ID.

## Pros and Cons of the Options
Independent documents need a catalog entry and editor host. Artificial lectures would appear in schedules and would require a player even for a blank sheet.

## Links
- [Lecture study canvas](../specs/0019-lecture-study-canvas.md)
- [Notein notebook types](https://help.note-in.com/category/getting-started/standard-note-vs-infinite-canvas?lang=zh)
