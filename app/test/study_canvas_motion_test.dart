import 'dart:ui' show PointerDeviceKind;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/theme/glass.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, close;

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

  testWidgets('selection handles follow the card on each canvas zoom frame', (
    t,
  ) async {
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
    final input = find.byKey(const ValueKey('study-input'));
    final origin = t.getTopLeft(input);
    final card = find.byKey(const ValueKey('study-card/text'));
    await t.tapAt(origin + const Offset(550, 346));
    await t.pumpAndSettle();
      final actual = key.currentState!.document!;

    Offset handle(int corner) =>
        t.getCenter(find.byKey(ValueKey('study-resize-handle-$corner')));
    final rotate = find.byKey(const ValueKey('study-rotation-handle'));
    void expectAligned() {
      final bounds = Rect.fromPoints(
        t.getTopLeft(card),
        t.getBottomRight(card),
      );
      final corners = [
        bounds.topLeft,
        bounds.topRight,
        bounds.bottomRight,
        bounds.bottomLeft,
      ];
      for (final (index, corner) in corners.indexed) {
        expect((handle(index) - corner).distance, lessThan(.01));
      }
      expect(
        (t.getCenter(rotate) - bounds.topCenter - const Offset(0, -32))
            .distance,
        lessThan(.01),
      );
      expect(t.getSize(rotate), const Size(44, 44));
    }

    await t.tap(find.byTooltip('放大'));
    await t.pump();
      expect(actual.zoom, greaterThan(1));
    expectAligned();

    final focus = t.getCenter(card);
    final trackpad = await t.createGesture(kind: PointerDeviceKind.trackpad);
      final trackpadZoom = actual.zoom;
    await trackpad.panZoomStart(focus);
    await trackpad.panZoomUpdate(focus, scale: .8, pan: const Offset(15, 12));
    await t.pump();
      expect(actual.zoom, closeTo(trackpadZoom * .8, .001));
    expectAligned();
    await trackpad.panZoomUpdate(focus, scale: 1.25, pan: const Offset(45, 30));
    await t.pump();
      expect(actual.zoom, closeTo(trackpadZoom * 1.25, .001));
    expectAligned();
    await trackpad.panZoomEnd();

    final center = t.getCenter(card);
      final touchZoom = actual.zoom;
    final first = await t.startGesture(center - const Offset(45, 0));
    final second = await t.startGesture(center + const Offset(45, 0));
    await first.moveBy(const Offset(-18, 0));
    await t.pump();
      expect(actual.zoom, greaterThan(touchZoom));
    expectAligned();
    await second.moveBy(const Offset(32, 0));
    await t.pump();
    expectAligned();
    await first.up();
    await second.up();

      expect(actual.canUndo, isFalse);
      final original = actual.cards.single.rect;
      final zoom = actual.zoom;
      final resize = await t.startGesture(handle(2));
      await resize.moveBy(const Offset(54, 33));
      await t.pump();
      expectAligned();
      await resize.up();
      await t.pump();
      final resized = actual.cards.single.rect;
      expect(resized.width, greaterThan(original.width));
      actual.undo();
      await t.pump();
      expect(actual.cards.single.rect, original);
      expectAligned();
      actual.redo();
      await t.pump();
      expect(actual.cards.single.rect, resized);
      expect(actual.zoom, zoom);
      expectAligned();
    await close(t);
  });

  testWidgets('显示全部材料 carries the view there and a touch stops it', (t) async {
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
    await t.tap(find.byTooltip('显示全部材料'));
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
    await t.tap(find.byTooltip('显示全部材料'));
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

  testWidgets('narrow canvas offers fit in the overflow menu', (t) async {
    await fixture.open(t, fixture.FixtureApi(sheet().json()));
    t.view.physicalSize = const Size(550, 900);
    await t.pumpAndSettle();
    expect(find.byTooltip('显示全部材料'), findsNothing);
    await t.tap(find.byTooltip('画布菜单'));
    await t.pumpAndSettle();
    expect(
      find.widgetWithText(PopupMenuItem<String>, '显示全部材料'),
      findsOneWidget,
    );
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
