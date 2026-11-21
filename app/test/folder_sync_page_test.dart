import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/features/settings/folder_sync_page.dart';

void main() {
  // The form polls and its text fields blink a cursor, so the tests step frames
  // by hand instead of waiting for the tree to settle.
  Future<void> settle(WidgetTester tester) async {
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
  }

  testWidgets('a WebDAV account plus selected courses is all the sync needs', (
    tester,
  ) async {
    final calls = <String>[];
    Map? saved;
    Future<dynamic> call(String m, String p, [Object? body]) async {
      calls.add('$m $p');
      if (p == '/api/sync/folder/preview') {
        return {
          'courses': {'pilot': '操作系统期末速成', 'other': '另一门课'},
          'files': 165,
          'bytes': 433000000,
        };
      }
      if (m == 'PUT') {
        saved = body as Map;
        return {};
      }
      return {'kind': 'folder', 'configured': false};
    }

    tester.view.physicalSize = const Size(1000, 2200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(MaterialApp(home: FolderSyncPage(api: call)));
    await settle(tester);
    await tester.enterText(
      find.widgetWithText(TextField, '服务器地址'),
      'https://nas.example.com/dav/',
    );
    await tester.enterText(find.widgetWithText(TextField, '账号'), 'me');
    await tester.enterText(find.widgetWithText(TextField, '应用密码'), 'secret');
    await settle(tester);
    await tester.tap(find.text('刷新课程'));
    await settle(tester);
    expect(find.text('操作系统期末速成'), findsOneWidget);
    await tester.tap(find.text('操作系统期末速成'));
    await settle(tester);
    await tester.ensureVisible(find.text('连接并同步'));
    await tester.tap(find.text('连接并同步'));
    await settle(tester);
    expect(find.text('连接课程同步'), findsOneWidget);
    expect(saved, isNull);
    await tester.tap(find.widgetWithText(FilledButton, '连接并同步').last);
    await settle(tester);
    expect(saved?['course_ids'], ['pilot']);
    expect(saved?['kind'], 'folder');
    expect(saved?['url'], 'https://nas.example.com/dav/');
    expect(saved?['password'], 'secret');
    expect(calls, contains('POST /api/sync/run'));
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('shared folder reconnects without WebDAV credentials', (
    tester,
  ) async {
    Map? saved;
    Future<dynamic> call(String m, String p, [Object? body]) async {
      if (m == 'PUT') saved = body as Map;
      if (p.endsWith('/preview'))
        return {
          'courses': {'pilot': '操作系统速成'},
        };
      return {
        'kind': 'folder',
        'configured': true,
        'local_folder': '/sync/Coursive',
        'course_ids': ['pilot'],
        'media': true,
        'auto': true,
      };
    }

    tester.view.physicalSize = const Size(1000, 2200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(MaterialApp(home: FolderSyncPage(api: call)));
    await settle(tester);
    expect(find.text('/sync/Coursive'), findsOneWidget);
    expect(find.widgetWithText(TextField, '服务器地址'), findsNothing);
    expect(find.text('操作系统速成'), findsOneWidget);
    await tester.ensureVisible(find.text('连接并同步'));
    await tester.tap(find.text('连接并同步'));
    await settle(tester);
    await tester.tap(find.widgetWithText(FilledButton, '连接并同步').last);
    await settle(tester);
    expect(saved?['local_folder'], '/sync/Coursive');
    expect(saved?['course_ids'], ['pilot']);
    expect(saved?.containsKey('url'), false);
    expect(saved?.containsKey('password'), false);
    await tester.tap(find.text('使用 WebDAV'));
    await settle(tester);
    expect(find.widgetWithText(TextField, '服务器地址'), findsOneWidget);
    expect(find.text('已连接同步文件夹'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('the form becomes editable only after loading the saved values', (
    tester,
  ) async {
    final first = Completer<Map>();
    await tester.pumpWidget(
      MaterialApp(
        home: FolderSyncPage(api: (m, p, [body]) async => first.future),
      ),
    );
    expect(find.byType(TextField), findsNothing);
    first.complete({
      'kind': 'folder',
      'configured': true,
      'url': 'https://old.example',
    });
    await settle(tester);
    final field = find.widgetWithText(TextField, '服务器地址');
    await tester.enterText(field, 'https://new.example');
    await tester.pump(const Duration(seconds: 6));
    expect(
      tester.widget<TextField>(field).controller!.text,
      'https://new.example',
    );
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets(
    'Android reopens the configured tree and retains it while previewing another',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      const channel = MethodChannel('coursive/android');
      const current = 'content://provider/tree/current';
      const candidate = 'content://provider/tree/candidate';
      final messenger = tester.binding.defaultBinaryMessenger;
      final events = <String>[];
      messenger.setMockMethodCallHandler(channel, (call) async {
        events.add(call.method);
        final restore = call.method == 'syncFolderBridge';
        expect(call.arguments[restore ? 'tree' : 'configuredTree'], current);
        return {
          'local_folder': restore ? current : candidate,
          'bridge_url': 'http://127.0.0.1:12345',
          'bridge_token': 'fixture',
        };
      });
      addTearDown(() => messenger.setMockMethodCallHandler(channel, null));
      final previews = <String>[];
      Future<dynamic> call(String method, String path, [Object? body]) async {
        expect(method, isNot('PUT'));
        if (path.endsWith('/preview')) {
          previews.add((body as Map)['local_folder'] as String);
          return {
            'courses': {'test': 'Test course'},
          };
        }
        return {
          'configured': true,
          'kind': 'folder',
          'local_folder': current,
          'course_ids': ['test'],
        };
      }

      try {
        await tester.pumpWidget(MaterialApp(home: FolderSyncPage(api: call)));
        await settle(tester);
        await tester.tap(find.text('选择 Syncthing 同步文件夹'));
        await settle(tester);
        expect(events, ['syncFolderBridge', 'pickSyncFolder']);
        expect(previews, [current, candidate]);
        expect(find.text('已连接同步文件夹'), findsOneWidget);
      } finally {
        await tester.pumpWidget(const SizedBox());
        debugDefaultTargetPlatformOverride = null;
      }
    },
  );

  for (final platform in [TargetPlatform.macOS, TargetPlatform.android]) {
    testWidgets(
      'directory picker cancel, rejected preview and confirmed save on ${platform.name}',
      (tester) async {
        debugDefaultTargetPlatformOverride = platform;
        final android = platform == TargetPlatform.android;
        final channel = MethodChannel(
          android ? 'coursive/android' : 'plugins.flutter.io/file_selector',
        );
        final messenger = tester.binding.defaultBinaryMessenger;
        final path = android
            ? 'content://provider/tree/selected'
            : '/sync/Coursive';
        Object? choice;
        var picks = 0;
        messenger.setMockMethodCallHandler(channel, (call) async {
          expect(call.method, android ? 'pickSyncFolder' : 'getDirectoryPath');
          picks++;
          return choice;
        });
        addTearDown(() => messenger.setMockMethodCallHandler(channel, null));
        tester.view.physicalSize = const Size(1000, 2200);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.reset);
        final previews = <Map>[];
        Map? saved;
        var reject = true;
        Future<dynamic> call(
          String method,
          String route, [
          Object? body,
        ]) async {
          if (route.endsWith('/preview')) {
            previews.add(body as Map);
            if (reject) throw StateError('folder unavailable');
            return {
              'courses': {'test': 'Test course'},
            };
          }
          if (method == 'PUT') saved = body as Map;
          return {
            'configured': true,
            'kind': 'folder',
            'url': 'https://saved.example',
            'user': 'local',
          };
        }

        try {
          await tester.pumpWidget(MaterialApp(home: FolderSyncPage(api: call)));
          await settle(tester);
          final pick = find.text('选择 Syncthing 同步文件夹');
          await tester.tap(pick);
          await settle(tester);
          expect(picks, 1);
          expect(previews, isEmpty);
          expect(saved, isNull);
          choice = android
              ? {
                  'local_folder': path,
                  'bridge_url': 'http://127.0.0.1:12345',
                  'bridge_token': 'fixture',
                }
              : path;
          await tester.tap(pick);
          await settle(tester);
          expect(find.textContaining('folder unavailable'), findsOneWidget);
          expect(saved, isNull);
          reject = false;
          await tester.tap(find.text('刷新课程'));
          await settle(tester);
          await tester.tap(find.text('Test course'));
          await settle(tester);
          await tester.ensureVisible(find.text('连接并同步'));
          await tester.tap(find.text('连接并同步'));
          await settle(tester);
          expect(saved, isNull);
          await tester.tap(find.widgetWithText(FilledButton, '连接并同步').last);
          await settle(tester);
          expect(saved, previews.last);
          expect(saved?['local_folder'], path);
          expect(saved?['course_ids'], ['test']);
          expect(saved!.keys, isNot(contains('url')));
          expect(saved!.keys, isNot(contains('password')));
          if (android) expect(saved?['bridge_token'], 'fixture');
        } finally {
          await tester.pumpWidget(const SizedBox());
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  }
}
