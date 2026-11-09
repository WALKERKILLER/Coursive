import 'package:coursive/core/api.dart';
import 'package:coursive/features/plugins/plugin_actions.dart';
import 'package:flutter_test/flutter_test.dart';

/// A daemon stand-in that records the requests it received and answers from
/// [answer].
class _FakeApi {
  _FakeApi(this.answer);

  final Future<dynamic> Function(Map<String, dynamic> body) answer;
  final methods = <String>[];
  final paths = <String>[];
  final bodies = <Map<String, dynamic>>[];

  Future<dynamic> call(String method, String path, [Object? body]) {
    methods.add(method);
    paths.add(path);
    final sent = (body as Map).cast<String, dynamic>();
    bodies.add(sent);
    return answer(sent);
  }
}

Map<String, dynamic> _ok(Map<String, dynamic> result, {String id = 'req-1'}) =>
    {'ok': true, 'requestId': id, 'traceId': 'trace-1', 'result': result};

void main() {
  test('the request carries the action contract', () {
    final request = PluginActionRequest(
      pluginId: 'review',
      actionId: 'refresh',
      viewId: 'review.main',
      instanceId: 'page-1',
      args: const {'courseId': 'c1'},
      contextRevision: 3,
      requestId: 'req-1',
      deadlineMs: 2500,
      traceId: 'trace-1',
    );
    expect(request.toJson(), {
      'pluginId': 'review',
      'actionId': 'refresh',
      'viewId': 'review.main',
      'instanceId': 'page-1',
      'args': {'courseId': 'c1'},
      'contextRevision': 3,
      'requestId': 'req-1',
      'deadlineMs': 2500,
      'traceId': 'trace-1',
    });
  });

  test('a request id is generated when the caller names none', () {
    final first = PluginActionRequest(pluginId: 'a', actionId: 'b');
    final second = PluginActionRequest(pluginId: 'a', actionId: 'b');
    expect(first.requestId, isNotEmpty);
    expect(first.requestId, isNot(second.requestId));
    expect(first.deadlineMs, PluginActionRequest.defaultDeadlineMs);
  });

  test('an action is sent to the bridge with the daemon token', () async {
    final api = _FakeApi((_) async => _ok(const {}));
    final client = PluginActionClient(call: api.call);
    await client.invoke(
      PluginActionRequest(
        pluginId: 'review',
        actionId: 'refresh',
        viewId: 'review.main',
        requestId: 'req-1',
      ),
    );
    expect(api.methods, ['POST']);
    expect(api.paths, ['/api/plugin-actions']);
    expect(api.bodies.single['requestId'], 'req-1');
  });

  test('a state patch is applied and its revision remembered', () async {
    final api = _FakeApi(
      (_) async => _ok({
        'statePatch': {
          'viewId': 'review.main',
          'revision': 4,
          'idempotencyKey': 'review:req-1',
          'patch': {'page': 2},
          'state': {'page': 2, 'courseId': 'c1'},
        },
      }),
    );
    final applied = <PluginStatePatch>[];
    final client = PluginActionClient(
      call: api.call,
      onStatePatch: applied.add,
    );
    final outcome = await client.invoke(
      PluginActionRequest(
        pluginId: 'review',
        actionId: 'refresh',
        viewId: 'review.main',
        requestId: 'req-1',
      ),
    );
    expect(outcome.ok, isTrue);
    expect(applied.single.revision, 4);
    expect(applied.single.idempotencyKey, 'review:req-1');
    expect(applied.single.state['page'], 2);
    expect(client.revisionOf('review', 'review.main'), 4);
  });

  test('navigation, a view patch, a notification and a proposal each reach '
      'their callback', () async {
    final seen = <String>[];
    Future<dynamic> answer(Map<String, dynamic> body) async =>
        _ok(switch (body['actionId']) {
          'open' => {
            'navigation': {
              'route': 'review.home',
              'title': '复习',
              'replace': true,
            },
          },
          'repaint' => {
            'viewPatch': {
              'viewId': 'review.main',
              'revision': 5,
              'patch': {'title': '新'},
            },
          },
          'sync' => {
            'notification': {'text': '已同步', 'level': 'warning'},
          },
          _ => {
            'proposal': {
              'id': 'proposal-1',
              'pluginId': 'review',
              'actionId': 'save',
              'summary': '保存复习卡',
              'approvalMode': 'ask',
              'status': 'pending',
              'preview': {'cardId': 'c-1'},
            },
          },
        });
    final api = _FakeApi(answer);
    final client = PluginActionClient(
      call: api.call,
      onNavigation: (n) => seen.add('navigation ${n.route} ${n.replace}'),
      onViewPatch: (p) => seen.add('viewPatch ${p.viewId} ${p.revision}'),
      onNotification: (n) => seen.add('notification ${n.text} ${n.level}'),
      onProposal: (p) => seen.add('proposal ${p.id} ${p.status}'),
    );
    for (final action in ['open', 'repaint', 'sync', 'save']) {
      await client.invoke(
        PluginActionRequest(
          pluginId: 'review',
          actionId: action,
          viewId: 'review.main',
          requestId: 'req-$action',
        ),
      );
    }
    expect(seen, [
      'navigation review.home true',
      'viewPatch review.main 5',
      'notification 已同步 warning',
      'proposal proposal-1 pending',
    ]);
    expect(client.revisionOf('review', 'review.main'), 5);
  });

  test('a typed refusal is surfaced without dispatching', () async {
    final api = _FakeApi(
      (_) async => {
        'ok': false,
        'requestId': 'req-1',
        'error': {
          'code': 'revision_conflict',
          'message': '视图 review.main 已更新到版本 4，请重新读取后再试',
          'details': {'viewId': 'review.main', 'current': 4},
          'recoverable': true,
        },
      },
    );
    var dispatched = 0;
    final client = PluginActionClient(
      call: api.call,
      onStatePatch: (_) => dispatched++,
      onNavigation: (_) => dispatched++,
      onNotification: (_) => dispatched++,
      onProposal: (_) => dispatched++,
    );
    final outcome = await client.invoke(
      PluginActionRequest(
        pluginId: 'review',
        actionId: 'refresh',
        viewId: 'review.main',
        requestId: 'req-1',
      ),
    );
    expect(outcome.ok, isFalse);
    expect(outcome.failure?.code, 'revision_conflict');
    expect(outcome.failure?.recoverable, isTrue);
    expect(outcome.failure?.details['current'], 4);
    expect(outcome.failure.toString(), contains('版本 4'));
    expect(dispatched, 0);
  });

  test('one request id is sent and dispatched once', () async {
    var calls = 0;
    final api = _FakeApi((_) async {
      calls++;
      return _ok({
        'notification': {'text': '已同步'},
      });
    });
    var dispatched = 0;
    final client = PluginActionClient(
      call: api.call,
      onNotification: (_) => dispatched++,
    );
    final request = PluginActionRequest(
      pluginId: 'review',
      actionId: 'sync',
      viewId: 'review.main',
      requestId: 'req-1',
    );
    final both = await Future.wait([
      client.invoke(request),
      client.invoke(request),
    ]);
    expect(calls, 1);
    expect(dispatched, 1);
    expect(both[0].requestId, both[1].requestId);

    // A retry after the answer replays it instead of writing again.
    await client.invoke(request);
    expect(calls, 1);
    expect(dispatched, 1);
  });

  test('a timeout is not settled, so the same request id may retry', () async {
    var calls = 0;
    final api = _FakeApi((_) async {
      calls++;
      if (calls == 1) {
        return {
          'ok': false,
          'requestId': 'req-1',
          'error': {
            'code': 'deadline_exceeded',
            'message': '插件动作超时',
            'recoverable': true,
          },
        };
      }
      return _ok({
        'notification': {'text': '已同步'},
      });
    });
    var dispatched = 0;
    final client = PluginActionClient(
      call: api.call,
      onNotification: (_) => dispatched++,
    );
    final request = PluginActionRequest(
      pluginId: 'review',
      actionId: 'sync',
      requestId: 'req-1',
    );
    final timedOut = await client.invoke(request);
    expect(timedOut.failure?.code, 'deadline_exceeded');
    final retried = await client.invoke(request);
    expect(retried.ok, isTrue);
    expect(calls, 2);
    expect(dispatched, 1);
  });

  test('a transport failure is thrown and can be retried', () async {
    var calls = 0;
    final api = _FakeApi((_) async {
      calls++;
      if (calls == 1) throw ApiError(503, 'daemon is starting');
      return _ok({
        'notification': {'text': '已同步'},
      });
    });
    final client = PluginActionClient(call: api.call);
    final request = PluginActionRequest(
      pluginId: 'review',
      actionId: 'sync',
      requestId: 'req-1',
    );
    await expectLater(client.invoke(request), throwsA(isA<ApiError>()));
    final outcome = await client.invoke(request);
    expect(outcome.ok, isTrue);
    expect(calls, 2);
  });
}
