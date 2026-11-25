import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

class _LessonApi extends fixture.FixtureApi {
  _LessonApi() : super(StudyDocument().json());
  final calls = <Map>[];
  final noteId = 'lesson-${'a' * 32}';
  @override
  Future<dynamic> post(String path, [Object? body, Duration? timeout]) async {
    if (!path.endsWith('/study')) return super.post(path, body, timeout);
    final input = body as Map;
    calls.add(input);
    return {
      'checkpoint': 'v${calls.length}',
      'noteId': noteId,
      'pending': {'id': 'q1', 'question': '为什么要保存寄存器？'},
      if (input['action'] == 'open')
        'note': {
          'id': noteId,
          'lecture_id': 'fixture',
          'title': '本节讲义',
          'body': '## 上下文切换\n\n保存寄存器。',
        },
    };
  }
}

void main() {
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
