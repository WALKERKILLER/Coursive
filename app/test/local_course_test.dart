import 'dart:async';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/core/models.dart';
import 'package:coursive/features/courses/courses_page.dart';
import 'package:coursive/features/courses/local_course_dialog.dart';
import 'package:provider/provider.dart';

class LocalApi extends Api {
  LocalApi() : super('http://localhost', 'test');
  int creates = 0;
  bool failSecond = true;
  final uploaded = <String>[];
  @override
  Future<dynamic> createLocalCourse(String title, {String teacher = ''}) async {
    creates++;
    return {'id': 'local-test'};
  }

  @override
  Future<dynamic> importLocalVideo(
    String course,
    String name,
    Stream<List<int>> data,
    int length,
  ) async {
    await data.drain<void>();
    if (name == '02.mp4' && failSecond) throw ApiError(400, '无法读取视频');
    uploaded.add(name);
    return {'id': 'lesson'};
  }
}

/// Holds course creation until [gate] completes.
class _SlowApi extends LocalApi {
  final gate = Completer<void>();
  @override
  Future<dynamic> createLocalCourse(String title, {String teacher = ''}) async {
    await gate.future;
    return super.createLocalCourse(title, teacher: teacher);
  }
}

/// A connected app with no source plugin and no course.
class _App extends AppState {
  _App(this._api) : super(DaemonService());
  final Api _api;
  @override
  Api get api => _api;
  @override
  bool get connected => true;
  @override
  Future<void> loadCourses({bool sync = false, bool quiet = false}) async {}
}

XFile video(String name) => XFile.fromData(Uint8List(1), path: name);

/// Reports a size past the import limit without holding the bytes.
class _Huge extends XFile {
  _Huge(String name) : super.fromData(Uint8List(1), path: name);
  @override
  Future<int> length() async => 5 << 30;
}

Future<void> pumpDialog(
  WidgetTester tester,
  LocalApi api,
  List<XFile> files,
) async {
  tester.view.physicalSize = const Size(900, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: LocalCourseDialog(api: api, pickFiles: () async => files),
      ),
    ),
  );
}

void main() {
  testWidgets(
    'local course creation validates name and resumes remaining uploads',
    (tester) async {
      final api = LocalApi();
      await pumpDialog(tester, api, [video('02.mp4'), video('01.mp4')]);
      expect(find.text('新建课程'), findsOneWidget);
      await tester.tap(find.widgetWithText(FilledButton, '创建课程'));
      await tester.pump();
      expect(find.text('请填写课程名称'), findsOneWidget);
      expect(api.creates, 0);
      await tester.enterText(find.byType(TextFormField).first, '操作系统期末速成');
      await tester.tap(find.text('选择视频'));
      await tester.pumpAndSettle();
      expect(find.text('2 个视频 · 共 2 B'), findsOneWidget);
      await tester.tap(find.widgetWithText(FilledButton, '创建并导入 2 个视频'));
      await tester.pumpAndSettle();
      expect(api.creates, 1);
      expect(api.uploaded, ['01.mp4']);
      expect(find.text('无法导入“02.mp4”：无法读取视频'), findsOneWidget);
      expect(find.widgetWithText(TextButton, '稍后添加'), findsOneWidget);
      api.failSecond = false;
      await tester.tap(find.widgetWithText(FilledButton, '继续导入'));
      await tester.pumpAndSettle();
      expect(api.creates, 1);
      expect(api.uploaded, ['01.mp4', '02.mp4']);
    },
  );

  testWidgets('an oversized video blocks import until it is removed', (
    tester,
  ) async {
    final api = LocalApi()..failSecond = false;
    await pumpDialog(tester, api, [_Huge('01.mp4'), video('02.mp4')]);
    await tester.enterText(find.byType(TextFormField).first, '线性代数');
    await tester.tap(find.text('选择视频'));
    await tester.pumpAndSettle();
    expect(find.text('超过 4 GiB，请移除后再导入'), findsOneWidget);
    final submit = find.widgetWithText(FilledButton, '创建并导入 2 个视频');
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);
    await tester.tap(find.byTooltip('移除').first);
    await tester.pump();
    expect(find.text('01.mp4'), findsNothing);
    await tester.tap(find.widgetWithText(FilledButton, '创建并导入 1 个视频'));
    await tester.pumpAndSettle();
    expect(api.uploaded, ['02.mp4']);
  });

  testWidgets('an empty course list starts a local course', (tester) async {
    final app = _App(LocalApi());
    await tester.pumpWidget(
      ChangeNotifierProvider<AppState>.value(
        value: app,
        child: const MaterialApp(home: Scaffold(body: CoursesPage())),
      ),
    );
    expect(find.text('还没有课程'), findsOneWidget);
    expect(find.widgetWithText(TextButton, '安装来源插件'), findsOneWidget);
    expect(find.byTooltip('从来源同步课程'), findsNothing);
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.byType(SearchBar), findsNothing);
    await tester.tap(find.widgetWithText(FilledButton, '新建课程'));
    await tester.pumpAndSettle();
    expect(find.widgetWithText(FilledButton, '创建课程'), findsOneWidget);
    expect(find.text('课堂录像'), findsOneWidget);
  });

  testWidgets('the create window is a bottom sheet on compact windows', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(400, 860);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ChangeNotifierProvider<AppState>.value(
        value: _App(LocalApi()),
        child: const MaterialApp(home: Scaffold(body: CoursesPage())),
      ),
    );
    await tester.tap(find.widgetWithText(FilledButton, '新建课程'));
    await tester.pumpAndSettle();
    expect(find.byType(BottomSheet), findsOneWidget);
    expect(find.byType(Dialog), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('the create window closes from Esc and its barrier', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final api = _SlowApi();
    String? result = 'unset';
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (c) => Scaffold(
            body: TextButton(
              onPressed: () async =>
                  result = await openLocalCourseDialog(c, api),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    );
    Future<void> open() async {
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.byType(LocalCourseDialog), findsOneWidget);
    }

    await open();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byType(LocalCourseDialog), findsNothing);
    expect(result, isNull);

    await open();
    await tester.tapAt(const Offset(8, 8));
    await tester.pumpAndSettle();
    expect(find.byType(LocalCourseDialog), findsNothing);

    // While the course is being created the window stays open; closing it
    // afterwards still reports the course.
    await open();
    await tester.enterText(find.byType(TextFormField).first, '线性代数');
    await tester.tap(find.widgetWithText(FilledButton, '创建课程'));
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.tapAt(const Offset(8, 8));
    await tester.pump();
    expect(find.byType(LocalCourseDialog), findsOneWidget);
    api.gate.complete();
    await tester.pumpAndSettle();
    expect(find.byType(LocalCourseDialog), findsNothing);
    expect(result, 'local-test');
  });

  test('local course has a distinct label', () {
    expect(Course({'id': 'local-test', 'term': 'local'}).termLabel, '本地课程');
    expect(Course({'id': '40309', 'term': '20261'}).isLocal, false);
  });
}
