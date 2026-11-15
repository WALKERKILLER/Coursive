import 'dart:io';

import 'package:flutter/material.dart';

import '../lib/features/player/pdf_dock.dart';
import 'pdf_dock_test.dart' show TestPDF;

import 'dart:ui' show PointerDeviceKind;

import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../lib/features/study_canvas/study_canvas.dart';
import '../lib/features/study_canvas/study_document.dart';
import '../lib/features/study_canvas/study_canvas_surface.dart';
import '../lib/theme/prefs.dart';
import '../lib/theme/theme.dart';
import 'study_canvas_ui_test.dart' show FixtureApi;

class _NotebookApi extends FixtureApi {
  _NotebookApi() : super(StudyDocument().json());
  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async {
    expect(path, '/api/notebooks/book/canvas');
    return doc;
  }

  @override
  Future<dynamic> put(String path, Object body) async {
    expect(path, '/api/notebooks/book/canvas');
    return doc = {
      ...body as Map<String, dynamic>,
      'revision': (doc['revision'] as int) + 1,
      'base_hash': 'saved',
    };
  }
}

void main() {
  testWidgets('PDF scroll and ink stay inside the canvas card', (t) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await UiPrefs.load(), api = _NotebookApi();
    final tmp = await t.runAsync(
      () => Directory.systemTemp.createTemp('coursive-card-test-'),
    );
    final pdf = TestPDF();
    final doc = StudyDocument()..zoom = 1;
    doc.cards.add(
      StudyCard(
        id: 'slides',
        type: 'pdfdoc',
        layer: 'main',
        title: 'Lecture.pdf',
        source: 'notebooks/book/study-assets/source.pdf',
        rect: const Rect.fromLTWH(220, 180, 580, 420),
        page: 1,
        pageCount: 4,
      ),
    );
    api.doc = doc.json();
    final canvasKey = GlobalKey<StudyCanvasState>();
    t.view.physicalSize = const Size(1400, 900);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    await t.pumpWidget(
      MaterialApp(
        theme: buildTheme(buildScheme(prefs, Brightness.light), prefs),
        home: Scaffold(
          body: StudyCanvasSurface(
            prefs: prefs,
            child: StudyCanvas.notebook(
              key: canvasKey,
              api: api,
              id: 'book',
              name: 'Canvas',
              pdfLibraryRoot: tmp!.path,
              openCanvasPDF: (src, out) async {
                expect(out, endsWith('courseware/Lecture · 批注版.pdf'));
                return pdf;
              },
              onBack: () {},
            ),
          ),
        ),
      ),
    );
    await t.pump();
    final reader = find.descendant(
      of: find.byKey(const ValueKey('study-card/slides')),
      matching: find.byType(PDFDock),
    );
    for (int i = 0; i < 20 && reader.evaluate().isEmpty; i++) {
      await t.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 20)),
      );
      await t.pump();
    }
    expect(reader, findsOneWidget);
    final list = find.descendant(of: reader, matching: find.byType(ListView));
    final before = canvasKey.currentState!.document!.camera;
    await t.drag(list, const Offset(0, -210), kind: PointerDeviceKind.touch);
    await t.pump(const Duration(seconds: 1));
    expect(canvasKey.currentState!.document!.camera, before);
    final pos = t
        .state<ScrollableState>(
          find.descendant(of: list, matching: find.byType(Scrollable)),
        )
        .position;
    expect(pos.pixels, greaterThan(100));
    expect(pdf.strokes, isEmpty);
    final marker = find.byKey(const ValueKey('study-pen-4'));
    final listener = t.renderObject(find.byKey(const ValueKey('study-input')));
    for (final kind in [PointerDeviceKind.mouse, PointerDeviceKind.stylus]) {
      final tool = kind == PointerDeviceKind.mouse
          ? find.byKey(const ValueKey('study-pen-1'))
          : marker;
      final hit = t.hitTestOnBinding(t.getCenter(tool)).path;
      expect(hit.any((entry) => identical(entry.target, listener)), isFalse);
      final pick = await t.startGesture(t.getCenter(tool), kind: kind);
      await pick.up();
      await t.pumpAndSettle();
    }
    expect(pdf.strokes, isEmpty);
    expect(canvasKey.currentState!.document!.strokes, isEmpty);
    final offset = pos.pixels;
    await t.dragFrom(
      t.getCenter(list),
      const Offset(30, 15),
      kind: PointerDeviceKind.stylus,
    );
    await t.pump();
    expect(pdf.strokes.length, 1);
    expect(pdf.savedCount, 1);
    final mark = pdf.strokes.single;
    expect(mark.marker && mark.square, isTrue);
    expect(
      mark.argb,
      const Color(0xffffd34f).withValues(alpha: .35).toARGB32(),
    );
    expect(mark.width, greaterThan(.003));
    expect(canvasKey.currentState!.document!.strokes, isEmpty);
    expect(pos.pixels, offset);
    await t.tap(find.byTooltip('PDF 视图'));
    await t.pumpAndSettle();
    await t.tap(find.text('左右翻页').last);
    await t.pumpAndSettle();
    expect(find.byType(PageView), findsOneWidget);
    expect(
      canvasKey.currentState!.document!.cards.single.json()['pdf_paged'],
      true,
    );
    await t.tap(find.byTooltip('PDF 视图'));
    await t.pumpAndSettle();
    await t.tap(find.text('上下滚动').last);
    await t.pumpAndSettle();
    expect(
      find.descendant(of: reader, matching: find.byType(ListView)),
      findsOneWidget,
    );
    await t.pump(const Duration(seconds: 2));
    final activeScroll = t
        .state<ScrollableState>(
          find.descendant(of: list, matching: find.byType(Scrollable)),
        )
        .position;
    final actual = canvasKey.currentState!.document!;
    final center = t.getCenter(list);
    final savedOffset = activeScroll.pixels;
    final a = await t.startGesture(center - const Offset(60, 0), pointer: 51);
    final b = await t.startGesture(center + const Offset(60, 0), pointer: 52);
    await t.pump();
    await a.moveBy(const Offset(-30, -20));
    await b.moveBy(const Offset(30, 20));
    await t.pump();
    expect(actual.zoom, greaterThan(1.45));
    expect(activeScroll.pixels, closeTo(savedOffset, 1));
    await a.up();
    await b.up();
    await t.pumpAndSettle();
    await t.pumpWidget(const SizedBox());
    await t.runAsync(() => tmp.delete(recursive: true));
  });
  testWidgets('draw, pan and reopen a notebook without a lecture player', (
    t,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await UiPrefs.load(), api = _NotebookApi();
    t.view.physicalSize = const Size(1400, 900);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    Future<StudyCanvasState> open() async {
      final key = GlobalKey<StudyCanvasState>();
      await t.pumpWidget(
        MaterialApp(
          theme: buildTheme(buildScheme(prefs, Brightness.light), prefs),
          home: Scaffold(
            body: StudyCanvasSurface(
              prefs: prefs,
              child: StudyCanvas.notebook(
                key: key,
                api: api,
                id: 'book',
                name: 'Test',
                onBack: () {},
              ),
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      return key.currentState!;
    }

    final state = await open();
    expect(state.document!.cards, isEmpty);
    final pen = await t.startGesture(
      const Offset(500, 360),
      kind: PointerDeviceKind.stylus,
    );
    await pen.moveBy(const Offset(75, 64));
    await pen.up();
    await t.pump(const Duration(seconds: 1));
    expect(state.document!.strokes, isNotEmpty);
    await t.tap(find.byKey(const ValueKey('study-tool-select')));
    await t.pump();
    final before = state.document!.camera;
    await t.dragFrom(const Offset(800, 420), const Offset(90, -70));
    final camera = state.document!.camera;
    expect(camera, isNot(before));
    expect(await state.flush(), true);
    final n = state.document!.strokes.length;
    await t.pumpWidget(const SizedBox());
    final reopened = await open();
    expect(reopened.document!.strokes.length, n);
    expect(reopened.document!.camera, camera);
    await t.pumpWidget(const SizedBox());
  });
}
