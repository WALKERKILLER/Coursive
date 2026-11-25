import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import '../lib/features/settings/models_page.dart';
import '../lib/features/assistant/assistant_controller.dart';
import '../lib/core/app_state.dart';

import 'study_canvas_ui_test.dart' as canvas;

import '../lib/features/study_canvas/study_document.dart';

import 'assistant_preferences_test.dart' show FakeApi, FakeApp;

void main() {
  testWidgets(
    'canvas sidebar offers the same usable model picker without overflow',
    (t) async {
      final api = FakeApi(), owner = FakeApp(FakeApi());
      final a = AssistantController(FakeApp(api), ScreenContext(), PlayerHub());
      a.models = {
        'providers': [],
        'selected': {'provider': 'demo', 'model': 'demo-model'},
      };
      await canvas.open(
        t,
        canvas.FixtureApi(StudyDocument().json()),
        wrap: (child) => ChangeNotifierProvider.value(value: a, child: child),
      );
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('assistant-model-picker')),
        findsOneWidget,
      );
      expect(t.takeException(), isNull);
      await t.tap(find.byKey(const ValueKey('assistant-model-picker')));
      await t.pump();
      api.read!.complete(a.models);
      await t.pumpAndSettle();
      expect(find.byType(SearchBar), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      a.dispose();
      owner.dispose();
    },
  );

  testWidgets(
    'finger scroll reaches every model and selection waits for persistence',
    (t) async {
      final api = FakeApi(), app = FakeApp(FakeApi());
      final owner = FakeApp(api);
      final a = AssistantController(owner, ScreenContext(), PlayerHub());
      a.models = {
        'providers': [
          {
            'id': 'demo',
            'name': 'Demo',
            'configured': true,
            'models': [
              for (var i = 0; i < 52; i++)
                {'id': 'model-$i', 'name': 'Model $i', 'available': true},
            ],
          },
        ],
        'selected': {'provider': 'demo', 'model': 'model-0'},
      };
      await t.pumpWidget(
        ChangeNotifierProvider.value(
          value: a,
          child: MaterialApp(
            home: Builder(
              builder: (c) => Scaffold(
                body: TextButton(
                  onPressed: () => showModelPicker(c),
                  child: const Text('choose'),
                ),
              ),
            ),
          ),
        ),
      );
      await t.tap(find.text('choose'));
      await t.pumpAndSettle();
      expect(
        t.testTextInput.isVisible,
        isFalse,
        reason: 'search does not obscure the initial list',
      );
      final list = find.byKey(const ValueKey('model-picker-list'));
      await t.dragUntilVisible(
        find.text('Model 51'),
        list,
        const Offset(0, -450),
      );
      await t.tap(find.text('Model 51'));
      await t.pump();
      expect(
        find.byType(SearchBar),
        findsOneWidget,
        reason: 'do not dismiss before the write succeeds',
      );
      api.writes.single.complete({
        'selected': {'provider': 'demo', 'model': 'model-51'},
      });
      await t.pumpAndSettle();
      expect(find.byType(SearchBar), findsNothing);
      expect(a.selected?['model'], 'model-51');
      await t.pumpWidget(const SizedBox());
      a.dispose();
      owner.dispose();
      app.dispose();
    },
  );
}
