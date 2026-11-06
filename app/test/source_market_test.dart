import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/features/plugins/plugins_page.dart';
import 'package:coursive/features/sources/source_market.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:provider/provider.dart';

const svgIcon =
    'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxIDEiPjxyZWN0IHdpZHRoPSIxIiBoZWlnaHQ9IjEiLz48L3N2Zz4=';
const pngIcon =
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

/// Plays the daemon's market and source routes for one listed plugin.
class MarketApi extends Api {
  MarketApi() : super('http://localhost', 'test');
  bool installed = false;
  String notice = '';
  bool listed = true;
  final calls = <String>[];

  Map<String, dynamic> get _plugin => {
    'id': 'uni',
    'title': '示例大学',
    'summary': '录播课和作业',
    'version': '0.1.0',
    'enabled': true,
    'available': true,
    'hosts': ['lectures.example.edu'],
    'sources': [
      {
        'id': 'uni/uni',
        'plugin': 'uni',
        'title': '示例大学',
        'process': 'ready',
        'auth': {'state': 'signed_out'},
        'fields': [
          {'key': 'username', 'title': '学号', 'kind': 'text'},
        ],
      },
    ],
  };

  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async {
    calls.add('GET $path');
    return switch (path) {
      '/api/sources/market' => {
        'configured': true,
        'notice': notice,
        'plugins': [
          if (listed)
            {
              'id': 'uni',
              'title': '示例大学',
              'summary': '录播课和作业',
              'icon': svgIcon,
              'author': 'WALKERKILLER',
              'official': true,
              'version': '0.1.0',
              'available': true,
              'installed': installed ? '0.1.0' : '',
            },
        ],
      },
      '/api/sources' => {
        'plugins': [if (installed) _plugin],
      },
      _ => throw ApiError(404, path),
    };
  }

  @override
  Future<dynamic> call(String method, String path, [Object? body]) async {
    calls.add('$method $path');
    if (path == '/api/sources/market/uni/stage') {
      return {'token': 't1', 'plugin': _plugin};
    }
    if (path == '/api/sources/market/install') {
      expect((body as Map)['token'], 't1');
      installed = true;
      return _plugin;
    }
    throw ApiError(404, path);
  }

  @override
  Future<dynamic> delete(String path) async {
    calls.add('DELETE $path');
    installed = false;
    return {'ok': true};
  }
}

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

Future<_App> pump(WidgetTester tester, MarketApi api) async {
  tester.view.physicalSize = const Size(1000, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final app = _App(api);
  await app.loadSources();
  await tester.pumpWidget(
    ChangeNotifierProvider<AppState>.value(
      value: app,
      child: const MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(child: SourceMarketSection()),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return app;
}

void main() {
  testWidgets('plugin glyphs render image icons and symbol names', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Row(
          children: [
            PluginGlyph(icon: svgIcon),
            PluginGlyph(icon: pngIcon),
            PluginGlyph(icon: 'school'),
            PluginGlyph(icon: 'data:image/png;base64,%%%'),
          ],
        ),
      ),
    );
    expect(find.byType(SvgPicture), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
    expect(find.byIcon(Icons.school_outlined), findsOneWidget);
    // A broken data: URL falls back to the generic icon.
    expect(find.byIcon(Icons.extension_outlined), findsOneWidget);
  });

  testWidgets('a listed source installs after consent and offers sign-in', (
    tester,
  ) async {
    final api = MarketApi();
    await pump(tester, api);
    expect(find.text('示例大学'), findsOneWidget);
    expect(find.text('Coursive 官方'), findsOneWidget);
    expect(find.text('版本 0.1.0'), findsOneWidget);
    // The listed icon is the package's image.
    expect(find.byType(SvgPicture), findsOneWidget);

    await tester.tap(find.widgetWithText(FilledButton, '安装'));
    await tester.pumpAndSettle();
    expect(find.text('安装「示例大学」？'), findsOneWidget);
    expect(find.text('lectures.example.edu'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, '安装插件'));
    await tester.pumpAndSettle();

    expect(api.calls, contains('POST /api/sources/market/uni/stage'));
    expect(api.calls, contains('POST /api/sources/market/install'));
    // Installing offers to sign the new source in right away.
    expect(find.text('登录「示例大学」'), findsOneWidget);
  });

  testWidgets('an installed source shows its state, sign-in and uninstall', (
    tester,
  ) async {
    final api = MarketApi()..installed = true;
    await pump(tester, api);
    expect(find.text('未登录'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, '登录'), findsOneWidget);

    await tester.tap(find.widgetWithText(TextButton, '卸载'));
    await tester.pumpAndSettle();
    expect(find.text('卸载「示例大学」？'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, '卸载插件'));
    await tester.pumpAndSettle();

    expect(api.calls, contains('DELETE /api/sources/uni'));
    expect(find.widgetWithText(FilledButton, '安装'), findsOneWidget);
  });

  for (final (notice, listed, text) in [
    ('unverified', true, '无法验证插件目录，以下是上次验证过的目录。'),
    ('stale', true, '无法更新插件目录，以下是上次验证过的目录。'),
    ('unverified', false, '无法验证插件目录，已忽略这次下载的目录。'),
  ]) {
    testWidgets('the market notes a $notice index (listed: $listed)', (
      tester,
    ) async {
      await pump(
        tester,
        MarketApi()
          ..notice = notice
          ..listed = listed,
      );
      expect(find.text(text), findsOneWidget);
      expect(find.text('示例大学'), listed ? findsOneWidget : findsNothing);
      expect(find.textContaining('还没有来源插件'), findsNothing);
    });
  }

  testWidgets('a verified index shows no note', (tester) async {
    await pump(tester, MarketApi());
    expect(find.byIcon(Icons.gpp_maybe_outlined), findsNothing);
  });
}
