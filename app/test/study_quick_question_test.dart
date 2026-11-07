// Real widget rasterization and snapshot lifecycle; no claim of model quality.
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_selection_tools_test.dart' show settleAsync;

class SelectionApi extends fixture.FixtureApi {
  SelectionApi(super.doc);
  final uploads = <Uint8List>[];
  final metadata = <Map>[];
  bool failUpload = false;
  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    selection = query;
    return {
      'sources': [
        for (final id in query['objects'] as List? ?? [])
          {
            'id': 'object-$id',
            'label': 'Selected page',
            'text': 'Page text',
            'availability': 'text',
            'anchor': {
              'id': 'object-$id',
              'kind': 'text',
              'object_id': id,
              'workspace_id': 'fixture',
              'fingerprint': 'page',
            },
          },
      ],
    };
  }

  @override
  Future<dynamic> uploadStudyAttachment(
    String lecture,
    Uint8List bytes,
    Map selection,
  ) async {
    uploads.add(Uint8List.fromList(bytes));
    metadata.add(jsonDecode(jsonEncode(selection)) as Map);
    if (failUpload) throw Exception('upload failed');
    return {
      'id': 'snapshot-${uploads.length}',
      'sha256': 'fixture-hash',
      'resource': 'saved.png',
      'mime_type': 'image/png',
      'width': 100,
      'height': 100,
      'selection': selection,
    };
  }
}

Future<void> selectPolygon(WidgetTester t, List<Offset> points) async {
  await t.tap(find.byKey(const ValueKey('study-tool-region')));
  await t.pumpAndSettle();
  await t.tap(find.byTooltip('自由套索'));
  await t.pumpAndSettle();
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(
    origin + points.first,
    kind: PointerDeviceKind.stylus,
  );
  for (final p in points.skip(1)) {
    await g.moveTo(origin + p);
    await t.pump();
  }
  await g.up();
  await t.pumpAndSettle();
}

Future<void> enclose(WidgetTester t, Rect rect) async {
  await selectPolygon(t, [
    rect.topLeft,
    rect.topRight,
    rect.bottomRight,
    rect.bottomLeft,
    rect.topLeft,
  ]);
  await t.tap(find.text('问小 c').last);
  await settleAsync(t);
  expect(find.byKey(const ValueKey('study-quick-question')), findsNothing);
  expect(find.byType(TextField), findsOneWidget);
}

Future<void> send(WidgetTester t, SelectionApi api) async {
  await t.enterText(find.byType(TextField).last, 'Explain this page');
  await t.tap(find.byTooltip('发送').last);
  await settleAsync(t);
}

StudyDocument document() => StudyDocument()
  ..camera = Offset.zero
  ..zoom = 1
  ..cards = [
    for (var i = 0; i < 3; i++)
      StudyCard(
        id: 'p$i',
        type: 'text',
        layer: 'main',
        rect: Rect.fromLTWH(200 + i * 220, 220, 180, 200),
        title: 'Page $i',
        text: 'Content $i',
      ),
  ];
void main() {
  testWidgets(
    'default rectangle captures the drawn area without snapping to selected objects',
    (t) async {
      final api = SelectionApi(document().json());
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-tool-region')));
      await t.pumpAndSettle();
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final gesture = await t.startGesture(
        origin + const Offset(180, 200),
        kind: PointerDeviceKind.stylus,
      );
      await gesture.moveTo(origin + const Offset(400, 440));
      await gesture.up();
      await t.pumpAndSettle();
      await t.tap(find.text('问小 c').last);
      await settleAsync(t);
      expect(api.metadata.single['bounds'], [180.0, 200.0, 220.0, 240.0]);
      expect(api.metadata.single['polygon'], [
        [180.0, 200.0],
        [400.0, 200.0],
        [400.0, 440.0],
        [180.0, 440.0],
      ]);
      await t.runAsync(() async {
        final codec = await ui.instantiateImageCodec(api.uploads.single);
        final frame = await codec.getNextFrame();
        final rgba = (await frame.image.toByteData())!.buffer.asUint8List();
        expect(rgba[3], 255);
        expect(rgba[rgba.length - 1], 255);
        expect(frame.image.width / frame.image.height, closeTo(220 / 240, .01));
        frame.image.dispose();
        codec.dispose();
      });
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('a partial material rectangle retains the drawn area', (t) async {
    final api = SelectionApi(document().json());
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-tool-region')));
    await t.pumpAndSettle();
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    final gesture = await t.startGesture(
      origin + const Offset(220, 260),
      kind: PointerDeviceKind.stylus,
    );
    await gesture.moveTo(origin + const Offset(320, 360));
    await gesture.up();
    await t.pumpAndSettle();
    await t.tap(find.text('问小 c').last);
    await settleAsync(t);
    expect(api.metadata.single['bounds'], [220.0, 260.0, 100.0, 100.0]);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'three sources produce one immutable screenshot in the single sidebar composer',
    (t) async {
      final api = SelectionApi(document().json());
      final key = await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(180, 200, 840, 440));
      expect(api.uploads, hasLength(1));
      expect(api.metadata.single['sources'], hasLength(3));
      expect(api.metadata.single['bounds'], [180.0, 200.0, 660.0, 240.0]);
      expect(find.text('当前课程本地材料'), findsOneWidget);
      expect(find.text('看图'), findsNothing);
      final saved = api.conversations.values.single;
      expect(saved['draft_attachments'], hasLength(1));
      expect(saved['version'], 2);
      await send(t, api);
      expect(api.request?['attachments'], hasLength(1));
      expect(api.request?['visuals'], isEmpty);
      expect(api.request?['context_scope'], 'course');
      final turn = (api.conversations.values.single['turns'] as List).single;
      expect(turn['attachments'], hasLength(1));
      expect(api.conversations.values.single['draft_attachments'], isEmpty);
      key.currentState!.document!.cards.first.text = 'changed after sending';
      expect(api.uploads, hasLength(1));
      await t.runAsync(() async {
        final codec = await ui.instantiateImageCodec(api.uploads.single);
        final frame = await codec.getNextFrame();
        expect(frame.image.width / frame.image.height, closeTo(660 / 240, .01));
        expect(frame.image.width, greaterThan(660));
        frame.image.dispose();
        codec.dispose();
      });
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets(
    'failed upload retries identical pixels after the original card changes',
    (t) async {
      final api = SelectionApi(document().json())..failUpload = true;
      final key = await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(220, 260, 320, 360));
      await t.enterText(find.byType(TextField).last, 'keep draft');
      key.currentState!.document!.cards.first.text = 'different content';
      api.failUpload = false;
      await t.tap(find.byTooltip('重试上传'));
      await settleAsync(t);
      expect(api.uploads, hasLength(2));
      expect(api.uploads.first, api.uploads.last);
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        'keep draft',
      );
      expect(
        api.conversations.values.single['draft_attachments'],
        hasLength(1),
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets(
    'lasso masks pixels outside its contour and preserves actual rectangle',
    (t) async {
      final api = SelectionApi(document().json());
      await fixture.open(t, api);
      await selectPolygon(t, [
        const Offset(180, 180),
        const Offset(580, 180),
        const Offset(180, 580),
        const Offset(180, 180),
      ]);
      await t.tap(find.text('问小 c').last);
      await settleAsync(t);
      await t.runAsync(() async {
        final codec = await ui.instantiateImageCodec(api.uploads.single);
        final frame = await codec.getNextFrame();
        final rgba = (await frame.image.toByteData())!.buffer.asUint8List();
        final outside =
            ((frame.image.height - 10) * frame.image.width +
                    frame.image.width -
                    10) *
                4 +
            3;
        expect(rgba[outside], 0);
        expect(rgba[(10 * frame.image.width + 10) * 4 + 3], 255);
        frame.image.dispose();
        codec.dispose();
      });
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets('snapshot drag attaches without moving the selected material', (
    t,
  ) async {
    final api = SelectionApi(document().json());
    final key = await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await selectPolygon(t, [
      const Offset(180, 180),
      const Offset(400, 180),
      const Offset(400, 440),
      const Offset(180, 440),
      const Offset(180, 180),
    ]);
    final before = key.currentState!.document!.cards.first.rect;
    final handle = find.byTooltip('拖入对话，附上选区截图');
    final g = await t.startGesture(t.getCenter(handle));
    await g.moveBy(const Offset(20, 10));
    await settleAsync(t);
    await g.moveTo(t.getCenter(find.byType(TextField).last));
    await t.pump();
    await g.up();
    await settleAsync(t);
    expect(api.conversations.values.single['draft_attachments'], hasLength(1));
    expect(key.currentState!.document!.cards.first.rect, before);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
}
