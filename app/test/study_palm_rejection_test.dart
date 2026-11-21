import 'dart:ui' as ui;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, close;
import 'study_pdf_column_test.dart' as pdf;

void main() {
  testWidgets('hover does not block a fresh camera gesture or a title drag', (
    t,
  ) async {
    final key = await fixture.open(
      t,
      pdf.PDFApi((sheet()..cards.add(pdf.pdf())).json()),
    );
    final d = key.currentState!.document!,
        origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    await t.sendEventToBinding(
      PointerHoverEvent(
        kind: ui.PointerDeviceKind.stylus,
        position: origin + const Offset(600, 500),
      ),
    );
    await t.dragFrom(origin + const Offset(900, 800), const Offset(60, 20));
    expect(d.camera, const Offset(60, 20));
    final before = d.cards.single.rect;
    await t.dragFrom(
      origin + d.camera + before.topLeft + const Offset(100, 18),
      const Offset(40, 20),
    );
    expect(d.cards.single.rect, before.shift(const Offset(40, 20)));
    await close(t);
  });

  for (final end in ['up', 'cancel']) {
    testWidgets(
      'stationary pen and $end protect against palms until a new touch',
      (t) async {
        final key = await fixture.open(
          t,
          pdf.PDFApi((sheet()..cards.add(pdf.pdf())).json()),
        );
        final d = key.currentState!.document!;
        final rect = d.cards.single.rect;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        // Arrow mode has no ink pointer; pen presence must still win.
        final pen = await t.startGesture(
          origin + const Offset(320, 350),
          kind: ui.PointerDeviceKind.stylus,
          pointer: 61,
        );
        await t.pump(const Duration(milliseconds: 1500));
        final palm = await t.startGesture(
          origin + const Offset(400, 400),
          pointer: 62,
        );
        await palm.moveBy(const Offset(90, 30));
        if (end == 'up') {
          await pen.up();
        } else {
          await pen.cancel();
        }
        final latePalm = await t.startGesture(
          origin + const Offset(450, 450),
          pointer: 63,
        );
        await t.pump(const Duration(milliseconds: 1100));
        await palm.moveBy(const Offset(80, 40));
        expect(d.camera, Offset.zero, reason: 'the old palm remains blocked');
        await latePalm.moveBy(const Offset(100, 20));
        await palm.up();
        await latePalm.up();
        expect(
          d.camera,
          const Offset(100, 20),
          reason: 'fresh contacts need no cooldown',
        );
        expect(d.zoom, 1);
        expect(d.cards.single.rect, rect);
        await t.dragFrom(origin + const Offset(800, 650), const Offset(60, 20));
        expect(d.camera, const Offset(160, 40));
        await close(t);
      },
    );
  }

  testWidgets(
    'palm before and during writing cannot pan or corrupt the stroke',
    (t) async {
      final key = await fixture.open(
        t,
        pdf.PDFApi((sheet()..cards.add(pdf.pdf())).json()),
      );
      await t.tap(find.byKey(const ValueKey('study-tool-pen')));
      await t.pumpAndSettle();
      final d = key.currentState!.document!;
      final rect = d.cards.single.rect;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final first = await t.startGesture(
        origin + const Offset(450, 430),
        pointer: 71,
      );
      final pen = await t.startGesture(
        origin + const Offset(300, 350),
        kind: ui.PointerDeviceKind.stylus,
        pointer: 72,
      );
      await pen.moveBy(const Offset(40, 10));
      final second = await t.startGesture(
        origin + const Offset(400, 450),
        pointer: 73,
      );
      await first.moveBy(const Offset(80, 30));
      await second.moveBy(const Offset(100, 30));
      await second.up();
      await pen.moveBy(const Offset(40, 10));
      await pen.up();
      await t.pump(const Duration(milliseconds: 1200));
      await first.moveBy(const Offset(100, 30));
      await first.up();
      expect(d.camera, Offset.zero);
      expect(d.zoom, 1);
      expect(d.cards.single.rect, rect);
      expect(d.strokes, hasLength(1));
      expect(d.strokes.single.points.first.p, const Offset(300, 350));
      expect(d.strokes.single.points.last.p.dx, greaterThan(350));
      await close(t);
    },
  );
}
