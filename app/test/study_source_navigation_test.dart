import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

Map<String, dynamic> evidence(int number) => {
  'id': 'file-$number',
  'path': 'course/lecture-$number/notes.txt',
  'lecture_id': 'lecture-$number',
  'label': 'notes.txt',
  'text': 'Original passage $number',
  'fingerprint': 'fingerprint-$number',
  'offset': 0,
  'end': 18,
  'offset_unit': 'rune',
  'representation': 'file_text',
  'kind': 'txt',
  'availability': 'readable',
};

class NavigationApi extends fixture.FixtureApi {
  NavigationApi()
    : super(
        (StudyDocument()
              ..camera = const Offset(80, 90)
              ..zoom = .8
              ..cards = [
                StudyCard(
                  id: 'chat',
                  type: 'conversation',
                  conversationId: 'saved',
                  layer: 'main',
                  rect: const Rect.fromLTWH(0, 0, 200, 200),
                ),
              ])
            .json(),
      ) {
    conversations['saved'] = {
      'version': 2,
      'revision': 1,
      'id': 'saved',
      'draft': 'still here',
      'query': {'objects': [], 'selection': {}},
      'context_scope': 'course',
      'draft_attachments': [],
      'turns': [
        {
          'id': 'turn',
          'question': 'question',
          'answer': 'Compare [1](coursive://citation/e1) and [2](coursive://citation/e2)',
          'state': 'done',
          'result': {
            'citations': [
              for (var i = 1; i <= 2; i++)
                {
                  'id': 'e$i',
                  'number': i,
                  'label': 'Lecture $i · notes.txt · 文本摘录',
                  'source': evidence(i),
                },
            ],
          },
        },
      ],
    };
  }
  int resolves = 0;
  bool unavailable = false;
  Completer<void>? gate;
  @override
  Future<dynamic> resolveStudySource(String lecture, Map source) async {
    resolves++;
    if (gate != null) await gate!.future;
    if (unavailable) throw Exception('source changed');
    return jsonDecode(jsonEncode(source));
  }
}

void main() {
  testWidgets('a user pan interrupts citation navigation immediately', (
    t,
  ) async {
    final api = NavigationApi();
    final key = await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await t.tap(find.byKey(const ValueKey('study-tool-hand')));
    await t.tap(
      find.widgetWithText(ActionChip, 'Lecture 1 · notes.txt · 文本摘录'),
    );
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));
    await t.pump(const Duration(milliseconds: 50));
    final d = key.currentState!.document!;
    final moving = d.camera;
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    final gesture = await t.startGesture(origin + const Offset(40, 700));
    await gesture.moveBy(const Offset(40, 25));
    await gesture.up();
    await t.pump();
    final stopped = d.camera;
    expect((stopped - moving).distance, greaterThan(10));
    await t.pump(const Duration(seconds: 1));
    expect(d.camera, stopped);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'cross lecture citation reuses excerpt; consecutive returns preserve edits and draft',
    (t) async {
      final api = NavigationApi();
      final key = await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      final d = key.currentState!.document!;
      final start = d.camera;
      final zoom = d.zoom;
      await t.tap(
        find.widgetWithText(ActionChip, 'Lecture 1 · notes.txt · 文本摘录'),
      );
      await t.pumpAndSettle();
      expect(
        d.cards.where((c) => c.originalPath == evidence(1)['path']),
        hasLength(1),
      );
      final first = d.cards.where((c) => c.type == 'text').single;
      expect(first.sourceLectureId, 'lecture-1');
      final camera1 = d.camera;
      final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      final screen = origin + first.rect.center * d.zoom + d.camera;
      expect(screen.dx, lessThan(t.getTopLeft(find.byType(TextField).last).dx));
      await t.tap(
        find.widgetWithText(ActionChip, 'Lecture 2 · notes.txt · 文本摘录'),
      );
      await t.pumpAndSettle();
      first.text = 'User edit while viewing another source';
      await t.tap(find.text('返回提问处'));
      await t.pumpAndSettle();
      expect((d.camera - camera1).distance, lessThan(1));
      await t.tap(find.text('返回提问处'));
      await t.pumpAndSettle();
      expect((d.camera - start).distance, lessThan(1));
      expect(d.zoom, closeTo(zoom, .001));
      expect(d.cards.where((c) => c.type == 'text'), hasLength(2));
      expect(first.text, 'User edit while viewing another source');
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        'still here',
      );
      await t.tap(
        find.widgetWithText(ActionChip, 'Lecture 1 · notes.txt · 文本摘录'),
      );
      await t.pumpAndSettle();
      expect(
        d.cards.where((c) => c.originalPath == evidence(1)['path']),
        hasLength(1),
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets(
    'failed resolution keeps camera and duplicate clicks share one import',
    (t) async {
      final api = NavigationApi()..unavailable = true;
      final key = await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      final d = key.currentState!.document!,
          before = key.currentState!.document!.camera;
      final chip = find.widgetWithText(
        ActionChip,
        'Lecture 1 · notes.txt · 文本摘录',
      );
      await t.tap(chip);
      await t.pumpAndSettle();
      expect(d.camera, before);
      expect(d.cards, hasLength(1));
      api.unavailable = false;
      api.gate = Completer<void>();
      await t.tap(chip);
      await t.pump();
      await t.tap(chip);
      await t.pump();
      expect(api.resolves, 2);
      api.gate!.complete();
      await t.pumpAndSettle();
      expect(d.cards.where((c) => c.type == 'text'), hasLength(1));
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
