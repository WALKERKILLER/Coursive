import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../test/study_canvas_ui_test.dart' as fixture;

// Run with flutter test benchmark/dense_canvas_test.dart --reporter expanded.
// The widget driver measures input and framework work, not display latency.
StudyDocument dense() {
  final d = StudyDocument()
    ..camera = Offset.zero
    ..zoom = 1;
  for (int i = 0; i < 120; i++) {
    final x = 2400.0 * (i % 12), y = 1800.0 * (i ~/ 12);
    final type = i == 0 ? 'lecture' : const ['pdf', 'image', 'text'][i % 3];
    d.cards.add(
      StudyCard(
        id: 'c$i',
        type: type,
        layer: 'main',
        rect: Rect.fromLTWH(x + 50, y + 80, 400, 260),
        source: type == 'text' ? 'a.md' : '',
        text: 'Study notes',
        page: 1,
        pageCount: 4,
      ),
    );
  }
  for (int i = 0; i < 10000; i++) {
    final k = i ~/ 84,
        j = i % 84,
        x = 2400.0 * (k % 12) + 60 + j % 8 * 40,
        y = 1800.0 * (k ~/ 12) + 140 + j ~/ 8 * 12;
    d.strokes.add(
      StudyStroke(
        id: 's$i',
        layer: 'main',
        anchor: i % 2 == 0 ? 'c$k' : '',
        width: 2,
        color: 0xff000000,
        points: [
          for (int n = 0; n < 16; n++)
            InkPoint(Offset(x + n, y + ((n % 3) * .6)), .55),
        ],
      ),
    );
  }
  return d;
}

void expectWithinBudget(String name, List<int> samples, Duration budget) {
  samples.sort();
  final p50 = samples[samples.length ~/ 2];
  final p95 = samples[(samples.length * .95).ceil() - 1];
  debugPrint(
    'PERF $name p50_us=$p50 p95_us=$p95 max_us=${samples.last} '
    'budget_us=${budget.inMicroseconds} n=${samples.length}',
  );
  expect(
    p95,
    lessThanOrEqualTo(budget.inMicroseconds),
    reason: '$name p95 exceeded its widget benchmark budget in microseconds',
  );
}

void main() {
  test('document eraser mixed media', () {
    final d = dense();
    for (final s in d.strokes) {
      s.bounds;
    }
    for (final name in ['miss', 'hit']) {
      final t = <int>[];
      for (int i = 0; i < 60; i++) {
        final x = name == 'miss' ? 1400.0 : 65.0,
            y = name == 'miss' ? 1400.0 : 140.0;
        if (name == 'hit') d.checkpoint();
        final w = Stopwatch()..start();
        final hit = d.erase(Offset(x, y), Offset(x + 1, y + 1), 2);
        w.stop();
        expect(hit, name == 'hit');
        if (name == 'hit') d.undo();
        if (i >= 10) t.add(w.elapsedMicroseconds);
      }
      expectWithinBudget(
        name,
        t,
        Duration(milliseconds: name == 'miss' ? 2 : 6),
      );
    }
  });
  testWidgets('mixed media draw / camera', (t) async {
    final key = await fixture.open(t, fixture.FixtureApi(dense().json()));
    await t.tap(find.byKey(const ValueKey('study-tool-pen')));
    await t.pumpAndSettle();
    const writes = 37;
    final document = key.currentState!.document!;
    final strokesBefore = document.strokes.length;
    final written = <int>[];
    for (int i = 0; i < writes; i++) {
      final w = Stopwatch()..start();
      final g = await t.startGesture(
        Offset(700, 440.0 + i * 2),
        kind: ui.PointerDeviceKind.stylus,
      );
      await g.moveBy(const Offset(36, 2));
      await g.up();
      await t.pump();
      w.stop();
      if (i >= 5) written.add(w.elapsedMicroseconds);
    }
    expect(document.strokes.length, strokesBefore + writes);
    expectWithinBudget('write', written, const Duration(milliseconds: 45));
    await t.tap(find.byKey(const ValueKey('study-tool-select')));
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    await t.tapAt(origin + const Offset(145, 96));
    await t.pumpAndSettle();
    final frame = find.byKey(const ValueKey('study-rotation-handle'));
    expect(frame, findsOneWidget);
    final gripBefore = t.getCenter(frame);
    await t.sendKeyDownEvent(LogicalKeyboardKey.space);
    final pan = await t.startGesture(
      const Offset(730, 530),
      kind: ui.PointerDeviceKind.mouse,
    );
    final pans = <int>[];
    for (int i = 0; i < 32; i++) {
      final w = Stopwatch()..start();
      await pan.moveBy(const Offset(5, 3));
      await t.pump();
      w.stop();
      if (i >= 5) pans.add(w.elapsedMicroseconds);
    }
    await pan.up();
    await t.sendKeyUpEvent(LogicalKeyboardKey.space);
    await t.pump();
    expect(t.getCenter(frame), gripBefore + const Offset(160, 96));
    expectWithinBudget(
      'pan with card selected',
      pans,
      const Duration(milliseconds: 45),
    );

    await t.tap(find.byKey(const ValueKey('study-tool-pan')));
    await t.pumpAndSettle();
    final a = await t.startGesture(const Offset(450, 470), pointer: 31),
        b = await t.startGesture(const Offset(650, 470), pointer: 32);
    final zooms = <int>[];
    for (int i = 0; i < 32; i++) {
      final w = Stopwatch()..start();
      await a.moveBy(const Offset(-2, 0));
      await b.moveBy(const Offset(2, 0));
      await t.pump();
      w.stop();
      if (i >= 5) zooms.add(w.elapsedMicroseconds);
    }
    await a.up();
    await b.up();
    expect(key.currentState!.document!.zoom, greaterThan(1));
    expectWithinBudget('zoom', zooms, const Duration(milliseconds: 35));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
}
