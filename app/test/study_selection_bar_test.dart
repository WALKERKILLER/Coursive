// The selection bar and the in-place composer (spec 0027, selection and asking).
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import 'study_canvas_ui_test.dart' as fixture;

Future<void> lasso(WidgetTester t, Rect rect, {bool tool = true}) async {
  if (tool) {
    await t.tap(find.byKey(const ValueKey('study-tool-region')));
    await t.pumpAndSettle();
  }
  final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
  final g = await t.startGesture(
    origin + rect.topLeft,
    kind: PointerDeviceKind.stylus,
  );
  for (final p in [
    rect.topRight,
    rect.bottomRight,
    rect.bottomLeft,
    rect.topLeft,
  ]) {
    await g.moveTo(origin + p);
    await t.pump();
  }
  await g.up();
  await t.pumpAndSettle();
}

KeyDownEvent key(LogicalKeyboardKey k, PhysicalKeyboardKey p) =>
    KeyDownEvent(physicalKey: p, logicalKey: k, timeStamp: Duration.zero);

StudyDocument page() => StudyDocument()
  ..camera = Offset.zero
  ..zoom = 1
  ..cards.add(
    StudyCard(
      id: 'page',
      type: 'pdfdoc',
      layer: 'main',
      rect: const Rect.fromLTWH(200, 200, 400, 420),
      title: '讲义',
      pageAspect: .75,
      page: 2,
      pageCount: 5,
      text: 'Cache misses cost time.',
      textBoxes: [
        {'start': 0, 'end': 23, 'x': .1, 'y': .2, 'w': .5, 'h': .05},
      ],
    ),
  );

void main() {
  testWidgets(
    'the bar sits off the top-right corner and flips below near the top',
    (t) async {
      await fixture.open(t, fixture.FixtureApi(page().json()));
      await lasso(t, const Rect.fromLTRB(280, 400, 560, 560));
      final bar = find.byKey(const ValueKey('study-selection-bar'));
      expect(bar, findsOneWidget);
      expect(find.text('问 AI'), findsOneWidget);
      expect(find.text('摘录'), findsOneWidget);
      expect(find.text('关联'), findsOneWidget);
      expect(
        find.text('复制'),
        findsNothing,
        reason: 'editing lives in the menu',
      );
      var r = t.getRect(bar);
      expect(r.bottom, closeTo(400 - 8, .5));
      expect(r.right, closeTo(560, .5));
      expect(r.height, greaterThanOrEqualTo(44));

      await lasso(t, const Rect.fromLTRB(230, 60, 560, 300), tool: false);
      r = t.getRect(bar);
      expect(
        r.top,
        closeTo(300 + 8, .5),
        reason: 'the top bars cover the space above',
      );
      expect(r.left, greaterThanOrEqualTo(12));
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('Enter opens the composer, Esc steps back and keeps the draft', (
    t,
  ) async {
    final canvas = await fixture.open(t, fixture.FixtureApi(page().json()));
    await lasso(t, const Rect.fromLTRB(280, 290, 440, 340));
    final s = canvas.currentState!;
    expect(
      s.handleKey(key(LogicalKeyboardKey.enter, PhysicalKeyboardKey.enter)),
      isTrue,
    );
    await t.pumpAndSettle();
    final composer = find.byKey(const ValueKey('study-quick-composer'));
    expect(composer, findsOneWidget);
    expect(t.getSize(composer).width, lessThanOrEqualTo(420));
    // A region over real words reads text; 「看图」 stays off.
    expect(find.byIcon(Icons.short_text_rounded), findsOneWidget);
    expect(
      find.byKey(const ValueKey('study-quick-needs-vision')),
      findsNothing,
    );
    await t.enterText(
      find.byKey(const ValueKey('study-quick-question')),
      '为什么慢',
    );
    await t.sendKeyEvent(LogicalKeyboardKey.escape);
    await t.pumpAndSettle();
    expect(composer, findsNothing);
    expect(find.byKey(const ValueKey('study-selection-bar')), findsOneWidget);
    expect(
      s.handleKey(key(LogicalKeyboardKey.enter, PhysicalKeyboardKey.enter)),
      isTrue,
    );
    await t.pumpAndSettle();
    expect(
      t
          .widget<TextField>(find.byKey(const ValueKey('study-quick-question')))
          .controller!
          .text,
      '为什么慢',
    );
    FocusManager.instance.primaryFocus?.unfocus();
    await t.pump();
    expect(
      s.handleKey(key(LogicalKeyboardKey.escape, PhysicalKeyboardKey.escape)),
      isTrue,
    );
    expect(
      s.handleKey(key(LogicalKeyboardKey.escape, PhysicalKeyboardKey.escape)),
      isTrue,
    );
    await t.pumpAndSettle();
    expect(find.byKey(const ValueKey('study-selection-bar')), findsNothing);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });

  testWidgets(
    'a blank page region says it needs 「看图」, and a chip can be removed',
    (t) async {
      await fixture.open(t, fixture.FixtureApi(page().json()));
      await lasso(t, const Rect.fromLTRB(280, 480, 440, 560));
      await t.tap(find.text('问 AI'));
      await t.pumpAndSettle();
      expect(find.byIcon(Icons.image_search_rounded), findsOneWidget);
      expect(
        find.byKey(const ValueKey('study-quick-needs-vision')),
        findsOneWidget,
      );
      expect(find.text('第 2 页 · 局部'), findsOneWidget);
      await t.tap(find.byTooltip('移除此资料'));
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('study-bound-page')), findsNothing);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
