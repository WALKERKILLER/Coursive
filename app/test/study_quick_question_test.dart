import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

class SelectionApi extends fixture.FixtureApi {
  SelectionApi(super.doc);

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    selection = query;
    return {
      'sources': [
        for (final id in query['objects'] as List)
          {
            'id': 'object-$id',
            'label': 'Selected page',
            'text': 'Page text',
            'availability': 'text',
            'anchor': {'object_id': id, 'fingerprint': 'page'},
          },
      ],
    };
  }
}

Future<void> enclose(WidgetTester t, Rect rect) async {
  await t.tap(find.byKey(const ValueKey('study-tool-region')));
  await t.pumpAndSettle();
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(
    origin + rect.topLeft,
    kind: PointerDeviceKind.stylus,
  );
  for (final p in [
    rect.topRight,
    rect.bottomRight,
    rect.bottomLeft,
    rect.topLeft,
  ]) {
    await g.moveTo(origin + p);
    await t.pump();
  }
  await g.up();
  await t.pumpAndSettle();
  await t.tap(find.text('问 AI').last);
  await t.pumpAndSettle();
  await t.enterText(
    find.byKey(const ValueKey('study-quick-question')),
    'Explain this page',
  );
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
      if (images) await t.tap(find.text('看图'));
      await send(t, api);
      expect(api.request?['question'], 'Explain this page');
      expect(api.selection?['objects'], ['page']);
      expect(api.selection?['object_pages'], {'page': 3});
      final attachments = api.request!['visuals'] as List;
      if (images) {
        expect(attachments, hasLength(1));
        expect(attachments.single['source_id'], 'object-page');
        final bytes = base64Decode(attachments.single['data'] as String);
        expect(bytes.take(8), [137, 80, 78, 71, 13, 10, 26, 10]);
        final codec = (await t.runAsync(
          () => ui.instantiateImageCodec(bytes),
        ))!;
        final frame = (await t.runAsync(codec.getNextFrame))!;
        expect(frame.image.width, greaterThan(400));
        expect(frame.image.width, lessThanOrEqualTo(1200));
        expect(frame.image.height, lessThanOrEqualTo(1200));
        frame.image.dispose();
        codec.dispose();
      } else {
        expect(attachments, isEmpty);
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
    'multiple cards attach separate crops; limit errors retain the question and allow text retry',
    (t) async {
      final d = StudyDocument()
        ..camera = Offset.zero
        ..zoom = 1;
      for (var i = 0; i < 3; i++) {
        d.cards.add(
          StudyCard(
            id: 'p$i',
            type: 'text',
            layer: 'main',
            rect: Rect.fromLTWH(200 + i * 220, 220, 180, 200),
            text: 'Page $i',
          ),
        );
      }
      final api = SelectionApi(d.json());
      final key = await fixture.open(t, api);
      await enclose(t, const Rect.fromLTRB(180, 200, 840, 440));
      await t.tap(find.text('看图'));
      await send(t, api);
      expect(api.request, isNull);
      expect(api.conversations, isEmpty);
      expect(find.textContaining('每次最多附两张'), findsOneWidget);
      expect(
        t
            .widget<TextField>(
              find.byKey(const ValueKey('study-quick-question')),
            )
            .controller!
            .text,
        'Explain this page',
      );
      await t.tap(find.text('看图'));
      await send(t, api);
      expect(api.request?['visuals'], isEmpty);
      expect(api.selection?['objects'], ['p0', 'p1', 'p2']);
      expect(
        key.currentState!.document!.cards.where(
          (c) => c.type == 'conversation',
        ),
        hasLength(1),
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('two selected cards attach only their own bounds', (t) async {
    final d = StudyDocument()
      ..camera = Offset.zero
      ..zoom = 1;
    for (var i = 0; i < 2; i++) {
      d.cards.add(
        StudyCard(
          id: 'p$i',
          type: 'text',
          layer: 'main',
          rect: Rect.fromLTWH(200 + i * 320, 220, 180, 200),
          text: 'Page $i',
        ),
      );
    }
    final api = SelectionApi(d.json());
    await fixture.open(t, api);
    await enclose(t, const Rect.fromLTRB(180, 200, 720, 440));
    await t.tap(find.text('看图'));
    await send(t, api);
    final images = api.request!['visuals'] as List;
    expect(images.map((v) => v['source_id']), ['object-p0', 'object-p1']);
    for (final image in images) {
      final codec = (await t.runAsync(
        () => ui.instantiateImageCodec(base64Decode(image['data'] as String)),
      ))!;
      final frame = (await t.runAsync(codec.getNextFrame))!;
      expect(frame.image.width, greaterThan(180));
      expect(frame.image.width / frame.image.height, closeTo(.9, .01));
      expect(frame.image.height, lessThanOrEqualTo(1200));
      frame.image.dispose();
      codec.dispose();
    }
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
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
      await t.tap(find.byKey(const ValueKey('study-tool-region')));
      await t.pumpAndSettle();
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final g = await t.startGesture(
        origin + const Offset(150, 150),
        kind: PointerDeviceKind.stylus,
      );
      for (final p in [
        const Offset(750, 150),
        const Offset(150, 750),
        const Offset(150, 150),
      ]) {
        await g.moveTo(origin + p);
        await t.pump();
      }
      await g.up();
      await t.pumpAndSettle();
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
}
