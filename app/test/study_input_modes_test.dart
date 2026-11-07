import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, stroke, close, selectInk;
import 'study_pdf_column_test.dart' as pdf;

void main() {
  testWidgets('duplicated PDF handwriting stays on its source page after reopening', (t) async {
    final d = sheet()..cards.add(pdf.pdf());
    d.strokes.add(StudyStroke.fromJson({...stroke('page-ink').json(), 'anchor': 'pdf', 'page': 2}));
    final api = pdf.PDFApi(d.json());
    final key = await fixture.open(t, api);
    await selectInk(t);
    await t.tap(find.text('副本'));
    await t.pumpAndSettle();
    final current = key.currentState!.document!;
    expect(current.strokes, hasLength(2));
    expect(current.strokes.last.page, 2);
    await key.currentState!.flush();
    final reopened = StudyDocument.fromJson(api.doc);
    reopened.cards.single.page = 1;
    expect(reopened.strokes.where(reopened.strokeVisible), isEmpty);
    reopened.cards.single.page = 2;
    expect(reopened.strokes.where(reopened.strokeVisible), hasLength(2));
    await close(t);
  });

  for (final type in ['note', 'text', 'pdfdoc', 'image']) {
    testWidgets('$type stays movable after hold release with pen hovering', (
      t,
    ) async {
      final card = StudyCard.fromJson({...pdf.pdf().json(), 'type': type})
        ..rect = const Rect.fromLTWH(300, 300, 400, 300);
      final key = await fixture.open(
        t,
        pdf.PDFApi((sheet()..cards.add(card)).json()),
      );
      final d = key.currentState!.document!;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final before = d.cards.single.rect;
      final hold = await t.startGesture(
        origin + before.topLeft + const Offset(80, 18),
      );
      await t.pump(const Duration(milliseconds: 610));
      await hold.up();
      await t.pump();
      await t.sendEventToBinding(
        PointerHoverEvent(
          kind: ui.PointerDeviceKind.stylus,
          position: origin + const Offset(800, 600),
        ),
      );
      await t.dragFrom(origin + before.center, const Offset(60, 30));
      await t.pumpAndSettle();
      expect(d.cards.single.rect, before.shift(const Offset(60, 30)));
      expect(d.camera, Offset.zero);
      d.undo();
      expect(d.cards.single.rect, before);
      await close(t);
    });
  }

  testWidgets('finger holds subtitle body and moves it with its ink', (
    t,
  ) async {
    final card = StudyCard(
      id: 'subtitle',
      type: 'note',
      layer: 'main',
      rect: const Rect.fromLTWH(300, 300, 400, 300),
      title: '字幕',
      text: '课堂字幕',
    );
    final d = sheet()
      ..cards.add(
        StudyCard.fromJson({
          ...card.json(),
          'blocks': [
            {'type': 'paragraph', 'text': '课堂字幕内容'},
          ],
        }),
      );
    d.strokes.add(pdf.ink('subtitle', 0, const Offset(400, 400)));
    final key = await fixture.open(t, fixture.FixtureApi(d.json()));
    final actual = key.currentState!.document!;
    final before = actual.cards.single.rect;
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    final g = await t.startGesture(origin + before.center);
    await t.pump(const Duration(milliseconds: 610));
    await g.moveBy(const Offset(60, 30));
    await g.up();
    await t.pumpAndSettle();
    expect(actual.cards.single.rect, before.shift(const Offset(60, 30)));
    expect(actual.camera, Offset.zero);
    expect(actual.strokes.single.points.single.p, const Offset(460, 430));
    actual.undo();
    expect(actual.cards.single.rect, before);
    await close(t);
  });

  testWidgets(
    'hold static material selects and drags without panning or losing ink',
    (t) async {
      for (final kind in [
        ui.PointerDeviceKind.touch,
        ui.PointerDeviceKind.mouse,
      ]) {
        final d = sheet()..cards.add(pdf.pdf());
        d.strokes.add(pdf.ink('pdf', 2, const Offset(300, 350)));
        final api = pdf.PDFApi(d.json());
        final key = await fixture.open(t, api);
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final g = await t.startGesture(
          origin + const Offset(300, 350),
          kind: kind,
        );
        await t.pump(const Duration(milliseconds: 500));
        await g.moveBy(const Offset(2, 2));
        await t.pump(const Duration(milliseconds: 110));
        expect(actual.camera, Offset.zero);
        await g.moveBy(const Offset(80, 40));
        await g.up();
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(80, 40)));
        expect(actual.camera, Offset.zero);
        expect(actual.strokes.single.points.single.p, const Offset(380, 390));
        actual.undo();
        expect(actual.cards.single.rect, before);
        expect(actual.strokes.single.points.single.p, const Offset(300, 350));
        await close(t);
      }
    },
  );

  testWidgets('finger holds video and PDF bodies with hand or pen selected', (
    t,
  ) async {
    for (final type in ['lecture', 'pdfdoc']) {
      for (final tool in ['hand', 'pen']) {
        final d = sheet()
          ..cards.add(StudyCard.fromJson({...pdf.pdf().json(), 'type': type}));
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        await t.tap(find.byKey(ValueKey('study-tool-$tool')));
        await t.pumpAndSettle();
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect, camera = actual.camera;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final g = await t.startGesture(origin + before.center);
        await t.pump(const Duration(milliseconds: 610));
        await g.moveBy(const Offset(60, 40));
        await g.up();
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(60, 40)));
        expect(actual.camera, camera);
        expect(actual.strokes, isEmpty);
        actual.undo();
        expect(actual.cards.single.rect, before);
        await close(t);
      }
    }
  });

  testWidgets('moving early or adding a second finger cancels material hold', (
    t,
  ) async {
    final d = sheet()..cards.add(pdf.pdf());
    final key = await fixture.open(t, pdf.PDFApi(d.json()));
    final actual = key.currentState!.document!;
    final before = actual.cards.single.rect;
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    var g = await t.startGesture(origin + const Offset(300, 350));
    await t.pump(const Duration(milliseconds: 500));
    await g.moveBy(const Offset(50, 30));
    await t.pump(const Duration(milliseconds: 900));
    await g.moveBy(const Offset(50, 30));
    await g.up();
    await t.pumpAndSettle();
    expect(actual.camera, const Offset(100, 60));
    expect(actual.cards.single.rect, before);
    g = await t.startGesture(
      origin + actual.camera + const Offset(250, 350),
      pointer: 21,
    );
    final second = await t.startGesture(
      origin + actual.camera + const Offset(400, 350),
      pointer: 22,
    );
    await t.pump(const Duration(milliseconds: 900));
    await second.moveBy(const Offset(70, 0));
    await t.pump();
    expect(actual.zoom, greaterThan(1));
    expect(actual.cards.single.rect, before);
    await g.up();
    await second.up();
    await close(t);
  });
  testWidgets('material title grips move cards at normal and reduced zoom', (
    t,
  ) async {
    for (final type in ['lecture', 'pdfdoc']) {
      for (final zoom in [1.0, .42]) {
        final d = sheet()..zoom = zoom;
        final card = StudyCard.fromJson({...pdf.pdf().json(), 'type': type});
        card.rect = const Rect.fromLTWH(500, 500, 600, 472);
        d.cards.add(card);
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect, camera = actual.camera;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final grip =
            origin + camera + Offset(before.right - 16, before.top + 18) * zoom;
        final g = await t.startGesture(
          grip,
          kind: zoom == 1
              ? ui.PointerDeviceKind.touch
              : ui.PointerDeviceKind.mouse,
        );
        if (zoom == 1) await t.pump(const Duration(milliseconds: 610));
        await g.moveBy(const Offset(60, 30));
        await g.up();
        await t.pumpAndSettle();
        expect(
          actual.cards.single.rect.left,
          closeTo(before.left + 60 / zoom, .01),
          reason: '$type at $zoom',
        );
        expect(
          actual.cards.single.rect.top,
          closeTo(before.top + 30 / zoom, .01),
        );
        expect(actual.camera, camera);
        actual.undo();
        expect(actual.cards.single.rect, before);
        await close(t);
      }
    }
  });
  testWidgets(
    'a finger pans from the title and only a stationary hold moves the card',
    (t) async {
      final d = sheet()..cards.add(pdf.pdf());
      final key = await fixture.open(t, pdf.PDFApi(d.json()));
      final actual = key.currentState!.document!;
      final before = actual.cards.single.rect;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final grip = origin + before.topLeft + const Offset(80, 18);
      await t.dragFrom(grip, const Offset(60, 30));
      await t.pumpAndSettle();
      expect(actual.camera, const Offset(60, 30));
      expect(actual.cards.single.rect, before);
      await close(t);
    },
  );

  for (final type in ['lecture', 'pdfdoc', 'image', 'note', 'text']) {
    testWidgets(
      'stylus short movement writes and enclosing selection moves $type',
      (t) async {
        final card = StudyCard.fromJson({...pdf.pdf().json(), 'type': type})
          ..rect = const Rect.fromLTWH(300, 300, 400, 300);
        final d = sheet()..cards.add(card);
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        var g = await t.startGesture(
          origin + before.topLeft + const Offset(80, 18),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pump(const Duration(milliseconds: 100));
        await g.moveBy(const Offset(50, 30));
        await g.up();
        await t.pumpAndSettle();
        expect(
          actual.cards.single.rect,
          before,
          reason: 'short pen contact must not move an unselected card',
        );
        await t.tap(find.byKey(const ValueKey('study-tool-pen')));
        await t.pumpAndSettle();
        await t.dragFrom(
          origin + before.center,
          const Offset(40, 20),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before);
        expect(actual.strokes, isNotEmpty);
        final inkBefore = actual.strokes.last.points.first.p;
        await t.tap(find.byKey(const ValueKey('study-tool-region')));
        await t.pumpAndSettle();
        await t.tap(find.byTooltip('矩形框选'));
        await t.pumpAndSettle();
        g = await t.startGesture(
          origin + before.topLeft - const Offset(20, 20),
          kind: ui.PointerDeviceKind.stylus,
        );
        await g.moveTo(origin + before.bottomRight + const Offset(20, 20));
        await g.up();
        await t.pumpAndSettle();
        await t.dragFrom(
          origin + before.center,
          const Offset(60, 30),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(60, 30)));
        expect(
          actual.strokes.last.points.first.p,
          inkBefore + const Offset(60, 30),
        );
        // After the pen leaves, a finger can move the same selected region.
        await t.pump(const Duration(seconds: 2));
        await t.dragFrom(
          origin + actual.cards.single.rect.center,
          const Offset(20, 10),
        );
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(80, 40)));
        expect(actual.camera, Offset.zero);
        actual.undo();
        actual.undo();
        expect(actual.cards.single.rect, before);
        expect(actual.strokes.last.points.first.p, inkBefore);
        await close(t);
      },
    );
  }

  for (final type in ['pdfdoc', 'image', 'note', 'text']) {
    for (final tool in ['select', 'pen']) {
      testWidgets('stylus holds $type body then regrabs it in $tool mode', (
        t,
      ) async {
        final card = StudyCard.fromJson({...pdf.pdf().json(), 'type': type})
          ..rect = const Rect.fromLTWH(300, 300, 400, 300);
        final d = sheet()..cards.add(card);
        d.strokes.add(pdf.ink(card.id, 0, const Offset(400, 400)));
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        await t.tap(find.byKey(ValueKey('study-tool-$tool')));
        await t.pumpAndSettle();
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect;
        final inkBefore = actual.strokes.single.points.single.p;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        var g = await t.startGesture(
          origin + before.center,
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pump(const Duration(milliseconds: 610));
        await g.moveBy(const Offset(60, 30));
        await g.up();
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(60, 30)));
        expect(actual.camera, Offset.zero);
        expect(
          actual.strokes.length,
          1,
          reason: 'hold does not leave an ink dot',
        );
        expect(
          actual.strokes.single.points.single.p,
          inkBefore + const Offset(60, 30),
        );
        g = await t.startGesture(
          origin + actual.cards.single.rect.center,
          kind: ui.PointerDeviceKind.stylus,
        );
        await g.moveBy(const Offset(40, 20));
        await g.up();
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(100, 50)));
        expect(actual.strokes.length, 1);
        actual.undo();
        expect(actual.cards.single.rect, before.shift(const Offset(60, 30)));
        actual.undo();
        expect(actual.cards.single.rect, before);
        expect(actual.strokes.single.points.single.p, inkBefore);
        await t.tap(find.byKey(const ValueKey('study-tool-pen')));
        await t.pumpAndSettle();
        await t.dragFrom(
          origin + before.center,
          const Offset(40, 20),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pumpAndSettle();
        expect(
          actual.cards.single.rect,
          before,
          reason: 'choosing a pen resumes writing',
        );
        expect(actual.strokes.length, greaterThan(1));
        await close(t);
      });
    }
  }

  testWidgets(
    'pen hold selection and cancellation leave no ink or undo entry',
    (t) async {
      final original = sheet()..cards.add(pdf.pdf());
      final key = await fixture.open(t, pdf.PDFApi(original.json()));
      await t.tap(find.byKey(const ValueKey('study-tool-pen')));
      await t.pumpAndSettle();
      final d = key.currentState!.document!;
      final before = jsonEncode(d.json());
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      var g = await t.startGesture(
        origin + d.cards.single.rect.center,
        kind: ui.PointerDeviceKind.stylus,
      );
      await t.pump(const Duration(milliseconds: 400));
      await g.cancel();
      await t.pumpAndSettle();
      expect(jsonEncode(d.json()), before);
      g = await t.startGesture(
        origin + d.cards.single.rect.center,
        kind: ui.PointerDeviceKind.stylus,
      );
      await t.pump(const Duration(milliseconds: 610));
      await g.up();
      await t.pumpAndSettle();
      expect(jsonEncode(d.json()), before);
      expect(d.canUndo, false);
      // A palm landing during a deliberate pen drag cannot move the camera.
      g = await t.startGesture(
        origin + d.cards.single.rect.center,
        kind: ui.PointerDeviceKind.stylus,
        pointer: 70,
      );
      final palm = await t.startGesture(
        origin + const Offset(800, 600),
        pointer: 71,
      );
      await palm.moveBy(const Offset(100, 50));
      await g.moveBy(const Offset(60, 30));
      await g.up();
      await palm.up();
      await t.pumpAndSettle();
      expect(d.camera, Offset.zero);
      expect(
        d.cards.single.rect,
        original.cards.single.rect.shift(const Offset(60, 30)),
      );
      d.undo();
      expect(jsonEncode(d.json()), before);
      await close(t);
    },
  );

  for (final mode in ['partial', 'stroke', 'lasso']) {
    for (final trigger in ['pressure', 'button', 'inverted']) {
      testWidgets('$trigger respects $mode and restores pen after lift', (
        t,
      ) async {
        SharedPreferences.setMockInitialValues({
          'study.brush-presets.v1': jsonEncode({
            'presets': [
              {'name': '钢笔'},
            ],
            'eraser_mode': mode,
          }),
        });
        final d = sheet()
          ..strokes.add(stroke('original'))
          ..cards.add(pdf.pdf());
        final cardRect = d.cards.single.rect;
        final api = fixture.FixtureApi(d.json());
        final key = await fixture.open(t, api, resetPreferences: false);
        await t.tap(find.byKey(const ValueKey('study-tool-pen')));
        await t.pumpAndSettle();
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final kind = trigger == 'inverted'
            ? ui.PointerDeviceKind.invertedStylus
            : ui.PointerDeviceKind.stylus;
        final point =
            origin +
            (mode == 'lasso' ? const Offset(275, 320) : const Offset(400, 350));
        await t.sendEventToBinding(
          PointerDownEvent(
            pointer: 91,
            kind: kind,
            position: point,
            buttons: trigger == 'button' ? kPrimaryStylusButton : 0,
            pressure: trigger == 'pressure' ? .95 : .35,
            pressureMin: 0,
            pressureMax: 1,
          ),
        );
        await t.pump(const Duration(milliseconds: 900));
        if (mode == 'lasso') {
          expect(key.currentState!.document!.strokes.single.id, 'original');
          for (final corner in [
            const Offset(530, 320),
            const Offset(530, 390),
            const Offset(275, 390),
            const Offset(275, 320),
          ]) {
            await t.sendEventToBinding(
              PointerMoveEvent(
                pointer: 91,
                kind: kind,
                position: origin + corner,
              ),
            );
            await t.pump();
          }
        }
        await t.sendEventToBinding(
          PointerUpEvent(pointer: 91, kind: kind, position: point),
        );
        await t.pumpAndSettle();
        final actual = key.currentState!.document!;
        expect(
          actual.cards.single.rect,
          cardRect,
          reason: 'eraser must not become a held card',
        );
        expect(actual.camera, Offset.zero);
        if (mode != 'partial') {
          expect(actual.strokes, isEmpty);
        } else {
          expect(actual.strokes.length, 2);
          expect(
            actual.strokes
                .expand((s) => s.points)
                .any((p) => (p.p.dx - 400).abs() < 15),
            false,
          );
        }
        actual.undo();
        expect(actual.strokes.single.id, 'original');
        await t.dragFrom(
          origin + const Offset(600, 450),
          const Offset(50, 20),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pumpAndSettle();
        expect(actual.strokes.any((s) => s.points.first.p.dx >= 600), true);
        await key.currentState!.flush();
        expect(
          StudyDocument.fromJson(api.doc).strokes.length,
          actual.strokes.length,
        );
        await close(t);
      });
    }
  }
  for (final type in ['pdfdoc', 'pdf', 'image']) {
    for (final kind in [
      ui.PointerDeviceKind.touch,
      ui.PointerDeviceKind.mouse,
    ]) {
      testWidgets('$kind drags canvas from $type without moving material', (
        t,
      ) async {
        final d = sheet();
        final card = StudyCard.fromJson({...pdf.pdf().json(), 'type': type});
        d.cards.add(card);
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        final actual = key.currentState!.document!;
        final before = actual.cards.single.rect;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        await t.dragFrom(
          origin + const Offset(320, 350),
          const Offset(100, 60),
          kind: kind,
        );
        await t.pumpAndSettle();
        expect(actual.camera, const Offset(100, 60));
        expect(actual.cards.single.rect, before);
        expect(actual.strokes, isEmpty);
        await close(t);
      });
    }
  }
  testWidgets(
    'PDF pinch, paging and header drag retain text without selection UI',
    (t) async {
      final d = sheet()..cards.add(pdf.pdf());
      final api = pdf.PDFApi(d.json());
      final key = await fixture.open(t, api);
      final actual = key.currentState!.document!;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final first = await t.startGesture(
        origin + const Offset(200, 350),
        pointer: 51,
      );
      final second = await t.startGesture(
        origin + const Offset(400, 350),
        pointer: 52,
      );
      await second.moveBy(const Offset(100, 0));
      await t.pump();
      expect(actual.zoom, closeTo(1.5, .01));
      await first.up();
      await second.up();
      await t.pumpAndSettle();
      expect(find.text('选择文字'), findsNothing);
      expect(find.byTooltip('选择文档文字'), findsNothing);
      final camera = actual.camera;
      await t.tap(find.byTooltip('下一页'));
      await t.pumpAndSettle();
      expect(actual.cards.single.page, 3);
      expect(actual.cards.single.text, 'Page 3');
      expect(actual.camera, camera);
      await key.currentState!.flush();
      expect(StudyDocument.fromJson(api.doc).cards.single.text, 'Page 3');
      final before = actual.cards.single.rect;
      final header =
          origin +
          actual.camera +
          (before.topLeft + const Offset(100, 18)) * actual.zoom;
      await t.dragFrom(
        header,
        const Offset(60, 30),
        kind: ui.PointerDeviceKind.mouse,
      );
      await t.pumpAndSettle();
      expect(actual.cards.single.rect.left, closeTo(before.left + 40, .01));
      expect(actual.camera, camera);
      await close(t);
    },
  );
}
