import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/core/models.dart';
import 'package:coursive/features/schedule/schedule_store.dart';

class _App extends AppState {
  _App(this.client) : super(DaemonService());
  final Api client;
  @override
  Api get api => client;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides.global = null;
  test(
    'namespaced assignments remain one URL segment for done, hide and undo',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final api = Api('http://127.0.0.1:${server.port}', 'test');
      final app = _App(api);
      final store = ScheduleStore(app);
      addTearDown(() async {
        store.dispose();
        app.dispose();
        await server.close(force: true);
      });
      const id = 'lms:plugin/source:作业 #1%?';
      final original = {'id': id, 'title': 'Assignment', 'source': 'lms'};
      final calls = <String>[];
      server.listen((request) async {
        final parts = request.uri.pathSegments;
        final valid = parts.length == 3 && parts[2] == id;
        request.response.statusCode = valid ? 200 : 404;
        request.response.headers.contentType = ContentType.json;
        final body = await utf8.decoder.bind(request).join();
        if (valid) calls.add(request.method);
        request.response.write(
          jsonEncode(
            valid
                ? {...original, if (body.isNotEmpty) ...jsonDecode(body) as Map}
                : {'error': 'HTTP 404'},
          ),
        );
        await request.response.close();
      });
      final item = ScheduleItem(original);
      store.items = [item];
      await store.patch(item, {'done': true});
      expect(store.byId(id)!.done, isTrue);
      await store.remove(item);
      expect(store.byId(id), isNull);
      await store.restore(item);
      expect(store.byId(id), isNotNull);
      expect(calls, ['PATCH', 'DELETE', 'PATCH']);
    },
  );
}
