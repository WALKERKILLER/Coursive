import 'dart:math' as math;
import 'dart:ui';

Rect inkBounds(List<Offset> points) {
  var bounds = Rect.fromPoints(points.first, points.first);
  for (final point in points.skip(1)) {
    bounds = bounds.expandToInclude(Rect.fromPoints(point, point));
  }
  return bounds;
}

/// One section of an eraser trail in page coordinates. Prepare it once for all
/// candidate strokes. Line and dot bounds are deliberately inclusive.
class InkEraser {
  InkEraser(List<Offset> trail, this.paper, this.radius, this.mode)
    : line = [
        for (final p in trail) Offset(p.dx * paper.width, p.dy * paper.height),
      ] {
    region = inkBounds(line).inflate(radius);
    if (mode == 'lasso') polygon = Path()..addPolygon(line, true);
  }

  final Size paper;
  final double radius;
  final String mode;
  final List<Offset> line;
  late final Rect region;
  Path? polygon;

  bool touchesBounds(Rect normalized) =>
      normalized.left * paper.width <= region.right &&
      normalized.right * paper.width >= region.left &&
      normalized.top * paper.height <= region.bottom &&
      normalized.bottom * paper.height >= region.top;

  bool _inside(Offset p) {
    if (polygon != null) return line.length >= 3 && polygon!.contains(p);
    final r2 = radius * radius;
    if ((p - line.first).distanceSquared <= r2) return true;
    for (var i = 1; i < line.length; i++) {
      final a = line[i - 1], v = line[i] - a, d = p - a;
      final t = v.distanceSquared == 0
          ? 0.0
          : ((d.dx * v.dx + d.dy * v.dy) / v.distanceSquared).clamp(0.0, 1.0);
      if ((d - v * t).distanceSquared <= r2) return true;
    }
    return false;
  }

  List<List<Offset>> clip(List<Offset> points, {Rect? bounds}) {
    if (points.isEmpty || !touchesBounds(bounds ?? inkBounds(points)))
      return [points];
    Offset px(Offset p) => Offset(p.dx * paper.width, p.dy * paper.height);
    var hit = false;
    final result = <List<Offset>>[];
    var fragment = <Offset>[];
    bool sample(Offset p, Offset pixel) {
      if (!_inside(pixel)) {
        fragment.add(p);
        return false;
      }
      hit = true;
      if (fragment.isNotEmpty) {
        result.add(fragment);
        fragment = [];
      }
      return mode != 'partial';
    }

    var previous = px(points.first);
    if (sample(points.first, previous)) return [];
    for (var i = 1; i < points.length; i++) {
      final end = px(points[i]);
      if (Rect.fromPoints(previous, end).inflate(.5).overlaps(region)) {
        final steps = ((end - previous).distance / math.max(2, radius / 2))
            .ceil()
            .clamp(1, 4096);
        for (var j = 1; j <= steps; j++) {
          final t = j / steps;
          if (sample(
            Offset.lerp(points[i - 1], points[i], t)!,
            Offset.lerp(previous, end, t)!,
          ))
            return [];
        }
      } else {
        fragment.add(points[i]);
      }
      previous = end;
    }
    if (!hit) return [points];
    if (fragment.isNotEmpty) result.add(fragment);
    return result;
  }
}

List<List<Offset>> clipInk(
  List<Offset> points,
  List<Offset> trail,
  Size paper,
  double radius,
  String mode,
) => trail.isEmpty
    ? [points]
    : InkEraser(trail, paper, radius, mode).clip(points);
