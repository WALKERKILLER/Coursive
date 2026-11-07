import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/features/study_canvas/study_query.dart';
import 'study_canvas_ui_test.dart' as fixture;

class StaleConversationApi extends fixture.FixtureApi {
  StaleConversationApi(super.doc);
  @override
  Future<dynamic> studyConversation(String lecture, String id) async => {
    'version': 1,
    'revision': 2,
    'id': id,
    'draft': '保留我的问题',
    'query': {
      'objects': ['deleted-card', 'text'],
      'selection': {},
      'include_video': false,
    },
    'turns': [],
  };
  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    if ((query['objects'] as List).contains('deleted-card')) {
      throw Exception('无效的学习画布');
    }
    return super.studyContext(id, query);
  }
}

Map<String, dynamic> lectureQuery() => {
  'objects': ['lecture', 'text'],
  'include_video': true,
  'video_ms': 5000,
  'from_ms': 1000,
  'to_ms': 10000,
  'object_video_ms': {'lecture': 5000, 'text': 2000},
  'video_ranges': {
    'lecture': [1000, 10000],
    'text': [0, 3000],
  },
};

class StaleLectureApi extends StaleConversationApi {
  StaleLectureApi(super.doc);
  @override
  Future<dynamic> studyConversation(String lecture, String id) async => {
    ...await super.studyConversation(lecture, id) as Map,
    'query': lectureQuery(),
  };
}

void main() {
  test('daemon zero-valued video defaults do not invalidate a PDF binding', () {
    final d = StudyDocument()
      ..cards.add(
        StudyCard(
          id: 'page',
          type: 'pdf',
          layer: 'main',
          rect: const Rect.fromLTWH(0, 0, 200, 100),
        ),
      );
    final q = <String, dynamic>{
      'objects': ['page'],
      'selection': {},
      'include_video': false,
      'video_ms': 0,
      'from_ms': 0,
      'to_ms': 0,
    };
    expect(
      reconcileStudyQuery(q, d),
      q,
      reason:
          'the daemon serializes these defaults when reopening a conversation',
    );
  });

  for (final state in [
    'deleted',
    'hidden',
    'unselected',
    'selected',
    'disabled',
  ]) {
    test('lecture context follows the $state lecture binding', () {
      final d = StudyDocument()
        ..layers.add(StudyLayer('hidden', 'hidden', visible: false));
      for (final id in ['lecture', 'text']) {
        if (id == 'lecture' && state == 'deleted') continue;
        d.cards.add(
          StudyCard(
            id: id,
            type: id == 'lecture' ? 'lecture' : 'video',
            layer: id == 'lecture' && state == 'hidden' ? 'hidden' : 'main',
            rect: const Rect.fromLTWH(0, 0, 200, 100),
          ),
        );
      }
      final q = lectureQuery();
      if (state == 'unselected') q['objects'] = ['text'];
      if (state == 'disabled') q['include_video'] = false;
      final before = jsonEncode(q);
      final fixed = reconcileStudyQuery(q, d);
      final retained = state == 'selected' || state == 'disabled';
      expect(fixed['include_video'], state == 'selected');
      for (final field in ['video_ms', 'from_ms', 'to_ms']) {
        expect(fixed.containsKey(field), retained);
        if (retained) expect(fixed[field], q[field]);
      }
      expect((fixed['video_ranges'] as Map).containsKey('lecture'), retained);
      expect(
        (fixed['object_video_ms'] as Map).containsKey('lecture'),
        retained,
      );
      expect(fixed['video_ranges']['text'], [0, 3000]);
      expect(fixed['object_video_ms']['text'], 2000);
      expect(jsonEncode(q), before);
      expect(reconcileStudyQuery(fixed, d), fixed);
    });
  }

  for (final hidden in [false, true]) {
    testWidgets(
      'reopened ${hidden ? 'hidden' : 'deleted'} lecture stays out of AI requests',
      (t) async {
        final d = StudyDocument()
          ..layers.add(StudyLayer('hidden', 'hidden', visible: false))
          ..cards.add(
            StudyCard(
              id: 'text',
              type: 'text',
              layer: 'main',
              rect: const Rect.fromLTWH(0, 0, 200, 100),
              text: '保留的资料',
            ),
          );
        if (hidden) {
          d.cards.add(
            StudyCard(
              id: 'lecture',
              type: 'lecture',
              layer: 'hidden',
              rect: const Rect.fromLTWH(300, 0, 200, 100),
            ),
          );
        }
        final api = StaleLectureApi(d.json());
        await fixture.open(t, api);
        await t.tap(find.byTooltip('添加模块'));
        await t.pumpAndSettle();
        await t.tap(find.widgetWithText(PopupMenuItem<String>, 'AI 对话'));
        await t.pumpAndSettle();
        expect(api.request, isNull);
        expect(api.conversations.values.single['draft'], '保留我的问题');
        await t.tap(find.byTooltip('发送').last);
        await t.pumpAndSettle();
        for (final q in [
          api.selection!,
          api.conversations.values.single['query'],
          api.request!['selection'],
        ]) {
          expect(q['objects'], ['text']);
          expect(q['include_video'], false);
          for (final field in ['video_ms', 'from_ms', 'to_ms']) {
            expect(q.containsKey(field), false);
          }
          expect(q['video_ranges'].containsKey('lecture'), false);
          expect(q['object_video_ms'].containsKey('lecture'), false);
        }
        await t.pumpWidget(const SizedBox());
        await t.pumpAndSettle();
      },
    );
  }

  test(
    'stale references are removed without widening scope or changing the input',
    () {
      final d = StudyDocument()
        ..cards.add(
          StudyCard(
            id: 'keep',
            type: 'text',
            layer: 'main',
            rect: const Rect.fromLTWH(0, 0, 100, 100),
          ),
        );
      final q = <String, dynamic>{
        'objects': ['gone', 'keep'],
        'selection': {
          'object_id': 'gone',
          'stroke_ids': ['old-ink'],
        },
        'selections': {
          'gone': {'object_id': 'gone'},
        },
        'object_pages': {'gone': 3, 'keep': 2},
        'object_video_ms': {'gone': 100},
        'video_ranges': {
          'gone': [0, 100],
        },
        'include_video': false,
      };
      final before = jsonEncode(q);
      final fixed = reconcileStudyQuery(q, d);
      expect(fixed['objects'], ['keep']);
      expect(fixed['selection'], isEmpty);
      expect(fixed['selections'], isEmpty);
      expect(fixed['object_pages'], {'keep': 2});
      expect(fixed['object_video_ms'], isEmpty);
      expect(fixed['video_ranges'], isEmpty);
      expect(fixed['include_video'], false);
      expect(jsonEncode(q), before);
      expect(reconcileStudyQuery(fixed, d), fixed);
      expect(
        reconcileStudyQuery({
          'selection': {
            'stroke_ids': ['gone'],
          },
        }, d)['selection'],
        {'stroke_ids': []},
      );
    },
  );

  testWidgets(
    'reopening a stale conversation repairs and persists its sources',
    (t) async {
      final d = StudyDocument()
        ..cards.add(
          StudyCard(
            id: 'text',
            type: 'text',
            layer: 'main',
            rect: const Rect.fromLTWH(0, 0, 200, 100),
            text: '保留的资料',
          ),
        );
      final api = StaleConversationApi(d.json());
      final key = await fixture.open(t, api);
      await t.tap(find.byTooltip('添加模块'));
      await t.pumpAndSettle();
      await t.tap(find.widgetWithText(PopupMenuItem<String>, 'AI 对话'));
      await t.pumpAndSettle();
      expect(find.text('无效的学习画布'), findsNothing);
      expect(find.textContaining('已移除失效'), findsOneWidget);
      expect(api.selection!['objects'], ['text']);
      expect(api.conversations.values.single['query']['objects'], ['text']);
      expect(api.conversations.values.single['draft'], '保留我的问题');
      expect(
        api.request,
        isNull,
        reason: 'repair must not send a model request',
      );
      // A card deleted while the conversation is open must not silently send
      // the question with a different reference set.
      key.currentState!.document!.cards.removeWhere((c) => c.id == 'text');
      await t.tap(find.byTooltip('发送').last);
      await t.pumpAndSettle();
      expect(api.request, isNull);
      expect(api.conversations.values.single['query']['objects'], isEmpty);
      expect(api.conversations.values.single['draft'], '保留我的问题');
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
