import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/plugin_navigation.dart';
import 'package:coursive/features/plugins/plugin_web_fallback.dart';
import 'package:coursive/l10n/l10n.dart';

PluginPageContribution _page({
  String title = '虚拟实验室',
  String renderer = 'web-ui',
  Map<String, dynamic> view = const {},
}) => PluginPageContribution(
  id: 'lab',
  pluginId: 'virtual-lab',
  title: title,
  renderer: renderer,
  view: view,
);

void main() {
  test('admission and reason codes match the daemon vocabulary', () {
    expect(
      PluginWebAdmission.fromCode('supported'),
      PluginWebAdmission.supported,
    );
    expect(
      PluginWebAdmission.fromCode('needs-review'),
      PluginWebAdmission.needsReview,
    );
    expect(
      PluginWebAdmission.fromCode('unsupported'),
      PluginWebAdmission.unsupported,
    );
    // An unknown or missing code never claims the platform is admitted.
    expect(PluginWebAdmission.fromCode(null), PluginWebAdmission.needsReview);
    expect(
      PluginWebAdmission.fromCode('whatever'),
      PluginWebAdmission.needsReview,
    );

    expect(
      PluginWebFallbackReason.fromCode('platform-unsupported'),
      PluginWebFallbackReason.platformUnsupported,
    );
    expect(
      PluginWebFallbackReason.fromCode('platform-needs-review'),
      PluginWebFallbackReason.platformNeedsReview,
    );
    expect(
      PluginWebFallbackReason.fromCode('resources-missing'),
      PluginWebFallbackReason.resourcesMissing,
    );
    // No published reason means this build has no WebView renderer.
    expect(
      PluginWebFallbackReason.fromCode(null),
      PluginWebFallbackReason.rendererMissing,
    );
  });

  testWidgets(
    'an unsupported platform explains itself and keeps L1/L2 usable',
    (tester) async {
      var openedNative = 0;
      await tester.pumpWidget(
        MaterialApp(
          home: PluginWebFallbackPage(
            pluginTitle: '虚拟实验室',
            admission: PluginWebAdmission.unsupported,
            reason: PluginWebFallbackReason.platformUnsupported,
            onOpenNative: () => openedNative++,
          ),
        ),
      );

      expect(find.text('虚拟实验室'), findsOneWidget);
      expect(find.text(l10n.pluginWebUnavailableTitle), findsOneWidget);
      expect(
        find.text(l10n.pluginWebReasonPlatformUnsupported),
        findsOneWidget,
      );
      expect(find.text(l10n.pluginWebAdmissionUnsupported), findsOneWidget);
      expect(find.text(l10n.pluginWebNativeKeptTitle), findsOneWidget);
      expect(find.text(l10n.pluginWebNativeKept), findsOneWidget);

      await tester.tap(find.text(l10n.pluginWebOpenNative));
      expect(openedNative, 1);
    },
  );

  testWidgets('a platform under review says so instead of failing silently', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: PluginWebFallbackPage(
          pluginTitle: '虚拟实验室',
          admission: PluginWebAdmission.needsReview,
          reason: PluginWebFallbackReason.platformNeedsReview,
        ),
      ),
    );

    expect(find.text(l10n.pluginWebReasonNeedsReview), findsOneWidget);
    expect(find.text(l10n.pluginWebAdmissionNeedsReview), findsOneWidget);
    expect(find.text(l10n.pluginWebReasonPlatformUnsupported), findsNothing);
    // No native page to open, so the page offers none.
    expect(find.text(l10n.pluginWebOpenNative), findsNothing);
    expect(find.text(l10n.commonBack), findsOneWidget);
  });

  testWidgets(
    'an admitted platform without a renderer reports the missing one',
    (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: PluginWebFallbackPage(
            pluginTitle: '虚拟实验室',
            admission: PluginWebAdmission.supported,
            reason: PluginWebFallbackReason.rendererMissing,
            detail: 'ui/index.html 缺失',
          ),
        ),
      );

      expect(find.text(l10n.pluginWebAdmissionSupported), findsOneWidget);
      expect(find.text(l10n.pluginWebReasonRendererMissing), findsOneWidget);
      expect(find.text('ui/index.html 缺失'), findsOneWidget);
    },
  );

  testWidgets(
    'the web-ui page builder shows the fallback instead of a blank page',
    (tester) async {
      final builders = PluginPageBuilderRegistry({
        'web-ui': pluginWebPageBuilder,
      });
      await tester.pumpWidget(
        MaterialApp(
          home: PluginPageOutlet(page: _page(), builders: builders),
        ),
      );
      expect(find.byKey(const ValueKey('plugin-web-fallback')), findsOneWidget);
      expect(find.text(l10n.pluginWebReasonRendererMissing), findsOneWidget);

      // A contribution that publishes its own admission and reason is explained
      // with those instead of the default.
      await tester.pumpWidget(
        MaterialApp(
          home: PluginPageOutlet(
            page: _page(
              view: const {
                'admission': 'unsupported',
                'fallbackReason': 'platform-unsupported',
              },
            ),
            builders: builders,
          ),
        ),
      );
      expect(
        find.text(l10n.pluginWebReasonPlatformUnsupported),
        findsOneWidget,
      );
      expect(find.text(l10n.pluginWebAdmissionUnsupported), findsOneWidget);
    },
  );

  testWidgets('back returns to the host page', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) =>
                      const PluginWebFallbackPage(pluginTitle: '虚拟实验室'),
                ),
              ),
              child: const Text('打开插件页'),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('打开插件页'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('plugin-web-fallback')), findsOneWidget);

    await tester.tap(find.text(l10n.commonBack));
    await tester.pumpAndSettle();
    expect(find.text('打开插件页'), findsOneWidget);
    expect(find.byKey(const ValueKey('plugin-web-fallback')), findsNothing);
  });
}
