import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show close, sheet;
import 'study_selection_tools_test.dart' show settleAsync, ToolsApi;

StudyCard card({
  String id = 'source',
  String type = 'text',
  String path = '',
}) => StudyCard(
  id: id,
  type: type,
  layer: 'main',
  title: '示例',
  path: path,
  rect: const Rect.fromLTWH(260, 260, 340, 230),
);

Future<void> selectCard(WidgetTester t, StudyCard c) async {
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  await t.tapAt(origin + c.rect.topLeft + const Offset(84, 18));
  await t.pumpAndSettle();
}

class _CropApi extends ToolsApi {
  _CropApi(super.doc, this.image);
  final Uint8List image;

  @override
  Future<Uint8List> studyImageBytes(String path) async => image;
}

void main() {
  testWidgets('hovering the more button leaves the paper above the toolbar clear', (t) async {
    final capture = GlobalKey();
    final key = await fixture.open(
      t,
      fixture.FixtureApi((sheet()..cards.add(card())).json()),
      capture: capture,
    );
    await selectCard(t, key.currentState!.document!.cards.single);
    final more = find.byKey(const ValueKey('study-card-selection-more'));
    final bounds = t.getRect(more);
    final boundary =
        capture.currentContext!.findRenderObject() as RenderRepaintBoundary;
    // The pill has a 6 dp inset, but the button's touch area is 44 dp.
    final sample = boundary.globalToLocal(
      Offset(bounds.center.dx, bounds.top + 3),
    );
    Future<int> paperPixel() async => (await t.runAsync(() async {
      final image = await boundary.toImage();
      try {
        final bytes = (await image.toByteData(
          format: ui.ImageByteFormat.rawRgba,
        ))!;
        final pixel = sample.dy.floor() * image.width + sample.dx.floor();
        return bytes.getUint32(pixel * 4);
      } finally {
        image.dispose();
      }
    }))!;
    final before = await paperPixel();
    final mouse = await t.createGesture(kind: ui.PointerDeviceKind.mouse);
    await mouse.addPointer(location: const Offset(1300, 400));
    await mouse.moveTo(bounds.center);
    await t.pump();
    await t.pump(const Duration(milliseconds: 240));
    expect(await paperPixel(), before);
    await t.tap(more);
    await t.pumpAndSettle();
    expect(find.text('取消选择'), findsOneWidget);
    await mouse.removePointer();
    await close(t);
  });

  testWidgets('the card toolbar locks and unlocks a saved position', (t) async {
    final api = fixture.FixtureApi((sheet()..cards.add(card())).json());
    final key = await fixture.open(t, api);
    final d = key.currentState!.document!;
    final c = d.cards.single;
    await selectCard(t, c);
    final bar = find.byKey(const ValueKey('study-card-selection-bar'));
    final grip = find.byKey(const ValueKey('study-rotation-handle'));
    expect(bar, findsOneWidget);
    expect(t.getRect(bar).bottom, lessThan(t.getRect(grip).top));
    expect(find.byTooltip('矩形裁剪'), findsNothing);
    expect(find.byTooltip('剪切'), findsNothing);
    expect(find.byTooltip('复制'), findsNothing);
    await t.tap(find.byKey(const ValueKey('study-card-selection-more')));
    await t.pumpAndSettle();
    expect(find.text('取消选择'), findsOneWidget);
    expect(find.text('粘贴'), findsNothing);
    await t.sendKeyEvent(LogicalKeyboardKey.escape);
    await t.pumpAndSettle();
    await t.tap(find.byTooltip('锁定位置'));
    await t.pumpAndSettle();
    expect(grip, findsNothing);
    expect(find.byTooltip('删除'), findsNothing);
    expect(c.positionLocked, isTrue);
    expect(await key.currentState!.flush(), isTrue);
    expect(api.doc['version'], 8);
    await close(t);

    final reopened = await fixture.open(t, api);
    final saved = reopened.currentState!.document!.cards.single;
    await selectCard(t, saved);
    expect(saved.positionLocked, isTrue);
    expect(grip, findsNothing);
    await t.tap(find.byTooltip('解锁位置'));
    await t.pumpAndSettle();
    expect(saved.positionLocked, isFalse);
    expect(grip, findsOneWidget);
    await close(t);
  });

  for (final type in ['lecture', 'video', 'text', 'pdf', 'image', 'conversation']) {
    testWidgets('the toolbar for a $type card docks at the top edge', (t) async {
      final nearTop = card(id: type == 'lecture' ? 'lecture' : 'source', type: type)
        ..rect = const Rect.fromLTWH(260, 100, 340, 230);
      if (type == 'video') nearTop.sourceLectureId = 'fixture';
      if (type == 'conversation') {
        nearTop.generation = 'markdown-aside';
        nearTop.conversationId = 'test-aside';
      }
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(nearTop)).json()),
      );
      await selectCard(t, key.currentState!.document!.cards.single);
      final bar = t.getRect(find.byKey(const ValueKey('study-card-selection-bar')));
      final tools = t.getRect(find.byKey(const ValueKey('study-top-tools')));
      expect(bar.top, greaterThanOrEqualTo(tools.bottom));
      expect(bar.top, lessThan(nearTop.rect.top));
      expect(find.byTooltip('锁定位置').hitTestable(), findsOneWidget);
      await close(t);
    });
  }

  test('a lock in a legacy document raises the format even after unlock', () {
    final legacy = (sheet()..cards.add(card()..positionLocked = true)).json()
      ..['version'] = 3;
    final restored = StudyDocument.fromJson(legacy);
    expect(restored.cards.single.positionLocked, isTrue);
    restored.cards.single.positionLocked = false;
    expect(restored.json()['version'], 8);
    expect(sheet().json()['version'], 3);
  });

  testWidgets('an aside cannot create a second card for the same chat', (
    t,
  ) async {
    final aside = card(type: 'conversation')
      ..generation = 'markdown-aside'
      ..conversationId = 'chat-already-in-use';
    final key = await fixture.open(
      t,
      fixture.FixtureApi((sheet()..cards.add(aside)).json()),
    );
    await selectCard(t, key.currentState!.document!.cards.single);
    expect(
      find.byKey(const ValueKey('study-card-selection-bar')),
      findsOneWidget,
    );
    expect(find.byTooltip('创建副本'), findsNothing);
    expect(find.byTooltip('删除'), findsOneWidget);
    await close(t);
  });

  testWidgets('selection controls leave and return with the card', (t) async {
    final key = await fixture.open(
      t,
      fixture.FixtureApi((sheet()..cards.add(card())).json()),
    );
    await selectCard(t, key.currentState!.document!.cards.single);
    final bar = find.byKey(const ValueKey('study-card-selection-bar'));
    expect(bar, findsOneWidget);
    final input = t.getRect(find.byKey(const ValueKey('study-input')));
    await t.sendKeyDownEvent(LogicalKeyboardKey.space);
    final scroll = await t.startGesture(
      input.center + const Offset(120, 90),
      kind: ui.PointerDeviceKind.mouse,
    );
    await scroll.moveBy(const Offset(-1000, 0));
    await t.pump();
    expect(bar, findsNothing);
    await scroll.moveBy(const Offset(1000, 0));
    await t.pump();
    expect(bar, findsOneWidget);
    await scroll.up();
    await t.sendKeyUpEvent(LogicalKeyboardKey.space);
    await close(t);
  });

  testWidgets('creating a copy duplicates the card and its attached ink', (
    t,
  ) async {
    final doc = sheet()
      ..cards.addAll([
        card(),
        card(id: 'related')..rect = const Rect.fromLTWH(720, 260, 300, 230),
      ])
      ..connections.add(StudyConnection('link', 'source', 'related', ''));
    doc.strokes.add(
      StudyStroke(
        id: 'ink',
        anchor: 'source',
        layer: 'main',
        points: const [
          InkPoint(Offset(290, 320), .7),
          InkPoint(Offset(322, 340), .7),
        ],
        color: 0xff112233,
        width: 2,
      ),
    );
    final key = await fixture.open(t, fixture.FixtureApi(doc.json()));
    final d = key.currentState!.document!;
    await selectCard(t, d.cards.first);
    await t.tap(find.byTooltip('创建副本'));
    await t.pumpAndSettle();
    expect(d.cards, hasLength(3));
    expect(d.cards.first.id, 'source');
    expect(d.cards.last.id, isNot('source'));
    expect(d.strokes.first.id, 'ink');
    expect(d.strokes.first.anchor, 'source');
    expect(d.strokes.last.anchor, d.cards.last.id);
    expect(d.connections.single.id, 'link');
    d.undo();
    await t.pumpAndSettle();
    expect(d.cards, hasLength(2));
    expect(d.strokes.single.id, 'ink');
    expect(d.cards.first.id, 'source');
    expect(d.connections.single.id, 'link');
    await close(t);
  });

  for (final free in [false, true]) {
    testWidgets(
      '${free ? 'free' : 'rectangular'} crop keeps the image and adds a PNG',
      (t) async {
        final png = await t.runAsync(() async {
          final recorder = ui.PictureRecorder();
          Canvas(recorder).drawRect(
            const Rect.fromLTWH(0, 0, 100, 100),
            Paint()..color = Colors.red,
          );
          final picture = recorder.endRecording();
          final image = await picture.toImage(100, 100);
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          image.dispose();
          picture.dispose();
          return bytes!.buffer.asUint8List();
        });
        final source = card(type: 'image', path: 'original.png');
        final api = _CropApi((sheet()..cards.add(source)).json(), png!);
        final key = await fixture.open(t, api);
        final d = key.currentState!.document!;
        await selectCard(t, d.cards.single);
        await t.runAsync(() async {
          await t.tap(find.byTooltip(free ? '自由裁剪' : '矩形裁剪'));
        });
        await settleAsync(t);
        final preview = t.getRect(
          find.byKey(const ValueKey('study-card-crop-image')),
        );
        Offset at(double x, double y) =>
            preview.topLeft + Offset(x * preview.width, y * preview.height);
        final gesture = await t.startGesture(at(.25, .25));
        if (free) await gesture.moveTo(at(.75, .25));
        await gesture.moveTo(at(.75, .75));
        await gesture.up();
        await t.pumpAndSettle();
        await t.runAsync(() async {
          await t.tap(find.text('裁剪为副本'));
          await t.pump();
        });
        await settleAsync(t);
        expect(d.cards, hasLength(2));
        expect(d.cards.first.path, 'original.png');
        expect(d.cards.last.type, 'image');
        await t.runAsync(() async {
          final codec = await ui.instantiateImageCodec(api.uploaded!);
          final frame = (await codec.getNextFrame()).image;
          final rgba = (await frame.toByteData())!;
          expect(frame.width, inInclusiveRange(49, 51));
          expect(frame.height, inInclusiveRange(49, 51));
          final alpha = rgba.getUint8(
            4 * (frame.height - 3) * frame.width + 11,
          );
          expect(alpha, free ? 0 : 255);
          frame.dispose();
          codec.dispose();
        });
        d.undo();
        await t.pumpAndSettle();
        expect(d.cards.single.path, 'original.png');
        await close(t);
      },
    );
  }
}
