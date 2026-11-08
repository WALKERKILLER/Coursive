// Protocol fixtures for host-issued citations, the views they lead to and
// pinned answer identity (spec 0027). They do not exercise a real model.
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';

import '../lib/features/study_canvas/study_canvas.dart';
import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

const passage = '第一句足够长了。';
const pageText = '$passage第二句是更多内容。';

Map<String, dynamic> evidence(
  String id,
  String object, {
  int start = 0,
  int end = 8,
  String quote = passage,
  String kind = 'exact_text',
  Map<String, dynamic> anchor = const {},
}) => {
  'id': id,
  'source_id': 'object-$object',
  'kind': kind,
  'status': 'verified',
  'label': 'Card $object',
  'anchor': {
    'id': 'object-$object',
    'workspace_id': 'fixture',
    'object_id': object,
    'kind': 'text',
    'start': start,
    'end': end,
    'quote': quote,
    ...anchor,
  },
};

Map<String, dynamic> turn(
  String id,
  String answer,
  List<Map> cited, {
  List suggestions = const [],
}) => {
  'id': id,
  'question': '问题 $id',
  'answer': answer,
  'state': 'done',
  'context_key': 'scope',
  'result': {
    'schema_version': 2,
    'sources': [],
    'evidence': cited,
    'suggestions': suggestions,
  },
};

StudyDocument canvas() => StudyDocument()
  ..camera = Offset.zero
  ..zoom = 1
  ..cards.addAll([
    StudyCard(
      id: 'text',
      type: 'text',
      layer: 'main',
      rect: const Rect.fromLTWH(100, 150, 300, 200),
      title: '讲义',
      text: pageText,
    ),
    StudyCard(
      id: 'far',
      type: 'text',
      layer: 'main',
      rect: const Rect.fromLTWH(2600, 1800, 300, 200),
      title: '远处',
      text: 'Far away text.',
    ),
    StudyCard(
      id: 'chat',
      type: 'conversation',
      layer: 'main',
      rect: const Rect.fromLTWH(500, 600, 300, 200),
      title: '对话 1',
      conversationId: 'chat',
    ),
  ]);

/// Opens the canvas with a saved conversation already on stage.
Future<GlobalKey<StudyCanvasState>> openWith(
  WidgetTester t,
  List<Map> turns, {
  fixture.FixtureApi? api,
  StudyDocument? doc,
  fixture.FixturePlayer? player,
}) async {
  final a = api ?? fixture.FixtureApi((doc ?? canvas()).json());
  a.conversations['chat'] = {
    'version': 1,
    'revision': 1,
    'id': 'chat',
    'draft': '',
    'query': {'objects': [], 'selection': {}, 'include_video': false},
    'turns': turns,
  };
  final key = await fixture.open(t, a, player: player);
  await t.tap(find.byKey(const ValueKey('study-ai-tab')));
  await t.pumpAndSettle();
  return key;
}

Finder marks() =>
    find.byWidgetPredicate((w) => w.runtimeType.toString() == '_CitationMark');

const escape = KeyDownEvent(
  physicalKey: PhysicalKeyboardKey.escape,
  logicalKey: LogicalKeyboardKey.escape,
  timeStamp: Duration.zero,
);

Future<void> close(WidgetTester t) async {
  await t.pumpWidget(const SizedBox());
  await t.pumpAndSettle();
}

class PagedApi extends fixture.FixtureApi {
  PagedApi(super.doc);
  @override
  Future<dynamic> studyPDFPage(String id, String source, int page) async => {
    'file': 'page-$page.png',
    'width': 600,
    'height': 800,
    'count': 3,
    'text': page == 2 ? pageText : 'Page one',
    'boxes': page == 2
        ? [
            {'start': 0, 'end': 8, 'x': .1, 'y': .2, 'w': .5, 'h': .04},
            {'start': 8, 'end': 17, 'x': .1, 'y': .26, 'w': .6, 'h': .04},
          ]
        : [],
  };
}

class FakeMedia implements Player {
  bool paused = false;
  @override
  PlayerState get state => const PlayerState();
  @override
  Future<void> pause() async => paused = true;
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class SeekPlayer extends fixture.FixturePlayer {
  final fake = FakeMedia();
  final seeks = <int>[];
  @override
  Player get player => fake;
  @override
  void seekTo(int ms) {
    seeks.add(ms);
    position.value = Duration(milliseconds: ms);
  }
}

void main() {
  test('markers become numbered links and pinned identities stay bounded', () {
    expect(
      studyEvidenceMarkdown('a [[e:e2]] b [[e:e1]][[e:e7]]', [
        {'id': 'e2'},
        {'id': 'e1'},
      ]),
      'a [1](coursive-evidence:e2) b [2](coursive-evidence:e1)',
    );
    expect(
      studyPlainAnswer('依据 [[e:e1]] 与 [[next:propose_outline]]'),
      '依据  与 ',
    );
    expect(studyPinGeneration('c', 't'), 'canvas-turn:c:t');
    final long = studyPinGeneration('c' * 80, 't' * 40);
    expect(long.length, lessThanOrEqualTo(100));
    expect(long, studyPinGeneration('c' * 80, 't' * 40));
    expect(long, isNot(studyPinGeneration('c' * 80, '${'t' * 39}u')));
  });

  testWidgets(
    'hover previews without moving; a click flies clear of the sidebar and Esc returns',
    (t) async {
      final key = await openWith(t, [
        turn('t1', '命中更快 [[e:e1]]，远处也提到 [[e:e2]][[e:e9]]。', [
          evidence('e1', 'text'),
          evidence('e2', 'far', end: 4, quote: 'Far '),
        ]),
      ]);
      final s = key.currentState!, d = s.document!;
      expect(marks(), findsNWidgets(2));
      expect(find.text('依据 2 段原文'), findsOneWidget);
      final camera = d.camera, zoom = d.zoom, undo = d.canUndo;
      final mouse = await t.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: const Offset(1, 1));
      addTearDown(mouse.removePointer);
      await mouse.moveTo(t.getCenter(marks().first));
      await t.pumpAndSettle();
      expect(find.text(passage), findsOneWidget, reason: 'preview quotes it');
      expect(d.camera, camera);
      expect(d.zoom, zoom);
      await mouse.moveTo(const Offset(1, 1));
      await t.pumpAndSettle();
      expect(find.text(passage), findsNothing);

      await t.tap(marks().at(1), kind: PointerDeviceKind.mouse);
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('study-return')), findsOneWidget);
      final far = d.cards.firstWhere((c) => c.id == 'far').rect;
      final shown = Rect.fromPoints(
        far.topLeft * d.zoom + d.camera,
        far.bottomRight * d.zoom + d.camera,
      );
      expect(shown.left, greaterThanOrEqualTo(0));
      expect(shown.top, greaterThanOrEqualTo(0));
      expect(
        shown.right,
        lessThanOrEqualTo(1600 - 420),
        reason: 'not under the sidebar',
      );
      expect(d.canUndo, undo, reason: 'following a citation is not an edit');

      expect(s.handleKey(escape), isTrue);
      await t.pumpAndSettle();
      expect(d.camera, camera);
      expect(d.zoom, zoom);
      expect(find.byKey(const ValueKey('study-return')), findsNothing);
      expect(s.handleKey(escape), isFalse, reason: 'nothing left to return to');
      await close(t);
    },
  );

  testWidgets('pointer input stops a flight where it is', (t) async {
    final key = await openWith(t, [
      turn('t1', '远处 [[e:e2]]', [evidence('e2', 'far', end: 4, quote: 'Far ')]),
    ]);
    final d = key.currentState!.document!;
    await t.tap(marks().first, kind: PointerDeviceKind.mouse);
    await t.pump(const Duration(milliseconds: 120));
    await t.tapAt(const Offset(700, 1000), kind: PointerDeviceKind.touch);
    final stopped = d.camera;
    await t.pumpAndSettle();
    expect(d.camera, stopped);
    await close(t);
  });

  testWidgets('stale and forged citations push nothing', (t) async {
    final key = await openWith(t, [
      turn('t1', '原文 [[e:e1]]', [evidence('e1', 'text')]),
    ]);
    final d = key.currentState!.document!;
    d.checkpoint();
    d.cards.firstWhere((c) => c.id == 'text').text = '改过的原文';
    d.changed();
    await t.pumpAndSettle();
    expect(find.textContaining('1 处已失效'), findsOneWidget);
    final camera = d.camera;
    await t.tap(marks().first, kind: PointerDeviceKind.mouse);
    await t.pumpAndSettle();
    expect(find.text('来源已失效：原文已修改或删除'), findsOneWidget);
    expect(find.byKey(const ValueKey('study-return')), findsNothing);
    expect(d.camera, camera);
    await close(t);
  });

  testWidgets(
    'a PDF page turned by a citation is reading state and turns back',
    (t) async {
      final doc = canvas()
        ..cards.add(
          StudyCard(
            id: 'pdf',
            type: 'pdfdoc',
            layer: 'main',
            rect: const Rect.fromLTWH(1200, 150, 400, 600),
            source: 'deck.pdf',
            page: 1,
            pageCount: 3,
            text: 'Page one',
          ),
        );
      final key = await openWith(t, [
        turn('t1', '第二页 [[e:e1]]', [
          evidence('e1', 'pdf', anchor: {'kind': 'pdf', 'page': 2}),
        ]),
      ], api: PagedApi(doc.json()));
      final s = key.currentState!, d = s.document!;
      final pdf = d.cards.firstWhere((c) => c.id == 'pdf');
      final undo = d.canUndo;
      await t.runAsync(() async {
        await t.tap(marks().first, kind: PointerDeviceKind.mouse);
        for (var i = 0; i < 10; i++) {
          await t.pump();
          await Future<void>.delayed(const Duration(milliseconds: 10));
        }
      });
      await t.pumpAndSettle();
      expect(pdf.page, 2);
      expect(pdf.text, pageText);
      expect(d.canUndo, undo);
      expect(s.returnFromSource(), isTrue);
      await t.pumpAndSettle();
      expect(pdf.page, 1);
      expect(pdf.text, 'Page one');
      await close(t);
    },
  );

  testWidgets(
    'a transcript citation seeks and pauses, and the return restores time',
    (t) async {
      final doc = canvas()
        ..cards.add(
          StudyCard(
            id: 'lecture',
            type: 'lecture',
            layer: 'main',
            rect: const Rect.fromLTWH(1200, 150, 480, 300),
            title: '课程视频',
          ),
        );
      final player = SeekPlayer();
      final key = await openWith(
        t,
        [
          turn('t1', '老师讲过 [[e:e1]]', [
            {
              'id': 'e1',
              'source_id': 'lecture-video',
              'kind': 'timestamped_transcript',
              'status': 'verified',
              'label': '课堂',
              'anchor': {
                'id': 'lecture-video',
                'workspace_id': 'fixture',
                'object_id': 'lecture',
                'kind': 'video',
                'time_start_ms': 512000,
                'time_end_ms': 515000,
                'quote': '缓存失效时需要读取主存',
              },
            },
          ]),
        ],
        doc: doc,
        player: player,
      );
      expect(find.text('依据 1 句讲解'), findsOneWidget);
      await t.tap(marks().first, kind: PointerDeviceKind.mouse);
      await t.pumpAndSettle();
      expect(player.seeks, [512000]);
      expect(player.fake.paused, isTrue);
      expect(key.currentState!.handleKey(escape), isTrue);
      await t.pumpAndSettle();
      expect(player.seeks, [512000, 530000]);
      await close(t);
    },
  );

  testWidgets(
    'identical answers pin separately, stay linked when edited and can pin again',
    (t) async {
      final cited = [evidence('e1', 'text')];
      final key = await openWith(t, [
        turn('t1', '同样的回答 [[e:e1]]', cited),
        turn('t2', '同样的回答 [[e:e1]]', cited),
      ]);
      final d = key.currentState!.document!;
      expect(find.text('留在画布'), findsNWidgets(2));
      await t.tap(
        find.descendant(
          of: find.byKey(const ValueKey('study-pin-answer-t1')),
          matching: find.text('留在画布'),
        ),
      );
      await t.pumpAndSettle();
      final pinned = d.cards.last;
      expect(pinned.generation, 'canvas-turn:chat:t1');
      expect(pinned.text, '同样的回答');
      expect(pinned.sources.single.objectId, 'text');
      expect(pinned.rect.overlaps(d.cards.first.rect.inflate(16)), isFalse);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      expect(find.text('留在画布'), findsOneWidget, reason: 't2 is its own answer');
      pinned.text = '我改写了它';
      d.changed();
      await t.pumpAndSettle();
      expect(find.text('已在画布 · 查看'), findsOneWidget);
      d.checkpoint();
      d.cards.remove(pinned);
      d.changed();
      await t.pumpAndSettle();
      expect(find.text('留在画布'), findsNWidgets(2));
      await close(t);
    },
  );
}
