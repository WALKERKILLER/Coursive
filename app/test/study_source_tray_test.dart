// The sidebar's bound materials: they fold into a list when they no longer
// fit beside 「看图」, and the one being read wears the reading light.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/reading_glow.dart';
import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_grounded_answers_test.dart' as grounded;

StudyDocument canvas(int n) => grounded.canvas()
  ..cards.addAll([
    for (var i = 1; i <= n; i++)
      StudyCard(
        id: 'm$i',
        type: 'text',
        layer: 'main',
        rect: Rect.fromLTWH(100 + i * 60.0, 420, 220, 120),
        title: '第 $i 份课堂讲义',
        text: '材料 $i',
      ),
  ]);

/// Context for whatever is bound; a run, once started, reads `m2`.
class TrayApi extends fixture.FixtureApi {
  TrayApi(int n) : super(canvas(n).json());
  bool running = false;

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    selection = query;
    return {
      'sources': [
        for (final o in query['objects'] as List? ?? const [])
          {
            'id': 'object-$o',
            'kind': 'text',
            'label': '第 ${'$o'.substring(1)} 份课堂讲义（含板书与例题推导）',
            'text': '材料',
            'availability': 'text',
            'anchor': {'object_id': '$o'},
          },
      ],
    };
  }

  @override
  Future<dynamic> canvasRun(String id) async => running
      ? {
          'state': 'running',
          'trace': [
            {
              'tool': 'read_canvas_source',
              'status': 'running',
              'source_id': 'object-m2',
            },
          ],
        }
      : super.canvasRun(id);
}

Future<TrayApi> openWith(WidgetTester t, int n) async {
  final api = TrayApi(n);
  api.conversations['chat'] = {
    'version': 1,
    'revision': 1,
    'id': 'chat',
    'draft': '',
    // 「看图」 shows beside the materials once images are allowed.
    'allow_images': true,
    'query': {
      'objects': [for (var i = 1; i <= n; i++) 'm$i'],
      'selection': {},
      'include_video': false,
    },
    'turns': [],
  };
  await fixture.open(t, api);
  await t.tap(find.byKey(const ValueKey('study-ai-tab')));
  await t.pumpAndSettle();
  return api;
}

final summary = find.byKey(const ValueKey('study-source-summary'));
final list = find.byKey(const ValueKey('study-source-list'));
Finder get vision =>
    find.ancestor(of: find.text('看图'), matching: find.byType(FilterChip));

void main() {
  testWidgets('materials that fit stay in one line with their remove buttons', (
    t,
  ) async {
    await openWith(t, 1);
    expect(summary, findsNothing);
    expect(find.byTooltip('移出对话'), findsOneWidget);
    expect(
      t.getRect(find.byTooltip('移出对话')).overlaps(t.getRect(vision)),
      isFalse,
    );
    await grounded.close(t);
  });

  testWidgets(
    'many materials fold into a list where every one can still be removed',
    (t) async {
      await openWith(t, 6);
      expect(summary, findsOneWidget);
      expect(find.text('6 份资料'), findsOneWidget);
      // Nothing hides under 「看图」, and no name is cut off by it.
      expect(t.getRect(summary).overlaps(t.getRect(vision)), isFalse);
      expect(find.byTooltip('移出对话'), findsNothing);

      await t.tap(summary);
      await t.pumpAndSettle();
      expect(list, findsOneWidget);
      final remove = find.descendant(
        of: list,
        matching: find.byTooltip('移出对话'),
      );
      expect(remove, findsAtLeastNWidgets(4));
      for (final r in remove.evaluate().take(4)) {
        expect(
          t.getRect(find.byWidget(r.widget)).overlaps(t.getRect(vision)),
          isFalse,
        );
      }
      expect(
        find.descendant(of: list, matching: find.text('第 1 份课堂讲义（含板书与例题推导）')),
        findsOneWidget,
      );

      await t.tap(remove.first);
      await t.pumpAndSettle();
      expect(find.text('5 份资料'), findsOneWidget);
      expect(list, findsOneWidget, reason: 'the list stays open while editing');

      // Down to one, the tray unfolds and the list closes itself.
      for (var i = 0; i < 4; i++) {
        await t.tap(
          find.descendant(of: list, matching: find.byTooltip('移出对话')).first,
        );
        await t.pumpAndSettle();
      }
      expect(summary, findsNothing);
      expect(list, findsNothing);
      expect(find.byTooltip('移出对话'), findsOneWidget);
      await grounded.close(t);
    },
  );

  testWidgets('the material being read wears the reading light', (t) async {
    final api = await openWith(t, 6);
    api.running = true;
    await t.enterText(find.byType(TextField).last, '讲讲第二份');
    await t.tap(find.byTooltip('发送').last);
    for (var i = 0; i < 20; i++) {
      await t.pump(const Duration(milliseconds: 100));
    }
    // Folded, the count gives way to what is being read, shimmering.
    expect(find.text('正在读 · 第 2 份课堂讲义（含板书与例题推导）'), findsOneWidget);
    final glow = t.widget<ReadingGlow>(
      find.ancestor(of: summary, matching: find.byType(ReadingGlow)).first,
    );
    expect(glow.active, isTrue);
    expect(find.byType(ReadingOrb), findsWidgets);
    await t.tap(summary);
    await t.pump(const Duration(milliseconds: 400));
    final lit = [
      for (final g in t.widgetList<ReadingGlow>(
        find.descendant(of: list, matching: find.byType(ReadingGlow)),
      ))
        g.active,
    ];
    expect(lit.where((a) => a), hasLength(1));
    await grounded.close(t);
  });
}
