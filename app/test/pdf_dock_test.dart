import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/player/pdf_dock.dart';

void main() {
  test('a timed out or closed reader terminates its native renderer', () async {
    final dir = await Directory.systemTemp.createTemp('pdf-worker-test-');
    addTearDown(() => dir.delete(recursive: true));
    final source = File('${dir.path}/source.pdf');
    await source.writeAsString('fixture');
    final helper = File('${dir.path}/helper');
    await helper.writeAsString(r'''#!/bin/sh
if [ "$1" = info ]; then
  echo '{"revision":"","canExpand":true,"margin":0,"sizes":[[100,150]]}'
else
  echo "$$" > "$(dirname "$0")/pid"
  exec sleep 30
fi
''');
    expect((await Process.run('chmod', ['700', helper.path])).exitCode, 0);
    final pdf = await DockPDF.open(
      source.path,
      '${dir.path}/out.pdf',
      executable: helper.path,
      helperTimeout: const Duration(seconds: 5),
    );
    final pid = File('${dir.path}/pid');
    Future<void> exited() async {
      final id = (await pid.readAsString()).trim();
      for (int i = 0; i < 30; i++) {
        if ((await Process.run('kill', ['-0', id])).exitCode != 0) return;
        await Future<void>.delayed(const Duration(milliseconds: 20));
      }
      fail('renderer still running');
    }

    await expectLater(pdf.render(1), throwsA(isA<TimeoutException>()));
    await exited();
    await pid.delete();
    final render = expectLater(pdf.render(1), throwsStateError);
    for (int i = 0; i < 200 && !await pid.exists(); i++) {
      await Future<void>.delayed(const Duration(milliseconds: 10));
    }
    expect(await pid.exists(), isTrue);
    await pdf.close();
    await render;
    await exited();
    expect(await pdf.cache.exists(), isFalse);
  }, skip: Platform.isWindows);
  testWidgets('jumping deep into a long PDF renders only nearby pages', (
    t,
  ) async {
    final pdf = TestPDF();
    addTearDown(() => pdf.cache.delete(recursive: true));
    pdf.sizes
      ..clear()
      ..addAll(List.generate(220, (i) => Size(960, i.isEven ? 540 : 720)));
    final key = GlobalKey<PDFDockState>();
    await t.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.topRight,
            child: SizedBox(
              width: 360,
              height: 450,
              child: PDFDock(key: key, pdf: pdf),
            ),
          ),
        ),
      ),
    );
    await t.pumpAndSettle();
    expect(key.currentState!.jumpToPage(170), isTrue);
    await t.pumpAndSettle();
    expect(pdf.rendered, contains(170));
    expect(
      pdf.rendered.length,
      lessThanOrEqualTo(10),
      reason: 'A distant seek must not rasterize every intervening page',
    );
    await t.pumpWidget(const SizedBox());
  });

  testWidgets(
    'PDF pen holds fit then adjust a shape, heavy touchdown temporarily erases',
    (t) async {
      final pdf = TestPDF();
      addTearDown(() => pdf.cache.delete(recursive: true));
      await t.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PDFDock(
              pdf: pdf,
              singlePage: 1,
              shapeAssist: true,
              pressureThreshold: .8,
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      final start = t.getCenter(
        find.byKey(const ValueKey('pdf-committed-ink')),
      );
      final g = await t.startGesture(start, kind: ui.PointerDeviceKind.stylus);
      await g.moveBy(const Offset(32, 1));
      await g.moveBy(const Offset(36, -1));
      await g.moveBy(const Offset(32, 1));
      await t.pump(const Duration(milliseconds: 710));
      await g.moveBy(const Offset(20, 0));
      await g.up();
      await t.pump();
      expect(pdf.strokes.single.points.length, 2);
      await t.sendEventToBinding(
        PointerDownEvent(
          pointer: 99,
          kind: ui.PointerDeviceKind.stylus,
          position: start + const Offset(50, 0),
          pressure: .95,
          pressureMin: 0,
          pressureMax: 1,
        ),
      );
      await t.pump(const Duration(milliseconds: 390));
      await t.sendEventToBinding(
        PointerUpEvent(
          pointer: 99,
          kind: ui.PointerDeviceKind.stylus,
          position: start + const Offset(50, 0),
        ),
      );
      await t.pump(const Duration(milliseconds: 250));
      expect(pdf.strokes, isEmpty);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'partial erase previews before release and cancellation restores ink',
    (t) async {
      final pdf = TestPDF();
      addTearDown(() => pdf.cache.delete(recursive: true));
      final stroke = DockStroke(
        1,
        [const Offset(.1, .5), const Offset(.9, .5)],
        marker: true,
        square: true,
        argb: 0x59ffd34f,
        width: .04,
      );
      pdf.strokes.add(stroke);
      pdf.savedCount = 1;
      final key = GlobalKey<PDFDockState>();
      await t.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: PDFDock(
              key: key,
              pdf: pdf,
              singlePage: 1,
              erasing: true,
              eraseMode: 'partial',
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      final center = t.getCenter(find.byType(AspectRatio));
      final marks = find.byKey(const ValueKey('pdf-committed-ink'));
      Object? painted() => t.widget<CustomPaint>(marks).painter;
      final before = painted();
      final first = await t.startGesture(
        center,
        kind: ui.PointerDeviceKind.stylus,
      );
      await t.pump();
      expect(painted(), isNot(same(before)));
      expect(pdf.strokes.single, same(stroke));
      await first.cancel();
      await t.pump();
      expect(pdf.strokes.single, same(stroke));
      final second = await t.startGesture(
        center,
        kind: ui.PointerDeviceKind.stylus,
      );
      await t.pump();
      Offset? cursor() {
        final painter =
            t
                    .widget<CustomPaint>(
                      find.byKey(const ValueKey('pdf-live-ink')),
                    )
                    .painter
                as dynamic;
        return painter.cursor as Offset?;
      }

      final at = cursor()!;
      await second.moveBy(const Offset(0, 24));
      await t.pump();
      expect(cursor()!.dy, greaterThan(at.dy));
      await second.up();
      await t.pump();
      expect(cursor(), isNull);
      expect(pdf.strokes, hasLength(2));
      expect(
        pdf.strokes.every((s) => s.marker && s.square && s.argb == stroke.argb),
        isTrue,
      );
      expect(pdf.savedCount, 1);
      expect(await key.currentState!.flush(), isTrue);
      expect(pdf.savedCount, 2);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'horizontal page and eraser remove saved ink without adding a pen stroke',
    (t) async {
      final pdf = TestPDF();
      addTearDown(() => pdf.cache.delete(recursive: true));
      pdf.strokes.add(
        DockStroke(2, [const Offset(.2, .5), const Offset(.8, .5)]),
      );
      pdf.savedCount = 1;
      await t.pumpWidget(
        MaterialApp(
          home: Scaffold(body: PDFDock(pdf: pdf, horizontal: true)),
        ),
      );
      expect(find.byIcon(Icons.border_right), findsNothing);
      await t.tap(find.byTooltip('下一页'));
      await t.pumpAndSettle();
      expect(find.text('2 / 4'), findsOneWidget);
      await t.tap(find.byTooltip('橡皮擦'));
      await t.pump();
      final page = find.byType(PageView);
      final center = t.getCenter(page);
      await t.dragFrom(
        center - const Offset(0, 15),
        const Offset(0, 30),
        kind: ui.PointerDeviceKind.stylus,
      );
      await t.pumpAndSettle();
      expect(pdf.strokes, isEmpty);
      await t.pump(const Duration(milliseconds: 250));
      expect(pdf.savedCount, 0);
      pdf.strokes.add(
        DockStroke(2, [const Offset(.2, .5), const Offset(.8, .5)]),
      );
      await t.tap(find.byTooltip('橡皮擦'));
      await t.pump();
      await t.dragFrom(
        center - const Offset(0, 15),
        const Offset(0, 30),
        kind: ui.PointerDeviceKind.invertedStylus,
      );
      await t.pumpAndSettle();
      expect(pdf.strokes, isEmpty);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'finger scrolls the dock; pen writes to the later page and commits on close',
    (tester) async {
      final pdf = TestPDF();
      addTearDown(() => pdf.cache.delete(recursive: true));
      final key = GlobalKey<PDFDockState>();
      var closed = false;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.topRight,
              child: SizedBox(
                width: 360,
                height: 450,
                child: PDFDock(
                  key: key,
                  pdf: pdf,
                  onClose: () {
                    closed = true;
                  },
                ),
              ),
            ),
          ),
        ),
      );
      final list = find.byType(ListView);
      final scroll = tester
          .state<ScrollableState>(
            find.descendant(of: list, matching: find.byType(Scrollable)).first,
          )
          .position;
      await tester.drag(
        list,
        const Offset(0, -300),
        kind: ui.PointerDeviceKind.touch,
      );
      await tester.pump(const Duration(milliseconds: 600));
      expect(scroll.pixels, greaterThan(100));
      expect(pdf.strokes, isEmpty);
      final position = scroll.pixels;
      final loc = tester.getCenter(list);
      await tester.dragFrom(
        loc,
        const Offset(40, 10),
        kind: ui.PointerDeviceKind.stylus,
      );
      await tester.runAsync(() async {
        expect(await key.currentState!.flush(), true);
      });
      await tester.pump();
      expect(scroll.pixels, position);
      expect(pdf.strokes.single.page, greaterThan(1));
      expect(pdf.savedCount, 1);
      await tester.tap(find.byTooltip('关闭'));
      expect(closed, true);
      await tester.pumpWidget(const SizedBox());
    },
  );
}

class TestPDF extends Fake implements DockPDF {
  @override
  final bool canExpand = true;
  @override
  final Directory cache = Directory.systemTemp.createTempSync(
    "coursive-test-reader-",
  );
  @override
  Future<void> close() => cache.delete(recursive: true);
  @override
  final String source = 'slides.pdf', output = 'slides · 批注版.pdf';
  @override
  String revision = '';
  @override
  final sizes = List.generate(4, (_) => const Size(600, 500));
  @override
  final int baseMargin = 0;
  @override
  int margin = 0, savedMargin = 0, savedCount = 0;
  @override
  final strokes = <DockStroke>[];
  @override
  bool get dirty => savedCount != strokes.length;
  @override
  Future<void> save() async {
    savedCount = strokes.length;
    revision = 'updated';
  }

  final rendered = <int>[];
  @override
  Future<Uint8List> render(int page) async {
    rendered.add(page);
    return base64Decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAADklEQVR4nGP4DwYMEAoAU7oL9ZisIGcAAAAASUVORK5CYII=',
    );
  }
}
