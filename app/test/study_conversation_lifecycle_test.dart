import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/core/api.dart';
import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

class SaveApi extends fixture.FixtureApi {
  SaveApi() : super(StudyDocument().json());
  bool fail = false;
  Completer<void>? gate;
  bool entered = false;
  Completer<Map<String, dynamic>>? startGate;
  final cancelled = <String>[];
  @override
  Future<dynamic> startCanvasRun(Map<String, dynamic> value) async {
    request = value;
    return startGate == null ? {'id': 'run'} : startGate!.future;
  }

  @override
  Future<dynamic> cancelCanvasRun(String id) async {
    cancelled.add(id);
    return {'id': id, 'state': 'cancelled'};
  }

  @override
  Future<dynamic> saveStudyConversation(
    String lecture,
    String id,
    Map data,
  ) async {
    if ((data['turns'] as List).any((t) => t['state'] == 'preparing')) {
      entered = true;
      if (gate != null) await gate!.future;
      if (fail) throw ApiError(503, 'temporary save failure');
    }
    return super.saveStudyConversation(lecture, id, data);
  }
}

Future<void> openConversation(WidgetTester t, SaveApi api) async {
  await fixture.open(t, api);
  await t.tap(find.byTooltip('添加模块'));
  await t.pumpAndSettle();
  await t.tap(find.widgetWithText(PopupMenuItem<String>, 'AI 对话'));
  await t.pumpAndSettle();
  await t.enterText(find.byType(TextField).last, '保留这个问题');
}

void main() {
  testWidgets(
    'saved snapshots survive reopen, cancellation and retry without consuming the next draft',
    (t) async {
      final api = SaveApi()..gate = Completer<void>();
      final snapshot = {
        'id': 'saved-snapshot',
        'resource': 'immutable.png',
        'sha256': 'original-pixels',
        'mime_type': 'image/png',
        'width': 120,
        'height': 80,
        'selection': {
          'bounds': [0, 0, 120, 80],
          'sources': [],
        },
      };
      api.doc =
          (StudyDocument()
                ..cards.add(
                  StudyCard(
                    id: 'chat',
                    type: 'conversation',
                    conversationId: 'saved',
                    layer: 'main',
                    rect: const Rect.fromLTWH(0, 0, 200, 200),
                  ),
                ))
              .json();
      api.conversations['saved'] = {
        'id': 'saved',
        'version': 2,
        'revision': 1,
        'draft': 'this screenshot',
        'context_scope': 'course',
        'draft_attachments': [snapshot],
        'query': {'objects': [], 'selection': {}, 'include_video': false},
        'turns': [],
      };
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      await t.tap(find.byTooltip('发送').last);
      await t.pump();
      expect(api.entered, true);
      await t.enterText(find.byType(TextField).last, 'next draft');
      await t.tap(find.byTooltip('停止').last);
      await t.pump();
      api.gate!.complete();
      await t.pumpAndSettle();
      expect(api.request, isNull);
      expect(api.conversations['saved']!['turns'].single['state'], 'cancelled');
      expect(api.conversations['saved']!['turns'].single['attachments'], [
        snapshot,
      ]);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      api.gate = null;
      await fixture.open(t, api);
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        'next draft',
      );
      await t.tap(find.widgetWithText(TextButton, '重试').last);
      await t.pumpAndSettle();
      expect(api.request!['attachments'], [snapshot]);
      expect(
        api.conversations['saved']!['turns'].single['id'],
        api.request!['turn_id'],
      );
      expect(api.request!['turn_id'], isNot('run'));
      expect(api.request!['question'], 'this screenshot');
      expect(api.uploadedSnapshot, isNull);
      expect(api.conversations['saved']!['turns'], hasLength(1));
      expect(api.conversations['saved']!['draft_attachments'], isEmpty);
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        'next draft',
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('save failure keeps both the failed question and a newer draft', (
    t,
  ) async {
    final api = SaveApi()
      ..gate = Completer<void>()
      ..fail = true;
    await openConversation(t, api);
    await t.tap(find.byTooltip('发送').last);
    await t.pump();
    await t.enterText(find.byType(TextField).last, '下一个问题');
    api.gate!.complete();
    await t.pumpAndSettle();
    expect(api.request, isNull);
    expect(
      t.widget<TextField>(find.byType(TextField).last).controller!.text,
      '下一个问题',
    );
    final saved = api.conversations.values.single;
    expect(saved['draft'], '下一个问题');
    expect((saved['turns'] as List).single['question'], '保留这个问题');
    expect((saved['turns'] as List).single['state'], 'error');
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'failed initial save preserves the unsent draft and permits retry',
    (t) async {
      final api = SaveApi()..fail = true;
      await openConversation(t, api);
      await t.tap(find.byTooltip('发送').last);
      await t.pumpAndSettle();
      expect(api.request, isNull);
      expect(
        t.widget<TextField>(find.byType(TextField).last).controller!.text,
        '保留这个问题',
      );
      api.fail = false;
      await t.tap(find.byTooltip('发送').last);
      await t.pumpAndSettle();
      expect(api.request!['question'], '保留这个问题');
      expect(api.conversations.values.single['turns'], hasLength(1));
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets(
    'closing while a question is saving never starts a model request',
    (t) async {
      final api = SaveApi()..gate = Completer<void>();
      await openConversation(t, api);
      await t.tap(find.byTooltip('发送').last);
      await t.pump();
      expect(api.entered, true);
      await t.pumpWidget(const SizedBox());
      api.gate!.complete();
      await t.pumpAndSettle();
      expect(api.request, isNull);
      expect(t.takeException(), isNull);
      expect(
        (api.conversations.values.single['turns'] as List).single['state'],
        'interrupted',
      );
    },
  );

  testWidgets('closing while the host creates a run cancels the late job', (
    t,
  ) async {
    final api = SaveApi()..startGate = Completer<Map<String, dynamic>>();
    await openConversation(t, api);
    await t.tap(find.byTooltip('发送').last);
    await t.pump();
    expect(api.request, isNotNull);
    await t.pumpWidget(const SizedBox());
    api.startGate!.complete({'id': 'late-run'});
    await t.pumpAndSettle();
    expect(api.cancelled, ['late-run']);
    expect(t.takeException(), isNull);
    expect(
      (api.conversations.values.single['turns'] as List).single['state'],
      'interrupted',
    );
  });

  testWidgets(
    'stop during the initial save cancels before contacting the model',
    (t) async {
      final api = SaveApi()..gate = Completer<void>();
      await openConversation(t, api);
      await t.tap(find.byTooltip('发送').last);
      await t.pump();
      await t.tap(find.byTooltip('停止').last);
      await t.pump();
      api.gate!.complete();
      await t.pumpAndSettle();
      expect(api.request, isNull);
      expect(
        (api.conversations.values.single['turns'] as List).single['state'],
        'cancelled',
      );
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
