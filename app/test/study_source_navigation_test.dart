import 'dart:async';
import 'dart:convert';

import 'package:flutter/gestures.dart';
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
  final texts = <String, String>{
    for (var i = 1; i <= 2; i++) 'file-$i:0:18': 'Original passage $i',
  };
  bool unavailable = false;
  Completer<void>? gate;
  Completer<void>? saveGate;
  @override
  Future<dynamic> saveStudyCanvas(
    String id,
    Map<String, dynamic> document,
  ) async {
    if (saveGate != null) await saveGate!.future;
    return super.saveStudyCanvas(id, document);
  }

  @override
  Future<dynamic> resolveStudySource(String lecture, Map source) async {
    resolves++;
    if (gate != null) await gate!.future;
    if (unavailable) throw Exception('source changed');
    // Saved evidence omits text; the real resolve endpoint reads it again.
    return {
      ...jsonDecode(jsonEncode(source)) as Map,
      'text': texts['${source['id']}:${source['offset']}:${source['end']}'],
    };
  }
}

/// Numbered citation [n] in an answer; course citations saved as links by
/// the first course-scope builds load as schema 2 evidence.
Finder mark(int n) => find.descendant(
  of: find.byWidgetPredicate((w) => '${w.runtimeType}' == '_CitationMark'),
  matching: find.text('$n'),
);

Future<void> follow(WidgetTester t, int n) =>
    t.tap(mark(n), kind: PointerDeviceKind.mouse);

final back = find.byKey(const ValueKey('study-return'));

void main() {
  for (final concurrent in [false, true]) {
    testWidgets(
      'different excerpt lengths stay distinct (concurrent: $concurrent)',
      (t) async {
        final api = NavigationApi();
        final turns = api.conversations['saved']!['turns'] as List;
        final citations = (turns.single['result'] as Map)['citations'] as List;
        final longer = '${evidence(1)['text']} with additional context';
        api.texts['file-1:0:${longer.runes.length}'] = longer;
        citations[1]['source'] = {
          ...evidence(1),
          'text': longer,
          'end': longer.runes.length,
        };
        final key = await fixture.open(t, api);
        if (!concurrent) {
          key.currentState!.document!.cards.add(
            StudyCard(
              id: 'legacy-excerpt',
              type: 'text',
              layer: 'main',
              rect: const Rect.fromLTWH(400, 400, 480, 360),
              text: evidence(1)['text'] as String,
              originalPath: evidence(1)['path'] as String,
              sourceLectureId: evidence(1)['lecture_id'] as String,
              documentId: 'excerpt:file-1:0:fingerprint-1',
            ),
          );
        }
        await t.tap(find.byKey(const ValueKey('study-ai-tab')));
        await t.pumpAndSettle();
        if (concurrent) api.gate = Completer<void>();
        await follow(t, 1);
        if (concurrent) {
          await t.pump();
        } else {
          await t.pumpAndSettle();
          expect(
            key.currentState!.document!.cards
                .where((c) => c.type == 'text')
                .single
                .id,
            'legacy-excerpt',
          );
        }
        await follow(t, 2);
        await t.pump();
        if (concurrent) {
          // Different ranges must both resolve; only identical clicks coalesce.
          final resolves = api.resolves;
          api.gate!.complete();
          await t.pumpAndSettle();
          expect(resolves, 2);
        } else {
          await t.pumpAndSettle();
        }
        final cards = key.currentState!.document!.cards.where(
          (c) => c.type == 'text',
        );
        expect(
          cards.map((c) => c.text),
          unorderedEquals([evidence(1)['text'], longer]),
        );
        expect(back, findsOneWidget);
        await t.pumpWidget(const SizedBox());
        await t.pumpAndSettle();
      },
    );
  }

  testWidgets(
    'source navigation waits for the imported reference to be saved',
    (t) async {
      final api = NavigationApi();
      final key = await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      // Saved links load as numbered citations, never as raw links.
      expect(find.textContaining('coursive://citation'), findsNothing);
      expect(mark(2), findsOneWidget);
      api.saveGate = Completer<void>();
      final d = key.currentState!.document!,
          before = key.currentState!.document!.camera;
      await follow(t, 1);
      await t.pump();
      await t.pump(const Duration(seconds: 1));
      expect(d.camera, before);
      expect(back, findsNothing);
      api.saveGate!.complete();
      await t.pumpAndSettle();
      expect(
        (api.doc['objects'] as List).where(
          (c) => c['original_path'] == evidence(1)['path'],
        ),
        hasLength(1),
      );
      expect(back, findsOneWidget);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('a user pan interrupts citation navigation immediately', (
    t,
  ) async {
    final api = NavigationApi();
    final key = await fixture.open(t, api);
    await t.tap(find.byKey(const ValueKey('study-ai-tab')));
    await t.pumpAndSettle();
    await t.tap(find.byKey(const ValueKey('study-tool-hand')));
    await follow(t, 1);
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
      await follow(t, 1);
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
      await follow(t, 2);
      await t.pumpAndSettle();
      first.text = 'User edit while viewing another source';
      await t.tap(back);
      await t.pumpAndSettle();
      expect((d.camera - camera1).distance, lessThan(1));
      await t.tap(back);
      await t.pumpAndSettle();
      expect((d.camera - start).distance, lessThan(1));
      expect(d.zoom, closeTo(zoom, .001));
      expect(d.cards.where((c) => c.type == 'text'), hasLength(2));
      expect(first.text, 'User edit while viewing another source');
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        'still here',
      );
      await follow(t, 1);
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
      await follow(t, 1);
      await t.pumpAndSettle();
      expect(d.camera, before);
      expect(d.cards, hasLength(1));
      api.unavailable = false;
      api.gate = Completer<void>();
      await follow(t, 1);
      await t.pump();
      await follow(t, 1);
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
