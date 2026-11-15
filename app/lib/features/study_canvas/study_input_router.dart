import 'dart:ui';

enum StudyContactOwner {
  controls,
  paper,
  ink,
  reading,
  card,
  camera,
  selection,
  blocked,
}

/// Ownership is resolved once at contact-down. A second touch may explicitly
/// promote navigation to the camera; content never acquires card movement.
StudyContactOwner studyContactOwner({
  required PointerDeviceKind kind,
  required bool controls,
  required bool blocked,
  required bool header,
  required bool paper,
  required bool reading,
  required bool inkTool,
  required bool lasso,
  bool pan = false,
  bool selectionHandle = false,
}) {
  if (controls) return StudyContactOwner.controls;
  if (blocked) return StudyContactOwner.blocked;
  if (pan) return StudyContactOwner.camera;
  final pen =
      kind == PointerDeviceKind.stylus ||
      kind == PointerDeviceKind.invertedStylus;
  if (lasso && (pen || kind == PointerDeviceKind.mouse || selectionHandle))
    return StudyContactOwner.selection;
  if (header) return StudyContactOwner.card;
  if (pen) return paper ? StudyContactOwner.paper : StudyContactOwner.ink;
  if (kind == PointerDeviceKind.mouse && inkTool)
    return paper ? StudyContactOwner.paper : StudyContactOwner.ink;
  if (reading) return StudyContactOwner.reading;
  return StudyContactOwner.camera;
}
