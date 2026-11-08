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

/// A free lasso through [points]; the region tool draws rectangles by default.
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

/// The default rectangle from [a] to [b].
Future<void> selectRect(WidgetTester t, Offset a, Offset b) async {
  await t.tap(find.byKey(const ValueKey('study-tool-region')));
  await t.pumpAndSettle();
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(origin + a, kind: PointerDeviceKind.stylus);
  await g.moveTo(origin + b);
  await g.up();
  await t.pumpAndSettle();
}

/// Lassoes [rect], opens 「问 AI」 in place and types the question.
Future<void> enclose(WidgetTester t, Rect rect) async {
  await selectPolygon(t, [
    rect.topLeft,
    rect.topRight,
    rect.bottomRight,
    rect.bottomLeft,
    rect.topLeft,
  ]);
  await t.tap(find.text('问 AI').last);
  await t.pumpAndSettle();
  await t.enterText(
    find.byKey(const ValueKey('study-quick-question')),
    'Explain this page',
  );
}

/// 「看图」 in the quick composer, which takes the screenshot to preview.
Future<void> allowImages(WidgetTester t) async {
  await t.tap(find.text('看图'));
  await settleAsync(t);
}

Future<void> send(WidgetTester t, SelectionApi api) async {
  await t.runAsync(() async {
    await t.tap(find.byTooltip('发送问题'));
    for (var i = 0; i < 80 && api.request == null; i++) {
      await t.pump();
      await Future<void>.delayed(const Duration(milliseconds: 10));
    }
  });
  await t.pumpAndSettle();
}

/// The selection bar's attach button: the region's pixels join the
/// conversation on stage, or start one.
Future<void> attachShot(WidgetTester t) async {
  await t.tap(find.byKey(const ValueKey('study-snapshot-attach')));
  await settleAsync(t);
}

Future<ui.Image> decode(WidgetTester t, Uint8List bytes) async =>
    (await t.runAsync(() async {
      final codec = await ui.instantiateImageCodec(bytes);
      return (await codec.getNextFrame()).image;
    }))!;

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
  for (final images in [false, true]) {
    testWidgets('whole PDF card quick question sends with images=$images', (
      t,
    ) async {
      final d = StudyDocument()
        ..camera = Offset.zero
        ..zoom = 1
        ..cards.add(
          StudyCard(
            id: 'page',
            type: 'pdfdoc',
            layer: 'main',
            rect: const Rect.fromLTWH(200, 200, 400, 420),
            title: 'Sample PDF',
            text: 'Page text',
            page: 3,
            pageCount: 5,
          ),
        );
      final api = SelectionApi(d.json());
      await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(180, 180, 620, 640));
      if (images) {
        await allowImages(t);
        // The learner sees the exact pixels before they go.
        expect(find.byKey(const ValueKey('study-quick-shot')), findsOneWidget);
      }
      await send(t, api);
      expect(api.request?['question'], 'Explain this page');
      expect(api.request?['context_scope'], 'selected');
      expect(api.selection?['objects'], ['page']);
      expect(api.selection?['object_pages'], {'page': 3});
      expect(api.request!['visuals'], isEmpty);
      if (images) {
        expect(api.uploads, hasLength(1));
        expect(api.request!['attachments'], hasLength(1));
        expect(api.uploads.single.take(8), [137, 80, 78, 71, 13, 10, 26, 10]);
        final image = await decode(t, api.uploads.single);
        expect(image.width, greaterThan(400));
        image.dispose();
      } else {
        // With 「看图」 off no screenshot is taken at all.
        expect(api.uploads, isEmpty);
        expect(api.request!['attachments'], isEmpty);
      }
      expect(api.conversations, hasLength(1));
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    });
  }

  testWidgets('PDF region uses the rendered page between the two 36px bars', (
    t,
  ) async {
    final d = StudyDocument()
      ..camera = Offset.zero
      ..zoom = 1;
    d.cards.add(
      StudyCard(
        id: 'page',
        type: 'pdfdoc',
        layer: 'main',
        rect: const Rect.fromLTWH(200, 200, 400, 420),
        pageAspect: .75,
        page: 3,
        pageCount: 5,
      ),
    );
    final api = SelectionApi(d.json());
    await fixture.open(t, api);
    await enclose(t, const Rect.fromLTRB(300, 300, 400, 400));
    await send(t, api);
    final region = (api.request!['selection']['selection']['region'] as List)
        .cast<num>();
    expect(region[0], closeTo(30.5 / 261, 1e-6));
    expect(region[1], closeTo(64 / 348, 1e-6));
    expect(region[2], closeTo(100 / 261, 1e-6));
    expect(region[3], closeTo(100 / 348, 1e-6));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'an empty lasso keeps the question instead of silently asking without sources',
    (t) async {
      final api = SelectionApi(
        (StudyDocument()
              ..camera = Offset.zero
              ..zoom = 1)
            .json(),
      );
      await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(200, 200, 450, 450));
      await send(t, api);
      expect(api.request, isNull);
      expect(api.conversations, isEmpty);
      expect(
        t
            .widget<TextField>(
              find.byKey(const ValueKey('study-quick-question')),
            )
            .controller!
            .text,
        'Explain this page',
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets(
    'three sources produce one immutable screenshot sent with their text',
    (t) async {
      final api = SelectionApi(document().json());
      final key = await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(180, 200, 840, 440));
      await allowImages(t);
      await send(t, api);
      expect(api.uploads, hasLength(1));
      expect(api.metadata.single['sources'], hasLength(3));
      expect(api.metadata.single['bounds'], [180.0, 200.0, 660.0, 240.0]);
      expect(api.selection?['objects'], ['p0', 'p1', 'p2']);
      expect(api.request?['attachments'], hasLength(1));
      expect(api.request?['visuals'], isEmpty);
      final saved = api.conversations.values.single;
      expect(saved['version'], 2);
      expect((saved['turns'] as List).single['attachments'], hasLength(1));
      expect(saved['draft_attachments'], isEmpty);
      // The sent pixels are fixed; later edits never re-upload.
      key.currentState!.document!.cards.first.text = 'changed after sending';
      await settleAsync(t);
      expect(api.uploads, hasLength(1));
      final image = await decode(t, api.uploads.single);
      expect(image.width / image.height, closeTo(660 / 240, .01));
      expect(image.width, greaterThan(660));
      image.dispose();
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets(
    'free lasso does not select a card outside its polygon but inside its bounding box',
    (t) async {
      final d = StudyDocument()
        ..camera = Offset.zero
        ..zoom = 1;
      d.cards.add(
        StudyCard(
          id: 'outside',
          type: 'text',
          layer: 'main',
          rect: const Rect.fromLTWH(500, 500, 180, 150),
          text: 'Outside triangle',
        ),
      );
      await fixture.open(t, SelectionApi(d.json()));
      await selectPolygon(t, const [
        Offset(150, 150),
        Offset(750, 150),
        Offset(150, 750),
        Offset(150, 150),
      ]);
      await t.tap(find.byKey(const ValueKey('study-selection-actions')));
      await t.pumpAndSettle();
      expect(
        t
            .widget<PopupMenuItem<String>>(
              find.widgetWithText(PopupMenuItem<String>, '副本'),
            )
            .enabled,
        isFalse,
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets(
    'default rectangle captures the drawn area without snapping to selected objects',
    (t) async {
      final api = SelectionApi(document().json());
      await fixture.open(t, api);
      await selectRect(t, const Offset(180, 200), const Offset(400, 440));
      await attachShot(t);
      expect(api.metadata.single['bounds'], [180.0, 200.0, 220.0, 240.0]);
      expect(api.metadata.single['polygon'], [
        [180.0, 200.0],
        [400.0, 200.0],
        [400.0, 440.0],
        [180.0, 440.0],
      ]);
      final image = await decode(t, api.uploads.single);
      final rgba = (await t.runAsync(() => image.toByteData()))!.buffer
          .asUint8List();
      expect(rgba[3], 255);
      expect(rgba[rgba.length - 1], 255);
      expect(image.width / image.height, closeTo(220 / 240, .01));
      image.dispose();
      // Attaching starts a conversation with the shot, scope unchanged.
      final saved = api.conversations.values.single;
      expect(saved['draft_attachments'], hasLength(1));
      expect(saved['context_scope'] ?? 'selected', 'selected');
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('a partial material rectangle retains the drawn area', (t) async {
    final api = SelectionApi(document().json());
    await fixture.open(t, api);
    await selectRect(t, const Offset(220, 260), const Offset(320, 360));
    await attachShot(t);
    expect(api.metadata.single['bounds'], [220.0, 260.0, 100.0, 100.0]);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'failed upload retries identical pixels after the original card changes',
    (t) async {
      final api = SelectionApi(document().json())..failUpload = true;
      final key = await fixture.open(t, api);
      await selectRect(t, const Offset(220, 260), const Offset(320, 360));
      await attachShot(t);
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
      await selectPolygon(t, const [
        Offset(180, 180),
        Offset(580, 180),
        Offset(180, 580),
        Offset(180, 180),
      ]);
      await attachShot(t);
      final image = await decode(t, api.uploads.single);
      final rgba = (await t.runAsync(() => image.toByteData()))!.buffer
          .asUint8List();
      final outside =
          ((image.height - 10) * image.width + image.width - 10) * 4 + 3;
      expect(rgba[outside], 0);
      expect(rgba[(10 * image.width + 10) * 4 + 3], 255);
      image.dispose();
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
    await selectPolygon(t, const [
      Offset(180, 180),
      Offset(400, 180),
      Offset(400, 440),
      Offset(180, 440),
      Offset(180, 180),
    ]);
    final before = key.currentState!.document!.cards.first.rect;
    final handle = find.byKey(const ValueKey('study-snapshot-attach'));
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

  testWidgets('「全课程」 is off by default and opts one conversation in', (
    t,
  ) async {
    final api = SelectionApi(document().json());
    await fixture.open(t, api);
    await enclose(t, const Rect.fromLTRB(180, 200, 400, 440));
    await send(t, api);
    expect(api.request?['context_scope'], 'selected');
    await settleAsync(t);
    final chip = find.byKey(const ValueKey('study-course-scope'));
    expect(t.widget<FilterChip>(chip).selected, isFalse);
    await t.tap(chip);
    await settleAsync(t);
    expect(t.widget<FilterChip>(chip).selected, isTrue);
    expect(api.conversations.values.single['context_scope'], 'course');
    api.request = null;
    await t.enterText(find.byType(TextField).last, 'And across the course?');
    await t.runAsync(() async {
      await t.tap(find.byTooltip('发送'));
      for (var i = 0; i < 80 && api.request == null; i++) {
        await t.pump();
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    });
    await t.pumpAndSettle();
    expect(api.request?['context_scope'], 'course');
    final turns = api.conversations.values.single['turns'] as List;
    expect(turns.last['context_scope'], 'course');
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
}
