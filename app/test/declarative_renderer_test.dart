import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/features/plugins/declarative_renderer.dart';

void main() {
  testWidgets('a binding with no value renders empty, not "null"', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: DeclarativeRenderer(
            document: {
              'schemaVersion': 1,
              'body': {
                'type': 'column',
                'children': [
                  {'type': 'text', 'text': r'${front}'},
                  {'type': 'button', 'text': r'${label}', 'action': 'go'},
                ],
              },
            },
          ),
        ),
      ),
    );
    expect(find.textContaining('null'), findsNothing);
    expect(find.byType(FilledButton), findsOneWidget);
  });

  testWidgets('renders text, cards, bindings and actions', (tester) async {
    String? action;
    Map<String, dynamic>? args;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: DeclarativeRenderer(
            document: {
              'schemaVersion': 1,
              'body': {
                'type': 'column',
                'children': [
                  {'type': 'text', 'variant': 'title', 'text': '今日复习'},
                  {
                    'type': 'text',
                    'text': {'ref': 'course'},
                  },
                  {
                    'type': 'button',
                    'text': '开始',
                    'onPress': {
                      'action': 'review.start',
                      'args': {'courseId': r'${courseId}'},
                    },
                  },
                ],
              },
            },
            state: {'course': '线性代数', 'courseId': 'c1'},
            onAction: (a, v) {
              action = a;
              args = v;
            },
          ),
        ),
      ),
    );
    expect(find.text('今日复习'), findsOneWidget);
    expect(find.text('线性代数'), findsOneWidget);
    await tester.tap(find.text('开始'));
    expect(action, 'review.start');
    expect(args, {'courseId': 'c1'});
  });

  testWidgets('unknown components render a diagnostic, not a crash', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: DeclarativeRenderer(
            document: {
              'body': {'type': 'graph', 'data': {}},
            },
          ),
        ),
      ),
    );
    expect(find.textContaining('graph'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('bad documents and depth limits return the error view', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: DeclarativeRenderer(
            document: {
              'schemaVersion': 99,
              'body': {'type': 'text', 'text': 'x'},
            },
          ),
        ),
      ),
    );
    expect(find.textContaining('当前版本的应用还不能显示'), findsOneWidget);

    Map<String, dynamic> deep = {'type': 'text', 'text': 'x'};
    for (var i = 0; i < 40; i++) {
      deep = {
        'type': 'column',
        'children': [deep],
      };
    }
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: DeclarativeRenderer(document: {'body': deep})),
      ),
    );
    expect(find.textContaining('当前版本的应用还不能显示'), findsOneWidget);
  });

  testWidgets('table, timeline and tabs use native widgets', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: DeclarativeRenderer(
              document: {
                'body': {
                  'type': 'column',
                  'children': [
                    {
                      'type': 'table',
                      'columns': ['项目', '状态'],
                      'rows': [
                        ['栈', '完成'],
                      ],
                    },
                    {
                      'type': 'timeline',
                      'items': [
                        {'time': '00:10', 'text': '开场'},
                      ],
                    },
                    {
                      'type': 'tabs',
                      'tabs': [
                        {
                          'label': 'A',
                          'children': [
                            {'type': 'text', 'text': 'tab-a'},
                          ],
                        },
                        {
                          'label': 'B',
                          'children': [
                            {'type': 'text', 'text': 'tab-b'},
                          ],
                        },
                      ],
                    },
                  ],
                },
              },
            ),
          ),
        ),
      ),
    );
    expect(find.text('项目'), findsOneWidget);
    expect(find.text('开场'), findsOneWidget);
    expect(find.text('tab-a'), findsOneWidget);
  });
}
