import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/core/plugin_navigation.dart';
import 'package:coursive/core/plugins.dart';
import 'package:coursive/core/ui_slots.dart';
import 'package:coursive/features/plugins/plugin_contributions.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

class _Store extends PluginStore {
  _Store() : super(AppState(DaemonService()));

  void set(List<Map<String, dynamic>> plugins) {
    installed = plugins;
    notifyListeners();
  }
}

Map<String, dynamic> _doc(List<Map<String, dynamic>> children) => {
  'schemaVersion': 1,
  'body': {'type': 'column', 'children': children},
};

/// A v3 plugin as the assistant host lists it: one page, one course slot and
/// the actions its buttons name.
Map<String, dynamic> _cards({
  String id = 'cards',
  String state = 'enabled',
  String? route,
  String slotTarget = 'course.detail.sections',
}) => {
  'id': id,
  'title': 'Cards',
  'state': state,
  'schema': 3,
  'contributes': {
    'pages': [
      {
        'id': 'deck',
        'title': '复习卡',
        'renderer': 'declarative',
        'route': ?route,
        'document': _doc([
          {'type': 'text', 'text': r'${front}'},
          {'type': 'button', 'text': '下一张', 'action': 'next'},
          {'type': 'button', 'text': '保存', 'action': 'save'},
        ]),
      },
    ],
    'slots': [
      {
        'id': 'course-review',
        'target': slotTarget,
        'mode': 'list',
        'document': _doc([
          {'type': 'text', 'text': r'${context.course_id}'},
          {'type': 'button', 'text': '开始复习', 'action': 'open'},
        ]),
      },
    ],
    'actions': [
      {'id': 'next', 'title': '下一张', 'action': 'next'},
      {'id': 'save', 'title': '保存', 'action': 'save', 'write': true},
      {'id': 'open', 'title': '打开', 'action': 'open'},
    ],
  },
};

/// The daemon's plugin-action routes, answered from [answers] by action id.
class _Daemon {
  _Daemon(this.answers);
  final Map<String, Map<String, dynamic>> answers;
  final sent = <Map<String, dynamic>>[];
  final settled = <String>[];
  var reads = 0;

  Future<dynamic> call(String method, String path, [Object? body]) async {
    if (method == 'GET') {
      reads++;
      return {
        'viewId': 'deck',
        'revision': 2,
        'state': {'front': 'Q2'},
      };
    }
    if (path.startsWith('/api/plugin-actions/proposals/')) {
      settled.add(path);
      return answers[path.split('/').last];
    }
    final request = (body as Map).cast<String, dynamic>();
    sent.add(request);
    return answers['${request['actionId']}'];
  }
}

void main() {
  test('active v3 plugins feed pages and slots, and leave with the plugin', () {
    final store = _Store();
    final pages = PluginNavigationRegistry();
    final slots = UiSlotRegistry();
    final binder = PluginContributionBinder(
      store: store,
      pages: pages,
      slots: slots,
    );
    addTearDown(binder.dispose);

    store.set([
      _cards(),
      // A v1 page is not host-rendered; it keeps its own action path.
      {
        'id': 'legacy',
        'state': 'enabled',
        'contributes': {
          'pages': [
            {'id': 'main', 'title': 'Legacy', 'action': 'open'},
          ],
        },
      },
    ]);
    expect(pages.pages.map((p) => p.routeId), ['plugin:cards:deck']);
    expect(
      slots
          .contributions('course.detail.sections')
          .map((s) => s.registrationKey),
      ['cards/course-review'],
    );

    store.set([_cards(state: 'disabled')]);
    expect(pages.pages, isEmpty);
    expect(slots.contributions('course.detail.sections'), isEmpty);
  });

  test('a colliding plugin is left out whole and the first one stays', () {
    final store = _Store();
    final pages = PluginNavigationRegistry();
    final slots = UiSlotRegistry();
    final binder = PluginContributionBinder(
      store: store,
      pages: pages,
      slots: slots,
    );
    addTearDown(binder.dispose);

    store.set([
      _cards(id: 'first', route: 'review.deck'),
      _cards(id: 'second', route: 'review.deck'),
      // A target outside the host catalog is never rendered.
      _cards(id: 'third', slotTarget: 'player.controls.trailing'),
    ]);
    expect(pages.pages.map((p) => p.pluginId), ['first', 'third']);
    expect(binder.skipped, {'second': 'review.deck'});
    expect(
      slots.contributions('course.detail.sections').map((s) => s.pluginId),
      ['first'],
    );
  });

  testWidgets('a page reads its state, applies a patch and shows a refusal', (
    tester,
  ) async {
    final daemon = _Daemon({
      'next': {
        'ok': true,
        'result': {
          'statePatch': {
            'viewId': 'deck',
            'revision': 3,
            'patch': {'front': 'Q3'},
            'state': {'front': 'Q3'},
          },
        },
      },
      'save': {
        'ok': false,
        'error': {'code': 'approval_unavailable', 'message': '写操作需要审批模式'},
      },
    });
    final store = _Store()..installed = [_cards()];
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<PluginStore>.value(value: store),
          ChangeNotifierProvider.value(value: PluginNavigationRegistry()),
        ],
        child: MaterialApp(
          home: Scaffold(
            body: PluginDeclarativeView(
              pluginId: 'cards',
              viewId: 'deck',
              document: (_cards()['contributes']['pages'][0]['document'] as Map)
                  .cast<String, dynamic>(),
              call: daemon.call,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(daemon.reads, 1);
    expect(find.text('Q2'), findsOneWidget);

    await tester.tap(find.text('下一张'));
    await tester.pumpAndSettle();
    expect(daemon.sent.single['contextRevision'], 2);
    expect(find.text('Q3'), findsOneWidget);

    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(daemon.sent.last['contextRevision'], 3);
    expect(find.text('无法完成「保存」：写操作需要审批模式'), findsOneWidget);
  });

  testWidgets('a slot reads its host context and opens its plugin page; the '
      'page leaves when the plugin is disabled', (tester) async {
    final daemon = _Daemon({
      'open': {
        'ok': true,
        'result': {
          'navigation': {'route': 'deck'},
        },
      },
    });
    final store = _Store();
    final pages = PluginNavigationRegistry();
    final slots = UiSlotRegistry();
    final binder = PluginContributionBinder(
      store: store,
      pages: pages,
      slots: slots,
    );
    addTearDown(binder.dispose);
    store.set([_cards()]);
    final builders = PluginPageBuilderRegistry({
      'declarative': (context, page) => Text('page:${page.title}'),
    });
    final slot = slots.contributions('course.detail.sections').single;
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<PluginStore>.value(value: store),
          ChangeNotifierProvider.value(value: pages),
          Provider.value(value: builders),
        ],
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) {
                final view = slot.builder(context, {'course_id': 'c-42'});
                // The test injects the daemon instead of the app's API.
                final v = view as PluginDeclarativeView;
                return PluginDeclarativeView(
                  pluginId: v.pluginId,
                  viewId: v.viewId,
                  document: v.document,
                  hostContext: v.hostContext,
                  call: daemon.call,
                );
              },
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('c-42'), findsOneWidget);

    await tester.tap(find.text('开始复习'));
    await tester.pumpAndSettle();
    expect(find.text('page:复习卡'), findsOneWidget);

    store.set([_cards(state: 'disabled')]);
    await tester.pumpAndSettle();
    expect(find.text('page:复习卡'), findsNothing);
    expect(find.text('c-42'), findsOneWidget);
  });

  testWidgets('a write asks first, and only an approval applies it', (
    tester,
  ) async {
    final daemon = _Daemon({
      'save': {
        'ok': true,
        'result': {
          'proposal': {
            'id': 'proposal-1',
            'pluginId': 'cards',
            'actionId': 'save',
            'viewId': 'deck',
            'summary': '保存',
            'status': 'pending',
          },
        },
      },
      'apply': {
        'ok': true,
        'result': {
          'notification': {'text': '已存为笔记'},
        },
      },
      'reject': {
        'ok': true,
        'result': {
          'proposal': {'id': 'proposal-1', 'status': 'rejected'},
        },
      },
    });
    final store = _Store()..installed = [_cards()];
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<PluginStore>.value(value: store),
          ChangeNotifierProvider.value(value: PluginNavigationRegistry()),
        ],
        child: MaterialApp(
          home: Scaffold(
            body: PluginDeclarativeView(
              pluginId: 'cards',
              viewId: 'deck',
              document: (_cards()['contributes']['pages'][0]['document'] as Map)
                  .cast<String, dynamic>(),
              call: daemon.call,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(find.text('允许「保存」吗？'), findsOneWidget);
    await tester.tap(find.text('放弃'));
    await tester.pumpAndSettle();
    expect(daemon.settled, ['/api/plugin-actions/proposals/proposal-1/reject']);

    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('批准写入'));
    await tester.pumpAndSettle();
    expect(
      daemon.settled.last,
      '/api/plugin-actions/proposals/proposal-1/apply',
    );
    expect(find.text('已存为笔记'), findsOneWidget);
  });

  testWidgets('a canvas tool binds to its slot and hands its view patch to '
      'the canvas around it', (tester) async {
    final daemon = _Daemon({
      'open': {
        'ok': true,
        'result': {
          'viewPatch': {
            'viewId': 'course-review',
            'patch': {
              'canvas': {'revision': 4},
            },
          },
        },
      },
    });
    final store = _Store();
    final slots = UiSlotRegistry();
    final binder = PluginContributionBinder(
      store: store,
      pages: PluginNavigationRegistry(),
      slots: slots,
    );
    addTearDown(binder.dispose);
    store.set([_cards(slotTarget: 'canvas.toolbar.tools')]);
    final slot = slots.contributions('canvas.toolbar.tools').single;
    final patches = <String>[];
    await tester.pumpWidget(
      MultiProvider(
        providers: [ChangeNotifierProvider<PluginStore>.value(value: store)],
        child: MaterialApp(
          home: Scaffold(
            body: PluginViewPatchScope(
              onPatch: (pluginId, patch) async =>
                  patches.add('$pluginId ${patch.patch['canvas']}'),
              child: Builder(
                builder: (context) {
                  final v = slot.builder(context, {
                    'course_id': 'c-1',
                  }) as PluginDeclarativeView;
                  return PluginDeclarativeView(
                    pluginId: v.pluginId,
                    viewId: v.viewId,
                    document: v.document,
                    hostContext: v.hostContext,
                    call: daemon.call,
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('开始复习'));
    await tester.pumpAndSettle();
    expect(patches, ['cards {revision: 4}']);
  });
}
