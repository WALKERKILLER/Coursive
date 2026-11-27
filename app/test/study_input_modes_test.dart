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
  testWidgets(
    'duplicated PDF handwriting stays on its source page after reopening',
    (t) async {
      final d = sheet()..cards.add(pdf.pdf());
      d.strokes.add(
        StudyStroke.fromJson({
          ...stroke('page-ink').json(),
          'anchor': 'pdf',
          'page': 2,
        }),
      );
      final api = pdf.PDFApi(d.json());
      final key = await fixture.open(t, api);
      await selectInk(t);
      await t.tap(find.byKey(const ValueKey('study-selection-actions')));
      await t.pumpAndSettle();
      await t.tap(find.text('创建副本'));
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
    },
  );

  // Selected content remains writable; movement belongs to the title.
  for (final type in ['text', 'note', 'image', 'pdfdoc']) {
    testWidgets(
      '$type title moves, corners resize, selected pen still writes',
      (t) async {
        final d = sheet()
          ..cards.add(
            StudyCard.fromJson({...pdf.pdf().json(), 'type': type})
              ..rect = const Rect.fromLTWH(300, 300, 400, 300),
          );
        final key = await fixture.open(t, pdf.PDFApi(d.json()));
        final actual = key.currentState!.document!;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final before = actual.cards.single.rect;
        await t.dragFrom(
          origin + before.topLeft + const Offset(100, 18),
          const Offset(60, 30),
        );
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, before.shift(const Offset(60, 30)));
        expect(actual.camera, Offset.zero);
        final opposite = actual.cards.single.rect.topLeft;
        final grip = find.byKey(const ValueKey('study-resize-handle-2'));
        expect(t.getSize(grip), const Size(44, 44));
        await t.drag(grip, const Offset(80, 36));
        await t.pumpAndSettle();
        expect(actual.cards.single.contentScale, 1);
        expect(actual.cards.single.rect.size, const Size(480, 336));
        expect(
          (actual.cards.single.rect.topLeft - opposite).distance,
          lessThan(.001),
        );
        // Finger body never moves a reader or its attached ink.
        final scaled = actual.cards.single.rect;
        if (type == 'text' || type == 'note' || type == 'pdfdoc') {
          await t.dragFrom(origin + scaled.center, const Offset(0, -40));
          expect(actual.cards.single.rect, scaled);
        }
        await t.dragFrom(
          origin + scaled.center,
          const Offset(50, 20),
          kind: ui.PointerDeviceKind.stylus,
        );
        await t.pumpAndSettle();
        expect(actual.cards.single.rect, scaled);
        expect(actual.strokes, isNotEmpty);
        await key.currentState!.flush();
        expect(
          StudyDocument.fromJson(key.currentState!.document!.json())
              .cards
              .single
              .rect,
          scaled,
        );
        await close(t);
      },
    );
  }
  testWidgets(
    'cancelled corner preview restores geometry and publishes no undo',
    (t) async {
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(pdf.pdf())).json()),
      );
      final d = key.currentState!.document!;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      await t.tapAt(
        origin + d.cards.single.rect.topLeft + const Offset(100, 18),
      );
      await t.pumpAndSettle();
      final before = jsonEncode(d.json()), undo = d.canUndo;
      final g = await t.startGesture(
        t.getCenter(find.byKey(const ValueKey('study-resize-handle-2'))),
      );
      await g.moveBy(const Offset(60, 40));
      await t.pump();
      await g.cancel();
      await t.pumpAndSettle();
      expect(jsonEncode(d.json()), before);
      expect(d.canUndo, undo);
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
