import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/features/courses/local_course_dialog.dart';
import 'package:coursive/features/home/first_course.dart';
import 'package:provider/provider.dart';

class _App extends AppState {
  _App() : super(DaemonService());
  final _api = Api('http://localhost', 'test');
  @override
  Api get api => _api;
  @override
  bool get connected => true;
}

Future<void> pump(WidgetTester tester, double width) async {
  tester.view.physicalSize = Size(width, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ChangeNotifierProvider<AppState>.value(
      value: _App(),
      child: const MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            padding: EdgeInsets.all(16),
            child: FirstCoursePanel(),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  for (final (name, width) in [('wide', 1000.0), ('compact', 380.0)]) {
    testWidgets('$name: the panel offers both ways to start a course', (
      tester,
    ) async {
      await pump(tester, width);
      expect(find.text('添加第一门课'), findsOneWidget);
      expect(find.widgetWithText(FilledButton, '新建课程'), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, '连接课程平台'), findsOneWidget);
      expect(find.text('录像、转写和笔记都保存在本机'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('新建课程 opens the create window', (tester) async {
    await pump(tester, 1000);
    await tester.tap(find.widgetWithText(FilledButton, '新建课程'));
    await tester.pumpAndSettle();
    expect(find.byType(LocalCourseDialog), findsOneWidget);
  });
}
