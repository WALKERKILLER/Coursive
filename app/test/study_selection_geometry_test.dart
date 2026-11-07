import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';

StudyStroke stroke(
  String id,
  double pressure, {
  StudyBrush? brush,
  String anchor = '',
}) => StudyStroke(
  id: id,
  layer: 'main',
  anchor: anchor,
  color: 0xff111111,
  width: 20,
  brush: brush,
  points: [
    InkPoint(const Offset(10, 50), pressure),
    InkPoint(const Offset(100, 50), pressure),
  ],
);
List<Offset> region(double y) => [
  Offset(30, y),
  Offset(80, y),
  Offset(80, y + 2),
  Offset(30, y + 2),
];
void main() {
  test('legacy paged PDF source anchors retain a persistable PDF kind', () {
    final anchor = StudyAnchor.fromJson({
      'id': 'object-page',
      'object_id': 'page',
      'kind': 'pdfdoc',
      'page': 3,
    });
    expect(anchor.json()['kind'], 'pdf');
    expect(anchor.page, 3);
    expect(anchor.objectId, 'page');
  });
  test('painted outline selects pressure-dependent width, caps and dots', () {
    final d = StudyDocument()
      ..strokes = [stroke('wide', 1, brush: const StudyBrush(pressure: 1))];
    expect(d.strokesInPolygon(region(57)), {'wide'});
    d.strokes = [stroke('thin', 0, brush: const StudyBrush(pressure: 1))];
    expect(d.strokesInPolygon(region(57)), isEmpty);
    expect(d.strokesInPolygon(region(49)), {'thin'});
    d.strokes = [stroke('cap', 1)];
    expect(
      d.strokesInPolygon([
        const Offset(1, 47),
        const Offset(4, 47),
        const Offset(4, 53),
        const Offset(1, 53),
      ]),
      {'cap'},
    );
  });
  test('mixed image and attached ink rotate once, round trip and undo', () {
    final d = StudyDocument()
      ..cards = [
        StudyCard(
          id: 'image',
          type: 'image',
          layer: 'main',
          rect: const Rect.fromLTWH(10, 20, 100, 200),
        ),
      ]
      ..strokes = [stroke('ink', 1, anchor: 'image')];
    final original = d.strokes.single.points.first.p;
    d.rotateSelection({'image'}, {'ink'}, Offset.zero);
    expect(d.cards.single.rect, const Rect.fromLTWH(-220, 10, 200, 100));
    expect(d.cards.single.quarterTurns, 1);
    expect(d.strokes.single.points.first.p, const Offset(-50, 10));
    final restored = StudyDocument.fromJson(d.json());
    expect(restored.cards.single.quarterTurns, 1);
    expect(restored.json()['version'], 4);
    d.undo();
    expect(d.strokes.single.points.first.p, original);
    expect(d.cards.single.quarterTurns, 0);
    d.redo();
    expect(d.cards.single.quarterTurns, 1);
    for (var i = 0; i < 3; i++) {
      d.rotateSelection({'image'}, {'ink'}, Offset.zero);
    }
    expect(d.cards.single.quarterTurns, 0);
    expect(d.strokes.single.points.first.p, original);
  });
  test('unsupported or locked selection does not partially rotate', () {
    final d = StudyDocument()
      ..cards = [
        StudyCard(
          id: 'video',
          type: 'video',
          layer: 'main',
          rect: const Rect.fromLTWH(0, 0, 200, 150),
        ),
      ]
      ..strokes = [stroke('ink', 1)];
    expect(d.canRotateSelection({'video'}, {'ink'}), false);
    d.rotateSelection({'video'}, {'ink'}, Offset.zero);
    expect(d.edit, 0);
    expect(d.canUndo, false);
    d.cards = [
      StudyCard(
        id: 'image',
        type: 'image',
        layer: 'main',
        rect: const Rect.fromLTWH(0, 0, 200, 150),
      ),
    ];
    d.layers.first.locked = true;
    expect(d.canRotateSelection({'image'}, {'ink'}), false);
  });
}
