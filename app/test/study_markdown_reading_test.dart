import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_document.dart';
import '../lib/widgets/coursive_markdown.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show close, sheet;
import 'study_pdf_column_test.dart' as pdf;

const line = 'process and process';
StudyCard imported([String source = 'assets/source.MARKDOWN']) => StudyCard(
  id: 'md',
  type: 'text',
  layer: 'main',
  source: source,
  title: '复习资料',
  rect: const Rect.fromLTWH(240, 200, 530, 400),
  text:
      '# CPU\n\n**process** and process\n\n${List.filled(18, '阅读下一段').join('\n\n')}',
);

Future<void> selectWord(WidgetTester t, int from, int to) async {
  final paragraph = t.renderObject<RenderParagraph>(
    find
        .byWidgetPredicate((w) => w is RichText && w.text.toPlainText() == line)
        .first,
  );
  final rect = paragraph.getBoxesForSelection(
    TextSelection(baseOffset: from, extentOffset: to),
  );
  final start = paragraph.localToGlobal(
    rect.first.toRect().centerLeft + const Offset(.2, 0),
  );
  final end = paragraph.localToGlobal(
    rect.last.toRect().centerRight - const Offset(.2, 0),
  );
  final pointer = await t.startGesture(start, kind: ui.PointerDeviceKind.mouse);
  await pointer.moveTo(Offset.lerp(start, end, .5)!);
  await t.pump();
  await pointer.moveTo(end);
  await pointer.up();
  await t.pumpAndSettle();
}

class _AsideApi extends fixture.FixtureApi {
  _AsideApi(super.doc);
  @override
  Future<dynamic> canvasRun(String id) async => {
    'state': 'done',
    'answer': '旁问回答：${request!['question']}',
    'context_key': 'test',
  };
}

Finder annotationInput() => find.descendant(
  of: find.byType(TextFormField),
  matching: find.byType(TextField),
);
void main() {
  test('recognizes daemon asset suffix even after a title change', () {
    expect(imported('assets/source.md').readerCard, isTrue);
    expect(StudyCard.fromJson(imported().json()).markdownReader, isTrue);
    expect(imported('assets/source.txt').markdownReader, isFalse);
  });

  testWidgets(
    'the selected Markdown corner reflows text and independently grows the viewport',
    (t) async {
      final card = imported()..text = List.filled(70, 'lecture').join(' ');
      final api = fixture.FixtureApi((sheet()..cards.add(card)).json());
      final key = await fixture.open(t, api);
      final current = key.currentState!.document!.cards.single;
      final input = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      await t.tapAt(input + current.rect.topLeft + const Offset(90, 18));
      await t.pumpAndSettle();
      final grip = find.byKey(const ValueKey('study-resize-handle-2'));
      RenderParagraph paragraph() => t.renderObject<RenderParagraph>(
        find
            .byWidgetPredicate(
              (w) =>
                  w is RichText &&
                  w.text.toPlainText().contains('lecture lecture'),
            )
            .first,
      );
      final previousHeight = paragraph().size.height, original = current.rect;
      await t.drag(grip, const Offset(160, 0));
      await t.pumpAndSettle();
      expect(current.rect.width, closeTo(original.width + 160, .01));
      expect(current.rect.height, original.height);
      expect(paragraph().size.height, lessThan(previousHeight));
      await t.drag(grip, const Offset(0, 80));
      await t.pumpAndSettle();
      expect(current.rect.height, closeTo(original.height + 80, .01));
      expect(current.rect.width, closeTo(original.width + 160, .01));
      expect(current.contentScale, 1);
      await key.currentState!.flush();
      expect(StudyDocument.fromJson(api.doc).cards.single.rect, current.rect);
      await close(t);
    },
  );

  testWidgets('a different annotation on each occurrence survives reopening', (
    t,
  ) async {
    final api = _AsideApi((sheet()..cards.add(imported())).json());
    final key = await fixture.open(t, api);
    final d = key.currentState!.document!;
    expect(find.byType(CoursiveMarkdown), findsOneWidget);
    Future<void> annotate(int from, String value) async {
      await selectWord(t, from, from + 7);
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.enterText(annotationInput(), value);
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
    }

    await annotate(0, '进程');
    await annotate(12, '处理');
    final notes = d.cards.where((c) => c.generation == 'markdown-annotation');
    expect(notes.map((n) => n.text), ['进程', '处理']);
    expect(notes.map((n) => n.sources.single.id).toSet(), hasLength(2));
    expect(notes.every((n) => n.sources.single.quote == 'process'), isTrue);
    final raw = d.cards.first.text;
    expect(notes.map((n) => n.sources.single.start), [
      raw.indexOf('process'),
      raw.lastIndexOf('process'),
    ]);
    expect(raw, imported().text);
    await selectWord(t, 12, 19);
    await t.tap(find.text('编辑 Markdown'));
    await t.pumpAndSettle();
    final editor = find.descendant(
      of: find.byType(AlertDialog),
      matching: find.byType(TextField),
    );
    final control = t.widget<TextField>(editor).controller!;
    expect(control.selection.textInside(control.text), 'process');
    await t.enterText(editor, '# 新标题\n\n$raw');
    await t.tap(find.text('保存').last);
    await t.pumpAndSettle();
    expect(d.cards.first.text, startsWith('# 新标题'));
    expect(
      notes.last.sources.single.start,
      d.cards.first.text.lastIndexOf('process'),
    );
    expect(await key.currentState!.flush(), isTrue);
    await close(t);
    final reopened = await fixture.open(t, api);
    await selectWord(t, 12, 19);
    await t.tap(find.text('编辑注释 · 处理'));
    await t.pumpAndSettle();
    final field = t.widget<TextField>(annotationInput());
    expect(field.controller!.text, '处理');
    await t.tap(find.byTooltip('保存'));
    await t.pumpAndSettle();
    await selectWord(t, 0, 7);
    await t.tap(find.text('旁问'));
    await t.pumpAndSettle();
    expect(
      t.widget<TextField>(find.byType(TextField).last).controller!.text,
      contains('「process」'),
    );
    expect(
      api.request,
      isNull,
      reason: 'opening an aside must not send a draft',
    );
    final aside = reopened.currentState!.document!.cards.last;
    expect(aside.passageAside, isTrue);
    expect(
      aside.sources.single.start,
      raw.indexOf('process') + '# 新标题\n\n'.length,
    );
    final frame = find.byKey(ValueKey('study-card/${aside.id}'));
    expect(frame, findsOneWidget);
    expect(
      find.byKey(const ValueKey('study-ai-close')).hitTestable(),
      findsNothing,
    );
    final input = find.descendant(of: frame, matching: find.byType(TextField));
    await t.enterText(input, '进程是什么？');
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('发送')));
    await t.pumpAndSettle();
    expect(api.request?['conversation_id'], aside.conversationId);
    expect(
      api.conversations[aside.conversationId]?['turns'].single['answer'],
      '旁问回答：进程是什么？',
    );
    await t.enterText(input, '我的旁问草稿');
    await t.pump(const Duration(seconds: 1));
    final before = aside.rect;
    await t.dragFrom(
      t.getTopLeft(frame) + const Offset(90, 18),
      const Offset(36, 42),
    );
    await t.pumpAndSettle();
    expect(aside.rect, before.shift(const Offset(36, 42)));
    expect(await reopened.currentState!.flush(), isTrue);
    await close(t);
    final again = await fixture.open(t, api);
    final restored = again.currentState!.document!.cards.last;
    expect(restored.rect, aside.rect);
    expect(restored.conversationId, aside.conversationId);
    expect(t.widget<TextField>(input).controller!.text, '我的旁问草稿');
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('返回原文')));
    await t.pumpAndSettle();
    await selectWord(t, 0, 7);
    await t.tap(find.text('打开旁问 1'));
    await t.pumpAndSettle();
    expect(
      again.currentState!.document!.cards.where((c) => c.passageAside),
      hasLength(1),
    );
    await close(t);
  });

  testWidgets(
    'returning from a note reveals and highlights its specific Markdown occurrence',
    (t) async {
      final source = imported()
        ..text = [line, ...List.filled(26, '阅读下一段'), line].join('\n\n');
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(source)).json()),
      );
      final card = find.byKey(const ValueKey('study-card/md'));
      final scrollable = find
          .descendant(of: card, matching: find.byType(Scrollable))
          .first;
      ScrollPosition position() =>
          t.state<ScrollableState>(scrollable).position;
      final passages = find.byWidgetPredicate(
        (widget) => widget is RichText && widget.text.toPlainText() == line,
      );
      position().jumpTo(position().maxScrollExtent);
      await t.pump();
      final paragraph = t.renderObject<RenderParagraph>(passages.last);
      final word = paragraph
          .getBoxesForSelection(
            const TextSelection(baseOffset: 12, extentOffset: 19),
          )
          .single
          .toRect();
      final start = paragraph.localToGlobal(
        word.centerLeft + const Offset(.2, 0),
      );
      final end = paragraph.localToGlobal(
        word.centerRight - const Offset(.2, 0),
      );
      final pointer = await t.startGesture(
        start,
        kind: ui.PointerDeviceKind.mouse,
      );
      await pointer.moveTo(Offset.lerp(start, end, .5)!);
      await t.pump();
      await pointer.moveTo(end);
      await pointer.up();
      await t.pumpAndSettle();
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      final anchor = key.currentState!.document!.cards.last.sources.single;
      expect(anchor.start, source.text.lastIndexOf('process'));
      position().jumpTo(0);
      await t.pump();

      await t.tap(find.byTooltip('返回原文'));
      await t.pump();
      await t.pump(const Duration(milliseconds: 350));
      expect(position().pixels, greaterThan(0));
      final paint = find
          .descendant(
            of: find.byKey(const ValueKey('markdown/md/-1')),
            matching: find.byWidgetPredicate(
              (widget) =>
                  widget is CustomPaint && widget.foregroundPainter != null,
            ),
          )
          .first;
      final layer = t.renderObject<RenderBox>(paint);
      final target = t.renderObject<RenderParagraph>(passages.last);
      final targetRect = MatrixUtils.transformRect(
        target.getTransformTo(layer),
        word,
      );
      expect(
        paint,
        paints..rrect(
          rrect: RRect.fromRectAndRadius(
            targetRect.inflate(2),
            const Radius.circular(4),
          ),
        ),
      );
      await t.pumpAndSettle();
      await close(t);
    },
  );

  testWidgets(
    'touch long press opens passage actions without moving the canvas',
    (t) async {
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(imported())).json()),
      );
      final paragraph = t.renderObject<RenderParagraph>(
        find
            .byWidgetPredicate(
              (w) => w is RichText && w.text.toPlainText() == line,
            )
            .first,
      );
      final word = paragraph.getBoxesForSelection(
        const TextSelection(baseOffset: 0, extentOffset: 7),
      );
      await t.longPressAt(paragraph.localToGlobal(word.first.toRect().center));
      await t.pumpAndSettle();
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(annotationInput(), findsOneWidget);
      expect(
        key.currentState!.document!.cards.last.sources.single.quote,
        'process',
      );
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      await close(t);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.android),
  );

  testWidgets(
    'a cross-format quotation stays readable without inventing a source offset',
    (t) async {
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(imported())).json()),
      );
      await selectWord(t, 0, 11);
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.enterText(annotationInput(), '跨格式的摘句');
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      final a = key.currentState!.document!.cards.last.sources.single;
      expect(a.quote, 'process and');
      expect(a.start, 0);
      expect(a.end, 0);
      await selectWord(t, 0, 11);
      expect(find.text('编辑注释 · 跨格式的摘句'), findsOneWidget);
      await close(t);
    },
  );

  for (final doc in ['markdown', 'pdf']) {
    testWidgets(
      'hand pans across $doc and arrow returns ownership to the reader',
      (t) async {
        final card = doc == 'pdf' ? pdf.pdf() : imported();
        final key = await fixture.open(
          t,
          pdf.PDFApi((sheet()..cards.add(card)).json()),
        );
        final d = key.currentState!.document!;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final original = d.cards.single.rect;
        await t.tap(find.byKey(const ValueKey('study-tool-pan')));
        await t.pumpAndSettle();
        await t.dragFrom(origin + original.center, const Offset(65, 45));
        await t.pumpAndSettle();
        expect(d.camera, const Offset(65, 45));
        expect(d.cards.single.rect, original);
        await t.tap(find.byKey(const ValueKey('study-tool-select')));
        await t.pumpAndSettle();
        final where = origin + original.center + d.camera;
        await t.dragFrom(where, const Offset(0, -55));
        await t.pumpAndSettle();
        expect(d.camera, const Offset(65, 45));
        if (doc == 'markdown') {
          final scroller = find
              .descendant(
                of: find.byKey(const PageStorageKey('study-reader/md')),
                matching: find.byType(Scrollable),
              )
              .first;
          expect(
            t.state<ScrollableState>(scroller).position.pixels,
            greaterThan(0),
          );
        } else {
          await t.tap(find.byTooltip('下一页'));
          await t.pumpAndSettle();
          expect(d.cards.single.page, greaterThan(card.page));
        }
        expect(d.strokes, isEmpty);
        await close(t);
      },
    );
  }
}
