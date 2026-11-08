import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/features/settings/asr_model_mark.dart';
import 'package:coursive/features/settings/asr_settings.dart';
import 'package:provider/provider.dart';

Map<String, dynamic> channel(String id, String name, {String? model}) => {
  'id': id,
  'name': name,
  'note': '$name 说明',
  'local': model != null,
  'model': ?model,
  'enabled': true,
  'state': 'ready',
  'busy': 0,
  'slots': 1,
  'used_secs': 0,
  'quota_secs': model != null ? 0 : 21600,
  'ok': 0,
  'fail': 0,
};

class _Api extends Api {
  _Api(this.channels) : super('http://localhost', 'test');
  final List<Map<String, dynamic>> channels;
  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async => {'channels': channels, 'events': []};
}

class _App extends AppState {
  _App(this._api) : super(DaemonService());
  final Api _api;
  @override
  Api get api => _api;
}

Future<void> open(WidgetTester tester, List<Map<String, dynamic>> channels) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ChangeNotifierProvider<AppState>.value(
      value: _App(_Api(channels)),
      child: MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: AsrSettings(server: {}, put: (_, _) async {}),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('the local channel wears its model maker\'s mark, not a 本', (tester) async {
    await open(tester, [channel('local', '本地 · Qwen3-ASR 0.6B', model: 'qwen3-asr-0.6b'), channel('bcut', '必剪')]);
    final marks = tester.widgetList<AsrMarkImage>(find.byType(AsrMarkImage)).map((m) => m.mark.asset);
    expect(marks, everyElement('assets/providers/qwen-color.svg'));
    expect(marks, hasLength(2), reason: 'the 本地识别 card and the channel badge');
    expect(find.text('本'), findsNothing);
    expect(find.image(const AssetImage('assets/brands/bcut.png')), findsOneWidget);
  });

  testWidgets('an unknown local model falls back to its own initial', (tester) async {
    await open(tester, [channel('local', '本地 · Zeta', model: 'zeta')]);
    expect(find.byType(AsrMarkImage), findsNothing);
    expect(find.text('Z'), findsOneWidget);
    expect(find.text('本'), findsNothing);
  });
}
