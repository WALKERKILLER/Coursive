import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/features/study_canvas/study_visual.dart';
import 'study_canvas_ui_test.dart' as fixture;

Map<String, dynamic> source() => {
  'id': 'object-page',
  'kind': 'pdf',
  'label': 'PDF · 第 4 页',
  'availability': 'vision_required',
  'text': '',
  'image_path': 'source/page-4.png',
  'anchor': {
    'object_id': 'page',
    'page': 4,
    'region': [0.5, 0.0, 0.5, 1.0],
    'fingerprint': 'empty-text',
  },
};

Future<Uint8List> pageImage() async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  canvas.drawRect(
    const Rect.fromLTWH(0, 0, 100, 60),
    Paint()..color = Colors.red,
  );
  canvas.drawRect(
    const Rect.fromLTWH(50, 0, 50, 60),
    Paint()..color = Colors.blue,
  );
  final picture = recorder.endRecording();
  final image = await picture.toImage(100, 60);
  try {
    return (await image.toByteData(format: ui.ImageByteFormat.png))!.buffer
        .asUint8List();
  } finally {
    image.dispose();
    picture.dispose();
  }
}

class RestoredApi extends fixture.FixtureApi {
  RestoredApi()
    : super(
        (StudyDocument()
              ..cards.addAll([
                StudyCard(
                  id: 'page',
                  type: 'pdfdoc',
                  layer: 'main',
                  rect: const Rect.fromLTWH(3000, 3000, 400, 300),
                  page: 4,
                ),
                StudyCard(
                  id: 'thread',
                  type: 'conversation',
                  layer: 'main',
                  rect: const Rect.fromLTWH(100, 100, 300, 200),
                  conversationId: 'saved',
                ),
              ]))
            .json(),
      ) {
    conversations['saved'] = {
      'version': 1,
      'revision': 2,
      'id': 'saved',
      'draft': '没看懂',
      'turns': [],
      'allow_images': true,
      'query': {
        'objects': ['page'],
        'selection': {
          'object_id': 'page',
          'region': [0.5, 0.0, 0.5, 1.0],
        },
        'include_video': false,
      },
    };
  }
  bool fail = false;
  int starts = 0;
  bool followQuery = false;
  @override
  Future<dynamic> startCanvasRun(Map<String, dynamic> value) async {
    starts++;
    return super.startCanvasRun(value);
  }

  int loads = 0;
  Completer<Uint8List>? gate;
  @override
  Future<Uint8List> studyImageBytes(String path) async {
    loads++;
    expect(path, 'source/page-4.png');
    if (fail) throw const FormatException('图像读取失败');
    return gate == null ? pageImage() : gate!.future;
  }

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    final item = source();
    if (followQuery) {
      final selected =
          (query['selections'] as Map?)?['page'] ?? query['selection'];
      (item['anchor'] as Map).remove('region');
      if (selected is Map &&
          selected['object_id'] == 'page' &&
          selected['region'] != null)
        item['anchor']['region'] = selected['region'];
    }
    return {
      'sources': (query['objects'] as List).contains('page') ? [item] : [],
    };
  }
}

Future<void> submit(WidgetTester t, RestoredApi api) async {
  await t.runAsync(() async {
    await t.tap(find.byTooltip('发送').last);
    for (var i = 0; i < 80 && api.request == null && !api.fail; i++) {
      await t.pump();
      await Future<void>.delayed(const Duration(milliseconds: 10));
    }
  });
  await t.pumpAndSettle();
}

void main() {
  test('unrelated query cleanup preserves crops but removed or changed bindings revoke them', () {
    final before = {
      'objects': ['page'],
      'selection': {
        'object_id': 'page',
        'region': [0, 0, 0.5, 1],
      },
      'video_ms': 123,
    };
    final after = {...before}..remove('video_ms');
    expect(studyVisualBindingChanged('object-page', before, after), false);
    expect(
      studyVisualBindingChanged('object-page', before, {
        ...after,
        'objects': [],
      }),
      true,
    );
    expect(
      studyVisualBindingChanged('object-page', before, {
        ...after,
        'selection': {},
      }),
      true,
    );
    final a = source(), b = source();
    (b['anchor'] as Map)['page'] = 5;
    expect(studyVisualFingerprint(a), isNot(studyVisualFingerprint(b)));
  });

  testWidgets('reopened offscreen PDF selection sends only approved pixels', (
    t,
  ) async {
    final api = RestoredApi();
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await submit(t, api);
    expect(api.loads, 1);
    expect(api.request?['question'], '没看懂');
    final image = (api.request!['visuals'] as List).single as Map;
    expect(image['source_id'], 'object-page');
    await t.runAsync(() async {
      final codec = await ui.instantiateImageCodec(base64Decode(image['data']));
      final frame = await codec.getNextFrame();
      expect(frame.image.width, 50);
      expect(frame.image.height, 60);
      final rgba = (await frame.image.toByteData())!.buffer.asUint8List();
      expect(rgba.sublist(0, 4), [
        33,
        150,
        243,
        255,
      ]); // blue, no red left-half pixels
      frame.image.dispose();
      codec.dispose();
    });
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets('failed crop recovery keeps draft and permits retry', (t) async {
    final api = RestoredApi()..fail = true;
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await submit(t, api);
    expect(api.request, isNull);
    expect(find.textContaining('图像读取失败'), findsOneWidget);
    expect(
      t.widget<TextField>(find.byType(TextField).last).controller!.text,
      '没看懂',
    );
    expect(api.conversations['saved']!['turns'], isEmpty);
    api.fail = false;
    await submit(t, api);
    expect(api.request?['visuals'], hasLength(1));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets('vision can be enabled after reopening a text-only selection', (
    t,
  ) async {
    final api = RestoredApi();
    api.conversations['saved']!['allow_images'] = false;
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    expect(find.text('看图'), findsOneWidget);
    await t.tap(
      find.ancestor(of: find.text('看图'), matching: find.byType(FilterChip)),
    );
    await submit(t, api);
    expect(api.request?['allow_images'], true);
    expect(api.request?['visuals'], hasLength(1));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'typing during recovery preserves the next draft and sends once',
    (t) async {
      final api = RestoredApi()..gate = Completer<Uint8List>();
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      await t.tap(find.byTooltip('发送').last);
      await t.pump();
      await t.enterText(find.byType(TextField).last, '下一个问题');
      await t.runAsync(() async {
        api.gate!.complete(await pageImage());
        for (var i = 0; i < 80 && api.request == null; i++) {
          await t.pump();
          await Future<void>.delayed(const Duration(milliseconds: 10));
        }
      });
      await t.pumpAndSettle();
      expect(api.request?['question'], '没看懂');
      expect(api.starts, 1);
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        '下一个问题',
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      expect(api.conversations['saved']!['draft'], '下一个问题');
    },
  );

  testWidgets('stop during recovery keeps draft and allows another send', (
    t,
  ) async {
    final api = RestoredApi()..gate = Completer<Uint8List>();
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await t.tap(find.byTooltip('发送').last);
    await t.pump();
    await t.tap(find.byTooltip('停止').last);
    await t.runAsync(() async {
      api.gate!.complete(await pageImage());
      for (var i = 0; i < 30; i++) {
        await t.pump();
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    });
    await t.pumpAndSettle();
    expect(api.request, isNull);
    expect(
      t.widget<TextField>(find.byType(TextField).last).controller!.text,
      '没看懂',
    );
    api.gate = null;
    await submit(t, api);
    expect(api.starts, 1);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'rebinding a whole card replaces its region and drops the old crop',
    (t) async {
      final api = RestoredApi()..followQuery = true;
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      await submit(t, api);
      expect(api.request?['visuals'], hasLength(1));
      final target = t.widget<DragTarget<Object>>(
        find.byType(DragTarget<Object>).last,
      );
      target.onAcceptWithDetails!(
        DragTargetDetails(data: 'page', offset: Offset.zero),
      );
      await t.pumpAndSettle();
      api.request = null;
      await t.enterText(find.byType(TextField).last, '整页讲讲');
      await submit(t, api);
      expect(api.request?['visuals'], isEmpty);
      expect((api.request!['selection'] as Map)['selections'], isEmpty);
      expect((api.request!['selection']['selection'] as Map)['region'], isNull);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('closing during image recovery cannot start a model request', (
    t,
  ) async {
    final api = RestoredApi()..gate = Completer<Uint8List>();
    await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await t.tap(find.byTooltip('发送').last);
    await t.pump();
    expect(api.loads, 1);
    await t.pumpWidget(const SizedBox());
    await t.runAsync(() async {
      api.gate!.complete(await pageImage());
      await Future<void>.delayed(const Duration(milliseconds: 100));
    });
    await t.pumpAndSettle();
    expect(api.request, isNull);
    expect(t.takeException(), isNull);
  });

  testWidgets(
    'removing a source or disabling vision never resends its cached pixels',
    (t) async {
      final api = RestoredApi();
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      await submit(t, api);
      expect(api.request?['visuals'], hasLength(1));
      api.request = null;
      await t.tap(
        find.ancestor(of: find.text('看图'), matching: find.byType(FilterChip)),
      );
      await t.pumpAndSettle();
      await t.enterText(find.byType(TextField).last, '仅使用文字');
      await submit(t, api);
      expect(api.request?['allow_images'], false);
      expect(api.request?['visuals'], isEmpty);
      expect(api.loads, 1);
      await t.tap(find.byTooltip('移出对话'));
      await t.pumpAndSettle();
      api.request = null;
      await t.enterText(find.byType(TextField).last, '一个新问题');
      await submit(t, api);
      expect(api.request?['visuals'], isEmpty);
      expect(api.request?['selection']['objects'], isEmpty);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('legacy image card crop accounts for title and contain padding', (
    t,
  ) async {
    await t.runAsync(() async {
      final s = source();
      // A 200x200 card displays the 100x60 asset in (0,58)-(200,178).
      (s['anchor'] as Map)['region'] = [0.5, 0.29, 0.5, 0.6];
      final v = await restoreStudyVisual(
        s,
        (_) => pageImage(),
        cardSize: const Size(200, 200),
      );
      final codec = await ui.instantiateImageCodec(base64Decode(v['data']));
      final frame = await codec.getNextFrame();
      expect(frame.image.width, 100);
      expect(frame.image.height, 120);
      final rgba = (await frame.image.toByteData())!.buffer.asUint8List();
      expect(rgba.sublist((10 * 100 + 10) * 4, (10 * 100 + 10) * 4 + 4), [
        33,
        150,
        243,
        255,
      ]);
      frame.image.dispose();
      codec.dispose();
    });
  });

  testWidgets(
    'rotation crops the displayed coordinates and invalid regions never load',
    (t) async {
      await t.runAsync(() async {
        final s = source();
        (s['anchor'] as Map)['region'] = [0.0, 0.5, 1.0, 0.5];
        final v = await restoreStudyVisual(
          s,
          (_) => pageImage(),
          quarterTurns: 1,
        );
        final codec = await ui.instantiateImageCodec(base64Decode(v['data']));
        final frame = await codec.getNextFrame();
        expect(frame.image.width, 60);
        expect(frame.image.height, 50);
        final rgba = (await frame.image.toByteData())!.buffer.asUint8List();
        expect(rgba.sublist(0, 4), [33, 150, 243, 255]);
        frame.image.dispose();
        codec.dispose();
        (s['anchor'] as Map)['region'] = [-0.1, 0, 1, 1];
        await expectLater(
          restoreStudyVisual(s, (_) => throw StateError('must not load')),
          throwsFormatException,
        );
      });
    },
  );
}
