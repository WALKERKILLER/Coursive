import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

class ToolsApi extends fixture.FixtureApi {
  ToolsApi(super.doc);
  Uint8List? uploaded;
  int starts = 0;
  @override
  Future<dynamic> importStudyAsset(
    String id,
    String name,
    Stream<List<int>> data,
    int length,
  ) async {
    final b = BytesBuilder();
    await for (final c in data) {
      b.add(c);
    }
    uploaded = b.toBytes();
    return {
      'objects': [
        StudyCard(
          id: 'capture',
          type: 'image',
          layer: 'main',
          rect: const Rect.fromLTWH(0, 0, 320, 240),
        ).json(),
      ],
    };
  }

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> q) async => {
    'sources': [
      for (final key in q['objects'] as List? ?? [])
        {
          'anchor': {
            'id': 'object-$key',
            'kind': 'text',
            'object_id': key,
            'page': 3,
          },
        },
      if (((q['selection'] as Map?)?['stroke_ids'] as List? ?? []).isNotEmpty)
        {
          'anchor': {
            'id': 'selected-ink',
            'kind': 'ink',
            'object_id': '',
            'stroke_ids': q['selection']['stroke_ids'],
          },
        },
    ],
  };
  @override
  Future<dynamic> startCanvasRun(Map<String, dynamic> v) async {
    starts++;
    request = v;
    return {'id': 'recognition'};
  }

  @override
  Future<dynamic> canvasRun(String id) async => {
    'state': 'done',
    'answer': 'Recognized writing',
  };
  @override
  Future<dynamic> cancelCanvasRun(String id) async => {'state': 'cancelled'};
}

StudyDocument doc() => StudyDocument()
  ..camera = Offset.zero
  ..zoom = 1
  ..cards = [
    StudyCard(
      id: 'text',
      type: 'text',
      layer: 'main',
      rect: const Rect.fromLTWH(200, 200, 400, 300),
      title: 'Source',
      text: 'Source text',
    ),
  ]
  ..strokes = [
    StudyStroke(
      id: 'ink',
      layer: 'main',
      points: [
        const InkPoint(Offset(260, 300), .8),
        const InkPoint(Offset(380, 300), .8),
      ],
      color: 0xff111111,
      width: 3,
    ),
  ];
Future<void> select(WidgetTester t, Rect r) async {
  await t.tap(find.byKey(const ValueKey('study-tool-region')));
  await t.pumpAndSettle();
  await t.tap(find.byTooltip('自由套索'));
  await t.pumpAndSettle();
  final o = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(o + r.topLeft, kind: PointerDeviceKind.stylus);
  for (final p in [r.topRight, r.bottomRight, r.bottomLeft, r.topLeft]) {
    await g.moveTo(o + p);
    await t.pump();
  }
  await g.up();
  await t.pumpAndSettle();
}

Future<void> menu(WidgetTester t, String text) async {
  await t.tap(find.byKey(const ValueKey('study-selection-actions')));
  await t.pumpAndSettle();
  await t.tap(find.text(text).last);
  await t.pump();
}

Future<void> settleAsync(WidgetTester t) async {
  await t.runAsync(() async {
    for (var i = 0; i < 35; i++) {
      await t.pump();
      await Future<void>.delayed(const Duration(milliseconds: 10));
    }
  });
  await t.pumpAndSettle();
}

void main() {
  testWidgets('filter excludes document while selecting ink', (t) async {
    final api = ToolsApi(doc().json());
    final key = await fixture.open(t, api);
    await select(t, const Rect.fromLTRB(180, 180, 620, 520));
    await menu(t, '套索选择对象');
    await t.pumpAndSettle();
    await t.tap(find.text('文字与知识卡片'));
    await t.tap(find.text('保存').last);
    await t.pumpAndSettle();
    await select(t, const Rect.fromLTRB(180, 180, 620, 520));
    await menu(t, '顺时针旋转 90°');
    await settleAsync(t);
    final d = key.currentState!.document!;
    expect(d.cards.single.rect, doc().cards.single.rect);
    expect(
      d.strokes.single.points.first.p.dx,
      closeTo(d.strokes.single.points.last.p.dx, .01),
    );
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets('capture imports PNG, retains sources and original ink', (
    t,
  ) async {
    final api = ToolsApi(doc().json());
    final key = await fixture.open(t, api);
    final ink = jsonEncode(
      key.currentState!.document!.strokes.map((s) => s.json()).toList(),
    );
    await select(t, const Rect.fromLTRB(180, 180, 620, 520));
    await menu(t, '截图成卡片');
    await settleAsync(t);
    expect(api.uploaded?.take(8), [137, 80, 78, 71, 13, 10, 26, 10]);
    final d = key.currentState!.document!;
    expect(d.cards.length, 2);
    expect(d.cards.last.sources.first.objectId, 'text');
    expect(jsonEncode(d.strokes.map((s) => s.json()).toList()), ink);
    d.undo();
    expect(d.cards.length, 1);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets(
    'recognition requires consent and insertion confirms corrected text',
    (t) async {
      final api = ToolsApi(doc().json());
      final key = await fixture.open(t, api);
      await select(t, const Rect.fromLTRB(240, 280, 400, 320));
      await menu(t, '手写转文字');
      await t.pump(const Duration(milliseconds: 350));
      expect(api.starts, 0);
      await t.tap(find.text('开始识别'));
      await settleAsync(t);
      expect(api.starts, 1);
      expect(api.request!['selection']['objects'], isEmpty);
      expect(api.request!['visuals'].single['source_id'], 'selected-ink');
      expect(key.currentState!.document!.cards.length, 1);
      await t.enterText(find.byType(TextField).last, 'Corrected text');
      await t.tap(find.text('插入文字卡片'));
      await settleAsync(t);
      final d = key.currentState!.document!;
      expect(d.cards.last.text, 'Corrected text');
      expect(d.strokes.length, 1);
      expect(d.cards.last.sources.single.strokeIds, ['ink']);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets('region links have reciprocal sources and one undo step', (
    t,
  ) async {
    final original = doc()
      ..cards.add(
        StudyCard(
          id: 'second',
          type: 'text',
          layer: 'main',
          rect: const Rect.fromLTWH(800, 200, 400, 300),
          text: 'Second',
        ),
      );
    final api = ToolsApi(original.json());
    final key = await fixture.open(t, api);
    await select(t, const Rect.fromLTRB(220, 220, 450, 400));
    await menu(t, '设为链接起点');
    await settleAsync(t);
    await select(t, const Rect.fromLTRB(820, 220, 1100, 400));
    await menu(t, '连接到此选区');
    await settleAsync(t);
    final d = key.currentState!.document!;
    expect(d.cards.length, 4);
    expect(d.connections.length, 1);
    expect(d.cards[2].sources.first.objectId, 'second');
    expect(d.cards[3].sources.first.objectId, 'text');
    expect(StudyDocument.fromJson(d.json()).connections.length, 1);
    d.undo();
    expect(d.cards.length, 2);
    expect(d.connections, isEmpty);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets('shape handles commit geometry with undo', (t) async {
    final api = ToolsApi(doc().json());
    final key = await fixture.open(t, api);
    await select(t, const Rect.fromLTRB(240, 280, 400, 320));
    await menu(t, '编辑形状锚点');
    await t.pump(const Duration(milliseconds: 350));
    final area = t.getRect(find.byKey(const ValueKey('study-shape-editor')));
    final x = area.left + area.width * 36 / 192;
    await t.dragFrom(Offset(x, area.center.dy), const Offset(0, 40));
    await t.pump();
    await t.tap(find.text('保存').last);
    await settleAsync(t);
    final d = key.currentState!.document!;
    expect(d.strokes.single.gesture, 'ink');
    expect(d.strokes.single.points.first.p.dy, isNot(300));
    d.undo();
    expect(d.strokes.single.points.first.p.dy, 300);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
}
