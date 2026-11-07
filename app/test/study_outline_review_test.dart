// Protocol fixtures for reviewing an outline proposal and saving it as a
// lecture note (spec 0028). They do not exercise a real model.
import 'dart:convert';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/core/api.dart';
import '../lib/features/study_canvas/outline_document.dart';
import '../lib/features/study_canvas/study_canvas.dart';
import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_grounded_answers_test.dart' as grounded;

const proposalId = '0f8c2d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f';

Map<String, dynamic> outlineTurn() => {
  'id': 'run-1',
  'task': 'propose_outline',
  'question': '整理为复习提纲',
  'answer': '提纲草案「缓存复习」：2 节，4 条。',
  'state': 'done',
  'context_key': 'scope',
  'result': {
    'schema_version': 2,
    'sources': [],
    'suggestions': [],
    'evidence': [
      grounded.evidence('e1', 'text'),
      grounded.evidence('e2', 'gone'),
    ],
    'outline_proposal': {
      'version': 1,
      'proposal_id': proposalId,
      'lecture_id': 'fixture',
      'conversation_id': 'chat',
      'turn_id': 'run-1',
      'source_revision': 1,
      'state': 'draft',
      'title': '缓存复习',
      'sections': [
        {
          'id': 's1',
          'heading': '概念',
          'level': 1,
          'items': [
            {
              'id': 'i1',
              'kind': 'fact',
              'text': '第一句足够长了。第二句补充说明。',
              'evidence_ids': ['e1'],
              'review': 'pending',
            },
            {
              'id': 'i2',
              'kind': 'fact',
              'text': '旧讲义里的结论',
              'evidence_ids': ['e2'],
              'review': 'pending',
            },
            {
              'id': 'i3',
              'kind': 'question',
              'text': '为什么这样？',
              'evidence_ids': [],
              'review': 'pending',
            },
          ],
        },
        {
          'id': 's2',
          'heading': '待确认的手写标记',
          'level': 1,
          'items': [
            {
              'id': 'i4',
              'kind': 'unresolved_mark',
              'text': '讲义旁的 ✓',
              'evidence_ids': [],
              'review': 'needs_interpretation',
              'source_id': 'selected-ink',
              'anchor': {
                'id': 'selected-ink',
                'object_id': '',
                'kind': 'ink',
                'stroke_ids': ['tick'],
              },
            },
          ],
        },
      ],
      'conflicts': [
        {
          'text': '两处说法不同',
          'evidence_ids': ['e1', 'e2'],
        },
      ],
    },
  },
};

StudyDocument canvas() => grounded.canvas()
  ..strokes.add(
    StudyStroke(
      id: 'tick',
      layer: 'main',
      points: const [
        InkPoint(Offset(420, 160), .5),
        InkPoint(Offset(430, 175), .5),
        InkPoint(Offset(455, 140), .5),
      ],
      color: 0xff1d2125,
      width: 3,
    ),
  );

/// Notes as the daemon keeps them: PUT with a stale base_body is a 409.
class NotesApi extends fixture.FixtureApi {
  NotesApi() : super(canvas().json());
  final notes = <String, Map>{};
  final puts = <Map>[];
  bool dropNextResponse = false;
  Uint8List? previewed;

  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async {
    if (path.startsWith('/api/notes/')) {
      final id = Uri.decodeComponent(path.substring('/api/notes/'.length));
      return notes[id] ?? (throw ApiError(404, 'sql: no rows in result set'));
    }
    return super.get(path, query);
  }

  @override
  Future<dynamic> put(String path, Object body) async {
    if (path != '/api/notes') return super.put(path, body);
    final n = Map<String, dynamic>.from(body as Map);
    puts.add(n);
    final old = notes[n['id']];
    if (old != null &&
        n.containsKey('base_body') &&
        old['body'] != n['base_body']) {
      throw ApiError(409, '其他设备已修改此笔记，请另存本机编辑');
    }
    notes['${n['id']}'] = {...n}..remove('base_body');
    if (dropNextResponse) {
      dropNextResponse = false;
      throw ApiError(502, 'connection closed');
    }
    return notes[n['id']];
  }

  @override
  Future<dynamic> previewExportPDF(Uint8List pdf, int page) async {
    previewed = pdf;
    return {
      'page': page,
      'count': 3,
      'width': 1,
      'height': 1,
      'text': '缓存复习',
      'png': base64Encode(_pixel),
    };
  }
}

// A 1×1 transparent PNG.
final _pixel = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
);

Future<(GlobalKey<StudyCanvasState>, NotesApi)> openReview(
  WidgetTester t, {
  NotesApi? api,
}) async {
  final a = api ?? NotesApi();
  // Saved conversations arrive as decoded JSON.
  final key = await grounded.openWith(t, [
    jsonDecode(jsonEncode(outlineTurn())) as Map,
  ], api: a);
  await t.tap(find.byKey(const ValueKey('study-outline-review-run-1')));
  await t.pumpAndSettle();
  return (key, a);
}

Map savedProposal(NotesApi api) =>
    ((api.conversations['chat']!['turns'] as List).first
            as Map)['result']['outline_proposal']
        as Map;

Map savedItem(NotesApi api, String id) => [
  for (final s in savedProposal(api)['sections'] as List)
    for (final i in (s as Map)['items'] as List) i as Map,
].firstWhere((i) => i['id'] == id);

Future<void> settleSave(WidgetTester t) async {
  await t.pump(const Duration(milliseconds: 600));
  await t.pumpAndSettle();
}

Finder row(String id) => find.byKey(ValueKey('outline-row-$id'));

/// The note the saved draft would become; card `gone` makes e2 stale.
String markdown(NotesApi api) {
  final turn = (api.conversations['chat']!['turns'] as List).first as Map;
  return outlineMarkdown(
    outlineExport(
      savedProposal(api),
      (turn['result'] as Map)['evidence'] as List,
      stale: {'e2'},
    ),
  );
}

void main() {
  testWidgets(
    'a draft opens for review with its marks, stale evidence and progress',
    (t) async {
      final (key, _) = await openReview(t);
      expect(find.byKey(const ValueKey('outline-title')), findsOneWidget);
      expect(find.text('已审核 0/4'), findsOneWidget);
      expect(find.text('1 个标记待解释'), findsWidgets);
      expect(find.text('依据失效 1'), findsOneWidget);
      // F-01: the tick shows its real ink and no meaning is chosen for it.
      final thumb = t.widget(
        find.byWidgetPredicate((w) => w.runtimeType.toString() == '_InkThumb'),
      ) as dynamic;
      expect((thumb.strokes as List).single.id, 'tick');
      for (final c in ['已掌握', '重点', '待复习', '自定义', '保持原样']) {
        expect(
          t.widget<ChoiceChip>(find.widgetWithText(ChoiceChip, c)).selected,
          isFalse,
        );
      }
      expect(find.text('依据已失效'), findsOneWidget);
      expect(find.textContaining('两处说法不同'), findsOneWidget);
      // The open review keeps the canvas clear: its area excludes the panel.
      final state = key.currentState!;
      expect(find.byKey(const ValueKey('study-quick-composer')), findsNothing);
      expect(state.document!.cards.length, 3);
      await grounded.close(t);
    },
  );

  testWidgets(
    'accept, flag, rewrite, split, merge, move and undo stay in the draft',
    (t) async {
      final (_, api) = await openReview(t);
      await t.tap(
        find.descendant(
          of: row('i1'),
          matching: find.byIcon(Icons.radio_button_unchecked_rounded),
        ),
      );
      await t.pumpAndSettle();
      expect(find.text('已审核 1/4'), findsOneWidget);
      await settleSave(t);
      expect(savedItem(api, 'i1')['review'], 'confirmed');

      // Keyboard on a focused row: F flags, Enter rewrites, Enter commits.
      await t.tap(find.descendant(of: row('i3'), matching: find.text('问题')));
      await t.pump();
      await t.sendKeyEvent(LogicalKeyboardKey.keyF);
      await t.pumpAndSettle();
      expect(
        find.descendant(of: row('i3'), matching: find.text('待查')),
        findsOneWidget,
      );
      await t.tap(
        find.descendant(of: row('i3'), matching: find.text('为什么这样？')),
      );
      await t.pumpAndSettle();
      await t.enterText(
        find.byKey(const ValueKey('outline-edit-i3')),
        '为什么要这样设计？',
      );
      await t.sendKeyEvent(LogicalKeyboardKey.enter);
      await t.pumpAndSettle();
      expect(find.text('为什么要这样设计？'), findsOneWidget);

      // Split after the first sentence, then merge back.
      await t.tap(find.byKey(const ValueKey('outline-menu-i1')));
      await t.pumpAndSettle();
      await t.tap(find.text('拆成两条'));
      await t.pumpAndSettle();
      expect(
        find.descendant(of: row('i1'), matching: find.textContaining('第二句')),
        findsNothing,
      );
      expect(find.textContaining('第二句补充说明。'), findsOneWidget);
      await t.tap(find.byKey(const ValueKey('outline-menu-i1')));
      await t.pumpAndSettle();
      await t.tap(find.text('合并下一条'));
      await t.pumpAndSettle();
      expect(
        find.descendant(
          of: row('i1'),
          matching: find.textContaining('第一句足够长了。\n第二句补充说明。'),
        ),
        findsOneWidget,
      );
      expect(find.textContaining('第二句补充说明。'), findsOneWidget);

      // Alt+↓ moves the question past the heading into the next section.
      await t.tap(find.descendant(of: row('i3'), matching: find.text('问题')));
      await t.pump();
      await t.sendKeyDownEvent(LogicalKeyboardKey.altLeft);
      await t.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await t.sendKeyUpEvent(LogicalKeyboardKey.altLeft);
      await t.pumpAndSettle();
      await settleSave(t);
      final sections = savedProposal(api)['sections'] as List;
      expect(
        [for (final i in sections[1]['items'] as List) i['id']],
        ['i3', 'i4'],
      );

      await t.tap(find.byKey(const ValueKey('outline-undo')));
      await t.pumpAndSettle();
      await settleSave(t);
      expect([
        for (final i
            in (savedProposal(api)['sections'] as List)[0]['items'] as List)
          i['id'],
      ], contains('i3'));
      expect(savedProposal(api)['state'], 'draft');
      expect(api.puts, isEmpty);
      await grounded.close(t);
    },
  );

  testWidgets(
    'only the learner gives a mark its meaning, and can take it back',
    (t) async {
      final (_, api) = await openReview(t);
      await t.tap(find.byKey(const ValueKey('outline-choice-i4-key')));
      await t.pumpAndSettle();
      await settleSave(t);
      expect(savedItem(api, 'i4'), containsPair('choice', 'key'));
      expect(savedItem(api, 'i4')['review'], 'confirmed');
      expect(markdown(api), contains('- **重点** 讲义旁的 ✓'));
      await t.tap(find.text('撤销选择'));
      await t.pumpAndSettle();
      await settleSave(t);
      expect(savedItem(api, 'i4').containsKey('choice'), isFalse);
      expect(savedItem(api, 'i4')['review'], 'needs_interpretation');
      // G-03: an unconfirmed tick is only in the appendix.
      final md = markdown(api);
      expect(md, contains('### 手写标记\n\n- **未确认** 讲义旁的 ✓'));
      expect(md, isNot(contains('已掌握')));
      await grounded.close(t);
    },
  );

  testWidgets('stale evidence is kept as 待查, never as a checked fact', (
    t,
  ) async {
    final (_, api) = await openReview(t);
    await t.tap(find.text('保留为待查'));
    await t.pumpAndSettle();
    await settleSave(t);
    final md = markdown(api);
    expect(md, contains('### 待查\n\n- **待查** 旧讲义里的结论 [2]'));
    expect(md, contains('（依据已失效）'));
    expect(md.split('## 待核实').first, isNot(contains('旧讲义里的结论')));
    await grounded.close(t);
  });

  testWidgets(
    'saving reuses one note: a lost response is not a second note, another device is never overwritten',
    (t) async {
      final (key, api) = await openReview(t);
      api.dropNextResponse = true;
      await t.tap(find.byKey(const ValueKey('outline-save')));
      await t.pumpAndSettle();
      expect(find.textContaining('无法保存笔记'), findsOneWidget);
      expect(savedProposal(api)['state'], 'draft');
      // F-05: the retry finds its own note and writes nothing new.
      await t.tap(find.text('重试'));
      await t.pumpAndSettle();
      expect(api.notes.keys, ['outline-$proposalId']);
      expect(api.puts.length, 1);
      expect(api.puts.single.containsKey('base_body'), isFalse);
      expect(api.puts.single['lecture_id'], 'fixture');
      expect(api.puts.single['source'], 'user');
      final saved = savedProposal(api);
      expect(saved['state'], 'accepted');
      expect(saved['note_id'], 'outline-$proposalId');
      expect(
        saved['note_hash'],
        outlineNoteHash('${api.notes['outline-$proposalId']!['body']}'),
      );
      expect(find.text('已保存为本课笔记'), findsOneWidget);
      expect(
        t
            .widget<FilledButton>(find.byKey(const ValueKey('outline-save')))
            .onPressed,
        isNull,
      );

      // An edit updates the same note with its base.
      await t.tap(find.descendant(of: row('i3'), matching: find.text('问题')));
      await t.pump();
      await t.sendKeyEvent(LogicalKeyboardKey.keyF);
      await t.pumpAndSettle();
      expect(find.text('有修改尚未更新到笔记'), findsOneWidget);
      await t.tap(find.byKey(const ValueKey('outline-save')));
      await t.pumpAndSettle();
      expect(api.puts.length, 2);
      expect(api.puts.last['base_body'], api.puts.first['body']);

      // F-06: another device changed it; nothing is overwritten.
      api.notes['outline-$proposalId']!['body'] = '另一台设备的版本';
      await t.tap(find.descendant(of: row('i3'), matching: find.text('问题')));
      await t.pump();
      await t.sendKeyEvent(LogicalKeyboardKey.keyF);
      await t.pumpAndSettle();
      await t.tap(find.byKey(const ValueKey('outline-save')));
      await t.pumpAndSettle();
      expect(find.textContaining('已在其他地方修改'), findsOneWidget);
      expect(api.notes['outline-$proposalId']!['body'], '另一台设备的版本');
      expect(api.puts.length, 2);
      await t.tap(find.text('另存为新笔记'));
      await t.pumpAndSettle();
      expect(api.notes.length, 2);
      expect(savedProposal(api)['note_id'], isNot('outline-$proposalId'));
      // F-04: notes are written, the canvas is not.
      expect(key.currentState!.document!.cards.length, 3);
      await grounded.close(t);
    },
  );

  testWidgets(
    'closing keeps the draft and leaves the canvas as it was; Esc closes too',
    (t) async {
      final (key, api) = await openReview(t);
      final before = jsonEncode(key.currentState!.document!.json());
      await t.tap(
        find.descendant(
          of: row('i1'),
          matching: find.byIcon(Icons.radio_button_unchecked_rounded),
        ),
      );
      await t.pumpAndSettle();
      await t.tap(find.byKey(const ValueKey('outline-close')));
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('outline-title')), findsNothing);
      await settleSave(t);
      expect(savedItem(api, 'i1')['review'], 'confirmed');
      expect(jsonEncode(key.currentState!.document!.json()), before);
      await t.tap(find.byKey(const ValueKey('study-outline-review-run-1')));
      await t.pumpAndSettle();
      expect(find.text('已审核 1/4'), findsOneWidget);
      await t.tap(find.descendant(of: row('i3'), matching: find.text('问题')));
      await t.pump();
      await t.sendKeyEvent(LogicalKeyboardKey.escape);
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('outline-title')), findsNothing);
      await grounded.close(t);
    },
  );

  testWidgets(
    'a citation in the review leads to its source and back to the same item',
    (t) async {
      final (key, _) = await openReview(t);
      final state = key.currentState!;
      final camera = state.document!.camera;
      final mark = find.descendant(of: row('i1'), matching: grounded.marks());
      await t.tap(mark, kind: PointerDeviceKind.mouse);
      await t.pumpAndSettle();
      expect(state.document!.camera, isNot(camera));
      expect(find.byKey(const ValueKey('study-return')), findsOneWidget);
      // Following hands keys to the canvas; the return gives them back.
      bool inItem() {
        final ctx = FocusManager.instance.primaryFocus?.context;
        return ctx != null &&
            find
                .descendant(of: row('i1'), matching: find.byWidget(ctx.widget))
                .evaluate()
                .isNotEmpty;
      }

      expect(inItem(), isFalse);
      await t.tap(find.byKey(const ValueKey('study-return')));
      await t.pumpAndSettle();
      expect(state.document!.camera, camera);
      expect(find.byKey(const ValueKey('outline-title')), findsOneWidget);
      expect(inItem(), isTrue);
      await grounded.close(t);
    },
  );

  testWidgets(
    'a narrow window reviews from the bottom and can step aside for the source',
    (t) async {
      await openReview(t);
      t.view.physicalSize = const Size(640, 900);
      await t.pumpAndSettle();
      final panel = t.getRect(find.byKey(const ValueKey('outline-title')));
      expect(panel.top, greaterThan(300));
      await t.tap(find.text('原文'));
      await t.pumpAndSettle();
      expect(find.text('已审核 0/4'), findsNothing);
      expect(find.byKey(const ValueKey('outline-title')), findsOneWidget);
      await t.tap(find.text('提纲'));
      await t.pumpAndSettle();
      expect(find.text('已审核 0/4'), findsOneWidget);
      await grounded.close(t);
    },
  );

  testWidgets('the PDF preview shows the bytes that are saved', (t) async {
    final (_, api) = await openReview(t);
    await t.tap(find.byKey(const ValueKey('outline-export')));
    await t.pumpAndSettle();
    await t.tap(find.byKey(const ValueKey('outline-export-pdf')));
    for (var i = 0; i < 40 && api.previewed == null; i++) {
      await t.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 100)),
      );
      await t.pump();
    }
    await t.pumpAndSettle();
    expect(api.previewed, isNotNull);
    expect(String.fromCharCodes(api.previewed!.take(5)), '%PDF-');
    expect(find.text('1 / 3'), findsOneWidget);
    expect(find.textContaining('文字可选择复制'), findsOneWidget);
    final a4 = api.previewed;
    await t.tap(find.text('Letter'));
    for (var i = 0; i < 40 && identical(api.previewed, a4); i++) {
      await t.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 100)),
      );
      await t.pump();
    }
    await t.pumpAndSettle();
    expect(api.previewed, isNot(equals(a4)));
    await grounded.close(t);
  });
}
