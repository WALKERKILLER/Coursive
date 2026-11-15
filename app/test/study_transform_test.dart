import 'dart:math' as math;
import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/features/study_canvas/study_transform.dart';
import '../lib/features/study_canvas/study_input_router.dart';

void main() {
  double rad(double n) => n * math.pi / 180;
  test(
    '45-degree attraction has 4/7 degree hysteresis and wraps through zero',
    () {
      final snap = StudyAngleSnap();
      expect(snap.update(rad(40)), closeTo(rad(40), 1e-10));
      expect(snap.update(rad(42)), closeTo(rad(45), 1e-10));
      expect(snap.update(rad(51)), closeTo(rad(45), 1e-10));
      expect(snap.update(rad(53)), closeTo(rad(53), 1e-10));
      expect(snap.update(rad(359)), closeTo(rad(360), 1e-10));
      expect(snap.update(rad(365)), closeTo(rad(360), 1e-10));
      expect(snap.update(rad(-89)), closeTo(rad(-90), 1e-10));
      expect(snap.update(rad(-89), enabled: false), closeTo(rad(-89), 1e-10));
      expect(snap.locked, isNull);
    },
  );
  StudyDocument fixture() {
    final d = StudyDocument();
    d.cards.add(
      StudyCard(
        id: 'card',
        type: 'text',
        layer: 'main',
        rect: const Rect.fromLTWH(100, 100, 400, 200),
        text: 'Line one\nLine two',
        rotation: .6,
      ),
    );
    d.strokes.add(
      StudyStroke(
        id: 'ink',
        layer: 'main',
        anchor: 'card',
        points: [
          const InkPoint(Offset(200, 150), .6),
          const InkPoint(Offset(220, 160), .8),
        ],
        color: 0xff111111,
        width: 3,
      ),
    );
    return d;
  }

  test(
    'uniform rotated scaling fixes the opposite corner and preserves layout',
    () {
      final d = fixture(), c = d.cards.single;
      final transform = StudyCardTransform(c),
          fixed = StudyCardTransform(c).corners[0];
      final logical = transform.layoutSize;
      final session = StudyTransformSession(d, {'card'}, {'ink'}, pivot: fixed);
      session.preview(scale: 2);
      expect(
        (StudyCardTransform(c).corners[0] - fixed).distance,
        lessThan(1e-8),
      );
      expect(
        (StudyCardTransform(c).layoutSize.width - logical.width).abs(),
        lessThan(1e-8),
      );
      expect(c.contentScale, 2);
      expect(d.strokes.single.width, 6);
      expect(d.edit, 0);
      expect(d.canUndo, isFalse);
      session.commit();
      expect(d.json()['version'], 7);
      expect(d.edit, 1);
      final reopen = StudyDocument.fromJson(d.json());
      expect(reopen.cards.single.contentScale, 2);
      expect(
        (StudyCardTransform(reopen.cards.single).layoutSize.width -
                logical.width)
            .abs(),
        lessThan(1e-8),
      );
      d.undo();
      expect(d.cards.single.contentScale, 1);
      expect(d.strokes.single.width, 3);
      d.redo();
      expect(d.cards.single.contentScale, 2);
    },
  );
  test(
    'many previews use original geometry and cancellation publishes no edit',
    () {
      final d = fixture();
      final original = d.cards.single.rect,
          point = d.strokes.single.points.first.p;
      final s = StudyTransformSession(d, {'card'}, {}, pivot: Offset.zero);
      for (var n = 0; n < 500; n++) s.preview(scale: 1.2, angle: .2);
      final after = d.strokes.single.points.first.p;
      s.preview(scale: 1.2, angle: .2);
      expect(d.strokes.single.points.first.p, after);
      s.cancel();
      expect(d.cards.single.rect, original);
      expect(d.strokes.single.points.first.p, point);
      expect(d.edit, 0);
      expect(d.canUndo, isFalse);
    },
  );
  test('card coordinate conversion round trips under scale and quarter/free rotation', () {
    final d = fixture();
    final c = d.cards.single
      ..contentScale = 2
      ..quarterTurns = 1;
    final t = StudyCardTransform(c), p = const Offset(23, 54);
    expect((t.toLocal(t.toWorld(p)) - p).distance, lessThan(1e-8));
  });
  test('locked attached ink prevents partial transforms', () {
    final d = fixture();
    d.layers.add(StudyLayer('locked', 'Locked', locked: true));
    d.strokes.add(
      StudyStroke(
        id: 'locked-ink',
        layer: 'locked',
        anchor: 'card',
        points: [const InkPoint(Offset(200, 150), 1)],
        color: 0xff111111,
        width: 2,
      ),
    );
    final before = d.cards.single.rect;
    final s = StudyTransformSession(d, {'card'}, {}, pivot: Offset.zero)
      ..preview(scale: 2);
    expect(s.commit(), isFalse);
    expect(d.cards.single.rect, before);
  });
  test('legacy layouts default to unit content scale', () {
    final d = fixture();
    final json = d.json();
    (json['objects'] as List).first.remove('content_scale');
    final read = StudyDocument.fromJson(json);
    expect(read.cards.single.contentScale, 1);
  });
  test(
    'controls precede palm rejection; pen content never becomes card movement',
    () {
      StudyContactOwner route(
        PointerDeviceKind k, {
        bool controls = false,
        bool blocked = false,
        bool header = false,
        bool reading = false,
        bool paper = false,
        bool lasso = false,
      }) => studyContactOwner(
        kind: k,
        controls: controls,
        blocked: blocked,
        header: header,
        reading: reading,
        paper: paper,
        inkTool: false,
        lasso: lasso,
      );
      expect(
        route(PointerDeviceKind.touch, controls: true, blocked: true),
        StudyContactOwner.controls,
      );
      expect(
        route(PointerDeviceKind.touch, blocked: true),
        StudyContactOwner.blocked,
      );
      expect(
        route(PointerDeviceKind.stylus, reading: true),
        StudyContactOwner.ink,
      );
      expect(
        route(PointerDeviceKind.stylus, paper: true),
        StudyContactOwner.paper,
      );
      expect(
        route(PointerDeviceKind.touch, header: true),
        StudyContactOwner.card,
      );
      expect(
        route(PointerDeviceKind.touch, reading: true),
        StudyContactOwner.reading,
      );
      expect(route(PointerDeviceKind.touch), StudyContactOwner.camera);
      expect(
        route(PointerDeviceKind.stylus, lasso: true),
        StudyContactOwner.selection,
      );
    },
  );
}
