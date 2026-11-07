import 'dart:math' as math;
import 'dart:ui';

// Independently implemented region editing. Reference observations are indexed
// in docs/notein-input-reference.md; these handles change the crop, not objects.
enum StudySelectionHandle {
  topLeft,
  top,
  topRight,
  right,
  bottomRight,
  bottom,
  bottomLeft,
  left,
}

Map<StudySelectionHandle, Offset> studySelectionHandles(Rect r) => {
  StudySelectionHandle.topLeft: r.topLeft,
  StudySelectionHandle.top: r.topCenter,
  StudySelectionHandle.topRight: r.topRight,
  StudySelectionHandle.right: r.centerRight,
  StudySelectionHandle.bottomRight: r.bottomRight,
  StudySelectionHandle.bottom: r.bottomCenter,
  StudySelectionHandle.bottomLeft: r.bottomLeft,
  StudySelectionHandle.left: r.centerLeft,
};

StudySelectionHandle? studySelectionHandleAt(
  Rect r,
  Offset p,
  double tolerance,
) {
  // Choose the nearest visible handle, including on very small selections.
  final handles = studySelectionHandles(r).entries.toList()
    ..sort((a, b) => (a.value - p).distance.compareTo((b.value - p).distance));
  if ((handles.first.value - p).distance <= tolerance) return handles.first.key;
  if (p.dx >= r.left && p.dx <= r.right) {
    if ((p.dy - r.top).abs() <= tolerance) return StudySelectionHandle.top;
    if ((p.dy - r.bottom).abs() <= tolerance)
      return StudySelectionHandle.bottom;
  }
  if (p.dy >= r.top && p.dy <= r.bottom) {
    if ((p.dx - r.left).abs() <= tolerance) return StudySelectionHandle.left;
    if ((p.dx - r.right).abs() <= tolerance) return StudySelectionHandle.right;
  }
  return null;
}

Rect studyResizeSelection(
  Rect r,
  StudySelectionHandle h,
  Offset delta,
  double minimum,
) {
  final left = [
    StudySelectionHandle.left,
    StudySelectionHandle.topLeft,
    StudySelectionHandle.bottomLeft,
  ].contains(h);
  final right = [
    StudySelectionHandle.right,
    StudySelectionHandle.topRight,
    StudySelectionHandle.bottomRight,
  ].contains(h);
  final top = [
    StudySelectionHandle.top,
    StudySelectionHandle.topLeft,
    StudySelectionHandle.topRight,
  ].contains(h);
  final bottom = [
    StudySelectionHandle.bottom,
    StudySelectionHandle.bottomLeft,
    StudySelectionHandle.bottomRight,
  ].contains(h);
  return Rect.fromLTRB(
    left ? math.min(r.right - minimum, r.left + delta.dx) : r.left,
    top ? math.min(r.bottom - minimum, r.top + delta.dy) : r.top,
    right ? math.max(r.left + minimum, r.right + delta.dx) : r.right,
    bottom ? math.max(r.top + minimum, r.bottom + delta.dy) : r.bottom,
  );
}

/// Find actual empty space; never clamp a toolbar back onto the selected area.
/// A null result asks the host for a separate footer outside the canvas.
Rect? studySelectionMenuRect(
  Rect selection,
  Rect viewport,
  List<Rect> obstacles,
) {
  var free = [viewport];
  for (final blocked in [
    selection.inflate(12),
    ...obstacles.map((r) => r.inflate(6)),
  ]) {
    free = [for (final r in free) ..._subtract(r, blocked)];
  }
  final choices = <Rect>[];
  for (final r in free) {
    if (r.width < 144 || r.height < 48) continue;
    final width = math.min(600.0, r.width);
    final x = (selection.center.dx - width / 2).clamp(r.left, r.right - width);
    final y = (selection.top - 60).clamp(r.top, r.bottom - 48);
    choices.add(Rect.fromLTWH(x, y, width, 48));
  }
  double score(Rect r) =>
      (r.center - selection.topCenter).distance + (600 - r.width) * .15;
  choices.sort((a, b) => score(a).compareTo(score(b)));
  return choices.firstOrNull;
}

List<Rect> _subtract(Rect r, Rect blocked) {
  if (!r.overlaps(blocked)) return [r];
  final cut = r.intersect(blocked);
  return [
    Rect.fromLTRB(r.left, r.top, r.right, cut.top),
    Rect.fromLTRB(r.left, cut.bottom, r.right, r.bottom),
    Rect.fromLTRB(r.left, cut.top, cut.left, cut.bottom),
    Rect.fromLTRB(cut.right, cut.top, r.right, cut.bottom),
  ].where((r) => !r.isEmpty).toList();
}
