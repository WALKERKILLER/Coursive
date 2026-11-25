import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';

StudyStroke mark(int id, {String anchor = '', Offset? position}) {
  final p = position ?? Offset(id * 100, 80);
  return StudyStroke(
    id: '$id',
    layer: 'main',
    anchor: anchor,
    color: 0xff000000,
    width: 2,
    points: [InkPoint(p, .6), InkPoint(p + const Offset(20, 0), .6)],
  );
}

void main() {
  test(
    'local candidates stay ordered across block boundaries, edits and undo',
    () {
      final d = StudyDocument();
      final region = const Rect.fromLTWH(6200, 72, 230, 20);
      void verify() {
        expect(
          d.inkIn(region),
          d.strokes.where((s) => s.bounds.overlaps(region)),
        );
        expect(
          d.inkOnCard('slides'),
          d.strokes.where((s) => s.anchor == 'slides'),
        );
        expect(
          d.inkPointCount,
          d.strokes.fold<int>(0, (n, s) => n + s.points.length),
        );
      }

      for (var i = 0; i < 195; i++) {
        d.strokes.add(mark(i, anchor: i.isEven ? 'slides' : ''));
        if (i % 63 == 0) verify();
      }
      verify();
      d.strokes[64] = mark(
        222,
        anchor: 'slides',
        position: const Offset(6210, 80),
      );
      verify();
      d.strokes.insertAll(63, [mark(300), mark(301)]);
      verify();
      d.strokes.removeRange(62, 65);
      verify();
      final original = d.strokes.toList();
      d.checkpoint();
      expect(
        d.erase(const Offset(6219, 77), const Offset(6220, 83), 2),
        isTrue,
      );
      verify();
      final erased = d.strokes.toList();
      d.undo();
      verify();
      expect(d.strokes, original);
      d.redo();
      verify();
      expect(d.strokes, erased);
      final reopened = StudyDocument.fromJson(d.json());
      expect(
        reopened.inkIn(region).map((s) => s.id),
        d.inkIn(region).map((s) => s.id),
      );
    },
  );

  test(
    'local eraser keeps hidden pages and locked strokes at the same spot',
    () {
      final d = StudyDocument()
        ..cards.add(
          StudyCard(
            id: 'pdf',
            type: 'pdfdoc',
            layer: 'main',
            rect: const Rect.fromLTWH(0, 0, 400, 400),
            page: 1,
          ),
        )
        ..layers.add(StudyLayer('locked', 'locked', locked: true));
      StudyStroke dot(String id, String layer, int page) => StudyStroke(
        id: id,
        layer: layer,
        anchor: 'pdf',
        page: page,
        width: 2,
        color: 1,
        points: const [InkPoint(Offset(30, 30), .5)],
      );
      d.strokes = [
        dot('page2', 'main', 2),
        dot('lock', 'locked', 1),
        dot('visible', 'main', 1),
      ];
      const p = Offset(30, 30);
      expect(d.erase(p, p, 5), isTrue);
      expect(d.strokes.map((s) => s.id), ['page2', 'lock']);
      expect(d.eraseWhole(p, p, 5), isFalse);
      d.cards.single.page = 2;
      expect(d.eraseWhole(p, p, 5), isTrue);
      expect(d.strokes.single.id, 'lock');
    },
  );
}
