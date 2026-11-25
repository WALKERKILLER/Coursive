import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/core/models.dart';
import '../lib/features/player/pdf_dock.dart';
import '../lib/features/study_canvas/study_canvas.dart';
import '../lib/features/study_canvas/study_document.dart';
import '../lib/widgets/coursive_markdown.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, close;

void main() {
  test('append to the existing ink path and rebuild at a different zoom', () {
    final points = [Offset.zero];
    final stroke = DockStroke(1, points);
    const size = Size(800, 600);
    final path = stroke.pathFor(size, live: true);
    for (int i = 1; i <= 4000; i++) {
      points.add(Offset(i / 4000, i / 8000));
      expect(identical(stroke.pathFor(size, live: true), path), isTrue);
    }
    expect(path.getBounds().bottomRight, const Offset(800, 300));
    expect(
      stroke.pathFor(size * 2).getBounds().bottomRight,
      const Offset(1600, 600),
    );
  });
  test(
    'seek matches the timestamp and the source lecture of the screenshot deck',
    () {
      const dir = 'courses/os/lecture1';
      const deck = '$dir/courseware/01 概述_课件截图版.pdf';
      final slides = [
        for (var i = 1; i <= 3; i++)
          Slide({'idx': i, 't_sec': i * 10, 'path': '$dir/slides/$i.png'}),
      ];
      expect(
        [
          0,
          19999,
          20000,
          99000,
        ].map((ms) => studySlidePage(deck, slides, 3, ms)),
        [1, 1, 2, 3],
      );
      expect(
        studySlidePage(
          deck.replaceAll('lecture1', 'lecture2'),
          slides,
          3,
          22000,
        ),
        isNull,
      );
      expect(studySlidePage(deck, slides, 4, 22000), isNull);
      expect(studySlidePage(deck, slides.reversed.toList(), 3, 22000), isNull);
    },
  );
  testWidgets(
    'saved AI response renders Markdown and scrolls inside its card',
    (t) async {
      final doc = sheet()
        ..cards.add(
          StudyCard(
            id: 'answer',
            type: 'text',
            layer: 'main',
            generation: 'canvas-turn:q:a',
            rect: const Rect.fromLTWH(200, 200, 500, 320),
            text: [
              '**并发**',
              '```text\nCPU\n```',
              for (var i = 0; i < 30; i++) '第 $i 段',
            ].join('\n\n'),
          ),
        );
      final key = await fixture.open(t, fixture.FixtureApi(doc.json()));
      final card = find.byKey(const PageStorageKey('study-reader/answer'));
      expect(
        find.descendant(of: card, matching: find.byType(CoursiveMarkdown)),
        findsOneWidget,
      );
      final before = key.currentState!.document!.camera;
      final sc = t.state<ScrollableState>(
        find.descendant(of: card, matching: find.byType(Scrollable)).first,
      );
      await t.drag(card, const Offset(0, -180));
      await t.pumpAndSettle();
      expect(sc.position.pixels, greaterThan(0));
      expect(key.currentState!.document!.camera, before);
      await close(t);
    },
  );
}
