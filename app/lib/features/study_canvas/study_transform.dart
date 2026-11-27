import 'dart:math' as math;
import 'dart:ui';

import 'study_document.dart';

double studyAngle(double radians) =>
    math.atan2(math.sin(radians), math.cos(radians));

/// Hysteresis is measured against the unsnapped pointer angle, never the preview.
class StudyAngleSnap {
  double? locked;
  double update(double angle, {bool enabled = true}) {
    if (!enabled) {
      locked = null;
      return angle;
    }
    if (locked != null && (angle - locked!).abs() <= 7 * math.pi / 180)
      return locked!;
    final nearest = (angle / (math.pi / 4)).round() * math.pi / 4;
    locked = (angle - nearest).abs() <= 4 * math.pi / 180 ? nearest : null;
    return locked ?? angle;
  }
}

/// Shared geometry for rendering, title hit testing and rotated corner grips.
class StudyCardTransform {
  StudyCardTransform(this.card);
  final StudyCard card;
  double get angle => card.rotation + card.quarterTurns * math.pi / 2;
  Size get layoutSize => card.contentRect.size / card.contentScale;
  Offset toWorld(Offset local) {
    final p =
        local * card.contentScale - card.contentRect.size.center(Offset.zero);
    return card.rect.center +
        Offset(
          p.dx * math.cos(angle) - p.dy * math.sin(angle),
          p.dx * math.sin(angle) + p.dy * math.cos(angle),
        );
  }

  Offset toLocal(Offset world) {
    final p = world - card.rect.center;
    return (Offset(
              p.dx * math.cos(angle) + p.dy * math.sin(angle),
              -p.dx * math.sin(angle) + p.dy * math.cos(angle),
            ) +
            card.contentRect.size.center(Offset.zero)) /
        card.contentScale;
  }

  List<Offset> get corners => [
    toWorld(Offset.zero),
    toWorld(Offset(layoutSize.width, 0)),
    toWorld(Offset(layoutSize.width, layoutSize.height)),
    toWorld(Offset(0, layoutSize.height)),
  ];

  /// Changes the local height while leaving the top edge in place.
  Rect withLayoutHeight(double height) {
    final worldHeight = math.max(24.0, height * card.contentScale);
    return Rect.fromCenter(
      center: toWorld(
        Offset(layoutSize.width / 2, worldHeight / card.contentScale / 2),
      ),
      width: card.quarterTurns.isEven ? card.rect.width : worldHeight,
      height: card.quarterTurns.isEven ? worldHeight : card.rect.height,
    );
  }
}

/// Selected geometry is previewed from one snapshot. No edit or history entry
/// is published until commit; PDF-native annotations are not part of this data.
class StudyTransformSession {
  StudyTransformSession(
    this.document,
    Set<String> cards,
    Set<String> ink, {
    required this.pivot,
  }) : singleCard = cards.length == 1 && ink.isEmpty,
       originals = [
         for (final c in document.cards)
           if (cards.contains(c.id)) c.copy(),
       ],
       originalInk = [
         for (final s in document.strokes)
           if (ink.contains(s.id) || cards.contains(s.anchor)) s,
       ],
       format = document.formatVersion {
    allowed = document.canRotateSelection(cards, ink);
  }
  final bool singleCard;
  final StudyDocument document;
  final List<StudyCard> originals;
  final List<StudyStroke> originalInk;
  final Offset pivot;
  final int format;
  late final bool allowed;
  double _scale = 1, _angle = 0;
  Offset _translation = Offset.zero;
  Rect? _resizedFrame;
  bool _closed = false;
  bool get changed =>
      (_resizedFrame != null && _resizedFrame != originals.single.rect) ||
      (_scale - 1).abs() > 1e-9 ||
      studyAngle(_angle).abs() > 1e-9 ||
      _translation.distance > 1e-9;

  double limitScale(double value) {
    var lo = .05, hi = 20.0;
    for (final c in originals) {
      lo = math.max(lo, math.max(24 / c.rect.width, 24 / c.rect.height));
      lo = math.max(lo, .05 / c.contentScale);
      hi = math.min(
        hi,
        math.min(100000 / c.rect.width, 100000 / c.rect.height),
      );
      hi = math.min(hi, 20 / c.contentScale);
    }
    return value.clamp(lo, hi);
  }

  bool get resizesSingleCard => singleCard && originals.length == 1;

  void previewCorner(int corner, Offset delta) {
    if (!delta.dx.isFinite || !delta.dy.isFinite) return;
    if (!resizesSingleCard) {
      throw StateError('Free resize requires one card');
    }
    final old = originals.single;
    final t = StudyCardTransform(old);
    final local =
        (t.toLocal(old.rect.center + delta) - t.toLocal(old.rect.center)) *
        old.contentScale;
    final dx = corner == 0 || corner == 3 ? -1.0 : 1.0;
    final dy = corner < 2 ? -1.0 : 1.0;
    final size = old.contentRect.size;
    final w = (size.width + dx * local.dx)
        .clamp(
          math.max(24.0, math.min(size.width, 240 * old.contentScale)),
          100000.0,
        )
        .toDouble();
    final h = (size.height + dy * local.dy)
        .clamp(
          math.max(24.0, math.min(size.height, 120 * old.contentScale)),
          100000.0,
        )
        .toDouble();
    final dw = w - size.width, dh = h - size.height;
    Offset offset(double x, double y) => Offset(
      x * math.cos(t.angle) - y * math.sin(t.angle),
      x * math.sin(t.angle) + y * math.cos(t.angle),
    );
    final frame = Rect.fromCenter(
      center: old.rect.center + offset(dx * dw / 2, dy * dh / 2),
      width: old.quarterTurns.isEven ? w : h,
      height: old.quarterTurns.isEven ? h : w,
    );
    // Canvas ink keeps its geometry relative to the card's top-left.
    _preview(
      resizedFrame: frame,
      translation: offset(dx < 0 ? -dw : 0, dy < 0 ? -dh : 0),
    );
  }

  void preview({
    double scale = 1,
    double angle = 0,
    Offset translation = Offset.zero,
  }) => _preview(scale: scale, angle: angle, translation: translation);

  void _preview({
    double scale = 1,
    double angle = 0,
    Offset translation = Offset.zero,
    Rect? resizedFrame,
  }) {
    if (_closed ||
        !allowed ||
        !scale.isFinite ||
        !angle.isFinite ||
        !translation.dx.isFinite ||
        !translation.dy.isFinite)
      return;
    _resizedFrame = resizedFrame;
    _scale = limitScale(scale);
    _angle = angle;
    _translation = translation;
    final cs = math.cos(angle), sn = math.sin(angle);
    Offset map(Offset p) {
      final v = (p - pivot) * _scale;
      return pivot +
          Offset(v.dx * cs - v.dy * sn, v.dx * sn + v.dy * cs) +
          translation;
    }

    for (final old in originals) {
      final card = document.cards.where((c) => c.id == old.id).firstOrNull;
      if (card == null) continue;
      card.rect =
          resizedFrame ??
          Rect.fromCenter(
            center: map(old.rect.center),
            width: old.rect.width * _scale,
            height: old.rect.height * _scale,
          );
      card.contentScale = old.contentScale * _scale;
      card.rotation = studyAngle(old.rotation + angle);
      final height = card.contentRect.height;
      card.expandedH =
          resizedFrame != null &&
              old.passageLinked &&
              (height - old.contentRect.height).abs() > .5
          ? height
          : old.expandedH * _scale;
    }
    if (originalInk.isEmpty) return;
    final ink = {for (final s in originalInk) s.id: s};
    document.strokes = [
      for (final s in document.strokes)
        if (ink[s.id] case final old?)
          StudyStroke(
            id: old.id,
            layer: old.layer,
            points: [
              for (final p in old.points) InkPoint(map(p.p), p.pressure),
            ],
            color: old.color,
            width: old.width * _scale,
            tool: old.tool,
            anchor: old.anchor,
            page: old.page,
            gesture: old.gesture,
            brush: old.brush,
          )
        else
          s,
    ];
  }

  void _restore() {
    for (final old in originals) {
      final c = document.cards.where((c) => c.id == old.id).firstOrNull;
      if (c != null) {
        c.rect = old.rect;
        c.rotation = old.rotation;
        c.contentScale = old.contentScale;
        c.expandedH = old.expandedH;
      }
    }
    final ink = {for (final s in originalInk) s.id: s};
    document.strokes = [for (final s in document.strokes) ink[s.id] ?? s];
    document.formatVersion = format;
  }

  bool commit() {
    if (_closed) return false;
    _restore();
    if (allowed && changed) {
      document.checkpoint();
      _preview(
        scale: _scale,
        angle: _angle,
        translation: _translation,
        resizedFrame: _resizedFrame,
      );
      if (originals.any((c) => (c.contentScale * _scale - 1).abs() > 1e-9))
        document.formatVersion = math.max(format, 7);
      else if (_angle != 0 && originals.isNotEmpty)
        document.formatVersion = math.max(format, 6);
      document.changed();
    }
    _closed = true;
    return allowed && changed;
  }

  void cancel() {
    if (!_closed) _restore();
    _closed = true;
  }
}
