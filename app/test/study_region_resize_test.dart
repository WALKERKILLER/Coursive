import 'dart:convert';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'study_canvas_ui_test.dart' as fixture;
import 'study_quick_question_test.dart' show SelectionApi, document;
import 'study_selection_tools_test.dart' show settleAsync;
import 'study_handwriting_test.dart' show stroke;

Future<void> drag(
  WidgetTester t,
  Offset a,
  Offset b, {
  bool cancel = false,
  PointerDeviceKind kind = PointerDeviceKind.stylus,
}) async {
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(origin + a, kind: kind);
  await g.moveTo(origin + b);
  await t.pump();
  if (cancel) {
    await g.cancel();
  } else {
    await g.up();
  }
  await t.pumpAndSettle();
}

void main() {
  for (final kind in [
    PointerDeviceKind.mouse,
    PointerDeviceKind.touch,
    PointerDeviceKind.stylus,
  ]) {
    testWidgets(
      '$kind adjusts all four edges without moving or scaling content',
      (t) async {
        final api = SelectionApi(
          (document()..strokes = [stroke('ink')]).json(),
        );
        final key = await fixture.open(t, api);
        final ink = jsonEncode(key.currentState!.document!.json()['strokes']);
        final original = jsonEncode(
          key.currentState!.document!.json()['cards'],
        );
        await t.tap(find.byKey(const ValueKey('study-tool-region')));
        await t.pumpAndSettle();
        await drag(t, const Offset(180, 200), const Offset(400, 440));
        await t.pump(
          const Duration(seconds: 1),
        ); // Palm rejection after pen lift.
        await drag(
          t,
          const Offset(180, 300),
          const Offset(160, 330),
          kind: kind,
        );
        await drag(
          t,
          const Offset(280, 200),
          const Offset(300, 180),
          kind: kind,
        );
        await drag(
          t,
          const Offset(400, 300),
          const Offset(420, 320),
          kind: kind,
        );
        await drag(
          t,
          const Offset(280, 440),
          const Offset(310, 460),
          kind: kind,
        );
        expect(
          jsonEncode(key.currentState!.document!.json()['cards']),
          original,
        );
        expect(jsonEncode(key.currentState!.document!.json()['strokes']), ink);
        await t.tap(find.byKey(const ValueKey('study-snapshot-attach')));
        await settleAsync(t);
        expect(api.metadata.single['bounds'], [160.0, 180.0, 260.0, 280.0]);
        await t.pumpWidget(const SizedBox());
        await t.pumpAndSettle();
      },
    );
  }
  testWidgets(
    'full viewport selection uses a separate footer and captures its entire bounds',
    (t) async {
      final api = SelectionApi(document().json());
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-tool-region')));
      await t.pumpAndSettle();
      // Cover the top gap too: with narrower fonts a menu can legitimately
      // fit there. Start below the fixed controls so the canvas owns the drag.
      await drag(t, const Offset(20, 1080), const Offset(1580, 20));
      final input = t.getRect(find.byKey(const ValueKey('study-input')));
      // No room beside the selection: the bar waits below the canvas.
      final button = find.byKey(const ValueKey('study-snapshot-attach'));
      expect(t.getRect(button).top, greaterThanOrEqualTo(input.bottom));
      await t.tap(button);
      await settleAsync(t);
      expect(api.metadata.single['bounds'], [20.0, 20.0, 1560.0, 1060.0]);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets(
    'cancelled edge drag restores crop and near-top menu stays outside it',
    (t) async {
      final api = SelectionApi(document().json());
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-tool-region')));
      await t.pumpAndSettle();
      const selection = Rect.fromLTRB(180, 100, 700, 440);
      await drag(t, selection.topLeft, selection.bottomRight);
      final button = find.byKey(const ValueKey('study-snapshot-attach'));
      final material = find.byKey(const ValueKey('study-selection-bar'));
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      expect(t.getRect(material).overlaps(selection.shift(origin)), isFalse);
      await drag(
        t,
        selection.centerRight,
        const Offset(740, 260),
        cancel: true,
      );
      await t.tap(button);
      await settleAsync(t);
      expect(api.metadata.single['bounds'], [180.0, 100.0, 520.0, 340.0]);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
