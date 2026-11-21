# Selection capture and model selection diverge from the displayed UI

Artifact-Version: 1

## Executive summary

A selected region could be fully visible when chosen but fail image capture after the keyboard reduced the canvas viewport. The model chooser exposed only twelve entries per provider, and asynchronous preference writes allowed the displayed choice and the model used for a request to disagree.

## Summary

The question remained visible beside a screenshot error. A provider failure appeared as a generic model error, leaving the user without a useful recovery action. These failures were independent of the PDF file's validity.

## Timeline

2026-11-15: Screenshots reproduced keyboard occlusion and a failed canvas request. Widget tests reproduced the occlusion and exercised a list with fifty-two usable models; orchestration fixtures reproduced an unavailable explicit choice and a failed preference write.

## Root cause

Capture used the resized repaint boundary. Scaffold removes consumed keyboard insets from its body's MediaQuery, so the capture path must consult the underlying Flutter view when restoring the viewport. Provider submenus truncated the model list, preference responses were not ordered, and automatic fallback could route an unavailable explicit model choice to the first usable provider. The canvas request did not carry the selected provider and model for validation.

## Guardrails

[Keyboard capture tests](../../app/test/study_quick_question_test.dart) verify the complete original bounds and retained question after image opt-in. [Picker tests](../../app/test/model_picker_scroll_test.dart) reach the last model by finger scrolling and await persistence before dismissal. [Preference tests](../../app/test/assistant_preferences_test.dart) protect ordering and failure recovery. [Canvas runner tests](../../agent/src/canvas-agent.test.mjs) reject a mismatched choice before invoking inference and map provider errors to safe recovery messages. Unknown SDK payloads are never exposed.
