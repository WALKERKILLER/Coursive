import 'dart:math' as math;
import 'dart:ui';

/// Completes only the newest sample interval. Emitted points are final, so the
/// PDF writer, eraser and live ink can all use exactly the same polyline.
class InkCurve {
  Offset? _previous;
  void reset() => _previous = null;

  void append(
    Offset from,
    Offset to,
    double zoom,
    void Function(Offset point, double fraction) emit, {
    int limit = 16,
    double maxStep = double.infinity,
  }) {
    final v = to - from;
    if (v == Offset.zero || limit < 1) return;
    final u = _previous;
    _previous = v;
    var bend = Offset.zero;
    if (u != null) {
      final lengths = math.sqrt(u.distanceSquared * v.distanceSquared);
      final dot = u.dx * v.dx + u.dy * v.dy;
      // Preserve deliberate corners and reversals. Limit the bow when two
      // consecutive samples are very unevenly spaced or at a page boundary.
      if (dot > lengths * .5) {
        final turn = (u.dx * v.dy - u.dy * v.dx) / (lengths + dot);
        final normal = Offset(v.dy, -v.dx) * (.5 * turn);
        final bow = math.min(1.0, u.distance / v.distance);
        bend = normal * bow;
      }
    }
    final curved = math.sqrt(bend.distance * .5 * zoom / .24).ceil();
    final spaced = (v.distance / maxStep).ceil();
    final count = math.max(curved, spaced).clamp(1, math.min(100, limit));
    for (var i = 1; i < count; i++) {
      final t = i / count;
      emit(from + v * t + bend * (2 * t * (1 - t)), t);
    }
    emit(to, 1);
  }
}
