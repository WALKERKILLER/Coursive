import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/features/settings/diagnostics.dart';

class _Api extends Api {
  _Api() : super('http://unused', 'test');
  bool installed = false;
  String backend = 'poppler';
  bool healthFails = false;
  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async {
    switch (path) {
      case '/api/health':
        if (healthFails) throw Exception('unreachable');
        return {
          'ffmpeg': installed,
          'pdf_backend': backend,
          'media_tools': {
            'ffmpeg': installed,
            'study_pdf': installed,
            'ffprobe': installed,
            'pdfinfo': true,
            'pdftoppm': true,
            'pdftotext': installed,
          },
        };
      case '/api/agent/runtime':
      case '/api/agent/status':
      case '/agent/models':
        throw Exception('assistant unavailable');
      case '/api/asr':
        return {
          'channels': [
            {'enabled': false},
          ],
        };
    }
    throw StateError(path);
  }
}

class _App extends AppState {
  _App(this.client, super.daemon);
  final Api client;
  @override
  Api get api => client;
  @override
  bool get connected => true;
}

void main() {
  testWidgets(
    'assistant failure does not skip media checks; recheck sees installation',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1300, 1800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final api = _Api();
      final daemon = DaemonService()..state = DaemonState.ready;
      final app = _App(api, daemon)..agent = {'state': 'failed'};
      addTearDown(app.dispose);
      await tester.pumpWidget(
        MultiProvider(
          providers: [
            ChangeNotifierProvider<AppState>.value(value: app),
            ChangeNotifierProvider<DaemonService>.value(value: daemon),
          ],
          child: const MaterialApp(
            home: Scaffold(body: SingleChildScrollView(child: Diagnostics())),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('缺少组件：ffmpeg, ffprobe'), findsOneWidget);
      expect(find.text('缺少组件：pdftotext'), findsOneWidget);
      expect(find.textContaining('没有启用的识别渠道'), findsOneWidget);
      expect(find.text('安装说明'), findsNWidgets(2));
      await tester.tap(find.text('安装说明').first);
      await tester.pumpAndSettle();
      if (Platform.isMacOS) {
        expect(find.text('brew install ffmpeg'), findsOneWidget);
      } else if (Platform.isLinux) {
        expect(find.text('sudo apt install ffmpeg'), findsOneWidget);
      }
      expect(
        find.textContaining('https://ffmpeg.org/download.html'),
        findsOneWidget,
      );
      await tester.tap(find.text('关闭'));
      await tester.pumpAndSettle();
      api.installed = true;
      await tester.tap(find.text('重新检查'));
      await tester.pumpAndSettle();
      expect(find.text('组件已找到（未测试解码）'), findsNWidgets(2));
      expect(find.text('安装说明'), findsNothing);
      expect(app.health['ffmpeg'], isTrue);
      api.healthFails = true;
      await tester.tap(find.text('重新检查'));
      await tester.pumpAndSettle();
      expect(find.text('组件已找到（未测试解码）'), findsNothing);
      api.healthFails = false;
      api.installed = false;
      api.backend = 'pdfkit';
      await tester.tap(find.text('重新检查'));
      await tester.pumpAndSettle();
      expect(find.textContaining('内置 PDFKit 组件缺失'), findsOneWidget);
      expect(find.text('缺少组件：pdftotext'), findsNothing);
      api.backend = 'unsupported';
      await tester.tap(find.text('重新检查'));
      await tester.pumpAndSettle();
      expect(find.textContaining('当前平台的画布 PDF 导入尚未支持'), findsOneWidget);
      expect(find.text('安装说明'), findsOneWidget);

      expect(find.text('未检查成功，请重新检查'), findsWidgets);
    },
  );
}
