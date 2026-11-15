import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/api.dart';
import 'package:coursive/core/app_state.dart';
import 'package:coursive/core/daemon_service.dart';
import 'package:coursive/features/assistant/assistant_controller.dart';

class FakeApi extends Api {
 FakeApi() : super('http://127.0.0.1', 'synthetic');
 final writes=<Completer<dynamic>>[];
 Completer<dynamic>? read;
 @override Future<dynamic> put(String path,Object body) { final c=Completer<dynamic>(); writes.add(c); return c.future; }
 @override Future<dynamic> get(String path,[Map<String,String>? query]) => (read ??= Completer<dynamic>()).future;
}
class FakeApp extends AppState {
 FakeApp(this.client) : super(DaemonService()) { agent={'state':'ready'}; }
 final FakeApi client;
 @override Api get api => client;
}
Future<void> tick() => Future<void>.delayed(Duration.zero);
void main() {
 TestWidgetsFlutterBinding.ensureInitialized();
 late FakeApi api; late FakeApp app; late AssistantController controller;
 setUp(() { api=FakeApi(); app=FakeApp(api); controller=AssistantController(app,ScreenContext(),PlayerHub()); });
 tearDown(() { controller.dispose(); app.dispose(); });
 test('rapid model choices persist in selection order',() async {
  final a=controller.setPrefs({'model':'first'}); final b=controller.setPrefs({'model':'second'});
  await tick(); expect(api.writes.length,1);
  api.writes[0].complete({'selected':{'provider':'demo','model':'first'}});
  await a; await tick(); expect(api.writes.length,2);
  api.writes[1].complete({'selected':{'provider':'demo','model':'second'}});
  await b; expect(controller.selected?['model'],'second');
 });
 test('a stale model refresh cannot undo a newer selection',() async {
  final read=controller.loadModels(); await tick();
  final write=controller.setPrefs({'model':'new'}); await tick();
  api.writes.single.complete({'selected':{'provider':'demo','model':'new'}}); await write;
  api.read!.complete({'selected':{'provider':'demo','model':'old'}}); await read;
  expect(controller.selected?['model'],'new');
 });
 test('a failed preference save blocks sending until a successful reselection',() async {
  final write=controller.setPrefs({'model':'new'}); final rejected=expectLater(write,throwsStateError);
  await tick(); api.writes.single.completeError(StateError('save failed')); await rejected;
  await controller.send('hello');
  expect(controller.messages.single.error,contains('模型设置保存失败'));
  expect(controller.busy,isFalse);
  final retry=controller.setPrefs({'model':'new'}); await tick();
  api.writes.last.complete({'selected':{'provider':'demo','model':'new'}}); await retry;
  expect(controller.modelsError,isNull);
 });
}
