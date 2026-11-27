import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;
import '../lib/widgets/md_reader.dart';

class _LessonApi extends fixture.FixtureApi {
  _LessonApi() : super(StudyDocument().json());
  final calls = <Map>[];
  String bodyText = '## 上下文切换\n\n保存寄存器。';
  Map? bookmark;
  final noteId = 'lesson-${'a' * 32}';
  @override
  Future<dynamic> post(String path, [Object? body, Duration? timeout]) async {
    if (!path.endsWith('/study')) return super.post(path, body, timeout);
    final input = body as Map;
    calls.add(input);
    if (input['action'] == 'position') bookmark = input['bookmark'] as Map;
    return {
      'checkpoint': 'v${calls.length}',
      'noteId': noteId,
      if (bookmark != null) 'bookmark': bookmark,
      'pending': {'id': 'q1', 'question': '为什么要保存寄存器？'},
      if (input['action'] == 'open')
        'note': {
          'id': noteId,
          'lecture_id': 'fixture',
          'title': '本节讲义',
          'body': bodyText,
        },
    };
  }
}

void main() {
  testWidgets('lesson reader saves a place and restores it when reopened', (t) async {
    final api = _LessonApi()..bodyText = List.generate(50, (i) => '## 第 $i 节\n\n本节内容 $i。').join('\n\n');
    final canvas = await fixture.open(t, api);
    await canvas.currentState!.openLesson();
    await t.pumpAndSettle();
    final openReader = find.descendant(of: find.byKey(ValueKey('study-card/${canvas.currentState!.document!.cards.firstWhere((c) => c.lessonNote).id}')), matching: find.byTooltip('展开阅读或播放'));
    await t.tap(openReader);
    await t.pumpAndSettle();
    final reader = find.byType(MarkdownReader);
    await t.drag(reader, const Offset(0, -450));
    await t.pumpAndSettle();
    expect(api.bookmark?['ratio'], greaterThan(0));
    await t.tap(find.byType(BackButton));
    await t.pumpAndSettle();
    await t.tap(openReader);
    await t.pumpAndSettle();
    final scroll = t.state<ScrollableState>(find.descendant(of: reader, matching: find.byType(Scrollable)).first);
    expect(scroll.position.pixels, greaterThan(100));
    expect(find.text('回答本节练习'), findsOneWidget);
  });
  testWidgets(
    'opening the lesson twice reuses one editable card; a cancelled answer never submits',
    (t) async {
      final api = _LessonApi();
      final canvas = await fixture.open(t, api);
      await canvas.currentState!.openLesson();
      await t.pumpAndSettle();
      await canvas.currentState!.openLesson();
      await t.pumpAndSettle();
      expect(
        canvas.currentState!.document!.cards.where((c) => c.lessonNote).length,
        1,
      );
      expect(find.text('回答本节练习'), findsOneWidget);
      await t.tap(find.text('回答本节练习'));
      await t.pumpAndSettle();
      await t.enterText(find.byType(TextField), '保存寄存器');
      await t.tap(find.text('取消'));
      await t.pumpAndSettle();
      expect(api.calls.where((c) => c['action'] == 'answer'), isEmpty);
      await t.tap(find.text('回答本节练习'));
      await t.pumpAndSettle();
      await t.tap(find.text('提交作答'));
      await t.pumpAndSettle();
      expect(api.calls.last['answer'], '保存寄存器');
      expect(api.calls.last['questionId'], 'q1');
    },
  );
}
