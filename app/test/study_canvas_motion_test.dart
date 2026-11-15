import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/theme/glass.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, close;
import 'study_pdf_column_test.dart' as pdf;

Finder _private(String type) =>
    find.byWidgetPredicate((w) => w.runtimeType.toString() == type);

void main() {
  testWidgets(
    'a selected text card rotates continuously with its handle and undo restores it',
    (t) async {
      final d = sheet()
        ..cards.add(
          StudyCard(
            id: 'text',
            type: 'text',
            layer: 'main',
            rect: const Rect.fromLTWH(480, 330, 360, 220),
            text: '示例',
          ),
        );
      final key = await fixture.open(t, fixture.FixtureApi(d.json()));
      await t.tap(find.byKey(const ValueKey('study-tool-select')));
      await t.pumpAndSettle();
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      await t.longPressAt(origin + const Offset(550, 346));
      await t.pumpAndSettle();
      final handle = find.byKey(const ValueKey('study-rotation-handle'));
      final at = t.getCenter(handle);
      final g = await t.startGesture(at);
      await g.moveBy(const Offset(55, 14));
      await t.pump();
      await g.moveBy(const Offset(35, 22));
      await t.pump();
      await g.up();
      await t.pumpAndSettle();
      final actual = key.currentState!.document!;
      expect(actual.cards.single.rotation, greaterThan(.3));
      actual.undo();
      expect(actual.cards.single.rotation, 0);
      await close(t);
    },
  );

  testWidgets('显示全部卡片 carries the view there and a touch stops it', (t) async {
    final d = sheet()
      ..cards.add(
        StudyCard(
          id: 'far',
          type: 'text',
          layer: 'main',
          rect: const Rect.fromLTWH(3000, 2400, 300, 200),
          text: '远处的笔记',
        ),
      );
    final key = await fixture.open(t, fixture.FixtureApi(d.json()));
    final actual = key.currentState!.document!;
    final start = actual.camera;
    await t.tap(find.byTooltip('显示全部卡片'));
    await t.pump();
    await t.pump(const Duration(milliseconds: 120));
    final mid = actual.camera;
    expect(mid, isNot(start), reason: 'the flight has begun');
    await t.pumpAndSettle();
    final end = actual.camera;
    expect(end, isNot(mid), reason: 'it eased toward the cards, not cut');

    // Again from the start; a touch on the canvas stops it where it is.
    actual
      ..camera = start
      ..zoom = 1;
    await t.tap(find.byTooltip('显示全部卡片'));
    await t.pump();
    await t.pump(const Duration(milliseconds: 120));
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    final g = await t.startGesture(origin + const Offset(700, 900));
    final stopped = actual.camera;
    await t.pump(const Duration(milliseconds: 300));
    expect(actual.camera, stopped);
    await g.up();
    await close(t);
  });

  testWidgets('a ring fills under a held card and leaves with an early move', (
    t,
  ) async {
    final d = sheet()..cards.add(pdf.pdf());
    await fixture.open(t, pdf.PDFApi(d.json()));
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    var g = await t.startGesture(origin + const Offset(300, 350));
    await t.pump(const Duration(milliseconds: 400));
    expect(_private('_HoldRing'), findsOneWidget);
    await g.moveBy(const Offset(40, 20));
    await t.pump();
    expect(_private('_HoldRing'), findsNothing);
    await g.up();
    await t.pumpAndSettle();

    // A completed hold hands over to the selection outline.
    g = await t.startGesture(origin + const Offset(300, 350));
    await t.pump(const Duration(milliseconds: 820));
    expect(_private('_HoldRing'), findsNothing);
    await g.up();
    await close(t);
  });

  testWidgets('the AI sidebar is one glass pane with tonal content', (t) async {
    final d = sheet();
    await fixture.open(t, fixture.FixtureApi(d.json()));
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pump();
    await t.pump(const Duration(milliseconds: 80));
    // It slides in rather than appearing at once.
    final opacity = t.widget<FadeTransition>(
      find
          .ancestor(
            of: find.byKey(const ValueKey('study-ai-close')),
            matching: find.byType(FadeTransition),
          )
          .first,
    );
    expect(opacity.opacity.value, inExclusiveRange(0, 1));
    await t.pumpAndSettle();
    final field = find.byType(TextField).last;
    expect(
      find.ancestor(of: field, matching: find.byType(GlassSurface)),
      findsOneWidget,
      reason: 'content inside the sidebar must not blur a second time',
    );
    await close(t);
  });
}
