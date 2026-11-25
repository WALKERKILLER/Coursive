# Canvas gestures, uniform scale and selection questions

Artifact-Version: 1
Status: Approved
Approved-By: local deployment owner, explicit implementation request on 2026-11-15

## Intent

Use the pen for writing and fingers for navigation and object manipulation on macOS and Android. All built-in cards scale uniformly with their existing internal layout and attached ink. Extend the accepted [canvas/PDF contract](0033-canvas-documents-and-pdf-ink.md) without expanding assistant permissions or changing plugin objects.

## Contract

A selection has four corner grips and one rotation grip with at least 44 logical pixels of touch area independent of camera zoom. Corner scaling keeps the opposite corner fixed, preserves aspect ratio, handles rotated cards, and transforms attached world ink once. PDF-native ink stays in page coordinates. Card `content_scale` defaults to 1 for older documents; a non-unit value requires format 7. Formats 1–6 remain readable, and source PDFs are unchanged. Bounds remain finite and within the existing document limits.

Rotation uses an absolute angle for a single card and gesture-relative angle for a group, preserving relative orientation. Multiples of 45 degrees attract within 4 degrees and release after 7 degrees. A label, guide and one touch haptic communicate a snap. Snapping is enabled by default and can be switched off; Option temporarily bypasses it on macOS. Rotation and scale preview from their starting geometry, create one undo entry only when changed, save after completion and revert on cancellation.

Controls and transform grips own their contacts before writing or navigation. Pen contact on a card title or in content writes or erases even when the card is selected; touchdown holds do not turn handwriting into a card drag. Finger title taps select and title drags move; finger content scrolls readers; blank drags pan. Two fingers navigate the camera even when they begin inside a reader. Second-finger takeover ends a valid single-finger transform and freezes reader scrolling at its current offset. Active pen writing rejects palm contacts outside controls; hover does not block navigation, and stale palm contacts remain blocked until lifted without delaying fresh gestures after pen-up. Mouse and trackpad navigation retain their shortcuts.

Opting in to an image attachment dismisses the keyboard to restore the selection viewport before capture. The captured attachment is immutable while composing. A PDF selection records its actual page and crop, so reading another page or resizing the screen cannot relabel or replace the attachment. Existing invalidated selections give a recoverable error without discarding the question. A failed model request terminates the busy state and permits retry. The canvas conversation and main assistant share a searchable, height-bounded, touch-scrollable model picker. Preference saves complete before a request starts; unavailable choices cannot silently route to a different provider.

### Model selection and sign-in

The authenticated local agent starts a single OpenAI browser sign-in with `POST /agent/openai-login`, returning an official authorization URL or a completed status. `GET /agent/openai-login` reports `idle`, `waiting`, `done` or `error`; `idle` after starting indicates the agent restarted. If the SDK cannot listen on port 1455 because it is occupied, the start request fails before returning a browser URL and polling reports `callback_busy`; exchange and authorization failures expose only their phase and a local error message. The SDK checks the callback state and handles the token exchange. The installation’s random UUID is stored in `oauth-device-id` in the private agent directory (mode 0600), outside course sync. Model writes are serialized. A failed model change blocks sending until a model is successfully selected; other preference writes do not change this state.

## Plan

Share gesture ownership and card coordinate conversion, implement snapshot-based object transforms and retain native PDF caches. Extend Flutter, Go and Node document contracts together. Supersede the touchdown-hold decision in [ADR 0040](../decisions/0040-pen-writing-and-uniform-card-transforms.md). Use generic fixtures for screenshots and recordings.

## Verification

Exercise snap hysteresis and wraparound, rotated scaling, locked groups, old/new document round trips, cancellation/undo, real widget contacts from all pointer kinds, keyboard occlusion, page-bound screenshots, model-list finger scrolling, unavailable models and failed-run recovery. On both devices run writing, reading, selecting, scaling, rotating and writing again, then verify data preservation and cross-device scale/rotation. Record profile frame timing; previews must neither save on every move nor reparse a complete PDF on each frame.

## Evidence

Regression evidence is added with the owning geometry, document, input, PDF, selection-question and model-picker tests. The [sign-in integration test](../../agent/src/chatgpt-login.test.mjs) holds port 1455 and exercises both the installed model runtime and its bundled OAuth provider with a temporary device identity. Device acceptance records remain outside the repository and contain no public course content.
