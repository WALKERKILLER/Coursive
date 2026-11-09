import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/core/plugin_navigation.dart';
import 'package:coursive/core/plugins.dart';

PluginPageContribution _page(
  String id, {
  String plugin = 'demo',
  int order = 100,
  String? route,
}) => PluginPageContribution(
  id: id,
  pluginId: plugin,
  title: id,
  order: order,
  route: route,
);

void main() {
  test('pages sort deterministically and registration is reversible', () {
    final registry = PluginNavigationRegistry();
    final disposeB = registry.register(_page('b', plugin: 'z', order: 20));
    final disposeA = registry.register(_page('a', plugin: 'a', order: 20));
    registry.register(_page('first', plugin: 'a', order: 10));
    expect(registry.pages.map((p) => p.id), ['first', 'a', 'b']);
    disposeA();
    expect(registry.pages.map((p) => p.id), ['first', 'b']);
    disposeB();
    expect(registry.pages.map((p) => p.id), ['first']);
  });

  test('duplicate route rejects the whole replaceAll batch', () {
    final registry = PluginNavigationRegistry();
    registry.register(_page('keep'));
    expect(
      () => registry.replaceAll([
        _page('a', route: 'same'),
        _page('b', route: 'same'),
      ]),
      throwsArgumentError,
    );
    expect(registry.pages.single.id, 'keep');
  });

  test('entry fields map to a contribution and plugin removal is scoped', () {
    final entry = PluginEntry(
      {'id': 'review', 'title': 'Review'},
      'pages',
      {
        'id': 'main',
        'title': '复习',
        'icon': 'style',
        'route': 'plugin:review:main',
        'renderer': 'declarative',
        'order': 7,
        'required_services': ['library.read'],
        'required_permissions': ['views'],
        'available_on': ['linux', 'android'],
        'min_host_version': '0.2.0',
        'fallback_route': 'plugins',
      },
    );
    final page = PluginPageContribution.fromEntry(entry);
    expect(page.routeId, 'plugin:review:main');
    expect(page.order, 7);
    expect(page.requiredServices, ['library.read']);
    expect(page.requiredPermissions, ['views']);
    expect(
      page.availableOnThisHost(hostVersion: '0.1.9', platform: 'linux'),
      isFalse,
    );
    expect(
      page.availableOnThisHost(hostVersion: '0.2.0', platform: 'linux'),
      isTrue,
    );
    expect(
      page.availableOnThisHost(hostVersion: '0.2.0', platform: 'windows'),
      isFalse,
    );

    final registry = PluginNavigationRegistry();
    registry.register(page);
    registry.register(_page('other', plugin: 'other'));
    registry.removePlugin('review');
    expect(registry.pages.map((p) => p.pluginId), ['other']);
  });

  testWidgets(
    'page outlet uses the registered renderer and falls back safely',
    (tester) async {
      final builders = PluginPageBuilderRegistry({
        'declarative': (context, page) => Text('body:${page.title}'),
      });
      await tester.pumpWidget(
        MaterialApp(
          home: PluginPageOutlet(page: _page('review'), builders: builders),
        ),
      );
      expect(find.text('review'), findsOneWidget);
      expect(find.text('body:review'), findsOneWidget);

      await tester.pumpWidget(
        MaterialApp(
          home: PluginPageOutlet(
            page: _page('unknown').copyWithRenderer('missing'),
            builders: builders,
          ),
        ),
      );
      expect(find.text('unknown'), findsWidgets);
      expect(find.textContaining('当前版本的应用还不能显示'), findsOneWidget);
    },
  );

  testWidgets('launcher lists active plugin pages and opens one', (
    tester,
  ) async {
    final store = PluginStore(AppState(DaemonService()))
      ..installed = [
        {
          'id': 'review',
          'title': 'Review',
          'state': 'enabled',
          'contributes': {
            'pages': [
              {'id': 'main', 'title': '记忆复习', 'icon': 'style'},
            ],
          },
        },
      ];
    final builders = PluginPageBuilderRegistry({
      'declarative': (context, page) => Text('opened:${page.title}'),
    });
    await tester.pumpWidget(
      Provider<PluginPageBuilderRegistry>.value(
        value: builders,
        child: MaterialApp(
          home: Scaffold(body: PluginPageLauncher(store: store)),
        ),
      ),
    );
    expect(find.text('记忆复习'), findsOneWidget);
    await tester.tap(find.text('记忆复习'));
    await tester.pumpAndSettle();
    expect(find.text('opened:记忆复习'), findsOneWidget);
  });
}

extension on PluginPageContribution {
  PluginPageContribution copyWithRenderer(String renderer) =>
      PluginPageContribution(
        id: id,
        pluginId: pluginId,
        title: title,
        icon: icon,
        renderer: renderer,
        route: route,
        order: order,
        requiredServices: requiredServices,
        requiredPermissions: requiredPermissions,
        availableOn: availableOn,
        minHostVersion: minHostVersion,
        fallbackRoute: fallbackRoute,
        view: view,
      );
}
