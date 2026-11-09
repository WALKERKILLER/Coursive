import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/ui_slots.dart';
import 'package:provider/provider.dart';

UiSlotContribution _slot(
  String id, {
  String target = 'course.detail.sections',
  String plugin = 'demo',
  UiSlotMode mode = UiSlotMode.list,
  int order = 100,
  int priority = 0,
  String? scope,
  String? key,
  Widget Function(BuildContext, Map<String, dynamic>)? builder,
}) => UiSlotContribution(
  target: target,
  id: id,
  pluginId: plugin,
  mode: mode,
  order: order,
  priority: priority,
  scope: scope,
  key: key,
  builder: builder ?? (context, data) => Text('$plugin:$id'),
);

void main() {
  test('list ordering is stable and dispose removes only its own entry', () {
    final registry = UiSlotRegistry();
    final disposeB = registry.register(_slot('b', plugin: 'z', order: 20));
    registry.register(_slot('a', plugin: 'a', order: 20));
    registry.register(_slot('first', plugin: 'a', order: 10));
    expect(registry.contributions('course.detail.sections').map((c) => c.id), [
      'first',
      'a',
      'b',
    ]);
    disposeB();
    expect(registry.contributions('course.detail.sections').map((c) => c.id), [
      'first',
      'a',
    ]);
  });

  test('duplicate keyed keys reject the whole replaceAll batch', () {
    final registry = UiSlotRegistry();
    registry.register(_slot('keep'));
    expect(
      () => registry.replaceAll([
        _slot('a', mode: UiSlotMode.keyed, key: 'card'),
        _slot('b', mode: UiSlotMode.keyed, key: 'card'),
      ]),
      throwsArgumentError,
    );
    expect(registry.contributions('course.detail.sections').single.id, 'keep');
  });

  test('scope filters contributions and removePlugin is scoped', () {
    final registry = UiSlotRegistry();
    registry.register(_slot('global'));
    registry.register(_slot('course', plugin: 'other', scope: 'course'));
    expect(
      registry
          .contributions('course.detail.sections', scope: 'course')
          .map((c) => c.id),
      ['global', 'course'],
    );
    registry.removePlugin('other');
    expect(
      registry
          .contributions('course.detail.sections', scope: 'course')
          .map((c) => c.id),
      ['global'],
    );
  });

  testWidgets(
    'list outlet renders every contribution and empty outlet is invisible',
    (tester) async {
      final registry = UiSlotRegistry();
      registry.register(_slot('one', plugin: 'a', order: 1));
      registry.register(_slot('two', plugin: 'b', order: 2));
      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: registry,
          child: const MaterialApp(
            home: Scaffold(
              body: UiSlotOutlet(
                target: 'course.detail.sections',
                context_: {'course_id': 'c'},
              ),
            ),
          ),
        ),
      );
      expect(find.text('a:one'), findsOneWidget);
      expect(find.text('b:two'), findsOneWidget);

      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: UiSlotRegistry(),
          child: const MaterialApp(
            home: Scaffold(
              body: UiSlotOutlet(
                target: 'empty.slot',
                fallback: Text('fallback'),
              ),
            ),
          ),
        ),
      );
      expect(find.text('fallback'), findsOneWidget);
    },
  );

  testWidgets(
    'keyed outlet picks the matching key and chain falls through failures',
    (tester) async {
      final keyed = UiSlotRegistry();
      keyed.register(
        _slot(
          'a',
          mode: UiSlotMode.keyed,
          key: 'card',
          builder: (_, _) => const Text('card-a'),
        ),
      );
      keyed.register(
        _slot(
          'b',
          mode: UiSlotMode.keyed,
          key: 'other',
          builder: (_, _) => const Text('card-b'),
        ),
      );
      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: keyed,
          child: const MaterialApp(
            home: Scaffold(
              body: UiSlotOutlet(
                target: 'course.detail.sections',
                slotKey: 'card',
              ),
            ),
          ),
        ),
      );
      expect(find.text('card-a'), findsOneWidget);
      expect(find.text('card-b'), findsNothing);

      final chain = UiSlotRegistry();
      chain.register(
        _slot(
          'bad',
          mode: UiSlotMode.chain,
          order: 1,
          builder: (_, _) => throw StateError('no'),
        ),
      );
      chain.register(
        _slot(
          'good',
          mode: UiSlotMode.chain,
          order: 2,
          builder: (_, _) => const Text('chain-good'),
        ),
      );
      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: chain,
          child: const MaterialApp(
            home: Scaffold(
              body: UiSlotOutlet(
                target: 'course.detail.sections',
                fallback: Text('fallback'),
              ),
            ),
          ),
        ),
      );
      expect(find.text('chain-good'), findsOneWidget);
    },
  );

  testWidgets('outlet without a registry stays invisible instead of throwing', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: UiSlotOutlet(target: 'course.detail.sections')),
      ),
    );
    expect(tester.takeException(), isNull);
  });
}
