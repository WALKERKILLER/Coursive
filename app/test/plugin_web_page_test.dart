import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:coursive/core/plugin_navigation.dart';
import 'package:coursive/features/plugins/plugin_web_fallback.dart';
import 'package:coursive/features/plugins/plugin_web_page.dart';
import 'package:coursive/l10n/l10n.dart';

const _page = PluginPageContribution(
  id: 'probe',
  pluginId: 'web-probe',
  title: '探针',
  renderer: 'web-ui',
);

/// A daemon stand-in: answers are looked up by "METHOD path", and every call
/// is recorded.
class _Daemon {
  _Daemon(this.answers);
  final Map<String, Object? Function(Object? body)> answers;
  final calls = <(String, String, Object?)>[];

  Future<dynamic> call(String method, String path, [Object? body]) async {
    calls.add((method, path, body));
    final answer = answers['$method $path'];
    if (answer == null) throw StateError('unexpected $method $path');
    return answer(body);
  }
}

Map<String, dynamic> _opened({bool preview = true}) => {
  'admission': {'platform': 'android', 'status': 'supported'},
  'preview': preview,
  'session': {
    'id': 'web-1',
    'pluginId': 'web-probe',
    'pageId': 'probe',
    'instanceId': 'web-1',
    'origin': 'coursive-plugin://web-probe',
    'baseUrl': 'coursive-plugin://web-probe/',
  },
  'html': '<!doctype html>',
};

void main() {
  test(
    'a session relays messages with the frame and settles writes with the user',
    () async {
      final proposal = {
        'id': 'proposal-1',
        'pluginId': 'web-probe',
        'actionId': 'save',
        'summary': '保存复习卡',
        'status': 'pending',
      };
      var approve = true;
      final daemon = _Daemon({
        'POST /api/plugin-web/sessions': (_) => _opened(),
        'POST /api/plugin-web/sessions/web-1/messages': (body) {
          final message = jsonDecode((body as Map)['message'] as String) as Map;
          return message['action'] == 'save'
              ? {
                  'ok': true,
                  'requestId': message['requestId'],
                  'result': {'proposal': proposal},
                }
              : {
                  'ok': true,
                  'requestId': message['requestId'],
                  'result': {
                    'notification': {'text': '3 张卡片'},
                  },
                };
        },
        'POST /api/plugin-actions/proposals/proposal-1/apply': (_) => {
          'ok': true,
          'requestId': 'web-forwarded',
          'result': {
            'notification': {'text': '已保存'},
          },
        },
        'POST /api/plugin-actions/proposals/proposal-1/reject': (_) => {
          'ok': true,
          'requestId': 'web-forwarded',
          'result': {
            'proposal': {...proposal, 'status': 'rejected'},
          },
        },
        'POST /api/plugin-web/sessions/web-1/navigate': (body) => {
          'decision': (body as Map)['url'] == 'https://example.org/'
              ? 'external'
              : 'deny',
          'reason': 'x',
        },
        'DELETE /api/plugin-web/sessions/web-1': (_) => {'ok': true},
      });
      final asked = <String>[];
      final session = PluginWebSession(
        call: daemon.call,
        confirm: (p) async {
          asked.add(p.summary);
          return approve;
        },
      );
      final opening = await session.open(
        pluginId: 'web-probe',
        pageId: 'probe',
        platform: 'android',
      );
      expect(opening.renders, isTrue);
      expect(opening.preview, isTrue);
      expect(opening.baseUrl, 'coursive-plugin://web-probe/');
      expect(daemon.calls.single.$3, {
        'pluginId': 'web-probe',
        'pageId': 'probe',
        'platform': 'android',
      });

      String message(String id, String action) =>
          jsonEncode({'requestId': id, 'action': action});
      final read = jsonDecode(
        await session.relay(message('r1', 'cards'), 'about:blank'),
      ) as Map;
      expect(read['requestId'], 'r1');
      expect(read['result']['notification']['text'], '3 张卡片');
      // The raw channel text and the recorded frame go to the daemon unchanged.
      expect(daemon.calls.last.$3, {
        'frameUrl': 'about:blank',
        'message': message('r1', 'cards'),
      });
      expect(asked, isEmpty);

      final applied = jsonDecode(
        await session.relay(
          message('w1', 'save'),
          'coursive-plugin://web-probe/',
        ),
      ) as Map;
      expect(asked, ['保存复习卡']);
      expect(applied['requestId'], 'w1');
      expect(applied['result']['notification']['text'], '已保存');

      approve = false;
      final declined = jsonDecode(
        await session.relay(
          message('w2', 'save'),
          'coursive-plugin://web-probe/',
        ),
      ) as Map;
      expect(declined['requestId'], 'w2');
      expect(declined['result']['proposal']['status'], 'rejected');
      expect(daemon.calls.where((c) => c.$2.endsWith('/apply')).length, 1);

      expect(
        await session.navigate('https://example.org/'),
        PluginWebNavigation.external,
      );
      expect(
        await session.navigate('http://127.0.0.1:1/'),
        PluginWebNavigation.deny,
      );

      await session.close();
      await session.close();
      expect(daemon.calls.where((c) => c.$1 == 'DELETE').length, 1);
    },
  );

  testWidgets(
    'a platform the daemon does not admit shows its fallback reason',
    (tester) async {
      for (final (reason, text) in [
        ('platform-needs-review', l10n.pluginWebReasonNeedsReview),
        ('review-pending', l10n.pluginWebReasonReviewPending),
        ('resources-missing', l10n.pluginWebReasonResourcesMissing),
      ]) {
        final daemon = _Daemon({
          'POST /api/plugin-web/sessions': (_) => {
            'admission': {'platform': 'linux', 'status': 'needs-review'},
            'preview': false,
            'fallback': {
              'reason': reason,
              'detail': '网页资源 web/probe.js 与安装时不一致',
            },
          },
        });
        await tester.pumpWidget(
          MaterialApp(
            home: PluginWebPage(
              key: ValueKey(reason),
              page: _page,
              call: daemon.call,
              platform: 'linux',
              renderer: (_, _, _) =>
                  throw StateError('rendered a fallback page'),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(
          find.byKey(const ValueKey('plugin-web-fallback')),
          findsOneWidget,
        );
        expect(find.text(text), findsOneWidget);
        expect(find.text('网页资源 web/probe.js 与安装时不一致'), findsOneWidget);
        expect(find.text(l10n.pluginWebAdmissionNeedsReview), findsOneWidget);
        expect(find.text(l10n.pluginWebOpenNative), findsNothing);
      }
    },
  );

  testWidgets(
    'an admitted page renders with the preview notice and closes its session',
    (tester) async {
      final daemon = _Daemon({
        'POST /api/plugin-web/sessions': (_) => _opened(),
        'DELETE /api/plugin-web/sessions/web-1': (_) => {'ok': true},
      });
      PluginWebOpening? rendered;
      await tester.pumpWidget(
        MaterialApp(
          home: PluginWebPage(
            page: _page,
            call: daemon.call,
            platform: 'android',
            renderer: (_, _, opening) {
              rendered = opening;
              return const Text('webview');
            },
          ),
        ),
      );
      expect(find.byKey(const ValueKey('plugin-web-loading')), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.text('webview'), findsOneWidget);
      expect(rendered?.html, '<!doctype html>');
      expect(find.text(l10n.pluginWebPreview), findsOneWidget);

      await tester.pumpWidget(const MaterialApp(home: SizedBox()));
      await tester.pumpAndSettle();
      expect(daemon.calls.last.$1, 'DELETE');
    },
  );

  testWidgets('a reviewed platform renders without the preview notice', (
    tester,
  ) async {
    final daemon = _Daemon({
      'POST /api/plugin-web/sessions': (_) => _opened(preview: false),
      'DELETE /api/plugin-web/sessions/web-1': (_) => {'ok': true},
    });
    await tester.pumpWidget(
      MaterialApp(
        home: PluginWebPage(
          page: _page,
          call: daemon.call,
          platform: 'android',
          renderer: (_, _, _) => const Text('webview'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('webview'), findsOneWidget);
    expect(find.byKey(const ValueKey('plugin-web-preview')), findsNothing);
  });

  testWidgets('a fallback offers the page native document', (tester) async {
    final daemon = _Daemon({
      'POST /api/plugin-web/sessions': (_) => {
        'admission': {'platform': 'linux', 'status': 'needs-review'},
        'fallback': {'reason': 'platform-needs-review'},
      },
      'GET /api/plugin-actions/state?pluginId=web-probe&viewId=probe': (_) => {
        'viewId': 'probe',
        'revision': 0,
        'state': {},
      },
    });
    const page = PluginPageContribution(
      id: 'probe',
      pluginId: 'web-probe',
      title: '探针',
      renderer: 'web-ui',
      view: {
        'document': {
          'schemaVersion': 1,
          'body': {'type': 'text', 'text': '原生探针'},
        },
      },
    );
    await tester.pumpWidget(
      MaterialApp(
        home: PluginWebPage(page: page, call: daemon.call, platform: 'linux'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.pluginWebOpenNative));
    await tester.pumpAndSettle();
    expect(find.text('原生探针'), findsOneWidget);
  });

  testWidgets(
    'a daemon that cannot be reached falls back instead of a blank page',
    (tester) async {
      final daemon = _Daemon({});
      await tester.pumpWidget(
        MaterialApp(
          home: PluginPageOutlet(
            page: _page,
            builders: PluginPageBuilderRegistry({
              'web-ui': (context, page) => PluginWebPage(
                page: page,
                call: daemon.call,
                platform: 'android',
              ),
            }),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('plugin-web-fallback')), findsOneWidget);
      expect(find.textContaining('unexpected POST'), findsOneWidget);
      expect(
        PluginWebFallbackReason.fromCode('review-pending'),
        PluginWebFallbackReason.reviewPending,
      );
    },
  );
}
